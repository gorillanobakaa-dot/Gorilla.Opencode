package tui

// GORILLA OVERRIDE (2026-08-19), from a screenshot of a real run.
//
// /arsenal renders exactly the terminal height on purpose. On screen it came
// back with its bottom rows and its bottom border missing.
//
// The page was right. PlaceOverlay clamps an overlay to the BACKGROUND's
// height — correctly, so a dialog can never be taller than what it sits on —
// and in scrollback mode the background is only the chat page plus the status
// bar. A full-screen page painted onto a short canvas gets cut, and the cut is
// silent.
//
// That is the mirror image of the bug the clamp was written to prevent, and it
// shows up only in scrollback mode, which is the mode this project steers older
// machines toward.

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/opencode-ai/opencode/internal/tui/layout"
)

func TestAFullScreenOverlaySurvivesAShortBackground(t *testing.T) {
	const termH, termW = 40, 100

	// A full-screen page, the shape /arsenal renders.
	var page strings.Builder
	for i := 0; i < termH; i++ {
		page.WriteString(strings.Repeat("x", termW-1))
		if i < termH-1 {
			page.WriteString("\n")
		}
	}
	overlay := page.String()
	if got := lipgloss.Height(overlay); got != termH {
		t.Fatalf("test fixture is %d rows, want %d", got, termH)
	}

	// The background as scrollback mode builds it: short.
	short := strings.Join([]string{"conversation", "status bar"}, "\n")

	clipped := layout.PlaceOverlay(0, 0, overlay, short, false)
	if h := lipgloss.Height(clipped); h >= termH {
		t.Fatalf("expected the short background to clip the overlay; got %d rows", h)
	}
	t.Logf("short background: full-screen overlay clipped from %d rows to %d", termH, lipgloss.Height(clipped))

	// The fix: pad the background to the terminal height first.
	padded := short + strings.Repeat("\n", termH-lipgloss.Height(short))
	ok := layout.PlaceOverlay(0, 0, overlay, padded, false)
	if h := lipgloss.Height(ok); h != termH {
		t.Fatalf("after padding the background the overlay is %d rows, want %d", h, termH)
	}
	// Every row of the page must survive, including the last.
	lines := strings.Split(ok, "\n")
	if !strings.Contains(lines[len(lines)-1], "x") {
		t.Errorf("the last row of the page was lost: %q", lines[len(lines)-1])
	}
}
