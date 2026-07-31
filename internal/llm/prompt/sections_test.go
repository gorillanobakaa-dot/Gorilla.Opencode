package prompt

import (
	"strings"
	"testing"

	"github.com/opencode-ai/opencode/internal/config"
	"github.com/opencode-ai/opencode/internal/llm/models"
)

// The guard that matters most: splitting into sections and rejoining them must
// not lose or reorder a single byte of the shipped prompt. A silent drop here
// would quietly change the agent's behaviour with nothing to point at.
func TestAllSectionsEnabledRoundTripsToTheOriginal(t *testing.T) {
	factory := Factory(PromptCoder)
	secs := ParseSections(factory)
	if len(secs) < 2 {
		t.Fatalf("expected several sections in the shipped prompt, got %d", len(secs))
	}

	var bodies []string
	for _, s := range secs {
		bodies = append(bodies, s.Body)
	}
	rejoined := strings.Join(bodies, "\n\n")

	if rejoined != factory {
		t.Errorf("round-trip changed the prompt.\nlen(original)=%d len(rejoined)=%d\n--- original ---\n%s\n--- rejoined ---\n%s",
			len(factory), len(rejoined), factory, rejoined)
	}
}

func TestParseSectionsShapeOfTheShippedPrompt(t *testing.T) {
	secs := ParseSections(Factory(PromptCoder))

	// First section is the unheaded preamble carrying the agent's identity.
	if secs[0].ID != SectionID("preamble") {
		t.Errorf("first section id = %q, want %q", secs[0].ID, SectionID("preamble"))
	}
	if secs[0].Header != "" {
		t.Errorf("preamble should have no header, got %q", secs[0].Header)
	}

	want := []string{"method", "build discipline", "verification", "honesty", "change reporting", "scope", "delegation", "memory", "tools", "output", "conduct"}
	var got []string
	for _, s := range secs[1:] {
		got = append(got, s.Header)
	}
	if len(got) != len(want) {
		t.Fatalf("headers = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("header %d = %q, want %q (order matters — sections assemble in this order)", i, got[i], want[i])
		}
	}

	for _, s := range secs {
		if s.Tokens <= 0 {
			t.Errorf("section %q has token estimate %d", s.ID, s.Tokens)
		}
		if strings.TrimSpace(s.Body) == "" {
			t.Errorf("section %q has an empty body", s.ID)
		}
	}
}

// A prompt with no "# " headers must still yield one toggleable section, so a
// user who rewrites it as flat prose is not left with no control at all.
func TestParseSectionsHandlesHeaderlessText(t *testing.T) {
	secs := ParseSections("just some flat prose with no headings at all")
	if len(secs) != 1 {
		t.Fatalf("got %d sections, want 1", len(secs))
	}
	if secs[0].ID != SectionID("preamble") {
		t.Errorf("id = %q, want the preamble id", secs[0].ID)
	}
}

func TestParseSectionsEmptyInput(t *testing.T) {
	if got := ParseSections("   \n\t\n "); got != nil {
		t.Errorf("blank input produced %d sections, want none", len(got))
	}
}

// Disabling one section must remove exactly that section and nothing else.
func TestDisablingASectionRemovesOnlyThatSection(t *testing.T) {
	dir := t.TempDir()
	if _, err := config.Load(dir, false); err != nil {
		t.Fatalf("config.Load: %v", err)
	}
	RegisterSectionComponents()
	invalidateSections()

	full := BaseCoderPrompt(models.ProviderLocal)
	if !strings.Contains(full, "# output") {
		t.Fatalf("baseline prompt has no '# output' section:\n%s", full)
	}

	target := SectionID("output")
	if config.LoadoutEnabled(target) {
		config.ToggleLoadout(target)
	}
	t.Cleanup(func() {
		if !config.LoadoutEnabled(target) {
			config.ToggleLoadout(target)
		}
	})

	trimmed := BaseCoderPrompt(models.ProviderLocal)
	if strings.Contains(trimmed, "# output") {
		t.Error("'# output' still present after disabling its section")
	}
	// Everything else must survive.
	for _, keep := range []string{"# method", "# honesty", "# conduct", "# verification"} {
		if !strings.Contains(trimmed, keep) {
			t.Errorf("disabling '# output' also removed %q", keep)
		}
	}
	if len(trimmed) >= len(full) {
		t.Errorf("prompt did not shrink: %d -> %d bytes", len(full), len(trimmed))
	}
}

// Turning EVERY section off is a lobotomy, not a configuration. An empty system
// prompt does not error — it silently produces a much worse agent — so fall back
// to the whole factory prompt rather than sending nothing.
func TestAllSectionsOffFallsBackToFactory(t *testing.T) {
	dir := t.TempDir()
	if _, err := config.Load(dir, false); err != nil {
		t.Fatalf("config.Load: %v", err)
	}
	RegisterSectionComponents()
	invalidateSections()

	secs := CoderSections()
	var toggled []string
	for _, s := range secs {
		if config.LoadoutEnabled(s.ID) {
			config.ToggleLoadout(s.ID)
			toggled = append(toggled, s.ID)
		}
	}
	t.Cleanup(func() {
		for _, id := range toggled {
			if !config.LoadoutEnabled(id) {
				config.ToggleLoadout(id)
			}
		}
	})

	got := BaseCoderPrompt(models.ProviderLocal)
	if strings.TrimSpace(got) == "" {
		t.Fatal("every section off produced an EMPTY system prompt — the agent would silently degrade with no error")
	}
	if got != Factory(PromptCoder) {
		t.Errorf("all-off should fall back to the full factory prompt; got %d bytes vs factory %d",
			len(got), len(Factory(PromptCoder)))
	}
}

// Every shipped section needs a hand-written tradeoff line, or the /context menu
// renders a row that does not say what turning it off costs.
func TestEveryShippedSectionHasATradeoff(t *testing.T) {
	for _, s := range ParseSections(Factory(PromptCoder)) {
		tradeoff, ok := SectionTradeoff[s.ID]
		if !ok || strings.TrimSpace(tradeoff) == "" {
			t.Errorf("section %q (%q) has no tradeoff text", s.ID, s.Header)
		}
	}
}

// honesty and preamble carry the anti-fabrication rules and the agent's
// identity. Losing them degrades trustworthiness rather than style, so the menu
// must warn.
func TestHonestyAndPreambleAreCritical(t *testing.T) {
	for _, id := range []string{SectionID("honesty"), SectionID("preamble")} {
		if !criticalSections[id] {
			t.Errorf("%q is not marked critical", id)
		}
	}
	if criticalSections[SectionID("output")] {
		t.Error("'output' is marked critical; it is a style section, and over-warning trains users to ignore warnings")
	}
}

func TestRegisterSectionComponentsIsIdempotent(t *testing.T) {
	dir := t.TempDir()
	if _, err := config.Load(dir, false); err != nil {
		t.Fatalf("config.Load: %v", err)
	}
	RegisterSectionComponents()
	n := len(config.LoadoutComponents)
	RegisterSectionComponents()
	if len(config.LoadoutComponents) != n {
		t.Errorf("second call added %d duplicate rows", len(config.LoadoutComponents)-n)
	}
}
