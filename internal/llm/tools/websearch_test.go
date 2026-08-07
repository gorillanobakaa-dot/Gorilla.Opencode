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
