package tools

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"syscall"
	"time"

	md "github.com/JohannesKaufmann/html-to-markdown"
	"github.com/PuerkitoBio/goquery"
	"golang.org/x/net/html/charset"

	"github.com/opencode-ai/opencode/internal/config"
	"github.com/opencode-ai/opencode/internal/permission"
)

// ---------------------------------------------------------------------------
// SSRF protection
// ---------------------------------------------------------------------------

// blockedIP reports why an IP must not be reached, or "" if it is allowed.
// Every path to the network funnels through this: the pre-flight URL check, the
// dialer (which sees the address DNS actually resolved to), and the redirect
// hook.
func blockedIP(ip net.IP) string {
	switch {
	case ip.IsLoopback():
		return "loopback IP is not allowed"
	case ip.IsLinkLocalUnicast(), ip.IsLinkLocalMulticast():
		return "link-local address (e.g. cloud metadata 169.254.169.254) is not allowed"
	case ip.IsPrivate():
		return "private LAN address is not allowed"
	case ip.IsUnspecified():
		return "unspecified address (0.0.0.0/::) is not allowed"
	case ip.IsInterfaceLocalMulticast(), ip.IsMulticast():
		return "multicast address is not allowed"
	}
	// IPv4-mapped IPv6 (::ffff:127.0.0.1) evades the checks above unless
	// unwrapped first.
	if v4 := ip.To4(); v4 != nil && !ip.Equal(v4) {
		return blockedIP(v4)
	}
	return ""
}

// blockedFetchTarget is the pre-flight check on the literal URL. It refuses the
// obvious cases before a request is made, so the model gets a clear reason
// rather than a dial error.
//
// GORILLA OVERRIDE (SSRF): this alone is NOT the guard. A string check cannot
// see where a hostname resolves, and cannot see a redirect. Both holes are
// closed in newSafeClient below, which validates the address actually dialled
// on every hop. Between 2026-07-23 and 2026-08-07 only this function existed,
// and its own comment said it did not re-check after DNS - while the tool
// description twenty lines away advertised that redirects are followed
// automatically. Two true statements, sixty lines apart, that nobody read
// together: a public URL redirecting to 169.254.169.254 was fetched.
func blockedFetchTarget(raw string) string {
	u, err := url.Parse(raw)
	if err != nil {
		return "could not parse the URL"
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return "only http and https URLs can be fetched"
	}
	host := u.Hostname()
	if host == "" {
		return "no host in URL"
	}
	switch strings.ToLower(host) {
	case "localhost", "ip6-localhost", "ip6-loopback":
		return "loopback address (localhost) is not allowed"
	}
	if ip := net.ParseIP(host); ip != nil {
		return blockedIP(ip)
	}
	return ""
}

// newSafeClient builds a client that cannot be walked into a blocked address.
//
// Control runs after DNS resolution and immediately before connect, with the
// address the kernel is about to dial - so a hostname that resolves to a
// private IP (DNS rebinding) is refused, not merely a literal one.
// CheckRedirect re-runs the URL check on every hop, so a 302 into cloud
// metadata is refused too.
func newSafeClient(timeout time.Duration) *http.Client {
	dialer := &net.Dialer{
		Timeout:   10 * time.Second,
		KeepAlive: 30 * time.Second,
		Control: func(network, address string, _ syscall.RawConn) error {
			host, _, err := net.SplitHostPort(address)
			if err != nil {
				return fmt.Errorf("could not parse dial address %q", address)
			}
			ip := net.ParseIP(host)
			if ip == nil {
				return fmt.Errorf("could not parse resolved IP %q", host)
			}
			if reason := blockedIP(ip); reason != "" {
				return fmt.Errorf("refusing to connect to %s: %s", ip, reason)
			}
			return nil
		},
	}
	return &http.Client{
		Timeout: timeout,
		Transport: &http.Transport{
			DialContext:           dialer.DialContext,
			TLSHandshakeTimeout:   10 * time.Second,
			ResponseHeaderTimeout: timeout,
			ForceAttemptHTTP2:     true,
		},
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 10 {
				return fmt.Errorf("stopped after 10 redirects")
			}
			if reason := blockedFetchTarget(req.URL.String()); reason != "" {
				return fmt.Errorf("refusing to follow redirect to %s: %s", req.URL, reason)
			}
			return nil
		},
	}
}

