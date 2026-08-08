package prompt

import (
	"strings"
	"testing"

	"github.com/opencode-ai/opencode/internal/config"
	"github.com/opencode-ai/opencode/internal/llm/models"
)

// GORILLA OVERRIDE: these tests guard the link between "the tool is loaded" and
// "the prompt says the tool exists". Before the [[needs ...]] markers those two
// facts were independent, so low-bandwidth mode - one keypress in /context -
// produced a prompt telling the model "never say you cannot reach a page" while
// tool.fetch was switched off. That is an instruction to fabricate.
//
// Every test here asserts BOTH directions. Asserting only that the line vanishes
// would pass against code that deleted the line unconditionally.

// withLoadout switches components off for one test and restores them after.
// Only the ones actually toggled are restored, so a component that was already
// off is left off rather than being switched on by the cleanup.
func withLoadout(t *testing.T, off ...string) {
	t.Helper()
	var toggled []string
	for _, id := range off {
		if config.LoadoutEnabled(id) {
			config.ToggleLoadout(id)
			toggled = append(toggled, id)
		}
		if config.LoadoutEnabled(id) {
			t.Fatalf("could not switch %s off", id)
		}
	}
	t.Cleanup(func() {
		for _, id := range toggled {
			config.ToggleLoadout(id)
		}
	})
}

func TestToolLinesAppearWhenTheToolIsEnabled(t *testing.T) {
	got := BaseCoderPrompt(models.ProviderLocal)
	for _, want := range []string{"web_fetch", "web_search", "sub-agent"} {
		if !strings.Contains(got, want) {
			t.Errorf("with everything enabled the prompt should mention %q", want)
		}
	}
	if strings.Contains(got, "[[needs") {
		t.Error("the [[needs ...]] marker must never reach the model")
	}
}

func TestDisablingFetchRemovesTheWebFetchClaim(t *testing.T) {
	withLoadout(t, "tool.fetch")

	got := BaseCoderPrompt(models.ProviderLocal)
	if strings.Contains(got, "web_fetch") {
		t.Errorf("tool.fetch is off, so the prompt must not claim web_fetch exists:\n%s", got)
	}
	if strings.Contains(got, "never say you cannot reach a page") {
		t.Error("the fabrication instruction must go with the tool it depends on")
	}
	// The inverse half: unrelated lines survive, so this is gating and not a
	// wholesale drop of the section.
	if !strings.Contains(got, "batch: independent calls") {
		t.Error("ungated lines in the same section must survive")
	}
	if !strings.Contains(got, "web_search") {
		t.Error("tool.websearch is still on, so its lines must remain")
	}
}

func TestDisablingWebSearchRemovesAllItsLines(t *testing.T) {
	withLoadout(t, "tool.websearch")

	got := BaseCoderPrompt(models.ProviderLocal)
	for _, gone := range []string{"web_search", "SearXNG", "remembered citations"} {
		if strings.Contains(got, gone) {
			t.Errorf("tool.websearch is off, so %q must not appear", gone)
		}
	}
	if !strings.Contains(got, "web_fetch") {
		t.Error("tool.fetch is still on, so its line must remain")
	}
}

// Low-bandwidth mode is the realistic way into this bug: a single keypress that
// turns off five tools at once.
func TestLowBandwidthPromptClaimsNoToolItRemoved(t *testing.T) {
	withLoadout(t, "tool.fetch", "tool.websearch", "tool.agent")

	got := BaseCoderPrompt(models.ProviderLocal)
	for _, gone := range []string{"web_fetch", "web_search", "sub-agent", "delegate independent subtasks"} {
		if strings.Contains(got, gone) {
			t.Errorf("%q must not appear when its tool is switched off", gone)
		}
	}
	// "# delegation" is nothing but tool.agent lines, so gating empties it and
	// the header must go too - a bare "# delegation" implies content was lost.
	if strings.Contains(got, "# delegation") {
		t.Error("a section reduced to a bare header must be dropped entirely")
	}
	// Sections that never mentioned a tool are untouched.
	for _, kept := range []string{"# honesty", "# method", "audit before reporting"} {
		if !strings.Contains(got, kept) {
			t.Errorf("%q is unrelated to tools and must survive", kept)
		}
	}
}

func TestApplyToolGatesLeavesUnmarkedTextAlone(t *testing.T) {
	in := "# tools\n- one\n- two"
	if got := applyToolGates(in); got != in {
		t.Errorf("text without markers must be returned unchanged, got %q", got)
	}
}
