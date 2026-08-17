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
	d := NewLoadoutDialogCmp().(*loadoutDialogCmp)
	d.Update(tea.WindowSizeMsg{Width: 130, Height: 50})
	view := d.View()
	vi := strings.Index(view, "View tool")
	wi := strings.Index(view, "Write tool")
	if vi < 0 || wi < 0 {
		t.Fatalf("view/write rows not rendered:\n%s", view)
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