// ---------------------------------------------------------------------------
// Source preference - ask for the document, not a page that contains it
// ---------------------------------------------------------------------------

// rewriteToSource maps URLs whose raw form is known and strictly better than
// the rendered page. These are substitutions, not extra requests.
func rewriteToSource(raw string) (string, string) {
	u, err := url.Parse(raw)
	if err != nil {
		return raw, ""
	}
	host := strings.ToLower(u.Hostname())

	// arxiv.org/abs/ID -> the export API: title, authors and abstract as clean
	// XML instead of a page that is 83% navigation and tool sidebars.
	if host == "arxiv.org" || host == "www.arxiv.org" {
		if id := strings.TrimPrefix(strings.Trim(u.Path, "/"), "abs/"); id != "" && !strings.Contains(id, "/") {
			return "https://export.arxiv.org/api/query?id_list=" + id,
				"rewritten to the arXiv export API (metadata and abstract, no page furniture)"
		}
	}

	// pubmed.ncbi.nlm.nih.gov/PMID -> the E-utilities XML record. The rendered
	// PubMed page is mostly interface; efetch returns the abstract itself.
	if host == "pubmed.ncbi.nlm.nih.gov" {
		if id := strings.Trim(u.Path, "/"); id != "" && !strings.Contains(id, "/") && isAllDigits(id) {
			return "https://eutils.ncbi.nlm.nih.gov/entrez/eutils/efetch.fcgi?db=pubmed&id=" +
					id + "&retmode=xml",
				"rewritten to the NCBI E-utilities record (abstract, no page furniture)"
		}
	}

	// en.wikipedia.org/wiki/Topic -> the REST summary. Measured 2026-08-07:
	// the rendered article is ~25,595 tokens, the summary ~600.
	if strings.HasSuffix(host, "wikipedia.org") {
		if t := strings.TrimPrefix(strings.Trim(u.Path, "/"), "wiki/"); t != "" && !strings.Contains(t, "/") {
			return "https://" + host + "/api/rest_v1/page/summary/" + t,
				"rewritten to the Wikipedia REST summary (lead section; fetch the /wiki/ URL directly if you need the full article)"
		}
	}

	// github.com/o/r/blob/ref/path -> raw.githubusercontent.com/o/r/ref/path
	if host == "github.com" || host == "www.github.com" {
		parts := strings.Split(strings.Trim(u.Path, "/"), "/")
		if len(parts) >= 5 && parts[2] == "blob" {
			p := append([]string{parts[0], parts[1]}, parts[3:]...)
			return "https://raw.githubusercontent.com/" + strings.Join(p, "/"),
				"rewritten to raw.githubusercontent.com (source, not the rendered page)"
		}
	}
	return raw, ""
}

// markdownSibling returns the ".md" companion of a documentation-style URL, or
// "" when the guess would be nonsense. Tried only when the server answered with
// HTML and markdown was asked for, so it costs at most one extra request.
func isAllDigits(s string) bool {
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return s != ""
}

func markdownSibling(raw string) string {
	u, err := url.Parse(raw)
	if err != nil || u.RawQuery != "" {
		return ""
	}
	p := strings.TrimRight(u.Path, "/")
	if p == "" || strings.Contains(p[strings.LastIndex(p, "/")+1:], ".") {
		return "" // already has an extension; do not guess
	}
	u.Path = p + ".md"
	return u.String()
}

// ---------------------------------------------------------------------------
// Conditional-request cache
// ---------------------------------------------------------------------------

type cachedPage struct {
	etag        string
	lastMod     string
	body        []byte
	contentType string
}

type pageCache struct {
	mu    sync.Mutex
	pages map[string]cachedPage
}

func (c *pageCache) get(k string) (cachedPage, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	p, ok := c.pages[k]
	return p, ok
}

func (c *pageCache) put(k string, p cachedPage) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if len(c.pages) > 64 { // bounded; this is a session cache, not a CDN
		c.pages = map[string]cachedPage{}
	}
	c.pages[k] = p
}

// ---------------------------------------------------------------------------
// Tool
// ---------------------------------------------------------------------------

