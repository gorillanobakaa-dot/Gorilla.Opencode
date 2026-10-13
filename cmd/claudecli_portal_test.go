// GORILLA OVERRIDE: guards the Claude sign-in row in the provider portal.
//
// Same reasoning as chatgpt_portal_test.go next door: TestEveryPortalRowIsHandled
// proves no row is UNhandled, but deleting a row entirely leaves it passing. A
// login screen quietly losing an entry is the failure these catch.
package cmd

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/opencode-ai/opencode/internal/llm/models"
	"github.com/opencode-ai/opencode/internal/tui/startup"
)

func TestClaudeCLIRowIsOffered(t *testing.T) {
	loadCfg(t)
	r := findRow(t, "claude-cli")

	// No typed input of any kind. This route holds no credential: the sign-in
	// is `claude login`, run by the user against Anthropic's own flow. A prompt
	// for a key here would mean somebody had rebuilt it the wrong way.
	if r.NeedsInput {
		t.Error("the Claude row asks for typed input; this route never takes a key or a token")
	}
	if r.Secret {
		t.Error("the Claude row is marked secret; it stores nothing to keep secret")
	}
	if r.Name == "" || !strings.Contains(r.Name, "Claude") {
		t.Errorf("the row does not say Claude: %q", r.Name)
	}
	// The two things a user has to know before choosing it.
	low := strings.ToLower(r.Warning + " " + r.What)
	if !strings.Contains(low, "claude login") {
		t.Errorf("nothing tells the user they must run `claude login` first: %q", r.Warning)
	}
	if !strings.Contains(low, "window") {
		t.Errorf("nothing says the limit is the plan's usage window rather than money: %q", r.Warning)
	}
}

// All four models must be routable and priced at nothing. A per-token price on
// this route would be money the user is not actually charged.
func TestClaudeCLIModelsAreRoutable(t *testing.T) {
	loadCfg(t)
	for _, id := range []models.ModelID{
		models.ClaudeCLIOpus, models.ClaudeCLIFable,
		models.ClaudeCLISonnet, models.ClaudeCLIHaiku,
	} {
		m, ok := models.SupportedModels[id]
		if !ok {
			t.Errorf("%s is not registered, so the picker cannot offer it", id)
			continue
		}
		if m.Provider != models.ProviderClaudeCLI {
			t.Errorf("%s routes to %q", id, m.Provider)
		}
		if m.CostPer1MIn != 0 || m.CostPer1MOut != 0 {
			t.Errorf("%s shows a per-token price for a route that bills none", id)
		}
	}
}

// The portal must actually draw, within the reference 80-column terminal. A row
// whose text overflows is the "no line wider than the terminal" trap in
// CLAUDE.md, which strands debris in the transcript on every render.
func TestClaudeCLIRowRendersWithin80Columns(t *testing.T) {
	loadCfg(t)
	rows, _ := providerPortalRows()
	m := startup.NewProviderModel(rows, false)
	m.Update(tea.WindowSizeMsg{Width: 80, Height: 30})

	view := m.View()
	if !strings.Contains(view, "Claude") {
		t.Fatalf("the rendered portal does not mention Claude:\n%s", view)
	}
	for i, line := range strings.Split(view, "\n") {
		if w := lipgloss.Width(line); w > 80 {
			t.Errorf("line %d is %d cells wide in an 80-column terminal: %q", i+1, w, line)
		}
	}
}
