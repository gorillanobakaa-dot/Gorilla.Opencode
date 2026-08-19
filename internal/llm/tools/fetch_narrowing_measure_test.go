package tools

import (
	"io"
	"net/http"
	"os"
	"testing"
)

// A MEASUREMENT, not a unit test. It touches the network, so it skips unless
// asked. It exists because the claim "narrowing saves tokens" should be
// checkable by anyone who doubts it, against a page of their choosing, rather
// than taken from a comment.
//
// Run:
//
//	MEASURE_URL=https://pkg.go.dev/net/http MEASURE_SEL=.Documentation-index \
//	  go test ./internal/llm/tools/ -run TestMeasureNarrowingOnARealPage -v
//
// Recorded 2026-08-19 on that exact page:
//
//	raw HTML       477,563 bytes
//	whole page     194,638 bytes  (~48,659 tokens)
//	narrowed         7,708 bytes  (~1,927 tokens)
//	saving                        96.0%
func TestMeasureNarrowingOnARealPage(t *testing.T) {
	url := os.Getenv("MEASURE_URL")
	sel := os.Getenv("MEASURE_SEL")
	if url == "" {
		t.Skip("set MEASURE_URL")
	}
	resp, err := http.Get(url)
	if err != nil {
		t.Skipf("network: %v", err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)

	full, err := convertHTMLToMarkdown(string(b))
	if err != nil {
		t.Fatal(err)
	}
	narrow, n, err := applySelector(string(b), sel, "text")
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("%s\n  raw HTML        %7d bytes\n  whole page md   %7d bytes  (~%d tokens)\n  selector %-12q %7d bytes  (~%d tokens, %d matched)\n  saving          %7d bytes (%.1f%%)",
		url, len(b), len(full), len(full)/4, sel, len(narrow), len(narrow)/4, n,
		len(full)-len(narrow), (1-float64(len(narrow))/float64(len(full)))*100)
}
