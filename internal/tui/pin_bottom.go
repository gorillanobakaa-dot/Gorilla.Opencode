// GORILLA OVERRIDE: this file did not exist upstream.
//
// It keeps the prompt at the BOTTOM of the window when the conversation lives in
// the terminal's scrollback.
//
// The problem, stated precisely: outside the alternate screen bubbletea draws its
// frame wherever the cursor happens to be, which is immediately after whatever was
// last printed. On a fresh session that is two or three rows down a tall window,
// so the prompt sits in the middle of the screen with dead space beneath it, and
// creeps downward as each reply is printed. It only settles once enough output has
// accumulated to fill the window. Reported as the prompt "jumping up and down".
//
// The fix is to make the cursor start on the last row, by scrolling the window
// once. From then on every printed line pushes older lines up into scrollback and
// the frame stays where it is — which is what a fixed footer actually is.
//
// Why NOT a scroll region (DECSTBM, ESC[t;br). That is the textbook way to pin a
// footer, and it would be wrong here: lines that scroll out of a scroll region are
// DISCARDED rather than added to the terminal's scrollback. It would pin the
// prompt beautifully and destroy the copyable history that is the entire point of
// leaving the alternate screen. Not a trade — just a loss.
package tui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/opencode-ai/opencode/internal/tui/components/chat"
)

// pinToBottom returns a command that scrolls the window so the cursor sits on its
// last row, and reports the height it pinned for.
//
// It emits blank lines rather than a scroll-up escape because the effect must end
// up in the scrollback either way, and newlines are the portable spelling. The
// cost is a screen of blank rows above the first message, visible only to someone
// who scrolls up past the start of the conversation.
func pinToBottom(rows int) tea.Cmd {
	if rows <= 1 {
		return nil
	}
	// tea.Println appends its own newline, so this emits exactly rows-1 of them:
	// enough to put the cursor on the last row without scrolling an extra time.
	return tea.Println(strings.Repeat("\n", rows-2))
}

// pinCmd decides whether anything needs pinning after a resize, and updates the
// record of what the frame is currently pinned for.
//
// Two cases matter. The first size message is the one that establishes the bottom.
// After that, only GROWTH needs handling: when a window gets taller the terminal
// adds rows below the frame, leaving exactly the gap this file exists to remove.
// Shrinking needs nothing — the terminal scrolls to keep the cursor visible, which
// already leaves the frame at the bottom.
func (a *appModel) pinCmd(rows int) tea.Cmd {
	if !a.scrollback || rows <= 1 {
		return nil
	}

	if !a.pinnedOnce {
		a.pinnedOnce = true
		a.pinnedRows = rows
		return pinToBottom(rows)
	}
	if rows > a.pinnedRows {
		grew := rows - a.pinnedRows
		a.pinnedRows = rows
		// Only the newly added rows, not another whole screen: the frame is already
		// at the old bottom, and over-scrolling would push the conversation out of
		// view for no reason.
		return pinToBottom(grew + 1)
	}
	a.pinnedRows = rows
	return nil
}

// bannerCmd prints the session's identity once, and only after the frame has been
// pinned — the pin scrolls the window, so anything printed before it is scrolled
// straight back out of view.
//
// Kept separate from pinCmd rather than batched into it: pinCmd's whole contract is
// "how many lines does it scroll", and wrapping that in a sequence made it
// unmeasurable, which broke its test while the behaviour was still correct.
func (a *appModel) bannerCmd() tea.Cmd {
	if !a.scrollback || a.bannerShown || !a.pinnedOnce || a.width <= 0 {
		return nil
	}
	banner := chat.SessionBanner(a.width)
	if banner == "" {
		return nil
	}
	a.bannerShown = true
	return tea.Println(banner)
}
