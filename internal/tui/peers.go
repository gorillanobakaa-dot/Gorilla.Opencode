// GORILLA (2026-10-10): session messaging in the full interface — /peers,
// /peers name NEW, /message NAME TEXT, and the notice shown when something
// arrives from another session.
package tui

import (
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/opencode-ai/opencode/internal/peers"
	"github.com/opencode-ai/opencode/internal/tui/util"
)

// peerNoticeTTL keeps an arrival in the status bar long enough to be read.
// With scrollback on (the default) it is also printed in full into the
// conversation, where it stays.
const peerNoticeTTL = 30 * time.Second

// peerNotice is the message an arrival becomes in this interface.
func peerNotice(m peers.Message) util.InfoMsg {
	return util.InfoMsg{Type: util.InfoTypeWarn, Msg: peers.HumanNotice(m), TTL: peerNoticeTTL, Echo: true}
}

// AttachPeers routes arrivals into the running program. Send is safe from the
// goroutine the notice runs on; the session calls it on its own goroutine, so a
// busy event loop never holds up the peer that sent the message.
func AttachPeers(p *tea.Program) {
	s := peers.Active()
	if p == nil || s == nil {
		return
	}
	s.SetNotify(func(m peers.Message) { p.Send(peerNotice(m)) })
}

// peersCommand handles /peers and /peers name NEW.
func (a *appModel) peersCommand(args string) tea.Cmd {
	sub, rest := peers.SplitFirst(args)
	switch {
	case sub == "":
		return a.openInfoPage("peers")
	case strings.EqualFold(sub, "name"):
		s := peers.Active()
		if s == nil {
			return util.ReportWarn(peers.NotRunning)
		}
		if rest == "" {
			return util.ReportWarn("Give the new name: /peers name NEWNAME (letters, digits, dots, dashes, underscores).")
		}
		got, err := s.Rename(rest)
		if err != nil {
			return util.ReportError(err)
		}
		return util.ReportInfo("This session is now called " + got + ". Other sessions see the new name at once.")
	}
	return util.ReportWarn("/peers takes nothing (the page), or: /peers name NEWNAME")
}

// messageCommand handles /message NAME TEXT. The send runs as a command, off
// the event loop: it is local and bounded by deadlines, but a frozen screen for
// even three seconds is not acceptable.
func (a *appModel) messageCommand(args string) tea.Cmd {
	to, text := peers.SplitFirst(args)
	if to == "" || text == "" {
		return util.ReportWarn("Type /message, a session name, then the text: /message NAME your text. /peers lists the names.")
	}
	s := peers.Active()
	if s == nil {
		return util.ReportWarn(peers.NotRunning)
	}
	return func() tea.Msg {
		res, err := s.Send(to, text, false)
		if err != nil {
			return util.InfoMsg{Type: util.InfoTypeError, Msg: err.Error()}
		}
		return util.InfoMsg{Type: util.InfoTypeInfo, Msg: res}
	}
}
