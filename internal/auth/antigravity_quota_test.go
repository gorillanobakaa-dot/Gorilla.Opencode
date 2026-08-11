package auth

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

// The struct tags must match the live wire shape, or the quota view is blank.
// This is the exact retrieveUserQuotaSummary body captured from agy 1.1.10.
const capturedQuotaJSON = `{
  "groups": [
    {
      "buckets": [
        {"bucketId":"gemini-weekly","displayName":"Weekly Limit","window":"weekly",
         "resetTime":"2026-08-10T14:34:46Z","remainingFraction":0.3094944}
      ],
      "displayName":"Gemini Models",
      "description":"Models within this group: Gemini Flash, Gemini Pro"
    },
    {
      "buckets": [
        {"bucketId":"3p-weekly","displayName":"Weekly Limit","window":"weekly",
         "resetTime":"2026-08-10T18:14:14Z","remainingFraction":1}
      ],
      "displayName":"Claude and GPT models",
      "description":"Models within this group: Claude Opus, Claude Sonnet, GPT-OSS"
    }
  ],
  "description":"Within each group, models share a weekly limit."
}`

func TestQuotaShapeAndFormatting(t *testing.T) {
	var q QuotaSummary
	if err := json.Unmarshal([]byte(capturedQuotaJSON), &q); err != nil {
		t.Fatalf("wire shape no longer unmarshals: %v", err)
	}
	if len(q.Groups) != 2 {
		t.Fatalf("expected 2 groups, got %d", len(q.Groups))
	}
	// remainingFraction must round to a percentage, resetTime to whole days from
	// a FIXED now so the assertion is deterministic.
	now := time.Date(2026, 8, 3, 14, 34, 46, 0, time.UTC) // exactly 7 days before Gemini reset
	line := FormatQuotaLine(&q, now)
	for _, want := range []string{
		"Gemini Models: 31%",
		"Claude and GPT models: 100%",
		"resets in 7d",
	} {
		if !strings.Contains(line, want) {
			t.Errorf("quota line missing %q\ngot: %s", want, line)
		}
	}
}

func TestQuotaEmptyGroups(t *testing.T) {
	if got := FormatQuotaLine(&QuotaSummary{}, time.Now()); !strings.Contains(got, "no quota groups") {
		t.Fatalf("empty summary should say so, got %q", got)
	}
}
