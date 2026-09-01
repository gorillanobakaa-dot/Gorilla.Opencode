package tools

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"unicode"
)

// GORILLA OVERRIDE: general web search with no service, no key and no config,
// by rendering a search engine's results page through lynx.
//
// WHY LYNX AND NOT AN HTTP CLIENT
//
// Measured 2026-08-08 against html.duckduckgo.com from this machine:
//
//	curl  -> HTTP 202, 14,241 bytes, 47 "anomaly" markers   (a block page)
//	lynx  ->  3,247 bytes of real results, exit 0
//
// The block was never DuckDuckGo refusing automation in general - it was that
// client. lynx handles cookies, redirects and conventional headers, and is
// simply let through where a bare Go HTTP client is not.
//
// WHY THE USER AGENT IS LEFT HONEST
//
// Spoofing Chrome measurably raises the hit rate. It also makes failure QUIETER,
// which is the wrong trade for this codebase. Same query, same machine:
//
//	honest UA blocked  ->   157 bytes, exit 1        (unmistakable)
//	Chrome UA blocked  -> 1,122 bytes, exit 0, and the body is a CAPTCHA page
//	                      reading "Select all squares containing a duck"
//
// A model handed the second one summarises the CAPTCHA. lynx even warns you it
// is being made to lie ("User-Agent string does not contain Lynx"). So: no
// spoofing. This backend does not fail less, it fails legibly.
//
// WHY SUCCESS IS MEASURED IN EXTRACTED RESULTS, NOT EXIT CODES
//
// Neither a byte count nor an exit code is sufficient, and both were tried:
// that CAPTCHA page is 1,122 bytes with exit 0, and a rate-limited DuckDuckGo
// returns 12,075 bytes with exit 0 and ZERO results. The only check that
// survives every observed failure is "did we extract at least one external
// result URL". Presence of results, not absence of errors.
//
// ENGINE CHOICE IS MEASURED, NOT ASSUMED
//
// Queried once each through lynx on 2026-08-08, counting external result URLs:
//
//	marginalia 43 · brave 28 · ecosia 27 · mojeek 19
//	bing 7 · yahoo 7 · dogpile 4
//	duckduckgo 0 (12 KB of exit-0 CAPTCHA) · google 0 (refuses text browsers
//	outright: "Your browser isn't supported any more") · startpage 0
//
// DuckDuckGo and Google are permanently excluded. DuckDuckGo is the worst of
// the list precisely because it fails at exit 0 with a plausible body - it is
// the case this file's discriminator exists to catch.
//
// Engines are tried in order and the first that yields results wins; the ones
// that failed are reported as PARTIAL rather than hidden. They are NOT
// independent: these are the same upstreams SearXNG queries, so a configured
// SearXNG is strictly better and is tried first. This is the fallback that
// needs no setup, not a second opinion.

type lynxEngine struct {
	name     string
	queryURL string // %s is the escaped query
	ownHost  string // this engine's own domain, filtered out of results
}

// lynxEngines is ordered mainstream-first: a coding agent usually wants the
// common answer. Marginalia is last because its index deliberately demotes
// commercial content - excellent for obscure corners, wrong as a default.
var lynxEngines = []lynxEngine{
	{"Brave", "https://search.brave.com/search?q=%s", "brave.com"},
	{"Ecosia", "https://www.ecosia.org/search?q=%s", "ecosia.org"},
	{"Mojeek", "https://www.mojeek.com/search?q=%s", "mojeek.com"},
	{"Marginalia", "https://old-search.marginalia.nu/search?query=%s", "marginalia.nu"},
}

var (
	// "   12. https://example.com/x" in lynx's References section.
	lynxRefRe = regexp.MustCompile(`^\s*(\d+)\.\s+(https?://\S+)\s*$`)
	// "[12]Some link text" in the body.
	lynxMarkRe = regexp.MustCompile(`\[(\d+)\]`)
)

// lynxPath reports where lynx is, or "" if it is not installed.
// GORILLA OVERRIDE: On Windows, checks multiple fallback paths since lynx is rare
func lynxPath() string {
	// First try PATH
	if p, err := exec.LookPath("lynx"); err == nil {
		return p
	}

	// Windows: Check common installation paths
	if runtime.GOOS == "windows" {
		windowsPaths := []string{
			filepath.Join(os.Getenv("ProgramFiles"), "Git", "usr", "bin", "lynx.exe"),
			filepath.Join(os.Getenv("ProgramFiles(x86)"), "Git", "usr", "bin", "lynx.exe"),
			"C:\\cygwin64\\bin\\lynx.exe",
			"C:\\cygwin\\bin\\lynx.exe",
			filepath.Join(os.Getenv("USERPROFILE"), "AppData", "Local", "lynx", "lynx.exe"),
		}
		for _, p := range windowsPaths {
			if _, err := os.Stat(p); err == nil {
				return p
			}
		}
	}

	return ""
}

