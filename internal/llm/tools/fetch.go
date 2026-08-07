package tools

import (
	"context"
	"encoding/json"
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
}

type FetchPermissionsParams struct {
	URL     string `json:"url"`
	Format  string `json:"format,omitempty"`
	Timeout int    `json:"timeout,omitempty"`
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
- Rewrites github.com/…/blob/… to raw.githubusercontent.com automatically.
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
				"enum":        []string{"text", "markdown", "html"},
			},
			"timeout": map[string]any{
				"type":        "number",
				"description": "Optional timeout in seconds (max 120).",
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
	req.Header.Set("User-Agent", "gorilla-opencode/1.0 (+https://github.com/gorillanobakaa-dot/Gorilla.Opencode)")
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
	if err := json.Unmarshal([]byte(call.Input), &params); err != nil {
		return NewTextErrorResponse("Failed to parse web_fetch parameters: " + err.Error()), nil
	}
	if params.URL == "" {
		return NewTextErrorResponse("url parameter is required"), nil
	}

	format := strings.ToLower(strings.TrimSpace(params.Format))
	if format == "" {
		format = "markdown"
	}
	if format != "text" && format != "markdown" && format != "html" {
		return NewTextErrorResponse("format must be one of: text, markdown, html"), nil
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
		Params:      FetchPermissionsParams(params),
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
			excerpt = excerpt[:2000] + "\n…[error body truncated]"
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

	var out string
	switch {
	case format == "html":
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

	// GORILLA OVERRIDE: truncation must SAY so. A model handed a silent
	// fragment reasons about the fragment as if it were the whole document.
	if res.truncated {
		out += fmt.Sprintf("\n\n[TRUNCATED: the response exceeded the %d MB limit and was cut here. "+
			"The remainder was NOT read.]", maxFetchBytes/(1024*1024))
	}

	header := fmt.Sprintf("Fetched: %s", target)
	if len(notes) > 0 {
		header += "\nHow: " + strings.Join(notes, "; ")
	}
	return NewTextResponse(header + "\n\n" + out), nil
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
