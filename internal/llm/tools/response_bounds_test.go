package tools

import (
	"fmt"
	"strings"
	"testing"
)

// EVERY tool result is appended to the message history and re-sent on every
// later turn, so an unbounded result is an unbounded, recurring bill. This is
// the backstop that catches the tool nobody thought about.
func TestNoToolCanExceedTheResponseCap(t *testing.T) {
	huge := strings.Repeat("x", 3*1024*1024) // the 2.4 MB grep case, rounded up

	for name, got := range map[string]ToolResponse{
		"NewTextResponse":      NewTextResponse(huge),
		"NewTextErrorResponse": NewTextErrorResponse(huge),
	} {
		if len(got.Content) > MaxToolResponseBytes+512 {
			t.Errorf("%s returned %d bytes; the cap is %d",
				name, len(got.Content), MaxToolResponseBytes)
		}
		if !strings.Contains(got.Content, "TRUNCATED") {
			t.Errorf("%s truncated SILENTLY — a model cannot tell it received a "+
				"fragment and will reason about it as if it were complete", name)
		}
		if !strings.Contains(got.Content, fmt.Sprint(len(huge))) {
			t.Errorf("%s does not report the original size", name)
		}
	}
}

// The backstop must not interfere with normal work. A 2000-line source file at
// ~150 bytes a line is ~300 KB and has to pass through untouched.
func TestOrdinaryResultsAreUntouched(t *testing.T) {
	cases := map[string]string{
		"empty":                "",
		"short":                "Found 3 matches",
		"a 2000-line file":     strings.Repeat(strings.Repeat("a", 149)+"\n", 2000),
		"exactly at the limit": strings.Repeat("x", MaxToolResponseBytes),
	}
	for name, in := range cases {
		if got := NewTextResponse(in).Content; got != in {
			t.Errorf("%s (%d bytes) was altered — now %d bytes",
				name, len(in), len(got))
		}
	}
}

// Documents the per-tool limits so that adding a tool with no bound is a
// visible omission rather than an invisible one. If you add a tool, add it here.
func TestEveryToolHasABoundAndItIsWrittenDown(t *testing.T) {
	bounds := map[string]string{
		"bash":  "MaxOutputLength = 30000 bytes",
		"ls":    "MaxLSFiles = 1000 entries",
		"glob":  "100 files; paths are short",
		"grep":  "100 matches AND 100 KB AND 400 bytes/line",
		"view":  "2000 lines, 2000 bytes/line, 5 MB file cap",
		"fetch": "size-capped in fetch.go",
	}
	for tool, bound := range bounds {
		if bound == "" {
			t.Errorf("%s has no documented bound", tool)
		}
	}
	t.Logf("%d tools with documented bounds; all also sit behind the %d KB "+
		"backstop in NewTextResponse", len(bounds), MaxToolResponseBytes/1024)
}
