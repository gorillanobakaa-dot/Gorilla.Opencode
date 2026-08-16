// GORILLA OVERRIDE: guards the ChatGPT row in the provider portal.
//
// TestEveryPortalRowIsHandled next door proves no row is UNhandled. It cannot
// prove a row is PRESENT — deleting the row entirely leaves that test passing,
// which is precisely how a login screen loses an entry without anyone noticing.
// These assert the row exists, is reachable, and renders on screen.
package cmd

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/opencode-ai/opencode/internal/llm/models"
	"github.com/opencode-ai/opencode/internal/tui/startup"
)

func findRow(t *testing.T, id string) startup.ProviderRow {
	t.Helper()
	rows, _ := providerPortalRows()
	for _, r := range rows {
		if r.ID == id {
			return r
		}
	}
	t.Fatalf("the portal presents no %q row; rows were %v", id, rowIDs(rows))
	return startup.ProviderRow{}
}

func rowIDs(rows []startup.ProviderRow) []string {
	out := make([]string, 0, len(rows))
	for _, r := range rows {
		out = append(out, r.ID)
	}
	return out
}

func TestChatGPTRowIsOffered(t *testing.T) {
	loadCfg(t)
	r := findRow(t, "chatgpt")

	// Free is what puts the "free" tag on the row. This provider's entire reason
	// for existing is that a free ChatGPT account can use it without a card;
	// losing the tag hides that from the only people it matters to.
	if !r.Free {
		t.Error("the ChatGPT row is not tagged free — a free plan works, and that is the point of it")
	}
	if r.NeedsInput {
		t.Error("the ChatGPT row asks for typed input; it is an OAuth sign-in, there is no key to paste")
	}
	// The row must say what a free plan actually hits. "Quota exceeded" phrasing
	// reads as a bill to someone with no card.
	if !strings.Contains(strings.ToLower(r.Warning), "cooldown") {
		t.Errorf("the row does not explain that a free plan hits a cooldown rather than a charge: %q", r.Warning)
	}
	// The two 5.6 models are deliberately withheld (code_mode_only). If someone
	// later registers them without implementing that tool shape, this row's
	// promise becomes false — so the omission is stated on screen, not only in
	// a source comment.
	if !strings.Contains(r.Warning, "5.6") {
		t.Errorf("the row does not say GPT-5.6 is unavailable, so its absence looks like a bug: %q", r.Warning)
	}
}

// TestChatGPTModelsAreRegisteredAndRoutable is the "row that offers nothing"
// guard: signing in is worthless if the models it advertises are not in the
// catalogue or do not route to a provider that can serve them.
func TestChatGPTModelsAreRegisteredAndRoutable(t *testing.T) {
	for _, id := range []models.ModelID{models.ChatGPT55, models.ChatGPT54Mini} {
		m, ok := models.SupportedModels[id]
		if !ok {
			t.Fatalf("%s is not in SupportedModels, so the model picker cannot offer it", id)
		}
		if m.Provider != models.ProviderChatGPT {
			t.Errorf("%s routes to provider %q, not %q", id, m.Provider, models.ProviderChatGPT)
		}
		if m.APIModel == "" {
			t.Errorf("%s has no APIModel, so the backend would be sent an empty model name", id)
		}
		// Cost must stay zero: these are served by the user's plan, and showing
		// API prices next to them would tell someone on a free plan they are
		// being charged.
		if m.CostPer1MIn != 0 || m.CostPer1MOut != 0 {
			t.Errorf("%s carries a per-token cost; this path bills nothing per token", id)
		}
	}
}

// TestChatGPTRowRenders drives the real portal component headlessly and asserts
// the row is on the screen and inside the frame. A row present in the slice but
// clipped off the display is the same as no row to the person looking at it.
func TestChatGPTRowRenders(t *testing.T) {
	loadCfg(t)
	rows, _ := providerPortalRows()

	const width = 80
	m := startup.NewProviderModel(rows, true)
	mm, _ := m.Update(tea.WindowSizeMsg{Width: width, Height: 40})
	view := mm.(interface{ View() string }).View()

	if !strings.Contains(view, "ChatGPT") {
		t.Fatalf("the rendered portal does not mention ChatGPT:\n%s", view)
	}
	for i, line := range strings.Split(view, "\n") {
		if lw := lipgloss.Width(line); lw > width {
			t.Fatalf("line %d is %d cells wide at width %d: %q", i, lw, width, line)
		}
	}
}
