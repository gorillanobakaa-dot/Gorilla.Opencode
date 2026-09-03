// GORILLA OVERRIDE (2026-09-03): the low-bandwidth state must be visible for as
// long as it lasts, not announced once.
//
// Reported from the live screen: pressing `l` appeared to do nothing. It had
// switched seven components off and written them to disk, but six of the seven
// were below the visible window, so nothing the user was looking at changed.
// The only acknowledgement was a toast that fires once and fades.
//
// A toast is the wrong instrument for a state you are still in. These tests pin
// the banner: present while the preset is applied, gone once it is undone, and
// present in compact mode too, because a short terminal is exactly where rows
// are scarce and the user can see least of the list.
package dialog

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/opencode-ai/opencode/internal/config"
)

const lowBWBanner = "LOW-BANDWIDTH MODE"

// loadoutView builds the dialog at a realistic terminal size and renders it.
func loadoutView(t *testing.T, width, height int) string {
	t.Helper()
	m := NewLoadoutDialogCmp()
	m.Update(tea.WindowSizeMsg{Width: width, Height: height})
	return m.View()
}

func TestLowBandwidthBannerShowsWhileThePresetIsApplied(t *testing.T) {
	if strings.Contains(loadoutView(t, 120, 40), lowBWBanner) {
		t.Fatal("the banner is showing before the preset was applied: it would be permanent furniture")
	}

	if n := config.ApplyLowBandwidthLoadout(); n == 0 {
		t.Skip("nothing for the preset to switch off in this configuration")
	}
	t.Cleanup(func() { config.UndoLowBandwidthLoadout() })

	if !strings.Contains(loadoutView(t, 120, 40), lowBWBanner) {
		t.Error("the preset is applied and the screen does not say so. This is the bug the banner exists for")
	}
}

// The banner must name the key that undoes it. Telling someone they are in a
// mode without telling them the way out is half a message.
func TestLowBandwidthBannerNamesTheUndoKey(t *testing.T) {
	if n := config.ApplyLowBandwidthLoadout(); n == 0 {
		t.Skip("nothing for the preset to switch off in this configuration")
	}
	t.Cleanup(func() { config.UndoLowBandwidthLoadout() })

	view := loadoutView(t, 120, 40)
	line := ""
	for _, l := range strings.Split(view, "\n") {
		if strings.Contains(l, lowBWBanner) {
			line = l
			break
		}
	}
	if line == "" {
		t.Fatal("no banner line to inspect")
	}
	if !strings.Contains(line, "Press u") {
		t.Errorf("the banner does not name the undo key:\n%s", line)
	}
}

func TestLowBandwidthBannerDisappearsAfterUndo(t *testing.T) {
	if n := config.ApplyLowBandwidthLoadout(); n == 0 {
		t.Skip("nothing for the preset to switch off in this configuration")
	}
	config.UndoLowBandwidthLoadout()

	if strings.Contains(loadoutView(t, 120, 40), lowBWBanner) {
		t.Error("the preset was undone and the banner is still up. A stale mode line is worse than none")
	}
}

// A short terminal drops the explanatory header first. The banner is not
// explanation: it is state, and a cramped screen shows fewer rows, so the user
// can see even less of what changed. It must survive the squeeze.
func TestLowBandwidthBannerSurvivesACrampedTerminal(t *testing.T) {
	if n := config.ApplyLowBandwidthLoadout(); n == 0 {
		t.Skip("nothing for the preset to switch off in this configuration")
	}
	t.Cleanup(func() { config.UndoLowBandwidthLoadout() })

	for _, h := range []int{40, 24, 16, 12} {
		if !strings.Contains(loadoutView(t, 100, h), lowBWBanner) {
			t.Errorf("the banner was dropped at terminal height %d", h)
		}
	}
}

// The frame must not grow past the terminal. lipgloss WRAPS rather than
// overflowing, so an untruncated banner shows up as extra HEIGHT somewhere else
// entirely, the trap CLAUDE.md records four instances of.
func TestLowBandwidthBannerDoesNotOverflowTheFrame(t *testing.T) {
	if n := config.ApplyLowBandwidthLoadout(); n == 0 {
		t.Skip("nothing for the preset to switch off in this configuration")
	}
	t.Cleanup(func() { config.UndoLowBandwidthLoadout() })

	for _, w := range []int{60, 80, 100, 140} {
		const h = 24
		view := loadoutView(t, w, h)
		for i, line := range strings.Split(view, "\n") {
			if got := lipgloss.Width(line); got > w {
				t.Errorf("width %d: line %d is %d columns wide, wider than the terminal:\n%s",
					w, i, got, line)
			}
		}
	}
}
