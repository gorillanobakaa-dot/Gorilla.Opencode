package tools

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"
)

// These tests exist for one reason above all others: to prove that "every engine
// blocked us" is reported as a FAILURE and never as "no results". That single
// distinction is why SearXNG was chosen over a paid grounding API, and it is the
// exact lie that produced the fabricated citation table on 2026-08-07 - a model
// told "nothing found" concludes the thing does not exist.
//
// Each test drives the real searchSearxNG against an httptest server, so the
// HTTP handling, the content-type guard and the JSON shape are all exercised.

// searxStub starts a fake SearXNG that returns the given JSON body.
func searxStub(t *testing.T, body string) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/search" {
			t.Errorf("expected /search, got %s", r.URL.Path)
		}
		if got := r.URL.Query().Get("format"); got != "json" {
			t.Errorf("expected format=json, got %q", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)
	return srv
}

func newTestTool() *webSearchTool {
	return NewWebSearchTool(nil).(*webSearchTool)
}

func TestSearxNGParsesResults(t *testing.T) {
	srv := searxStub(t, `{
	  "results": [
	    {"url":"https://example.com/a","title":"Alpha","content":"first snippet","engine":"duckduckgo"},
	    {"url":"https://example.com/b","title":"Beta","content":"second snippet","engine":"brave","publishedDate":"2021-04-05T00:00:00"}
	  ],
	  "unresponsive_engines": []
	}`)
	t.Setenv(searxngEnvVar, srv.URL)

	hits, err := newTestTool().searchSearxNG(context.Background(), "anything", 8)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(hits) != 2 {
		t.Fatalf("want 2 hits, got %d", len(hits))
	}
	if hits[0].Title != "Alpha" || hits[0].URL != "https://example.com/a" {
		t.Errorf("first hit wrong: %+v", hits[0])
	}
	if hits[0].Abstract != "first snippet" {
		t.Errorf("content should map to Abstract, got %q", hits[0].Abstract)
	}
	if hits[0].Backend != "SearXNG/duckduckgo" {
		t.Errorf("backend should name the upstream engine, got %q", hits[0].Backend)
	}
	if hits[1].Year != "2021" {
		t.Errorf("publishedDate should yield year 2021, got %q", hits[1].Year)
	}
}

// The load-bearing test. Zero results with every engine dead must be an error,
// because the caller treats a nil error with no hits as "searched fine, nothing
// exists" and says so to the user.
func TestSearxNGAllEnginesDeadIsFailureNotEmptiness(t *testing.T) {
	srv := searxStub(t, `{
	  "results": [],
	  "unresponsive_engines": [["google","CAPTCHA"],["bing","timeout"]]
	}`)
	t.Setenv(searxngEnvVar, srv.URL)

	hits, err := newTestTool().searchSearxNG(context.Background(), "anything", 8)
	if err == nil {
		t.Fatalf("all engines dead must be an error, got nil with %d hits", len(hits))
	}
	// The message has a job: stop the model concluding absence.
	if !strings.Contains(err.Error(), "NOT") || !strings.Contains(err.Error(), "evidence") {
		t.Errorf("error must say this is not evidence of absence, got: %v", err)
	}
	for _, engine := range []string{"google", "bing", "CAPTCHA", "timeout"} {
		if !strings.Contains(err.Error(), engine) {
			t.Errorf("error should name the failed engine/reason %q, got: %v", engine, err)
		}
	}
}

// Guards the inverse: a genuinely empty index with all engines healthy is NOT an
// error. Without this, the test above would pass against code that simply errors
// on every empty result, which would be a different bug.
func TestSearxNGEmptyWithHealthyEnginesIsNotAnError(t *testing.T) {
	srv := searxStub(t, `{"results": [], "unresponsive_engines": []}`)
	t.Setenv(searxngEnvVar, srv.URL)

	hits, err := newTestTool().searchSearxNG(context.Background(), "anything", 8)
	if err != nil {
		t.Fatalf("empty results with healthy engines is a real 'no results', not an error: %v", err)
	}
	if len(hits) != 0 {
		t.Fatalf("want 0 hits, got %d", len(hits))
	}
}

func TestSearxNGPartialCoverageWarns(t *testing.T) {
	srv := searxStub(t, `{
	  "results": [{"url":"https://example.com/a","title":"Alpha","content":"x","engine":"mojeek"}],
	  "unresponsive_engines": [["google","CAPTCHA"]]
	}`)
	t.Setenv(searxngEnvVar, srv.URL)

	ctx, warnings := withSearchWarnings(context.Background())
	hits, err := newTestTool().searchSearxNG(ctx, "anything", 8)
	if err != nil {
		t.Fatalf("partial coverage still returns results: %v", err)
	}
	if len(hits) != 1 {
		t.Fatalf("want 1 hit, got %d", len(hits))
	}
	if len(*warnings) != 1 {
		t.Fatalf("want 1 warning about the dead engine, got %d: %v", len(*warnings), *warnings)
	}
	if !strings.Contains((*warnings)[0], "google") {
		t.Errorf("warning should name the dead engine, got %q", (*warnings)[0])
	}
}

// A SearXNG with format=json disabled answers 200 with an HTML page. Parsing that
// hopefully is precisely the "200-with-garbage" failure this file was written to
// prevent - and the resulting JSON syntax error would read as a bug in this tool
// rather than a setting on their instance.
func TestSearxNGRefusesHTMLBody(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte("<!DOCTYPE html><html><body>results</body></html>"))
	}))
	defer srv.Close()
	t.Setenv(searxngEnvVar, srv.URL)

	_, err := newTestTool().searchSearxNG(context.Background(), "anything", 8)
	if err == nil {
		t.Fatal("an HTML 200 must be refused, not parsed")
	}
	if !strings.Contains(err.Error(), "search.formats") {
		t.Errorf("error should tell the operator how to fix it, got: %v", err)
	}
}

