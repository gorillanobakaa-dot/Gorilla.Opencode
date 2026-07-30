// GORILLA OVERRIDE: this file did not exist upstream.
//
// It prints the session's identity banner once, when the conversation lives in
// the terminal's scrollback.
//
// WHY THERE IS NO LONGER A "PIN TO BOTTOM" HERE. This file used to also scroll
// the window on startup so the prompt sat on its last row, by printing a whole
// screen of blank lines (rows-2 of them — about 44 on a 900px window). The
// stated goal was to stop the prompt drifting downward as replies accumulated.
//
// It was removed on 2026-07-30 because the cure was worse than the disease. The
// blank lines are real scrollback: they put a screen-tall gap between the banner
// and the first prompt, and another one every time the window grew. Reported as
// "a huge gap in between the first lines on the screen and the last bit", with
// the interface appearing to start at the bottom and then "jump up and stay
// there" once printed output pushed through the padding.
//
// The drift it was written to prevent is not a defect. It is what every shell
// does: output is printed, the prompt follows the last line, and once the screen
// fills the prompt stays at the bottom on its own. Claude Code and Gemini CLI
// both behave exactly this way outside the alternate screen. Padding the screen
// to fake a fixed footer buys nothing the terminal does not already do, and
// costs a screenful of blank history to buy it.
//
// Do not reintroduce it. A DECSTBM scroll region is not the alternative either:
// lines that scroll out of a scroll region are DISCARDED rather than added to
// the scrollback, which would destroy the copyable history that is the entire
// point of leaving the alternate screen.
package tui

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/opencode-ai/opencode/internal/tui/components/chat"
)

// bannerCmd prints the session's identity exactly once per session.
func (a *appModel) bannerCmd() tea.Cmd {
	if !a.scrollback || a.bannerShown || a.width <= 0 {
		return nil
	}
	banner := chat.SessionBanner(a.width)
	if banner == "" {
		return nil
	}
	a.bannerShown = true
	return tea.Println(banner)
}
