// Version: 1.0.0 · updated 26-10-23-03-30
package diff

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
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

// MacBook Air, 2026-10-23 (air-2026-10-23-032550-owner-grey-blotches.png,
// Terminal.app, 256 colours, black "Pro" profile): an Edit result's side-by-side
// diff painted every row as a grey slab, wider than its text. Below 24-bit colour
// no row may carry a background, and every row stays inside the width it was
// given. On a 24-bit terminal the row backgrounds are kept.
func TestDiffRowsHaveNoBackgroundBelowTrueColor(t *testing.T) {
	saved := lipgloss.ColorProfile()
	defer lipgloss.SetColorProfile(saved)

	const width = 166 // a 167-column window less the last column
	udiff := "--- a/QUESTIONS.txt\n+++ b/QUESTIONS.txt\n@@ -17,3 +17,7 @@\n" +
		" context one\n context two\n context three\n" +
		"+\n+5. Sudo Permission Test (`sudo -n true`)\n" +
		"+   - Question: Do you authorize running `sudo -n true` to test non-interactive sudo? " +
		"This line is long on purpose so that it has to be cut at the column edge.\n+\n"

	lipgloss.SetColorProfile(termenv.ANSI256)
	out, err := FormatDiff(udiff, WithTotalWidth(width))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "Sudo Permission Test") {
		t.Fatalf("the added lines were not rendered, so nothing is tested:\n%s", out)
	}
	for i, line := range strings.Split(strings.TrimRight(out, "\n"), "\n") {
		if strings.Contains(line, "\x1b[48;") || strings.Contains(line, ";48;") {
			t.Errorf("row %d carries a background under ANSI256: %q", i, line)
		}
		if w := ansi.StringWidth(line); w > width {
			t.Errorf("row %d is %d columns, wider than %d", i, w, width)
		}
	}
	if !strings.Contains(out, "38;5;") {
		t.Errorf("no colour at all under ANSI256; the +/- marker must still be coloured")
	}

	lipgloss.SetColorProfile(termenv.TrueColor)
	if tc, _ := FormatDiff(udiff, WithTotalWidth(width)); !strings.Contains(tc, "48;2;") {
		t.Errorf("24-bit terminals lost their row backgrounds; only 256/16 colours should drop them")
	}
}
