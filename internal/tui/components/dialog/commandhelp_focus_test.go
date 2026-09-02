package dialog

import (
	"strings"
	"testing"
)

// FocusCommand backs `/port help` and `/review help`. If it renders the list
// without the explanation, those commands answer "how do I use this?" with a
// menu, which is the problem they were added to solve.
func TestFocusCommandShowsThatCommandsExplanation(t *testing.T) {
	for _, tc := range []struct {
		name  string
		wants []string
	}{
		{"port", []string{"/port", "forward-port", "backport"}},
		{"review", []string{"/review", "analysers"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m := newHelpAt(t, 100, 24)
			m.FocusCommand(tc.name)
			view := m.View()
			for _, want := range tc.wants {
				if !strings.Contains(view, want) {
					t.Errorf("focused on /%s, view does not mention %q", tc.name, want)
				}
			}
			// The selected row must BE that command, because the explanation
			// block underneath renders from the selection.
			sel := m.rows[m.selectedIdx]
			if sel.cmd == nil || sel.cmd.Name != tc.name {
				got := "a heading"
				if sel.cmd != nil {
					got = "/" + sel.cmd.Name
				}
				t.Errorf("selection is %s, want /%s", got, tc.name)
			}
		})
	}
}

// An unknown name must fall back to the whole list. A blank screen reading
// "nothing matches" is a worse answer than showing everything.
func TestFocusCommandFallsBackWhenNothingMatches(t *testing.T) {
	m := newHelpAt(t, 100, 24)
	m.FocusCommand("thiscommanddoesnotexist")
	if !m.hasAnyCommand() {
		t.Fatal("focusing an unknown command left no commands visible")
	}
	if m.filter != "" {
		t.Errorf("filter = %q; it should have been cleared on no match", m.filter)
	}
	if sel := m.rows[m.selectedIdx]; sel.cmd == nil {
		t.Error("selection landed on a heading rather than a command")
	}
}
