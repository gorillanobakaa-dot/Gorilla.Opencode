package config

import (
	"testing"

	"github.com/opencode-ai/opencode/internal/llm/models"
)

// THE SECOND BUG, reported 2026-08-14.
//
// The shadowing rule alone left background agents on ANOTHER PROVIDER. The
// Antigravity portal splits the agents at login by design — coder on Claude,
// background agents on Gemini Flash — so on a fresh install the helpers never
// equal the coder, and "not equal to prevCoder" was read as "the user chose
// this deliberately, leave it". Switch the coder to a local model and four
// agents stay wired to Antigravity, drawing quota from an account the user
// believes they walked away from. Nothing in the UI said so.
//
// These tests pin the distinction the fix rests on:
//
//	same provider, different model -> housekeeping, leave it
//	different provider             -> somebody else's bill, move it

// registerAt puts a model in the catalogue under a chosen provider. Unlike
// registerModel it does NOT register a local route, because these tests
// exercise helperMustMove/helperTargetFor, which never write config.
func registerAt(t *testing.T, id models.ModelID, provider models.ModelProvider) {
	t.Helper()
	if _, exists := models.SupportedModels[id]; exists {
		return
	}
	models.SupportedModels[id] = models.Model{
		ID: id, Provider: provider,
		ContextWindow: 8192, DefaultMaxTokens: 4096,
	}
	t.Cleanup(func() { delete(models.SupportedModels, id) })
}

func TestHelperOnAnotherProviderIsMovedEvenWhenDeliberatelySet(t *testing.T) {
	const prevCoder = models.ModelID("test.antigravity/claude-sonnet")
	const newCoder = models.ModelID("test.local/muse-glimmer")
	const helper = models.ModelID("test.antigravity/gemini-flash")

	registerAt(t, prevCoder, models.ProviderAntigravity)
	registerAt(t, newCoder, models.ProviderLocal)
	registerAt(t, helper, models.ProviderAntigravity)

	// The helper was never on prevCoder, so the shadowing rule alone leaves it.
	if helper == prevCoder {
		t.Fatal("fixture is wrong: the helper must NOT be shadowing, or this test proves nothing")
	}
	if !helperMustMove(helper, prevCoder, newCoder) {
		t.Error("a helper left on Antigravity while the coder moved to a local model was not moved; " +
			"it keeps drawing the Antigravity quota after the user switched away")
	}
}

func TestHelperOnTheSameProviderIsLeftAlone(t *testing.T) {
	const prevCoder = models.ModelID("test.antigravity/claude-sonnet")
	const newCoder = models.ModelID("test.antigravity/gemini-3.7")
	const helper = models.ModelID("test.antigravity/gemini-flash")

	registerAt(t, prevCoder, models.ProviderAntigravity)
	registerAt(t, newCoder, models.ProviderAntigravity)
	registerAt(t, helper, models.ProviderAntigravity)

	if helperMustMove(helper, prevCoder, newCoder) {
		t.Error("a cheap background model on the coder's OWN provider was overridden; " +
			"that is the same account and the split is deliberate — it roughly doubles a free tier")
	}
}

// The shadowing rule must survive the new one. This is the 2026-08-05 bug.
func TestShadowingStillMovesWithinOneProvider(t *testing.T) {
	const prevCoder = models.ModelID("test.local/yi-large")
	const newCoder = models.ModelID("test.local/llama-8b")

	registerAt(t, prevCoder, models.ProviderLocal)
	registerAt(t, newCoder, models.ProviderLocal)

	if !helperMustMove(prevCoder, prevCoder, newCoder) {
		t.Error("a helper that was shadowing the coder was not moved; it is now stranded on a model " +
			"the account may not run — the failure is invisible until a title or a summary fails")
	}
}

// A model the catalogue does not know must not be moved on a guess. Discarding
// a user's choice needs a demonstrated reason, not an absence of information.
func TestUnknownModelsAreLeftAlone(t *testing.T) {
	const newCoder = models.ModelID("test.local/llama-8b")
	registerAt(t, newCoder, models.ProviderLocal)

	if helperMustMove("test.unregistered/mystery", "test.local/other", newCoder) {
		t.Error("an unknown helper model was moved; we cannot show it bills elsewhere")
	}
	if helperMustMove("test.local/llama-8b-x", "test.local/other", "test.unregistered/mystery") {
		t.Error("moved a helper toward an unknown coder model; provider is undecidable")
	}
}