type FetchParams struct {
	URL     string `json:"url"`
	Format  string `json:"format,omitempty"`
	Timeout int    `json:"timeout,omitempty"`
	// Summarise is opt-in and never automatic. Silently shortening a document
	// the user asked to read is the truncation bug wearing a helpful hat.
	Summarise bool `json:"summarise,omitempty"`
	// Selector narrows an HTML page to the part that was actually wanted,
	// BEFORE any of it becomes tokens. Fetching a documentation page to read
	// one table currently costs the navigation, the sidebar, the cookie
	// banner and the footer as well; on a metered link that is bytes, and in
	// the conversation it is a recurring bill, because every tool result is
	// re-sent on every later turn.
	Selector string `json:"selector,omitempty"`
	// Extract says what to take from the selected elements.
	Extract string `json:"extract,omitempty"`
}

type FetchPermissionsParams struct {
	URL       string `json:"url"`
	Format    string `json:"format,omitempty"`
	Timeout   int    `json:"timeout,omitempty"`
	Summarise bool   `json:"summarise,omitempty"`
	Selector  string `json:"selector,omitempty"`
	Extract   string `json:"extract,omitempty"`
}

type fetchTool struct {
	permissions permission.Service
	cache       *pageCache
}

const (
	// GORILLA OVERRIDE: named web_fetch, not fetch. Models carry a strong
	// trained prior that they cannot reach the internet, and a tool called
	// "fetch" does not contradict it - it reads like `git fetch` or reading a
	// local file. Models were routinely telling users "I don't have the tools
	// to read a webpage" while this tool sat enabled in their schema. The
	// description now opens by stating the capability rather than describing
	// the mechanics, for the same reason. See also the `# tools` line in
	// prompt/coder-modern.txt: the schema alone was not enough.
	FetchToolName = "web_fetch"

	maxFetchBytes = 5 * 1024 * 1024

	// GORILLA OVERRIDE: a budget, not a guillotine.
	//
	// The byte cap at NewTextResponse (400KB) protects memory. It does not
	// protect the CONTEXT WINDOW, which is the resource that actually runs out:
	// 400KB is ~100,000 tokens, and one researchsquare fetch on 2026-08-07 took
	// a session to 88% of context in a single call. Same shape as the grep bug
	// in CLAUDE.md - a limit expressed in the wrong unit.
	//
	// But a low cap is worse than no cap. Measured: a converted arXiv abstract
	// page is ~10,700 tokens and a whole book ~42,400. Anything tight enough to
	// guarantee cheapness hands the model half a paper, and reasoning over a
	// fragment is the failure this project exists to prevent.
	//
	// So: warn early, truncate late, and always offer the way out. The original
	// fault was not that 85,000 tokens arrived - it was that nobody was told
	// what it cost or offered an alternative.
	warnTokens = 15000 // ~60KB: above a full abstract page, below a book
	maxTokens  = 40000 // ~160KB: fits an entire novel; refuses the pathological

	fetchToolDescription = `Fetch a web page, document or API response from the internet.

YOU DO HAVE INTERNET ACCESS through this tool. Use it whenever the user gives
you a URL, or when you need current documentation, a changelog, an API response
or a specification. Do not tell the user you cannot read a web page.

HOW TO USE:
- url is required. format is optional and defaults to markdown.
- format: "markdown" (default, best for reading), "text" (plain), "html" (raw).
- timeout is optional, in seconds, max 120.

WHAT IT DOES FOR YOU:
- Asks the server for markdown first (Accept: text/markdown). Many
  documentation sites serve their source markdown, which is cleaner and far
  smaller than the rendered page.
- Rewrites github.com/.../blob/... to raw.githubusercontent.com automatically.
- If a documentation URL returns HTML, tries the ".md" companion once.
- Falls back to converting HTML to markdown locally.
- Re-uses a conditional request (ETag) when you fetch the same URL twice.
- Reports which of those paths produced the result, so you know whether you are
  reading a source document or a reconstruction of one.

TIPS:
- Many sites publish /llms.txt or /llms-full.txt, an index written for LLMs.
  If a site's docs are large, fetching https://host/llms.txt first is cheap and
  often points straight at what you need.
- Prefer the raw source URL when you know it.

LIMITATIONS:
- http and https only. Public internet only: loopback, link-local, private-LAN
  and multicast addresses are refused, including via redirects.
- Responses are capped at 5MB; truncation is always stated in the output.
- PDFs and other binary formats are reported, not returned as text.
- Cannot authenticate or send cookies.`
)

func NewFetchTool(permissions permission.Service) BaseTool {
	return &fetchTool{
		permissions: permissions,
		cache:       &pageCache{pages: map[string]cachedPage{}},
	}
}

