package dialog

// GORILLA OVERRIDE: regression tests for the v0.1.87 real-user report:
// "Which is off/on, is it x'ed or unx'ed and greyed out, regardless the
// description still shows off". Two guarantees: state is a WORD (ON/OFF badge)
// that visibly flips when space is pressed, and the consequence text never
// opens with the bare "off:" that made enabled rows read as disabled.

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/opencode-ai/opencode/internal/config"
)

func renderLoadout(t *testing.T) (*loadoutDialogCmp, string) {
	t.Helper()
	d := NewLoadoutDialogCmp().(*loadoutDialogCmp)
	d.Update(tea.WindowSizeMsg{Width: 140, Height: 60})
	return d, d.View()
}

func TestLoadoutStateIsAWordNotACheckbox(t *testing.T) {
	if _, err := config.Load(t.TempDir(), false); err != nil {
		t.Fatalf("config.Load: %v", err)
	}
	_, view := renderLoadout(t)

	// The badges. Sparse ships default-off, everything critical ships on, so a
	// default render must show both words.
	if !strings.Contains(view, " ON ") {
		t.Errorf("no ON badge anywhere in the default view")
	}
	if !strings.Contains(view, " OFF ") {
		t.Errorf("no OFF badge anywhere in the default view (sparse defaults off)")
	}

	// The old idioms must be gone: checkboxes said nothing to the reported
	// user, and "  off:" / "OFF — " opened the consequence text with a state
	// word that contradicted the actual state.
	for _, relic := range []string{"[x]", "[ ]", "  off:", "OFF — "} {
		if strings.Contains(view, relic) {
			t.Errorf("old state idiom %q still rendered; state must be carried by the ON/OFF badge alone", relic)
		}
	}

	// The new grammar, both directions.
	if !strings.Contains(view, "turn off and: ") {
		t.Errorf("enabled rows do not introduce their consequence with %q", "turn off and: ")
	}
	if !strings.Contains(view, "while off: ") {
		t.Errorf("disabled rows do not introduce their consequence with %q", "while off: ")
	}
}

// Pressing space must visibly change the text of the row — the exact
// expectation the user reported ("expecting the text to change").
func TestLoadoutToggleVisiblyFlipsTheBadge(t *testing.T) {
	if _, err := config.Load(t.TempDir(), false); err != nil {
		t.Fatalf("config.Load: %v", err)
	}
	d, before := renderLoadout(t)

	rows := sortedLoadout()
	target := rows[0] // first feature row; ships enabled (critical tools all do)
	if !config.LoadoutEnabled(target.ID) {
		t.Fatalf("test premise broken: %s ships disabled", target.ID)
	}
	offBefore := strings.Count(before, " OFF ")

	d.selectedIdx = numDials
	d.Update(tea.KeyMsg{Type: tea.KeySpace})
	after := d.View()
	defer config.ToggleLoadout(target.ID) // restore for the rest of the package

	if got := strings.Count(after, " OFF "); got != offBefore+1 {
		t.Errorf("toggling row 0 changed OFF-badge count %d -> %d; want exactly one more — the text the user sees did not flip", offBefore, got)
	}
}