// NON-VACUOUS GUARD. Restore the OLD rule — move only when shadowing — and the
// cross-provider case must stop being detected. If this ever passes, the new
// rule has collapsed back into the old one and the tests above are decoration.
func TestTheOldShadowingOnlyRuleWouldMissTheCrossProviderCase(t *testing.T) {
	const prevCoder = models.ModelID("test.antigravity/claude-sonnet")
	const newCoder = models.ModelID("test.local/muse-glimmer")
	const helper = models.ModelID("test.antigravity/gemini-flash")

	registerAt(t, prevCoder, models.ProviderAntigravity)
	registerAt(t, newCoder, models.ProviderLocal)
	registerAt(t, helper, models.ProviderAntigravity)

	oldRule := func(helperModel, prev, _ models.ModelID) bool { return helperModel == prev }

	if oldRule(helper, prevCoder, newCoder) {
		t.Fatal("fixture no longer reproduces the bug: the old rule already caught this case")
	}
	if !helperMustMove(helper, prevCoder, newCoder) {
		t.Fatal("the new rule agrees with the old one here — the cross-provider fix is not in effect")
	}
}

// On a provider with a designated background model, helpers land on THAT, not
// on whatever the user just picked for the coder. This is what preserves the
// Antigravity two-pool split when the user switches away and later comes back.
func TestHelpersLandOnTheProvidersBackgroundModel(t *testing.T) {
	got := helperTargetFor(models.AGClaudeSonnet46)
	if got != models.AGGemini36Flash {
		t.Errorf("switching the coder to Antigravity Claude sends background agents to %q; "+
			"want %q, which draws the separate Gemini pool and leaves the Claude quota alone",
			got, models.AGGemini36Flash)
	}
}

// A provider with no curated entry gets the coder's own model. Deliberately
// dumb: picking a "cheap equivalent" out of the catalogue is the heuristic that
// once offered an embedding model as a chat model.
func TestProviderWithNoBackgroundModelGetsTheCoderModel(t *testing.T) {
	const newCoder = models.ModelID("test.local/muse-glimmer")
	registerAt(t, newCoder, models.ProviderLocal)

	if _, curated := backgroundModelByProvider[models.ProviderLocal]; curated {
		t.Skip("local now has a curated background model; this test no longer describes it")
	}
	if got := helperTargetFor(newCoder); got != newCoder {
		t.Errorf("helperTargetFor(%q) = %q, want the coder's own model", newCoder, got)
	}
}

// An unknown coder model must not send helpers somewhere invented.
func TestUnknownCoderModelTargetsItself(t *testing.T) {
	const unknown = models.ModelID("test.unregistered/mystery")
	if got := helperTargetFor(unknown); got != unknown {
		t.Errorf("helperTargetFor(%q) = %q, want the id back unchanged", unknown, got)
	}
}

// THE REPORTED SCENARIO, end to end, as a decision table.
//
// Antigravity login: coder=Claude, summarizer/task/title=Gemini Flash. User
// then picks a local model. Every background agent must leave Antigravity —
// the point is that NOTHING is still billing the account the user walked away
// from.
func TestNothingIsLeftOnTheOldProviderAfterSwitchingAway(t *testing.T) {
	const prevCoder = models.ModelID("test.antigravity/claude-sonnet")
	const newCoder = models.ModelID("test.local/muse-glimmer")
	const bg = models.ModelID("test.antigravity/gemini-flash")

	registerAt(t, prevCoder, models.ProviderAntigravity)
	registerAt(t, newCoder, models.ProviderLocal)
	registerAt(t, bg, models.ProviderAntigravity)

	target := helperTargetFor(newCoder)
	for _, name := range []string{"summarizer", "task", "title", "research"} {
		if !helperMustMove(bg, prevCoder, newCoder) {
			t.Fatalf("%s stays on Antigravity after the coder moved to a local model", name)
		}
		if p, _ := providerOf(target); p == models.ProviderAntigravity {
			t.Fatalf("%s would move to %q, which is still on Antigravity", name, target)
		}
	}
}