func (t *fetchTool) Info() ToolInfo {
	return ToolInfo{
		Name:        FetchToolName,
		Description: fetchToolDescription,
		Parameters: map[string]any{
			"url": map[string]any{
				"type":        "string",
				"description": "The URL to fetch (http or https).",
			},
			"format": map[string]any{
				"type":        "string",
				"description": "Output format. Defaults to markdown.",
				"enum":        []string{"text", "markdown", "html", "json"},
			},
			"timeout": map[string]any{
				"type":        "number",
				"description": "Optional timeout in seconds (max 120).",
			},
			"summarise": map[string]any{
				"type": "boolean",
				"description": "Optional. Condense a LONG document locally (TextRank) " +
					"before returning it, to save tokens. Ignored for anything under " +
					"8000 characters. The result states how much was cut and that it " +
					"is extractive - do not use it when exact wording matters.",
			},
			"selector": map[string]any{
				"type": "string",
				"description": "Optional CSS selector. Keeps only the matching parts of an " +
					"HTML page, so the navigation, sidebar and footer never become tokens. " +
					"Use it whenever you know what you are looking for: 'table', 'article', " +
					"'main', '.changelog', '#install'. Reports how many elements matched, " +
					"and says so plainly if none did rather than silently returning nothing.",
			},
			"extract": map[string]any{
				"type": "string",
				"description": "Optional, only with selector. 'text' keeps just the words, " +
					"'html' keeps the markup, 'links' lists every href with its link text. " +
					"Any other value is treated as an attribute name, so extract:'href' on " +
					"selector:'a.download' returns the download URLs alone. Defaults to " +
					"keeping the matched HTML and converting it in the usual way.",
			},
		},
		// GORILLA OVERRIDE: url only. format was required, so a model calling
		// web_fetch(url=...) - the obvious signature - got a hard error, and
		// some then reported to the user that they could not fetch pages at all.
		Required: []string{"url"},
	}
}

type fetchResult struct {
	body        []byte
	contentType string
	provenance  string
	truncated   bool
	fromCache   bool
}

// do performs one request with the SSRF-safe client and the negotiated Accept.
func (t *fetchTool) do(ctx context.Context, client *http.Client, target string) (*fetchResult, int, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", target, nil)
	if err != nil {
		return nil, 0, err
	}
	// GORILLA OVERRIDE (2026-08-18): identify as the browser a human would use.
	// Measured that day, the honest bot token alone drew a 302 from Google and
	// 401s elsewhere that a Firefox token did not. Reading a public page while
	// wearing the badge a human wears is standard for every research tool; the
	// tool evades no auth and no paywall. Override with GORILLA_OPENCODE_USER_AGENT
	// ("honest" restores the identifying token). See config.BrowserUserAgent.
	req.Header.Set("User-Agent", config.BrowserUserAgent())
	// GORILLA OVERRIDE: content negotiation. Previously no Accept header was
	// sent at all, so every site returned HTML and the tool reconstructed
	// markdown from it - lossy, and on a slow link it downloads a page of
	// navigation and cookie banners to produce a few KB of prose.
	req.Header.Set("Accept", "text/markdown, text/plain;q=0.9, text/html;q=0.8, application/json;q=0.8, */*;q=0.1")
	req.Header.Set("Accept-Language", "en")

	if c, ok := t.cache.get(target); ok {
		if c.etag != "" {
			req.Header.Set("If-None-Match", c.etag)
		}
		if c.lastMod != "" {
			req.Header.Set("If-Modified-Since", c.lastMod)
		}
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotModified {
		if c, ok := t.cache.get(target); ok {
			return &fetchResult{body: c.body, contentType: c.contentType, fromCache: true}, resp.StatusCode, nil
		}
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxFetchBytes+1))
	if err != nil {
		return nil, resp.StatusCode, err
	}
	truncated := int64(len(body)) > maxFetchBytes
	if truncated {
		body = body[:maxFetchBytes]
	}

	ct := resp.Header.Get("Content-Type")
	if resp.StatusCode == http.StatusOK {
		if et := resp.Header.Get("ETag"); et != "" {
			t.cache.put(target, cachedPage{etag: et, lastMod: resp.Header.Get("Last-Modified"), body: body, contentType: ct})
		}
	}
	return &fetchResult{body: body, contentType: ct, truncated: truncated}, resp.StatusCode, nil
}

