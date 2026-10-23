// Version: 1.0.0 · updated 26-10-23-02-50
package styles

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
)

// MacBook Air, macOS 10.13 Terminal.app, 2026-10-23: the model's replies were a
// dim grey (rgb 115,115,115 in air-2026-10-23-013154.png) and *emphasis* a grey
// italic instead of yellow. glamour wrote 24-bit colour ("38;2;R;G;B") whatever
// the terminal was; Terminal.app 10.13 has only 256 colours and reads the "2"
// as FAINT. On a 256- or 16-colour terminal no 24-bit sequence may be written,
// and the text must still carry a colour that terminal understands.
func TestMarkdownFollowsTheTerminalsColourProfile(t *testing.T) {
	saved := lipgloss.ColorProfile()
	defer lipgloss.SetColorProfile(saved)

	cases := []struct {
		profile termenv.Profile
		want    string // a sequence this profile does use
	}{
		{termenv.ANSI256, "38;5;"},
		{termenv.ANSI, "\x1b[9"}, // bright 16-colour foregrounds are 90-97
	}
	for _, c := range cases {
		lipgloss.SetColorProfile(c.profile)
		out, err := GetMarkdownRenderer(80).Render("Checking git. *Gorilla is analyzing this... with science...*")
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(out, "38;2;") || strings.Contains(out, "48;2;") {
			t.Errorf("profile %d: 24-bit colour written to a terminal without it (Terminal.app shows it FAINT): %q", c.profile, out[:min(len(out), 120)])
		}
		if !strings.Contains(out, c.want) && !strings.Contains(out, "\x1b[3") {
			t.Errorf("profile %d: no colour at all in the reply; the test would pass on plain text: %q", c.profile, out[:min(len(out), 120)])
		}
	}
	lipgloss.SetColorProfile(termenv.ANSI256)
	out, _ := GetMarkdownRenderer(80).Render("plain")
	if !strings.Contains(out, "38;5;") {
		t.Fatalf("ANSI256 render carries no 256-colour sequence, so the check above proves nothing: %q", out)
	}
}
