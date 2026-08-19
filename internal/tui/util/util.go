package util

import (
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// NoticeDeco bookends a Gorilla notice in the transcript — two gorillas
// (U+1F98D) and two warning signs (U+26A0 U+FE0F) at either end:
// `NoticeDeco message NoticeDeco`.
//
// Separated by a middle dot, and NOT by a space, for two reasons:
//
//   - ⚠️ is U+26A0 plus the U+FE0F variation selector, which makes it DRAW two
//     columns wide while terminals measure it as one. The cursor advances a
//     single column, so the next glyph is painted over it and the pair collides.
//     A width-1 character between them absorbs the difference.
//   - A space would let a word-wrapper break the decoration in half and orphan
//     the remainder at the left margin. A dot is not whitespace, so the whole
//     thing stays one unbreakable token.
//
// The middle dot rather than a full stop because it is already this project's
// separator (the footer reads "model X | in Y | context Z"), so it reads as
// deliberate rather than as punctuation.
//
// Kept here so every path that surfaces such a notice — the cold-start echo, a
// provider error — reads identically, whatever pipeline it came through.
// NoticeDeco is EXEMPT from the ASCII-drawing rule, deliberately and on the
// owner's instruction. The middle dots were chosen precisely because they have
// no space to break on and absorb the U+FE0F width mismatch that the warning
// emoji carries. Settled 2026-08-18 after several rejected designs ("stop
// inventing things... simple", "NO LINES"). Do not "fix" it to ASCII: that
// would re-derive a decision already made and lose the reason it was made.
const NoticeDeco = "🦍⚠️·⚠️·🦍"

func CmdHandler(msg tea.Msg) tea.Cmd {
	return func() tea.Msg {
		return msg
	}
}

func ReportError(err error) tea.Cmd {
	return CmdHandler(InfoMsg{
		Type: InfoTypeError,
		Msg:  err.Error(),
	})
}

type InfoType int

const (
	InfoTypeInfo InfoType = iota
	InfoTypeWarn
	InfoTypeError
)

func ReportInfo(info string) tea.Cmd {
	return CmdHandler(InfoMsg{
		Type: InfoTypeInfo,
		Msg:  info,
	})
}

// ReportInfoEcho is a notice important enough to survive the footer's width
// limit: it flashes in the status bar (truncated to fit, as any notice is) AND
// is printed in full into the transcript, where there is room for the whole
// sentence and where it stays scrollable and copyable.
//
// GORILLA OVERRIDE (2026-08-18): added because the cold-start "still warming up"
// notice was cut to "...First reply..." in the status bar — the half that told the
// user what to do was the half that got dropped. The footer answers "something
// is happening"; the transcript answers "here is the whole of it".
func ReportInfoEcho(info string) tea.Cmd {
	return CmdHandler(InfoMsg{
		Type: InfoTypeInfo,
		Msg:  info,
		Echo: true,
	})
}

func ReportWarn(warn string) tea.Cmd {
	return CmdHandler(InfoMsg{
		Type: InfoTypeWarn,
		Msg:  warn,
	})
}

type (
	InfoMsg struct {
		Type InfoType
		Msg  string
		TTL  time.Duration
		// Echo asks the app to ALSO print Msg, in full, into the transcript —
		// for notices too long to survive the status bar's one-line truncation.
		Echo bool
	}
	// ClearStatusMsg clears the status message IF it is still the one that
	// scheduled this clear. Seq carries the message generation the clear was
	// armed for; a stale clear from an earlier message is ignored, so a burst of
	// messages no longer wipes each other early. See status.go.
	ClearStatusMsg struct{ Seq int }
)

func Clamp(v, low, high int) int {
	if high < low {
		low, high = high, low
	}
	return min(high, max(low, v))
}