func (t *fetchTool) Run(ctx context.Context, call ToolCall) (ToolResponse, error) {
	var params FetchParams
	if err := UnmarshalToolInput(call.Input, &params); err != nil {
		return NewTextErrorResponse("Failed to parse web_fetch parameters: " + err.Error()), nil
	}
	if params.URL == "" {
		return NewTextErrorResponse("url parameter is required"), nil
	}

	format := strings.ToLower(strings.TrimSpace(params.Format))
	if format == "" {
		format = "markdown"
	}
	// GORILLA OVERRIDE: json is a passthrough, not a conversion. A model
	// calling a JSON API asked for format=json and was refused, which pushed it
	// toward scraping HTML search pages instead of reading a clean API.
	if format != "text" && format != "markdown" && format != "html" && format != "json" {
		return NewTextErrorResponse("format must be one of: text, markdown, html, json"), nil
	}
	if !strings.HasPrefix(params.URL, "http://") && !strings.HasPrefix(params.URL, "https://") {
		return NewTextErrorResponse("url must start with http:// or https://"), nil
	}
	if reason := blockedFetchTarget(params.URL); reason != "" {
		return NewTextErrorResponse("Refusing to fetch that URL: " + reason), nil
	}

	sessionID, messageID := GetContextValues(ctx)
	if sessionID == "" || messageID == "" {
		return ToolResponse{}, fmt.Errorf("session ID and message ID are required for web_fetch")
	}
	if !t.permissions.Request(permission.CreatePermissionRequest{
		SessionID:   sessionID,
		Path:        config.WorkingDirectory(),
		ToolName:    FetchToolName,
		Action:      "fetch",
		Description: fmt.Sprintf("Fetch content from URL: %s", params.URL),
		// GORILLA FIX (2026-08-19): scope the grant. Without it, "allow for
		// session" on one fetch authorised every later fetch in the session —
		// an exfiltration path, since a poisoned page can steer the model to a
		// URL of its choosing and the user is never asked again.
		//
		// The key is the HOST, not the full URL. Per-URL was the first
		// attempt and it is the wrong granularity: reading documentation means
		// dozens of pages on one site, so a per-URL grant produces a prompt
		// per page, and a prompt that fires constantly is a prompt that gets
		// answered without being read. Per-host still asks the question that
		// matters — "why is it talking to THAT site?" — which is the question
		// an exfiltration attempt fails.
		GrantKey: fetchGrantKey(params.URL),
		Egress:   true,
		Params:   FetchPermissionsParams(params),
	}) {
		return ToolResponse{}, permission.ErrorPermissionDenied
	}

	timeout := 30 * time.Second
	if params.Timeout > 0 {
		if params.Timeout > 120 {
			params.Timeout = 120
		}
		timeout = time.Duration(params.Timeout) * time.Second
	}
	reqCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	client := newSafeClient(timeout)

	target, note := rewriteToSource(params.URL)
	var notes []string
	if note != "" {
		notes = append(notes, note)
	}

	res, status, err := t.do(reqCtx, client, target)
	if err != nil {
		return NewTextErrorResponse("Failed to fetch URL: " + err.Error()), nil
	}
	if status != http.StatusOK && status != http.StatusNotModified {
		// GORILLA OVERRIDE: return the body, not just the code. A bare
		// "status 429" discards the Retry-After and the error JSON that say
		// what to do about it.
		excerpt := strings.TrimSpace(decodeBody(res.body, res.contentType))
		if len(excerpt) > 2000 {
			excerpt = excerpt[:2000] + "\n...[error body truncated]"
		}
		msg := fmt.Sprintf("Request failed with status %d (%s).", status, http.StatusText(status))
		if excerpt != "" {
			msg += "\n\nResponse body:\n" + excerpt
		}
		return NewTextErrorResponse(msg), nil
	}

	isHTML := looksLikeHTML(res.contentType, res.body)

	// One cheap retry: a docs URL that answered HTML may have a .md companion.
	if isHTML && format == "markdown" {
		if sib := markdownSibling(target); sib != "" {
			if alt, altStatus, altErr := t.do(reqCtx, client, sib); altErr == nil && altStatus == http.StatusOK &&
				!looksLikeHTML(alt.contentType, alt.body) && len(alt.body) > 0 {
				res, isHTML = alt, false
				notes = append(notes, "served markdown from the .md companion URL")
			}
		}
	}

	if kind := binaryKind(res.body); kind != "" {
		return NewTextErrorResponse(fmt.Sprintf(
			"That URL returned a %s, which this tool cannot convert to text. "+
				"Look for an HTML or markdown version of the same document.", kind)), nil
	}

	content := decodeBody(res.body, res.contentType)

	// GORILLA OVERRIDE (2026-08-19): narrow BEFORE converting, not after.
	//
	// Fetching a documentation page to read one table costs the navigation,
	// the sidebar, the cookie banner and the footer as well. On a metered
	// link that is bytes the user paid for; in the conversation it is a
	// RECURRING bill, because every tool result is re-sent on every later
	// turn. The existing chrome-stripping helps but cannot know that this
	// time only the install section was wanted.
	//
	// Measured 2026-08-19 on https://pkg.go.dev/net/http: the whole page
	// converts to 194,638 bytes (~48,659 tokens); selector ".Documentation-index"
	// gives 7,708 bytes (~1,927 tokens). A 96.0% saving, re-checkable with
	// fetch_narrowing_measure_test.go.
	//
	// It reports the match count and refuses to silently return nothing: a
	// selector that matched zero elements is a mistake to be told about, not
	// an empty document to reason over.
	if sel := strings.TrimSpace(params.Selector); sel != "" {
		if !isHTML {
			notes = append(notes, "selector ignored: this response is not HTML")
		} else {
			narrowed, matched, err := applySelector(content, sel, params.Extract)
			switch {
			case err != nil:
				return NewTextErrorResponse(fmt.Sprintf(
					"That selector could not be used: %s. It must be a CSS selector, "+
						"such as 'table', 'article', 'main', '.changelog' or '#install'.",
					err)), nil
			case matched == 0:
				return NewTextErrorResponse(fmt.Sprintf(
					"The selector %q matched nothing on %s. The page was fetched "+
						"successfully — this is a selector problem, not a network one. "+
						"Fetch it again without a selector to see its structure, then "+
						"narrow.", sel, target)), nil
			default:
				content = narrowed
				notes = append(notes, fmt.Sprintf("narrowed to %q (%d element(s) matched)", sel, matched))
				if ex := strings.TrimSpace(params.Extract); ex != "" {
					notes = append(notes, "extracted "+ex)
					// text, links and attributes are already plain text; do
					// not run them back through an HTML converter.
					if ex != "html" {
						isHTML = false
					}
				}
			}
		}
	}

	var out string
	switch {
	case format == "json", format == "html":
		out = content
	case isHTML && format == "markdown":
		converted, err := convertHTMLToMarkdown(content)
		if err != nil {
			return NewTextErrorResponse("Failed to convert HTML to Markdown: " + err.Error()), nil
		}
		out = converted
		notes = append(notes, "converted from HTML locally (not the site's own markdown)")
	case isHTML && format == "text":
		text, err := extractTextFromHTML(content)
		if err != nil {
			return NewTextErrorResponse("Failed to extract text from HTML: " + err.Error()), nil
		}
		out = text
		notes = append(notes, "text extracted from HTML")
	default:
		out = content
		if note := serverFormatNote(res.contentType); note != "" {
			notes = append(notes, note)
		}
	}

	if res.fromCache {
		notes = append(notes, "unchanged since the last fetch (304); served from this session's cache")
	}

	// Token budget. Estimated at 4 bytes/token - good to roughly +/-15%, and
	// the decision it informs is coarse enough that precision does not matter.
	if est := len(out) / 4; est > maxTokens {
		keep := maxTokens * 4
		out = out[:keep] + fmt.Sprintf(
			"\n\n[CUT AT THE TOKEN BUDGET: this document is ~%d tokens and only the "+
				"first ~%d were kept. THE REST WAS NOT READ - do not summarise this as "+
				"though it were complete. To read it properly, call web_fetch again with "+
				"summarise:true (condensed locally, costs you nothing), or fetch a "+
				"narrower source such as the paper's abstract or API record.]",
			est, maxTokens)
		notes = append(notes, fmt.Sprintf("CUT at the %d-token budget from ~%d", maxTokens, est))
	} else if est > warnTokens {
		out = fmt.Sprintf(
			"[LARGE DOCUMENT: ~%d tokens. This is being added to the conversation and "+
				"will be re-sent on every later turn, so it is a recurring cost, not a "+
				"one-off. If you only need part of it, web_fetch with summarise:true "+
				"condenses it locally for free, or fetch a narrower source.]\n\n", est) + out
		notes = append(notes, fmt.Sprintf("large: ~%d tokens, recurring on every turn", est))
	}

	// GORILLA OVERRIDE: truncation must SAY so. A model handed a silent
	// fragment reasons about the fragment as if it were the whole document.
	if res.truncated {
		out += fmt.Sprintf("\n\n[TRUNCATED: the response exceeded the %d MB limit and was cut here. "+
			"The remainder was NOT read.]", maxFetchBytes/(1024*1024))
	}

	// GORILLA OVERRIDE: local summarisation, opt-in. Runs on the user's own
	// machine so the tokens are never sent, never billed. Summarise() refuses
	// on short input and its Header() carries the compression ratio, so a model
	// can never mistake an extract for the whole document.
	if params.Summarise {
		sum := Summarise(out, 12)
		if sum.Summarised {
			notes = append(notes, fmt.Sprintf("summarised locally: %d of %d sentences, %.0f%% of original",
				sum.SentencesKept, sum.SentencesTotal, sum.CompressionRatio*100))
			out = sum.Header() + "\n\n" + sum.Text
		} else {
			notes = append(notes, "summarise requested but skipped: document too short to shorten safely")
		}
	}

	// GORILLA FIX (2026-08-19), audit: a page that produced no text must say
	// so, and say why it is not the same as an empty page.
	//
	// Same shape as the find and view findings. lynx and the HTML extractor
	// both return "" for a page whose content is built by JavaScript in the
	// browser — which is most of the modern web. Returning an empty fence
	// reads as "this page is blank", and a model reports that as a fact about
	// the site rather than a limit of the tool.
	if strings.TrimSpace(out) == "" {
		return NewTextErrorResponse(fmt.Sprintf(
			"%s was fetched successfully (HTTP %d, %d bytes) but no readable text could be "+
				"extracted from it.\n\n"+
				"This is NOT the same as the page being empty. The usual cause is a page whose "+
				"content is assembled by JavaScript in the browser, which nothing here executes. "+
				"Try format=\"html\" to see the raw markup, look for an API or .md/.txt version of "+
				"the same document, or ask the user to paste what they can see.",
			target, status, len(res.body))), nil
	}

	header := fmt.Sprintf("Fetched: %s", target)
	if len(notes) > 0 {
		header += "\nHow: " + strings.Join(notes, "; ")
	}
	// GORILLA FIX (2026-08-19): this returned the page raw. A fetched page is
	// written by whoever controls that server; it is evidence, never
	// instruction. MarkTainted is what stops the model acting on it — see
	// permission.MarkTainted.
	permission.MarkTainted(sessionID, fmt.Sprintf("web page fetched from %s", target))
	return NewUntrustedTextResponse("web page", target, header, out), nil
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func serverFormatNote(contentType string) string {
	ct := strings.ToLower(contentType)
	switch {
	case strings.Contains(ct, "markdown"):
		return "server returned markdown directly (no conversion)"
	case strings.Contains(ct, "json"):
		return "server returned JSON"
	case strings.Contains(ct, "text/plain"):
		return "server returned plain text"
	}
	return ""
}

// looksLikeHTML trusts Content-Type first and sniffs only when it is absent or
// generic. The old check tested Contains(ct, "text/html") alone, which missed
// application/xhtml+xml and anything served as octet-stream.
func looksLikeHTML(contentType string, body []byte) bool {
	ct := strings.ToLower(contentType)
	if strings.Contains(ct, "text/html") || strings.Contains(ct, "application/xhtml") {
		return true
	}
	if strings.Contains(ct, "markdown") || strings.Contains(ct, "json") ||
		strings.Contains(ct, "text/plain") || strings.Contains(ct, "xml") {
		return false
	}
	head := strings.ToLower(string(body[:min(len(body), 1024)]))
	return strings.Contains(head, "<!doctype html") || strings.Contains(head, "<html")
}

// binaryKind names a binary payload we should refuse rather than cast to a
// string. Previously a PDF became mojibake and the model tried to read it.
func binaryKind(body []byte) string {
	switch {
	case len(body) >= 5 && string(body[:5]) == "%PDF-":
		return "PDF"
	case len(body) >= 4 && string(body[:4]) == "\x7fELF":
		return "binary executable"
	case len(body) >= 2 && body[0] == 0x1f && body[1] == 0x8b:
		return "gzip archive"
	case len(body) >= 4 && string(body[:4]) == "PK\x03\x04":
		return "zip archive"
	case len(body) >= 8 && string(body[1:4]) == "PNG":
		return "PNG image"
	}
	return ""
}

// decodeBody converts the response to UTF-8 using the charset the server
// declared (or the one the document declares internally). Casting bytes to a
// string assumed UTF-8 and turned every Latin-1 or Shift-JIS page into mojibake.
func decodeBody(body []byte, contentType string) string {
	r, err := charset.NewReader(strings.NewReader(string(body)), contentType)
	if err != nil {
		return string(body)
	}
	decoded, err := io.ReadAll(r)
	if err != nil {
		return string(body)
	}
	return string(decoded)
}

func extractTextFromHTML(html string) (string, error) {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		return "", err
	}
	doc.Find("script, style, noscript, nav, header, footer, aside, form").Remove()
	return strings.Join(strings.Fields(doc.Text()), " "), nil
}

