package tools

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"
)

// GORILLA OVERRIDE: the offline tests below pin the parsing quirks that would
// silently produce empty results - OpenAlex's inverted-index abstracts and
// Crossref's JATS fragments. The live test is opt-in via GORILLA_LIVE_SEARCH=1,
// because a test suite that needs the network fails for the wrong reasons.

func TestOpenAlexAbstractRebuildsWordOrder(t *testing.T) {
	t.Parallel()
	// OpenAlex stores {word: [positions]}, not prose. Getting this wrong yields
	// a plausible-looking bag of words in the right place in the output.
	idx := map[string][]int{"hallucination": {2}, "Mitigating": {0}, "in": {3}, "LLM": {1}, "agents": {4}}
	got := openAlexAbstract(idx)
	want := "Mitigating LLM hallucination in agents"
	if got != want {
		t.Errorf("openAlexAbstract() = %q, want %q", got, want)
	}
	if openAlexAbstract(nil) != "" {
		t.Error("nil index should produce an empty abstract, not a panic or junk")
	}
}

func TestStripJATSRemovesMarkupNotContent(t *testing.T) {
	t.Parallel()
	in := "<jats:p>We study <jats:italic>hallucination</jats:italic> in LLMs.</jats:p>"
	got := stripJATS(in)
	for _, frag := range []string{"We study", "hallucination", "in LLMs."} {
		if !strings.Contains(got, frag) {
			t.Errorf("stripJATS dropped content %q from %q", frag, got)
		}
	}
	if strings.Contains(got, "<") || strings.Contains(got, "jats:") {
		t.Errorf("stripJATS left markup: %q", got)
	}
}

// Live check. Opt-in: GORILLA_LIVE_SEARCH=1 go test ./internal/llm/tools/ -run Live
//
// This asserts the thing that unit tests cannot: that the real endpoints still
// answer in the shape the parsers expect. An API that changes its JSON would
// otherwise degrade to "no results" - which reads as a legitimate empty search,
// and is exactly the silence-looks-like-success failure this project keeps
// hitting.
func TestLiveBackendsReturnParseableResults(t *testing.T) {
	if os.Getenv("GORILLA_LIVE_SEARCH") != "1" {
		t.Skip("set GORILLA_LIVE_SEARCH=1 to run (needs network)")
	}
	tool := &webSearchTool{client: newSafeClient(30 * time.Second)}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	for name, fn := range map[string]func(context.Context, string, int) ([]searchHit, error){
		"OpenAlex":  tool.searchOpenAlex,
		"EuropePMC": tool.searchEuropePMC,
		"Crossref":  tool.searchCrossref,
	} {
		hits, err := fn(ctx, "LLM hallucination", 3)
		if err != nil {
			t.Errorf("%s: %v", name, err)
			continue
		}
		if len(hits) == 0 {
			t.Errorf("%s: zero results for a query with thousands of hits — parser is probably broken", name)
			continue
		}
		if strings.TrimSpace(hits[0].Title) == "" {
			t.Errorf("%s: first result has no title — field mapping is wrong", name)
		}
		t.Logf("%s ok: %d hits, first = %.70s", name, len(hits), hits[0].Title)
	}
}

func isTransientNetErr(err error) bool {
	e := strings.ToLower(err.Error())
	for _, s := range []string{"connection refused", "connection reset", "timeout",
		"no such host", "i/o timeout", "eof", "http 429", "http 502", "http 503", "http 504"} {
		if strings.Contains(e, s) {
			return true
		}
	}
	return false
}

// Live check for the access-oriented backends. Same opt-in as above.
func TestLiveAccessBackends(t *testing.T) {
	if os.Getenv("GORILLA_LIVE_SEARCH") != "1" {
		t.Skip("set GORILLA_LIVE_SEARCH=1 to run (needs network)")
	}
	tool := &webSearchTool{client: newSafeClient(30 * time.Second)}
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	cases := []struct {
		name  string
		query string
		fn    func(context.Context, string, int) ([]searchHit, error)
		// The whole point of these backends: a free legal copy must be surfaced.
		wantFree bool
	}{
		{"Unpaywall", "10.1038/nature12373", tool.searchUnpaywall, true},
		{"DOAJ", "hallucination language model", tool.searchDOAJ, false},
		{"Gutenberg", "shakespeare", tool.searchGutendex, true},
		{"OpenLibrary", "shakespeare", tool.searchOpenLibrary, false},
		{"Wikipedia", "reverse proxy", tool.searchWikipedia, false},
	}
	for _, c := range cases {
		hits, err := c.fn(ctx, c.query, 3)
		if err != nil {
			// A refused connection or a 429 is the internet having a bad day;
			// a parse failure or an empty result set is our bug. Only the
			// second kind should fail the suite, or this test becomes noise
			// people learn to ignore.
			if isTransientNetErr(err) {
				t.Logf("%-12s SKIPPED (transient): %v", c.name, err)
				continue
			}
			t.Errorf("%s: %v", c.name, err)
			continue
		}
		if len(hits) == 0 {
			t.Errorf("%s: zero results — parser likely broken", c.name)
			continue
		}
		if strings.TrimSpace(hits[0].Title) == "" {
			t.Errorf("%s: first result has no title", c.name)
		}
		if c.wantFree && hits[0].FreePDF == "" {
			t.Errorf("%s: expected a free legal full-text link and got none", c.name)
		}
		t.Logf("%-12s %d hits | %.48s | free=%.52s", c.name, len(hits), hits[0].Title, hits[0].FreePDF)
	}
}

func TestOpenAlexSurfacesOpenAccessLink(t *testing.T) {
	if os.Getenv("GORILLA_LIVE_SEARCH") != "1" {
		t.Skip("set GORILLA_LIVE_SEARCH=1 to run (needs network)")
	}
	tool := &webSearchTool{client: newSafeClient(30 * time.Second)}
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	hits, err := tool.searchOpenAlex(ctx, "LLM hallucination", 10)
	if err != nil {
		t.Fatal(err)
	}
	free := 0
	for _, h := range hits {
		if h.FreePDF != "" {
			free++
		}
	}
	if free == 0 {
		t.Error("no result carried a free-access link; the best_oa_location field is being discarded again")
	}
	t.Logf("%d of %d results have a free legal copy", free, len(hits))
}
