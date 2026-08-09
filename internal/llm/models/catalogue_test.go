package models

import (
	"strings"
	"testing"
)

func TestIsBatchVariant(t *testing.T) {
	for id, want := range map[string]bool{
		"anthropic/claude-opus-5:batch": true,
		"google/gemini-3.6-flash:batch": true,
		"anthropic/claude-opus-5":       false,
		"openai/gpt-oss-20b:free":       false,
		"vendor/model-batchsize-32":     false, // "batch" inside a name is not a batch endpoint
	} {
		if got := IsBatchVariant(id); got != want {
			t.Errorf("IsBatchVariant(%q) = %v, want %v", id, got, want)
		}
	}
}

// Providers write descriptions in markdown, so links arrive intact and render
// as "[Poolside](<https://poolside.ai/>)" in a terminal — characters spent on a
// URL nobody can click, in the one field that has to earn its space.
func TestCleanCatalogueDescriptionStripsMarkdownLinks(t *testing.T) {
	in := "Laguna S 2.1 is the latest coding agent model from [Poolside](<https://poolside.ai/>) and is good"
	got := CleanCatalogueDescription(in, 262144, 0.40, 2.00)
	if strings.Contains(got, "](") || strings.Contains(got, "http") {
		t.Errorf("markdown survived: %q", got)
	}
	if !strings.Contains(got, "Poolside") {
		t.Errorf("the link TEXT must be kept, got %q", got)
	}
	if !strings.Contains(got, "262K ctx") {
		t.Errorf("context size missing: %q", got)
	}
}

// 90 characters cut almost every description mid-sentence, which defeats the
// point: this field is what stands between someone and a web search per model
// name, and a search per model is impossible on a single-digit-KB/s line.
func TestCleanCatalogueDescriptionKeepsUsefulLength(t *testing.T) {
	long := "Gemma 4 26B A4B IT is an instruction-tuned Mixture-of-Experts model from Google DeepMind " +
		"built for responsive agents and long-context work across many languages and tasks"
	got := CleanCatalogueDescription(long, 262144, 0, 0)
	if len(got) < 120 {
		t.Errorf("description cut too short to be useful (%d chars): %q", len(got), got)
	}
	if !strings.HasPrefix(got, "FREE — ") {
		t.Errorf("free models must be marked, got %q", got)
	}
}

func TestCleanCatalogueDescriptionHandlesEmpty(t *testing.T) {
	// Even with no prose, the price must still be stated — it is the field
	// someone is actually scanning for.
	got := CleanCatalogueDescription("", 0, 1.0, 2.0)
	if !strings.Contains(got, "per 1M") {
		t.Errorf("price must survive an empty description, got %q", got)
	}
}

// Every entry must lead with its price, because telling free from paid by the
// ABSENCE of a marker is not something anyone should have to know: 260 of 274
// entries were silent about cost.
func TestCleanCatalogueDescriptionAlwaysStatesPrice(t *testing.T) {
	free := CleanCatalogueDescription("Some model", 1000, 0, 0)
	if !strings.HasPrefix(free, "FREE — ") {
		t.Errorf("free model must say FREE, got %q", free)
	}
	paid := CleanCatalogueDescription("Some model", 1000, 0.4, 2.0)
	if !strings.Contains(paid, "$") || !strings.Contains(paid, "per 1M") {
		t.Errorf("paid model must show its price, got %q", paid)
	}
	// Cheap models need the cents, expensive ones do not.
	cheap := CleanCatalogueDescription("x", 1000, 0.04, 0.14)
	if !strings.Contains(cheap, "0.04") {
		t.Errorf("sub-dollar prices must keep two decimals, got %q", cheap)
	}
}