func convertHTMLToMarkdown(html string) (string, error) {
	// Strip chrome before conversion: navigation, cookie banners and footers
	// are pure token cost and crowd out the content the model was asked for.
	if doc, err := goquery.NewDocumentFromReader(strings.NewReader(html)); err == nil {
		doc.Find("script, style, noscript, nav, header, footer, aside, form, iframe").Remove()
		if cleaned, err := doc.Html(); err == nil {
			html = cleaned
		}
	}
	return md.NewConverter("", true, nil).ConvertString(html)
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// fetchGrantKey reduces a URL to the thing a session grant should cover: the
// scheme and host. Falls back to the whole URL if it will not parse, because
// an unparseable key is still a narrower grant than no key at all.
func fetchGrantKey(raw string) string {
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" {
		return raw
	}
	return u.Scheme + "://" + u.Host
}

// applySelector keeps only the parts of an HTML document matching sel, and
// optionally pulls one thing out of them.
//
// Returns the narrowed content and how many elements matched, so the caller
// can tell "you asked for something that is not there" from "here is what you
// asked for". A zero match must never be reported as an empty document.
func applySelector(html, sel, extract string) (string, int, error) {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		return "", 0, err
	}
	// Chrome is removed first: a selector like "table" would otherwise happily
	// match a navigation layout table.
	doc.Find("script, style, noscript").Remove()

	matches := doc.Find(sel)
	n := matches.Length()
	if n == 0 {
		return "", 0, nil
	}

	var b strings.Builder
	switch strings.ToLower(strings.TrimSpace(extract)) {
	case "text":
		matches.Each(func(_ int, s *goquery.Selection) {
			if t := strings.Join(strings.Fields(s.Text()), " "); t != "" {
				b.WriteString(t)
				b.WriteString("\n\n")
			}
		})
	case "links":
		// Both halves matter: a bare list of URLs makes the model guess which
		// one it wanted from the path.
		seen := map[string]bool{}
		matches.Find("a[href]").AddSelection(matches.Filter("a[href]")).Each(func(_ int, s *goquery.Selection) {
			href, _ := s.Attr("href")
			if href == "" || seen[href] {
				return
			}
			seen[href] = true
			text := strings.Join(strings.Fields(s.Text()), " ")
			if text == "" {
				text = "(no link text)"
			}
			fmt.Fprintf(&b, "%s\n  %s\n", text, href)
		})
	case "", "html":
		matches.Each(func(_ int, s *goquery.Selection) {
			if h, err := goquery.OuterHtml(s); err == nil {
				b.WriteString(h)
				b.WriteString("\n")
			}
		})
	default:
		// Anything else is an attribute name: extract:"href" on
		// selector:"a.download" returns the download URLs alone.
		attr := strings.TrimSpace(extract)
		matches.Each(func(_ int, s *goquery.Selection) {
			if v, ok := s.Attr(attr); ok && v != "" {
				b.WriteString(v)
				b.WriteString("\n")
			}
		})
	}

	out := strings.TrimRight(b.String(), "\n")
	if out == "" {
		// Matched, but the requested piece was not present. That is a
		// different failure from "matched nothing" and must not be laundered
		// into it, so it is reported as a match with an explanatory body.
		return fmt.Sprintf("[%d element(s) matched %q, but none of them had %q to extract.]",
			n, sel, extract), n, nil
	}
	return out, n, nil
}
