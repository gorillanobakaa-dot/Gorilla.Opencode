// Version: 1.0.0 · updated 26-08-21-14-40
package startup

import (
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/lipgloss"
)

// GORILLA FIX (2026-08-21): the notice must not lose its indent when it wraps.
//
// Photographed on the owner's screen: a paragraph starting six columns in whose
// tail landed at column 0, looking like broken output. The cause is the trap
// CLAUDE.md names — lipgloss fails by WRAPPING, silently, somewhere other than
// the mistake — and hand-indenting a line that a caller later wraps guarantees
// it. Wrapping happens here now, with the indent applied afterwards.
func TestStaleNoticeKeepsItsIndentAtEveryWidth(t *testing.T) {
	for _, w := range []int{40, 60, 66, 80, 120} {
		// Rendered directly, with a fixed age: the layout must hold regardless
		// of what is in this machine's cache directory.
		note := renderStaleNotice(w, 9*24*time.Hour, true)
		for i, line := range strings.Split(note, "\n") {
			if strings.TrimSpace(line) == "" {
				continue
			}
			if !strings.HasPrefix(line, " ") {
				t.Errorf("width %d, line %d starts at column 0: %q", w, i, line)
			}
			if got := lipgloss.Width(line); got > w {
				t.Errorf("width %d, line %d is %d columns wide — it will wrap again inside the box: %q",
					w, i, got, line)
			}
		}
	}
}

// The notice must ask for something the reader can do WITHOUT quitting. The CLI
// subcommand it used to name is the exact thing /update was added to replace,
// because "quit the session and run this" is a request people do not act on.
func TestStaleNoticeAsksForTheInSessionCommand(t *testing.T) {
	note := renderStaleNotice(80, 0, false)
	if !strings.Contains(note, "/update") {
		t.Error("the notice does not name /update")
	}
	if strings.Contains(note, "models refresh") {
		t.Error("the notice still tells the reader to quit and run a CLI subcommand")
	}
}
