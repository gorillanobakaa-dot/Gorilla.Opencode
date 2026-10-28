package core

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
	"github.com/opencode-ai/opencode/internal/tui/util"
)

const (
	redBg   = "48;2;255;51;51" // SpeedColor on a dark terminal
	blackBg = "48;2;0;0;0"
	blueBg  = "48;2;30;58;138"
)

func renderBar(t *testing.T, info util.InfoMsg) string {
	t.Helper()
	prev := lipgloss.ColorProfile()
	lipgloss.SetColorProfile(termenv.TrueColor)
	lipgloss.SetHasDarkBackground(true)
	t.Cleanup(func() { lipgloss.SetColorProfile(prev) })
	m := statusCmp{width: 167, info: info}
	return m.View()
}

// Owner, 2026-10-28 (MacBook Air screenshot): the middle of the bar was a black
// gap between the red speed chip and the blue model name. At rest it is red.
func TestStatusBarRestsRedWithNoGap(t *testing.T) {
	if got := renderBar(t, util.InfoMsg{}); !strings.Contains(got, redBg) {
		t.Errorf("idle bar has no red fill: %q", got)
	}
}

// A message must not look like the bar at rest: info is white on dark blue,
// an error white on black.
func TestStatusMessagesStandOutFromTheRed(t *testing.T) {
	if got := renderBar(t, util.InfoMsg{Type: util.InfoTypeInfo, Msg: "saved"}); !strings.Contains(got, blueBg) {
		t.Errorf("info message not on dark blue: %q", got)
	}
	if got := renderBar(t, util.InfoMsg{Type: util.InfoTypeError, Msg: "failed"}); !strings.Contains(got, blackBg) {
		t.Errorf("error message not on black: %q", got)
	}
}

// The Air screenshot at 09:46 still had black after the red: the mode chip and
// "No diagnostics" drew on the theme's near-black. Nothing in the bar at rest
// may sit on it now.
func TestStatusBarHasNoBlackStretchAtRest(t *testing.T) {
	if got := renderBar(t, util.InfoMsg{}); strings.Contains(got, "48;2;24;24;24") {
		t.Errorf("bar at rest still has a near-black stretch: %q", got)
	}
}
