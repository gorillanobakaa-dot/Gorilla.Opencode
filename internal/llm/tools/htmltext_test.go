package tools

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"
)

// A results page shaped like Mojeek's: furniture, a script, two results with
// snippets, an engine-internal link and a javascript: link.
const fakeResultsPage = `<!DOCTYPE html>
<html><head><title>q - Search</title><style>body{color:red}</style>
<script>var tracking = "never shown";</script></head>
<body>
<nav><a href="/preferences">Preferences</a> <a href="javascript:void(0)">Menu</a></nav>
<ul class="results">
 <li><a href="https://go.dev/doc/tutorial/generics">Tutorial: Getting started with generics</a>
     <p class="s">This tutorial introduces the basics of generics in Go &amp; shows type parameters.</p></li>
 <li><a href="https://example.org/blog/generics-deep-dive">A deep dive into Go generics and constraints</a>
     <p class="s">Constraints,   type sets
     and inference explained.</p></li>
</ul>
<footer><a href="/about">About us</a></footer>
</body></html>`

// TestHTMLToTextWritesLynxLayout: text without scripts or styles, a [n]
// marker where each link stands, entities decoded, whitespace collapsed,
// and a References list of absolute URLs.
func TestHTMLToTextWritesLynxLayout(t *testing.T) {
	base, _ := url.Parse("https://www.mojeek.com/search?q=go+generics")
	got := htmlToText(fakeResultsPage, base)

	for _, gone := range []string{"tracking", "color:red", "javascript:"} {
		if strings.Contains(got, gone) {
			t.Errorf("the text keeps %q:\n%s", gone, got)
		}
	}
	for _, want := range []string{
		"[2]Tutorial: Getting started with generics",
		"generics in Go & shows type parameters.",
		"Constraints, type sets and inference explained.",
		"\nReferences\n",
		"1. https://www.mojeek.com/preferences",
		"2. https://go.dev/doc/tutorial/generics",
		"3. https://example.org/blog/generics-deep-dive",
		"4. https://www.mojeek.com/about",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q in:\n%s", want, got)
		}
	}
	if strings.Contains(got, "5. ") {
		t.Errorf("a non-http link was numbered:\n%s", got)
	}
}

// TestHTMLToTextFeedsTheLynxParser: the one parser every engine shares
// reads the built-in reader's output as it reads lynx's: two results with
// their snippets, the engine's own links dropped.
func TestHTMLToTextFeedsTheLynxParser(t *testing.T) {
	base, _ := url.Parse("https://www.mojeek.com/search?q=go+generics")
	hits := parseLynxDump(htmlToText(fakeResultsPage, base), "mojeek.com", 10)
	if len(hits) != 2 {
		t.Fatalf("want 2 results, got %d: %+v", len(hits), hits)
	}
	if hits[0].URL != "https://go.dev/doc/tutorial/generics" || hits[0].Title != "Tutorial: Getting started with generics" {
		t.Errorf("first result = %+v", hits[0])
	}
	if !strings.Contains(hits[0].Abstract, "basics of generics") {
		t.Errorf("first snippet = %q", hits[0].Abstract)
	}
	if hits[1].URL != "https://example.org/blog/generics-deep-dive" {
		t.Errorf("second result = %+v", hits[1])
	}
}

// TestHTMLToTextBuiltinReaderSearchesWithoutLynx: the built-in reader
// fetches an engine page over HTTP, says who it is, and returns the results;
// a blocked engine before it is reported, not hidden.
func TestHTMLToTextBuiltinReaderSearchesWithoutLynx(t *testing.T) {
	var ua string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/blocked" {
			w.WriteHeader(http.StatusForbidden)
			_, _ = w.Write([]byte("<html><body>Access denied</body></html>"))
			return
		}
		ua = r.Header.Get("User-Agent")
		_, _ = w.Write([]byte(fakeResultsPage))
	}))
	defer srv.Close()

	oldEngines, oldWait := lynxEngines, lynxWait
	lynxEngines = []lynxEngine{
		{"Blocked", srv.URL + "/blocked?q=%s", "blocked.invalid"},
		{"Fake", srv.URL + "/search?q=%s", strings.TrimPrefix(srv.URL, "http://")},
	}
	lynxWait = func(string) {}
	t.Cleanup(func() { lynxEngines, lynxWait = oldEngines, oldWait })

	tool := &webSearchTool{client: srv.Client()}
	ctx, warns := withSearchWarnings(context.Background())
	hits, err := tool.searchBuiltin(ctx, "go generics", 5)
	if err != nil {
		t.Fatalf("searchBuiltin: %v", err)
	}
	if len(hits) != 2 || hits[0].Backend != "reader/Fake" {
		t.Fatalf("hits = %+v", hits)
	}
	if ua != politeContact {
		t.Errorf("User-Agent = %q, want the honest %q", ua, politeContact)
	}
	if w := strings.Join(*warns, "\n"); !strings.Contains(w, "Blocked") {
		t.Errorf("the blocked engine was not reported: %q", w)
	}
}

// TestHTMLToTextReaderIsOffUntilTheUserTurnsItOn: no new channel opens by
// itself. With no SearXNG and the switch off, the reader is off and the
// refusal text names all three missing routes; the environment switch turns
// it on.
func TestHTMLToTextReaderIsOffUntilTheUserTurnsItOn(t *testing.T) {
	t.Setenv(webReaderEnvVar, "")
	t.Setenv(searxngEnvVar, "")
	if builtinReaderEnabled() {
		t.Fatal("the built-in reader is on by default")
	}
	for _, goos := range []string{"windows", "linux", "darwin"} {
		text := searxngSetupFor(goos)
		for _, want := range []string{"MISSING ON THIS MACHINE", "built-in reader", webReaderEnvVar + "=builtin", `"webSearchBuiltin": true`} {
			if !strings.Contains(text, want) {
				t.Errorf("%s: the refusal does not say %q", goos, want)
			}
		}
	}
	status := strings.Join(WebSearchStatus(), "\n")
	if !strings.Contains(status, "built-in reader off") {
		t.Errorf("doctor does not say the reader is off:\n%s", status)
	}
	t.Setenv(webReaderEnvVar, "builtin")
	if !builtinReaderEnabled() {
		t.Error(webReaderEnvVar + "=builtin did not turn the reader on")
	}
	if status := strings.Join(WebSearchStatus(), "\n"); !strings.Contains(status, "built-in reader on") {
		t.Errorf("doctor does not say the reader is on:\n%s", status)
	}
}

// TestLiveBuiltinReaderFindsResults runs the built-in reader against the real
// engines. Opt-in: it sends a query to them.
func TestLiveBuiltinReaderFindsResults(t *testing.T) {
	if os.Getenv("GORILLA_LIVE_SEARCH") != "1" {
		t.Skip("set GORILLA_LIVE_SEARCH=1 to run (sends a query to Brave/Ecosia/Mojeek/Marginalia)")
	}
	ctx, warns := withSearchWarnings(context.Background())
	hits, err := newTestTool().searchBuiltin(ctx, "golang generics tutorial", 5)
	t.Logf("warnings: %q", *warns)
	if err != nil {
		t.Fatalf("searchBuiltin: %v", err)
	}
	for _, h := range hits {
		t.Logf("%s | %s | %s", h.Backend, h.Title, h.URL)
	}
	if len(hits) == 0 {
		t.Fatal("no results")
	}
}
