package page

import (
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/opencode-ai/opencode/internal/tui/components/chat"
)

// The frame's height must not change when the agent starts or stops working.
// Bubbletea erases by walking the cursor up by the last frame's row count, so a
// height that changes erases the wrong rows -- the footer marches down the
// screen and then jumps back up.
func TestFooterHeightIsIdenticalWorkingAndIdle(t *testing.T) {
	prompt := "> \n"
	info := "model X · context 1.2K\ntokens 900 in / 300 out"

	idle := reserveLiveRow("", shedToFit(30-chat.FooterReservedRows,
		footerArrangements("", prompt, info)))
	working := reserveLiveRow("⠋ Thinking... ", shedToFit(30-chat.FooterReservedRows,
		footerArrangements("", prompt, info)))

	hi, hw := lipgloss.Height(idle), lipgloss.Height(working)
	t.Logf("idle=%d working=%d", hi, hw)
	if hi != hw {
		t.Fatalf("frame is %d rows idle but %d rows working: it changes height every "+
			"time a turn starts or ends, which is exactly the up/down jumping", hi, hw)
	}
}
