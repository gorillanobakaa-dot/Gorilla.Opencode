package chat

import (
	"regexp"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

var plainRe = regexp.MustCompile("\x1b\\[[0-9;]*m")

// plainLines is the printed text, ANSI stripped, in order.
func plainLines(cmds []tea.Cmd) []string {
	var out []string
	for _, c := range cmds {
		if raw, ok := printedBody(c); ok {
			out = append(out, plainRe.ReplaceAllString(raw, ""))
		}
	}
	return out
}

// Three kinds of text meet at the closing marker: the model's private
// working-out, the boundary, and the answer addressed to the reader. They were
// stacked with no separation, so the answer began on the line immediately below
// the marker whose whole job is to say the thinking had ended.
func TestClosingMarkerHasABlankLineOnEachSide(t *testing.T) {
	m := printerFor(t, 100)
	lines := plainLines(m.flushReasoning(reasoningMsg("m1", "first thought\nsecond thought")))

	idx := -1
	for i, l := range lines {
		if strings.Contains(l, "done thinking") {
			idx = i
		}
	}
	if idx < 0 {
		t.Fatalf("closing marker never printed: %#v", lines)
	}
	if idx == 0 || strings.TrimSpace(lines[idx-1]) != "" {
		t.Errorf("no blank line BEFORE the closing marker:\n%#v", lines)
	}
	if idx == len(lines)-1 || strings.TrimSpace(lines[idx+1]) != "" {
		t.Errorf("no blank line AFTER the closing marker:\n%#v", lines)
	}
}

// Spacing that changes with the backend is spacing nobody can rely on.
// Nemotron's reasoning begins with a newline; another provider's may not.
func TestOpeningGapDoesNotDependOnTheProvider(t *testing.T) {
	blanksAfterOpen := func(thinking string) int {
		m := printerFor(t, 100)
		lines := plainLines(m.flushReasoning(reasoningMsg("m1", thinking)))
		open := -1
		for i, l := range lines {
			if strings.Contains(l, "thinking") && !strings.Contains(l, "done thinking") {
				open = i
				break
			}
		}
		if open < 0 {
			t.Fatalf("opening marker never printed: %#v", lines)
		}
		n := 0
		for i := open + 1; i < len(lines) && strings.TrimSpace(lines[i]) == ""; i++ {
			n++
		}
		return n
	}

	withNewline := blanksAfterOpen("\nfirst thought\nsecond")
	without := blanksAfterOpen("first thought\nsecond")
	t.Logf("leading newline: %d blank(s); no leading newline: %d blank(s)",
		withNewline, without)

	if withNewline != without {
		t.Errorf("the gap depends on what the provider sends: %d vs %d",
			withNewline, without)
	}
	if withNewline != 1 {
		t.Errorf("want exactly 1 blank line after the opening marker, got %d", withNewline)
	}
}