func TestSearxNGStatusCodesCarryFixHints(t *testing.T) {
	for _, tc := range []struct {
		name   string
		status int
		want   string
	}{
		{"format disabled", http.StatusForbidden, "search.formats"},
		{"limiter", http.StatusTooManyRequests, "limiter"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(tc.status)
			}))
			defer srv.Close()
			t.Setenv(searxngEnvVar, srv.URL)

			_, err := newTestTool().searchSearxNG(context.Background(), "anything", 8)
			if err == nil {
				t.Fatalf("HTTP %d must be an error", tc.status)
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Errorf("HTTP %d should hint at %q, got: %v", tc.status, tc.want, err)
			}
		})
	}
}

// The SSRF exemption for this backend is justified by the address coming from
// the operator, not the model. A redirect would let the instance move the target
// after the fact and inherit that exemption, so it must not be followed.
func TestSearxNGDoesNotFollowRedirects(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "http://169.254.169.254/latest/meta-data/", http.StatusFound)
	}))
	defer srv.Close()
	t.Setenv(searxngEnvVar, srv.URL)

	_, err := newTestTool().searchSearxNG(context.Background(), "anything", 8)
	if err == nil {
		t.Fatal("a redirect must be refused, not followed")
	}
	if !strings.Contains(err.Error(), "refusing to follow") {
		t.Errorf("error should say the redirect was refused, got: %v", err)
	}
}

// Live check against a REAL SearXNG. Opt-in, same convention as the other live
// tests in this package:
//
//	SEARXNG_URL=http://127.0.0.1:8888 GORILLA_LIVE_SEARCH=1 \
//	  go test ./internal/llm/tools/ -run LiveSearxNG -v
//
// The stubs above assert against a JSON shape I wrote from the documentation.
// This asserts against the shape the software actually emits, which is the only
// thing that can catch the documentation being wrong or the format moving. Run
// once against a fresh instance after upgrading SearXNG.
func TestLiveSearxNGReturnsParseableResults(t *testing.T) {
	if os.Getenv("GORILLA_LIVE_SEARCH") != "1" {
		t.Skip("set GORILLA_LIVE_SEARCH=1 and SEARXNG_URL to run (needs a running SearXNG)")
	}
	if searxngEndpoint() == "" {
		t.Skip("SEARXNG_URL not set")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	ctx, warnings := withSearchWarnings(ctx)

	hits, err := newTestTool().searchSearxNG(ctx, "searxng json api", 8)
	if err != nil {
		t.Fatalf("live SearXNG search failed: %v", err)
	}
	if len(hits) == 0 {
		t.Fatal("live SearXNG returned no hits for a query that certainly has some")
	}
	for i, h := range hits {
		if strings.TrimSpace(h.Title) == "" {
			t.Errorf("hit %d has no title: %+v", i, h)
		}
		if !strings.HasPrefix(h.URL, "http") {
			t.Errorf("hit %d has no usable URL: %+v", i, h)
		}
		if !strings.HasPrefix(h.Backend, "SearXNG") {
			t.Errorf("hit %d lost its backend attribution: %+v", i, h)
		}
	}
	// Not an assertion - upstream engines fail whenever they feel like it, and a
	// test that required either outcome would be flaky. Logged because seeing a
	// real one prove the path is the point.
	t.Logf("%d hits, %d degradation warning(s): %v", len(hits), len(*warnings), *warnings)
}

func TestSearxNGEndpointPrefersConfigThenEnv(t *testing.T) {
	t.Setenv(searxngEnvVar, "http://from-env:8888/")
	if got := searxngEndpoint(); got != "http://from-env:8888" {
		t.Errorf("env should be used and the trailing slash trimmed, got %q", got)
	}
	t.Setenv(searxngEnvVar, "")
	if got := searxngEndpoint(); got != "" {
		t.Errorf("unset means OFF, got %q", got)
	}
}

// Unconfigured web search must refuse with instructions BEFORE the permission
// prompt - asking someone to approve a search that cannot happen trains them to
// approve without reading, and no answer they give changes the outcome. The nil
// permission service here is the assertion: if Run reached the prompt it would
// panic, so passing proves the refusal came first.
func TestWebSearchRefusesUnconfiguredWebBeforeAskingPermission(t *testing.T) {
	t.Setenv(searxngEnvVar, "")

	in, err := json.Marshal(WebSearchParams{Query: "anything", Source: "web"})
	if err != nil {
		t.Fatal(err)
	}
	resp, err := newTestTool().Run(context.Background(), ToolCall{Input: string(in)})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !resp.IsError {
		t.Error("an unconfigured web search is an error response, not a result")
	}
	// The refusal must (a) say it is off, (b) name the ONE command to fix it,
	// (c) forbid improvising the steps, and (d) give a fallback. (c) is not
	// decoration: on 2026-08-08 a model relaying an earlier version of this text
	// silently dropped "pyyaml" from the pip line while merely paraphrasing.
	for _, want := range []string{"not configured", "setup-searxng.sh", "do not improvise", "web_fetch"} {
		if !strings.Contains(strings.ToLower(resp.Content), want) {
			t.Errorf("refusal text must mention %q so the user can fix it; got:\n%s", want, resp.Content)
		}
	}
}
