package tools

import (
	"context"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"
)

func TestLiveTextRankOnRealDocument(t *testing.T) {
	if os.Getenv("GORILLA_LIVE_SEARCH") != "1" {
		t.Skip("needs network")
	}
	// A genuinely long public-domain document.
	req, _ := http.NewRequestWithContext(context.Background(), "GET",
		"https://www.gutenberg.org/ebooks/1513.txt.utf-8", nil)
	resp, err := (&http.Client{Timeout: 60 * time.Second}).Do(req)
	if err != nil {
		t.Skip("unreachable:", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 400_000))
	text := string(body)

	sum := Summarise(text, 12)
	if !sum.Summarised {
		t.Fatalf("refused to summarise a %d-char document", len(text))
	}
	if sum.SummaryChars >= sum.OriginalChars {
		t.Fatal("summary is not smaller than the original")
	}
	if !strings.Contains(sum.Header(), "EXTRACTIVE SUMMARY") {
		t.Error("summary does not declare itself extractive")
	}
	if !strings.Contains(sum.Header(), "centrality") {
		t.Error("summary does not warn that centrality is not importance")
	}
	t.Logf("%d chars (~%d tok) -> %d chars (~%d tok) | %d of %d sentences | %.1f%%",
		sum.OriginalChars, sum.OriginalChars/4, sum.SummaryChars, sum.SummaryChars/4,
		sum.SentencesKept, sum.SentencesTotal, sum.CompressionRatio*100)

	// Rule 1: must refuse on short input.
	if Summarise("Short abstract. Two sentences only.", 5).Summarised {
		t.Error("summarised a tiny input; the refusal guard is not working")
	}
}
