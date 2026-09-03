package dialog

// GORILLA OVERRIDE: the /context feature rows are sorted alphabetically at
// display time ("the entries in /context have to be sorted alphabetically —
// right now is a mess", 2026-08-17). These tests hold the two invariants that
// make that safe: the rendered order really is alphabetical, and toggling row N
// flips the component DISPLAYED at row N — not the one that happens to sit at
// registry index N, which after sorting is a different component.

import (
	"sort"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/opencode-ai/opencode/internal/config"
)

func TestLoadoutRowsAreAlphabetical(t *testing.T) {
	if _, err := config.Load(t.TempDir(), false); err != nil {
		t.Fatalf("config.Load: %v", err)
	}

	rows := sortedLoadout()
	if len(rows) < 3 {
		t.Fatalf("expected a populated registry, got %d rows", len(rows))
	}
	if !sort.SliceIsSorted(rows, func(i, j int) bool {
		return strings.ToLower(rows[i].Name) < strings.ToLower(rows[j].Name)
	}) {
		names := make([]string, len(rows))
		for i, r := range rows {
			names[i] = r.Name
		}
		t.Errorf("sortedLoadout is not alphabetical:\n%s", strings.Join(names, "\n"))
	}

	// And the VIEW uses that order. The registry lists Write tool before View
	// tool; the alphabet wants them the other way round, so their relative
	// position on screen distinguishes sorted rendering from registry-order
	// rendering.
	// GORILLA FIX (2026-09-03): this asserted at a fixed Height: 50 and had
	// become FLAKY — measured on unmodified origin/main at v0.1.132, six runs
	// of this test alone gave three passes and three failures, and it was
	// already flaky before a 21st component made it fail nearly every time.
	//
	// The cause is arithmetic, not chance. The dialog WINDOWS its feature rows
	// and scrolls. At Height 50 it renders 49 lines of which 31 are chrome —
	// header, cost lines, dials, two section headings, the +RUN and scroll
	// notes, the extras block, the help line — leaving 18 rows for 21
	// components. View and Write are last in the alphabet, so they were below
	// the fold and the test failed with "rows not rendered", which points at
	// the renderer rather than at the real cause: the window was too small.
	//
	// Two earlier attempts at this are recorded because both were wrong. A
	// bigger fixed number is the same bug one component later. Deriving the
	// height as len(rows)+30 was still one line short of the measured chrome
	// and failed 5 times in 10 — a fix that was never checked against the
	// thing it was fixing.
	//
	// So: size the window from the registry plus the measured chrome plus real
	// margin, and if windowing STILL happens, say that in the failure instead
	// of blaming the renderer. A test about SORT ORDER must not also be a test
	// of whether the terminal is tall enough.
	const measuredChrome = 31
	height := len(rows) + measuredChrome + 9

	d := NewLoadoutDialogCmp().(*loadoutDialogCmp)
	d.Update(tea.WindowSizeMsg{Width: 130, Height: height})
	view := d.View()

	if strings.Contains(view, "up/down to reach the rest") {
		t.Fatalf("%d components still do not fit at height %d: the dialog is "+
			"windowing rows, so this test cannot see the whole list. Raise the "+
			"margin above %d.\n%s", len(rows), height, measuredChrome+9, view)
	}

	vi := strings.Index(view, "View tool")
	wi := strings.Index(view, "Write tool")
	if vi < 0 || wi < 0 {
		t.Fatalf("view/write rows not rendered at height %d with %d components:\n%s",
			height, len(rows), view)
	}
	if vi > wi {
		t.Errorf("View tool renders after Write tool — the display is not alphabetical (registry order leaked through)")
	}
}

// Space must toggle the row the user is LOOKING at. selectedIdx counts rows in
// display (sorted) order, so resolving it through the registry slice would
// flip a different component once the two orders diverge.
func TestLoadoutToggleFlipsTheDisplayedRow(t *testing.T) {
	if _, err := config.Load(t.TempDir(), false); err != nil {
		t.Fatalf("config.Load: %v", err)
	}

	rows := sortedLoadout()
	// Pick a display index where sorted and registry orders DISAGREE, so this
	// fails loudly if either side resolves through the wrong slice.
	idx := -1
	for i := range rows {
		if rows[i].ID != config.LoadoutComponents[i].ID {
			idx = i
			break
		}
	}
	if idx < 0 {
		t.Skip("sorted order equals registry order; nothing to distinguish")
	}
	target := rows[idx].ID

	before := config.LoadoutEnabled(target)
	d := NewLoadoutDialogCmp().(*loadoutDialogCmp)
	d.Update(tea.WindowSizeMsg{Width: 130, Height: 50})
	d.selectedIdx = numDials + idx
	d.Update(tea.KeyMsg{Type: tea.KeySpace})
	if config.LoadoutEnabled(target) == before {
		t.Errorf("toggling display row %d did not flip %s — space is acting on a different row than the one highlighted", idx, target)
	}
	config.ToggleLoadout(target) // restore for other tests in the package
}