// looksLikeURLText reports whether a link's text is really a displayed address
// or breadcrumb rather than a title. Engines print both, pointing at the same
// target; the title is the one worth keeping.
func looksLikeURLText(s string) bool {
	s = strings.TrimSpace(s)
	return s == "" || strings.HasPrefix(s, "http") || strings.Contains(s, "\u203a")
}

// chromeLinkText marks link text that belongs to the engine's furniture rather
// than to a result. "More on reddit.com" and friends point at a real external
// URL, so a host filter cannot catch them - but they are navigation, not an
// answer, and taking them produced results titled "More on reddit.com" on the
// first live run of this backend.
var chromeLinkText = []string{
	"more on", "view all", "see more results", "more results",
	"next page", "previous", "images", "videos", "news", "maps",
}

// cleanTitle strips the decoration engines put in front of a title - a favicon
// glyph, a bullet, a bare site name. Without it "🌐 youtube.com" reads as a
// 16-byte string containing a space and passes for a title; stripped, it is an
// 11-character domain and is correctly refused.
func cleanTitle(s string) string {
	s = strings.TrimSpace(s)
	s = strings.TrimLeftFunc(s, func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r)
	})
	return strings.TrimSpace(s)
}

func isChromeLink(s string) bool {
	l := strings.ToLower(strings.TrimSpace(s))
	if l == "" {
		return false
	}
	for _, c := range chromeLinkText {
		if strings.HasPrefix(l, c) {
			return true
		}
	}
	return false
}

// plausibleTitle decides whether a line can serve as a result title.
//
// Engines lay results out differently - Mojeek writes "[41]Title" inline, Brave
// puts a bare "[12]" then a favicon, a site name and a breadcrumb before the
// title arrives. Rather than model each layout (the per-engine parsing that
// makes SearXNG a large project), accept only lines that look like a title in
// ANY layout, and refuse the result outright when none is found. A missing
// result is recoverable; a result labelled with a breadcrumb is not, because the
// agent will quote it.
func plausibleTitle(s string) bool {
	s = cleanTitle(s)
	if len(s) < 15 || looksLikeURLText(s) || isChromeLink(s) {
		return false
	}
	if !strings.Contains(s, " ") { // a bare domain or token
		return false
	}
	// A line that is mostly punctuation/box-drawing is furniture.
	letters := 0
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') {
			letters++
		}
	}
	return letters*2 > len(s)
}

