package dialog

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/opencode-ai/opencode/internal/config"
	"github.com/opencode-ai/opencode/internal/llm/models"
)

// THE BUG, reported 2026-08-14 with two screenshots:
//
// The user switched the chat model to Claude Opus 4.6 (Thinking) and the
// /research screen carried on saying "priced at Gemini 2.0 Flash, the closest
// paid listing for this model" — with no indication anywhere that the helpers
// were not the model in the status bar.
//
//	"the research function is NOT MODEL AWARE... it is not aware i have changed
//	 the model and now i am on a opus"
//
// He was right, and the cause was a design decision of mine one message
// earlier: the helper/chat comparison was gated on PROVIDER, because I reasoned
// that same-provider means same bill and therefore nothing to report. That
// optimised for billing and ignored CAPABILITY. Opus and Flash are both
// Antigravity, so the gate stayed shut while the research ran on the weaker
// model — and nobody picks Opus to save money.
//
// A weaker helper is a worse answer. That has to be visible even when the bill
// is identical.

// registerAt puts a model in the catalogue under a chosen provider, with a
// local route so UpdateAgentModel accepts it.
func registerAt(t *testing.T, id models.ModelID, name string, provider models.ModelProvider, in, out float64) {
	t.Helper()
	if _, exists := models.SupportedModels[id]; exists {
		return
	}
	models.SupportedModels[id] = models.Model{
		ID: id, Name: name, Provider: provider,
		CostPer1MIn: in, CostPer1MOut: out,
		ContextWindow: 128000, DefaultMaxTokens: 4096,
	}
	models.RegisterLocalRouteForTestNamed(id, "http://127.0.0.1:1/v1", "k", "test-endpoint")
	t.Cleanup(func() {
		delete(models.SupportedModels, id)
		models.ClearLocalRouteForTest(id)
	})
}

// twoModels sets a strong chat model and a weak helper model ON THE SAME
// PROVIDER — the exact shape the old provider gate was blind to.
func twoModels(t *testing.T) (strong, weak models.ModelID) {
	t.Helper()
	if _, err := config.Load(t.TempDir(), false); err != nil {
		t.Fatalf("config.Load: %v", err)
	}
	strong = models.ModelID("local.test/strong-thinking")
	weak = models.ModelID("local.test/weak-flash")
	registerAt(t, strong, "Strong Thinking 4.6", models.ProviderLocal, 15, 75)
	registerAt(t, weak, "Weak Flash", models.ProviderLocal, 0.10, 0.40)

	if err := config.UpdateAgentModel(config.AgentCoder, strong); err != nil {
		t.Fatalf("set coder: %v", err)
	}
	if err := config.UpdateAgentModel(config.AgentTask, weak); err != nil {
		t.Fatalf("set task: %v", err)
	}
	// Same provider on purpose. If this ever stops being true the test is no
	// longer reproducing the reported bug.
	if models.SupportedModels[strong].Provider != models.SupportedModels[weak].Provider {
		t.Fatal("fixture broken: the two models must share a provider, or the old provider gate would have caught it")
	}
	return strong, weak
}

func costText(m ResearchDialogCmp) string {
	var b strings.Builder
	for _, l := range m.costLines() {
		b.WriteString(l.text)
		b.WriteString("\n")
	}
	return b.String()
}

func TestScreenNamesTheHelperModelWhenItDiffersFromTheChatModel(t *testing.T) {
	twoModels(t)
	text := costText(dialogAt("parallel", 4))

	if !strings.Contains(text, "HELPERS RUN ON: Weak Flash") {
		t.Errorf("the screen never names the helper model.\n%s", text)
	}
	if !strings.Contains(text, "Strong Thinking 4.6") {
		t.Errorf("the screen never names the model the user is actually chatting with.\n%s", text)
	}
	// The capability consequence, not just the name.
	if !strings.Contains(text, "as good as") {
		t.Errorf("the screen names both models but never says the answer is capped by the weaker one.\n%s", text)
	}
}

// NON-VACUOUS GUARD: the old provider-gated rule would have said nothing here,
// because both models share a provider. If this stops being true the fixture no
// longer reproduces the bug.
func TestTheOldProviderGateWouldHaveStayedSilent(t *testing.T) {
	strong, weak := twoModels(t)
	sp := models.SupportedModels[strong].Provider
	wp := models.SupportedModels[weak].Provider
	if sp != wp {
		t.Fatal("fixture drifted: models are on different providers, so the old gate would have fired")
	}
	if _, _, differentProvider := config.ResearchHelperModel(); differentProvider {
		t.Fatal("the provider gate fires on this fixture — it is not reproducing the silent case")
	}
	// ...and yet the screen must speak.
	if !strings.Contains(costText(dialogAt("parallel", 4)), "HELPERS RUN ON:") {
		t.Error("provider gate silent AND screen silent — this is the reported bug, unfixed")
	}
}

// Pressing m must actually change what runs, not just what is printed.
func TestPressingMPutsHelpersOnTheChatModel(t *testing.T) {
	strong, weak := twoModels(t)

	m := dialogAt("parallel", 4)
	if before, _, _ := config.ResearchModelChoice(); before.ID != weak {
		t.Fatalf("helper starts on %q, expected the weak model", before.ID)
	}

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'m'}})
	m = updated.(ResearchDialogCmp)

	after, chat, ok := config.ResearchModelChoice()
	if !ok {
		t.Fatal("ResearchModelChoice broke after the switch")
	}
	if after.ID != strong {
		t.Errorf("after pressing m the helper model is %q, want the chat model %q", after.ID, strong)
	}
	if after.ID != chat.ID {
		t.Errorf("helper %q still differs from chat %q", after.ID, chat.ID)
	}
	// And the screen must now agree.
	text := costText(m)
	if !strings.Contains(text, "the model you are chatting with") {
		t.Errorf("screen does not confirm the helpers moved.\n%s", text)
	}
	if strings.Contains(text, "Press «m» to run helpers on") {
		t.Error("the screen still offers a switch that has already happened")
	}
}

// The offer must be reachable: a key the user is never shown is not a route.
func TestTheHelperModelKeyIsAdvertised(t *testing.T) {
	var found bool
	for _, b := range (researchDialogKeyMap{}).ShortHelp() {
		if b.Help().Key == "m" {
			found = true
		}
	}
	if !found {
		t.Error(`no "m" in the key hints; the old screen told users to "set a research agent" ` +
			`with no way to do it, and this repeats that`)
	}
}
