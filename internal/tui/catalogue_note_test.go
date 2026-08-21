// Version: 1.0.0 · updated 26-08-21-14-10
package tui

import (
	"strings"
	"testing"

	"github.com/opencode-ai/opencode/internal/llm/models"
)

// GORILLA OVERRIDE (2026-08-21): a too-greedy non-chat filter must be visible.
//
// The filter that removes speech, image and embedding models from a fetched
// catalogue is a list of substrings, so it can throw away too much. From the
// usable count alone that is invisible: "OpenAI 5 usable" reads as a small
// catalogue, not as 73 models discarded by mistake. This release could not be
// tested against a paid provider, so the ratio is printed for whoever can.
func TestCatalogueNoteShowsWhatWasThrownAway(t *testing.T) {
	note := catalogueNote(models.CatalogueResult{
		Label: "OpenAI", Usable: 5, Skipped: 73,
	})
	if !strings.Contains(note, "5 usable") {
		t.Errorf("usable count missing: %q", note)
	}
	if !strings.Contains(note, "73 skipped") {
		t.Errorf("a filter that discarded 73 of 78 models is invisible: %q", note)
	}
}

// Nothing skipped, nothing said. A provider whose catalogue is all chat models
// should not carry a ", 0 skipped" that means nothing.
func TestCatalogueNoteStaysQuietWhenNothingWasSkipped(t *testing.T) {
	note := catalogueNote(models.CatalogueResult{Label: "DeepSeek", Usable: 2})
	if strings.Contains(note, "skipped") {
		t.Errorf("noise in the report: %q", note)
	}
	if note != "DeepSeek 2 usable" {
		t.Errorf("unexpected shape: %q", note)
	}
}

// A retirement is still named, and still the most useful thing in the line.
func TestCatalogueNoteNamesRetiredModels(t *testing.T) {
	note := catalogueNote(models.CatalogueResult{
		Label: "Groq", Usable: 8, Skipped: 3,
		Added:   []string{"groq.openai/gpt-oss-120b"},
		Removed: []string{"groq.llama-3.3-70b-versatile"},
	})
	for _, want := range []string{"8 usable", "(+1, -1)", "3 skipped", "retired: groq.llama-3.3-70b-versatile"} {
		if !strings.Contains(note, want) {
			t.Errorf("%q missing from %q", want, note)
		}
	}
}