// parseLynxDump turns `lynx -dump` output into hits.
//
// It keys off lynx's OWN formatting - a trailing "References" list of numbered
// URLs, and [n] markers in the body - not off any engine's HTML. That is what
// lets one parser serve every engine: the layout above it can change freely,
// and the numbering underneath does not.
func parseLynxDump(dump, ownHost string, max int) []searchHit {
	idx := strings.Index(dump, "\nReferences\n")
	if idx < 0 {
		return nil
	}
	body, refs := dump[:idx], dump[idx:]

	urls := map[string]string{}
	for _, line := range strings.Split(refs, "\n") {
		if m := lynxRefRe.FindStringSubmatch(line); m != nil {
			urls[m[1]] = m[2]
		}
	}
	if len(urls) == 0 {
		return nil
	}

	type entry struct {
		hit   searchHit
		order int
	}
	byURL := map[string]*entry{}
	var order []string

	lines := strings.Split(body, "\n")
	for i, line := range lines {
		m := lynxMarkRe.FindStringSubmatchIndex(line)
		if m == nil {
			continue
		}
		num := line[m[2]:m[3]]
		target, ok := urls[num]
		if !ok {
			continue
		}
		u, err := url.Parse(target)
		if err != nil || u.Host == "" {
			continue
		}
		// Drop the engine's own chrome: nav, prefs, "next page", its own logo.
		if host := strings.TrimPrefix(strings.ToLower(u.Host), "www."); host == ownHost ||
			strings.HasSuffix(host, "."+ownHost) {
			continue
		}
		inline := strings.TrimSpace(line[m[1]:])

		// A marker WITH text already carries its own label; that text is the
		// only honest title for this link. If it is unusable - engine furniture
		// like "More on reddit.com", or a breadcrumb - skip the link entirely.
		//
		// Do NOT go looking for a title elsewhere. Searching forward from a
		// labelled link is how the first version attached "SMB Performance
		// fixed by switching TCP congestion algo to BBR" to a URL about
		// Windows 11 and BBR2: lynx had WRAPPED "More on reddit.com" across two
		// lines, so the label read as the unusable "More on", and the scan ran
		// past the block's furniture into the NEXT result's title. A plausible
		// title on the wrong URL is the worst output this tool can produce -
		// it survives a spot-check.
		var title string
		snippetFrom := i + 1
		if inline != "" {
			if !plausibleTitle(inline) {
				continue
			}
			title = cleanTitle(inline)
		} else {
			// A BARE marker has no label, so the title genuinely follows it -
			// Brave prints a favicon, a site name and a breadcrumb first. Only
			// here is looking ahead legitimate, and it stops at the next marker.
			for j := i + 1; j < len(lines) && j < i+8; j++ {
				if lynxMarkRe.MatchString(lines[j]) {
					break
				}
				if plausibleTitle(lines[j]) {
					title, snippetFrom = cleanTitle(lines[j]), j+1
					break
				}
			}
			if title == "" {
				continue
			}
		}

		e, seen := byURL[target]
		if !seen {
			e = &entry{hit: searchHit{URL: target, Backend: "lynx"}}
			byURL[target] = e
			order = append(order, target)
		}
		// An engine prints the address and the title as separate links to the
		// same place. Prefer whichever reads like a title.
		if e.hit.Title == "" || (looksLikeURLText(e.hit.Title) && !looksLikeURLText(title)) {
			e.hit.Title = title
			e.hit.Abstract = lynxSnippet(lines, snippetFrom)
		}
	}

	hits := make([]searchHit, 0, len(order))
	for _, u := range order {
		h := byURL[u].hit
		if strings.TrimSpace(h.Title) == "" {
			continue
		}
		if looksLikeURLText(h.Title) {
			// Never found a real title for this link - it was almost certainly
			// chrome rather than a result.
			continue
		}
		hits = append(hits, h)
		if len(hits) >= max {
			break
		}
	}
	return hits
}

// lynxSnippet collects the description lines that follow a result's title:
// everything up to the next link marker or blank line.
func lynxSnippet(lines []string, from int) string {
	var parts []string
	for i := from; i < len(lines) && i < from+6; i++ {
		l := strings.TrimSpace(lines[i])
		if l == "" || strings.Contains(l, "[") {
			break
		}
		parts = append(parts, l)
	}
	s := strings.Join(parts, " ")
	if len(s) > 400 {
		s = s[:400] + "..."
	}
	return s
}

// runLynx renders one URL and returns the dump.
func runLynx(ctx context.Context, bin, target string) (string, error) {
	cmd := exec.CommandContext(ctx, bin,
		"-dump",
		"-connect_timeout=10",
		"-read_timeout=20",
		// No -useragent: see the header. Honest identification fails loudly,
		// and a loud failure is the feature.
		target)
	out, err := cmd.Output()
	return string(out), err
}

// searchLynx tries each engine until one yields results. Engines that answered
// with nothing usable are reported as warnings, never silently dropped.
func (t *webSearchTool) searchLynx(ctx context.Context, q string, n int) ([]searchHit, error) {
	bin := lynxPath()
	if bin == "" {
		return nil, fmt.Errorf("lynx is not installed")
	}
	esc := url.QueryEscape(q)

	var tried []string
	for _, e := range lynxEngines {
		dump, err := runLynx(ctx, bin, fmt.Sprintf(e.queryURL, esc))
		if err != nil && strings.TrimSpace(dump) == "" {
			tried = append(tried, fmt.Sprintf("%s (no response: %v)", e.name, err))
			continue
		}
		hits := parseLynxDump(dump, e.ownHost, n)
		if len(hits) == 0 {
			// The important case. A blocked engine can answer 200 with a
			// plausible page - a CAPTCHA, a "no results" shell - so "it did not
			// error" proves nothing. Zero extracted results IS the failure.
			tried = append(tried, fmt.Sprintf("%s (answered %d bytes, no results found in it)",
				e.name, len(dump)))
			continue
		}
		for i := range hits {
			hits[i].Backend = "lynx/" + e.name
		}
		if len(tried) > 0 {
			addSearchWarning(ctx, "web search fell back to %s; earlier engines gave nothing: %s",
				e.name, strings.Join(tried, "; "))
		}
		return hits, nil
	}

	return nil, fmt.Errorf(
		"every search engine failed, so nothing was searched (%s). This is NOT evidence "+
			"that no results exist - report the failure rather than answering from memory",
		strings.Join(tried, "; "))
}
