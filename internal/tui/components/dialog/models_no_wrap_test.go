package dialog

import (
	"testing"

	"github.com/opencode-ai/opencode/internal/llm/models"
)

// GORILLA OVERRIDE: the model list must have ends.
//
// It used to wrap: pressing up at the top jumped to the bottom and vice versa.
// With a handful of models that reads as convenience. NVIDIA NIM alone exposes
// 128, and at that length wrapping removes the only signal that tells you where
// you are — you sail past the boundary without noticing and lose your place.
//
// These assert the boundaries hold under the case that actually broke it:
// holding the key down, not pressing it once.
func newListOf(n int) *modelDialogCmp {
	m := &modelDialogCmp{}
	for i := 0; i < n; i++ {
		m.models = append(m.models, models.Model{ID: models.ModelID(string(rune('a' + i%26)))})
	}
	return m
}

func TestModelListStopsAtTop(t *testing.T) {
	m := newListOf(128)
	m.selectedIdx = 3
	for i := 0; i < 50; i++ { // held down, well past the boundary
		m.moveSelectionUp()
	}
	if m.selectedIdx != 0 {
		t.Fatalf("holding up must rest at the first entry, got %d", m.selectedIdx)
	}
	if m.scrollOffset != 0 {
		t.Errorf("the view should be at the top, offset=%d", m.scrollOffset)
	}
}

func TestModelListStopsAtBottom(t *testing.T) {
	m := newListOf(128)
	for i := 0; i < 500; i++ {
		m.moveSelectionDown()
	}
	if want := 127; m.selectedIdx != want {
		t.Fatalf("holding down must rest at the last entry (%d), got %d", want, m.selectedIdx)
	}
}

// The specific regression: from the top, one press up must NOT teleport to the
// end of the list.
func TestModelListDoesNotWrapEitherWay(t *testing.T) {
	m := newListOf(128)
	m.moveSelectionUp()
	if m.selectedIdx != 0 {
		t.Errorf("up at the top wrapped to %d", m.selectedIdx)
	}
	m.selectedIdx = 127
	m.moveSelectionDown()
	if m.selectedIdx != 127 {
		t.Errorf("down at the bottom wrapped to %d", m.selectedIdx)
	}
}

// A single-entry list is the degenerate case where an off-by-one would show up
// as an index panic rather than a wrap.
func TestModelListSingleEntry(t *testing.T) {
	m := newListOf(1)
	m.moveSelectionDown()
	m.moveSelectionUp()
	if m.selectedIdx != 0 {
		t.Errorf("a one-entry list must stay at 0, got %d", m.selectedIdx)
	}
}
