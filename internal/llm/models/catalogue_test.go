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

// A model reached through a different provider is still the same model, so a
// verdict already written for it applies. "deepseek-ai/deepseek-v4-pro" on NIM
// and "deepseek/deepseek-v4-pro" on OpenRouter are one thing.
func TestCuratedVerdictMatchesAcrossProviders(t *testing.T) {
	got, ok := CuratedVerdict("deepseek/deepseek-v4-pro")
	if !ok {
		t.Skip("that model is not in the curated metadata on this machine")
	}
	if got == "" {
		t.Error("matched but returned an empty verdict")
	}
	if _, ok := CuratedVerdict("vendor/definitely-not-a-real-model-xyz"); ok {
		t.Error("an unknown model must not match anything")
	}
}

// PreferFullerDetail guards the one regression a refresh could smuggle in:
// OpenRouter's list API truncates descriptions, the bundled catalogue carries
// the full page text, and a refresh must not trade the second for the first.
func TestPreferFullerDetailKeepsBundledFullText(t *testing.T) {
	fresh := "Vendor's own description (their claim, not our finding): Qwen3-VL is optimized for reasoning in STEM and math...."
	bundled := "Vendor's own description (their claim, not our finding): Qwen3-VL is optimized for reasoning in STEM and math. The series emphasizes robust perception and long-form visual comprehension."
	if got := PreferFullerDetail(fresh, bundled); got != bundled {
		t.Fatalf("a truncated refresh copy must not replace the full bundled text; got %q", got)
	}
}

func TestPreferFullerDetailTakesGenuinelyNewText(t *testing.T) {
	fresh := "Vendor's own description (their claim, not our finding): A complete rewrite the vendor shipped yesterday...."
	bundled := "Vendor's own description (their claim, not our finding): The old text, which is longer than the new one but no longer what the vendor says about this model at all."
	if got := PreferFullerDetail(fresh, bundled); got != fresh {
		t.Fatalf("a rewritten description must win even if shorter; got %q", got)
	}
}

func TestPreferFullerDetailHandlesEmpty(t *testing.T) {
	if got := PreferFullerDetail("", "kept"); got != "kept" {
		t.Fatalf("empty fresh must keep existing, got %q", got)
	}
	if got := PreferFullerDetail("fresh", ""); got != "fresh" {
		t.Fatalf("empty existing must take fresh, got %q", got)
	}
}

// The apology for upstream-truncated descriptions lives in the DATA, and it
// must only ever blame OpenRouter for OpenRouter's cut — never for our own
// 2400-character cap.
func TestDetailForPickerBlamesUpstreamForItsTruncation(t *testing.T) {
	d := DetailForPicker("mocklab/cut-short", "North Mini Code is an agentic coding model, optimized...")
	if !strings.Contains(d, "sorry lads — not our fault") {
		t.Fatalf("an upstream-truncated description must carry the apology; got %q", d)
	}
}

func TestDetailForPickerDoesNotBlameUpstreamForCompleteText(t *testing.T) {
	d := DetailForPicker("mocklab/whole", "A complete description that ends like a sentence should.")
	if strings.Contains(d, "sorry lads") {
		t.Fatalf("a complete description must not carry the apology; got %q", d)
	}
}

func TestDetailForPickerDoesNotBlameUpstreamForOurOwnCap(t *testing.T) {
	long := strings.Repeat("many words about a model with no trailing dots whatsoever ", 60) + "the end"
	d := DetailForPicker("mocklab/long", long)
	if !strings.Contains(d, "...") {
		t.Fatalf("text over the cap should be visibly cut; got tail %q", d[len(d)-80:])
	}
	if strings.Contains(d, "sorry lads") {
		t.Fatal("our own cap must never be blamed on OpenRouter")
	}
}

// The trap the shared constant exists to prevent: a refresh hands over the
// truncated API text WITH the apology already stamped on it. The apology ends
// in ")" so a naive prefix check never matches — and the truncated copy would
// replace the full bundled text, false apology and all.
func TestPreferFullerDetailStripsApologyBeforeComparing(t *testing.T) {
	fresh := DetailForPicker("mocklab/x", "The vendor text, cut by the API right here, optimized...")
	if !strings.Contains(fresh, "sorry lads") {
		t.Fatal("test setup: fresh refresh text should carry the apology")
	}
	bundled := DetailForPicker("mocklab/x", "The vendor text, cut by the API right here, optimized for the full story that only the web page carries, at length.")
	if got := PreferFullerDetail(fresh, bundled); got != bundled {
		t.Fatalf("apology-stamped truncated refresh must not beat the full bundled text; got %q", got)
	}
}
