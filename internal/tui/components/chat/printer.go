// GORILLA OVERRIDE: this file did not exist upstream.
//
// It is the half of the transcript that writes into the terminal's own output
// instead of into a viewport. When the alternate screen is off — the default —
// the conversation is not painted on a screen buffer at all: each finished
// message is printed, once, as ordinary terminal output. That is what makes the
// wheel, Select-All and Ctrl+Shift+C work, because the text really is in the
// terminal rather than drawn over it.
package chat

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/opencode-ai/opencode/internal/message"
)

// livePreviewRows caps how much of an in-flight reply is shown in the footer
// while it streams.
//
// This is a correctness limit, not a taste one. Outside the alternate screen
// bubbletea erases its previous frame by walking the cursor up by the number of
// LOGICAL lines it last drew; if the frame is taller than the window, the lines
// that scrolled off are not where the count thinks they are and every later
// erase lands in the wrong place. That is precisely the failure recorded in the
// 2026-07-28 screencast of the startup picker. A short, fixed preview keeps the
// footer well inside any usable window.
const livePreviewRows = 6

// messagesService reaches the message store, tolerating its absence.
//
// Only sub-agent task output needs it, so a component without an app can still
// render everything else. That is not a convenience for tests: printing is
// irreversible, and a nil dereference in the print path would take the program
// down mid-transcript, after some of it had already been written to the user's
// terminal and could not be withdrawn. Degrading to "render what we can" is the
// safer failure.
func (m *messagesCmp) messagesService() message.Service {
	if m.app == nil {
		return nil
	}
	return m.app.Messages
}

// printPending returns commands that print every message that has settled since
// the last call, in order, exactly once.
//
// Ordering is enforced by stopping at the first message that is not ready rather
// than skipping it. Printed output cannot be reordered afterwards, so emitting a
// later message first would leave the transcript permanently out of sequence —
// and unlike a viewport, there is no re-render that could repair it.
func (m *messagesCmp) printPending() []tea.Cmd {
	if !m.scrollback || m.width <= 0 {
		return nil
	}

	var cmds []tea.Cmd
	for i, msg := range m.messages {
		if m.printed[msg.ID] {
			continue
		}
		if !ScrollbackReady(msg) {
			// Not settled: stop, so that whatever follows cannot overtake it.
			break
		}
		text := RenderForScrollback(msg, i, m.messages, m.messagesService(), m.width)
		m.printed[msg.ID] = true
		if strings.TrimSpace(text) == "" {
			continue
		}
		cmds = append(cmds, tea.Println(text))
	}
	return cmds
}

// forgetPrinted resets the record of what has been printed.
//
// Called when the session changes. It deliberately does NOT try to unprint
// anything: the previous session's messages stay in the terminal's scrollback,
// which is correct — that is the history the user asked to be able to keep.
func (m *messagesCmp) forgetPrinted() {
	m.printed = make(map[string]bool, len(m.printed))
}

// livePreview is what the footer shows while a reply is still arriving: the tail
// of the message being generated, capped to livePreviewRows.
//
// It is not history and is never printed. The same text is printed in full, once,
// when the message finishes — so this exists purely so that a long answer can be
// watched as it forms rather than appearing all at once.
func (m *messagesCmp) livePreview() string {
	if !m.scrollback || m.width <= 0 || len(m.messages) == 0 {
		return ""
	}
	last := m.messages[len(m.messages)-1]
	if last.Role != message.Assistant || ScrollbackReady(last) {
		return ""
	}

	text := RenderForScrollback(last, len(m.messages)-1, m.messages, m.messagesService(), m.width)
	if strings.TrimSpace(text) == "" {
		return ""
	}

	lines := strings.Split(text, "\n")
	if len(lines) > livePreviewRows {
		lines = lines[len(lines)-livePreviewRows:]
	}
	return strings.Join(lines, "\n")
}

// FooterView is the whole of what the transcript contributes to the screen when
// the alternate screen is off: a capped preview of the reply in flight, plus the
// working indicator. Everything settled has already been printed.
//
// It is guaranteed to be at most livePreviewRows+1 lines tall, which the caller
// depends on to keep the frame smaller than the window.
func (m *messagesCmp) FooterView() string {
	parts := make([]string, 0, 2)
	if preview := m.livePreview(); preview != "" {
		parts = append(parts, preview)
	}
	if working := m.working(); strings.TrimSpace(working) != "" {
		parts = append(parts, working)
	}
	if len(parts) == 0 {
		return ""
	}
	return lipgloss.JoinVertical(lipgloss.Top, parts...)
}
