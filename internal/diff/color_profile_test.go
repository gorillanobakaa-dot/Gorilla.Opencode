// Version: 1.0.0 · updated 26-10-23-02-50
package diff

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
)

// Same Terminal.app 10.13 defect as the Markdown renderer (styles package): the
// diff's syntax highlighting and intra-line highlight wrote 24-bit colour
// unconditionally, which that terminal shows as FAINT.
func TestDiffColoursFollowTheTerminalsColourProfile(t *testing.T) {
	saved := lipgloss.ColorProfile()
	defer lipgloss.SetColorProfile(saved)

	lipgloss.SetColorProfile(termenv.ANSI256)
	line := highlightLine("main.go", `fmt.Println("hello")`, lipgloss.Color("#202020"))
	if strings.Contains(line, "38;2;") || strings.Contains(line, "48;2;") {
		t.Errorf("syntax highlight wrote 24-bit colour under ANSI256: %q", line)
	}
	if !strings.Contains(line, "38;5;") {
		t.Fatalf("no 256-colour sequence in the highlight, so the check above proves nothing: %q", line)
	}

	seg := []Segment{{Start: 0, End: 3, Type: LineAdded}}
	hl := applyHighlighting("abcdef", seg, LineAdded, lipgloss.AdaptiveColor{Dark: "#335533", Light: "#ccffcc"})
	if strings.Contains(hl, "38;2;") || strings.Contains(hl, "48;2;") {
		t.Errorf("intra-line highlight wrote 24-bit colour under ANSI256: %q", hl)
	}
	if !strings.Contains(hl, "48;5;") {
		t.Errorf("intra-line highlight lost its background under ANSI256: %q", hl)
	}
}
