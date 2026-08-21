// Version: 1.0.0 · updated 26-08-21-11-25
package tui

import (
	"strings"
	"testing"

	"github.com/opencode-ai/opencode/internal/lsp"
	"github.com/opencode-ai/opencode/internal/tui/components/core"
	"github.com/opencode-ai/opencode/internal/tui/util"

	tea "github.com/charmbracelet/bubbletea"
)

// statusAt builds a status bar sized to width, which is what gives InfoBudget
// a number to report.
func statusAt(width int) core.StatusCmp {
	s := core.NewStatusCmp(map[string]*lsp.Client{})
	m, _ := s.Update(tea.WindowSizeMsg{Width: width, Height: 40})
	return m.(core.StatusCmp)
}

// GORILLA OVERRIDE (2026-08-21): a notice that does not FIT the footer is
// echoed into the transcript, whether or not anyone marked it Echo.
//
// The footer truncates to one line. Before this, /update, /purge and the
// AGENTS.md verdict existed only as whatever fraction of themselves fitted —
// the screenshot that prompted the fix was cut at "2 configured endpoint(s)
// re-asked | bu...", losing the note that the built-in providers were NOT
// refreshed.
func TestOversizedNoticesAreEchoedToTheTranscript(t *testing.T) {
	a := appModel{width: 160, scrollback: true, status: statusAt(160)}
	budget := a.status.InfoBudget()
	if budget <= 0 {
		t.Fatalf("InfoBudget=%d after a WindowSizeMsg; the echo rule has nothing to measure against", budget)
	}

	long := util.InfoMsg{Type: util.InfoTypeInfo, Msg: strings.Repeat("x", budget+1)}
	if !a.shouldEchoNotice(long) {
		t.Error("a notice one column too wide was not echoed; the footer will cut it and nothing keeps the rest")
	}

	short := util.InfoMsg{Type: util.InfoTypeInfo, Msg: "copied to clipboard"}
	if a.shouldEchoNotice(short) {
		t.Error("a toast that fits was echoed; ordinary notices must not fill the scrollback")
	}
}

// Explicit ReportInfoEcho still echoes at any length — "important enough to
// keep" is a separate judgement from "too long to show".
func TestExplicitEchoStillEchoesWhenItFits(t *testing.T) {
	a := appModel{width: 160, scrollback: true, status: statusAt(160)}
	if !a.shouldEchoNotice(util.InfoMsg{Msg: "short but important", Echo: true}) {
		t.Error("an explicitly echoed notice was dropped because it happened to fit")
	}
}

// Two states where echoing is wrong: no scrollback (nowhere to print), and an
// empty message (a blank row in the user's history).
func TestEchoSkipsWhenThereIsNothingToPrintOrNowhereToPrintIt(t *testing.T) {
	long := util.InfoMsg{Msg: strings.Repeat("y", 400)}
	noScrollback := appModel{width: 160, scrollback: false, status: statusAt(160)}
	if noScrollback.shouldEchoNotice(long) {
		t.Error("echoed with scrollback off; tea.Println is the only safe path and it is not available")
	}
	a := appModel{width: 160, scrollback: true, status: statusAt(160)}
	if a.shouldEchoNotice(util.InfoMsg{Msg: "   ", Echo: true}) {
		t.Error("an empty notice was echoed, printing a blank row into the transcript")
	}
}

// Before a WindowSizeMsg arrives the footer does not truncate either (see
// truncateStatusMsg), so an unmeasurable budget must not cause a flood of
// echoes of messages that were never cut.
func TestUnknownWidthDoesNotEchoEverything(t *testing.T) {
	a := appModel{scrollback: true, status: core.NewStatusCmp(map[string]*lsp.Client{})}
	if a.shouldEchoNotice(util.InfoMsg{Msg: strings.Repeat("z", 400)}) {
		t.Error("echoed while the width was still unknown; nothing was truncated yet")
	}
}
