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
	"github.com/opencode-ai/opencode/internal/tui/styles"
	"github.com/opencode-ai/opencode/internal/tui/theme"
)

// ScrollbackFooter is the transcript component seen from outside the package,
// reduced to the one thing a caller needs when the conversation is being printed
// rather than drawn: what still has to appear in the frame.
//
// It exists because layout.Container hides its content, and the page needs the
// footer without the pane.
type ScrollbackFooter interface {
	FooterView() string
}

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

// FooterReservedRows is the number of rows the footer ALWAYS occupies when
// streaming is possible, regardless of whether a reply is in flight.
//
// GORILLA FIX (bugs: text vanishes, prompt drifts):
// Bubbletea erases its previous frame by walking the cursor UP by exactly the
// logical row count of the last View() it drew. When the live-preview block
// appears during streaming (up to livePreviewRows tall) and then disappears
// when the reply finishes, the frame shrinks. That shrink makes the erase
// over-reach: the cursor walks past the top of the frame and into real terminal
// output above it, wiping lines that were already printed into the scrollback.
//
// The fix is to never let the frame shrink: always occupy the same number of
// rows, padding with blank lines when the preview is absent. The frame height
// stays constant, the erase always lands in the right place, and nothing is
// wiped.
//
// Value: livePreviewRows (preview) + 1 (working indicator or blank) + 1 spare.
// The +1 spare absorbs a single-line status message without changing the height.
const FooterReservedRows = livePreviewRows + 2

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

	// Raw text, NOT the full message renderer.
	//
	// This used to call RenderForScrollback, which runs the whole Markdown pipeline
	// over the entire reply — on every frame, while the reply is still growing, to
	// display six lines of it. Measured: 0.96ms and 348KB per frame at 50 words,
	// rising to 21ms and 3.4MB at 3200 words. Linear in the length of the answer, so
	// the longer the reply the slower the interface, plus megabytes of garbage per
	// frame. That is a worse version of the O(n^2) streaming cost this mode was
	// supposed to remove.
	//
	// The preview is transient: it is overwritten on the next frame and never
	// scrolled back to. It does not need syntax highlighting or wrapped tables. The
	// finished message gets the full renderer exactly once, when it settles.
	text := last.Content().String()
	if strings.TrimSpace(text) == "" {
		// Nothing written yet — show the reasoning instead, so a model that thinks
		// for a while does not look like a model that has stalled.
		text = last.ReasoningContent().Thinking
	}
	if strings.TrimSpace(text) == "" {
		return ""
	}

	// Bound the input BEFORE any wrapping, so the cost cannot grow with the reply.
	// Only the tail can survive the row cap, and a generous overshoot covers the
	// case where every line is one character long.
	if budget := livePreviewRows * (m.width + 1) * 2; len(text) > budget {
		text = text[len(text)-budget:]
	}

	lines := tailLines(lipgloss.NewStyle().Width(m.width).Render(text), livePreviewRows)
	if strings.TrimSpace(lines) == "" {
		return ""
	}
	return styles.BaseStyle().Foreground(theme.CurrentTheme().TextMuted()).Render(lines)
}

// FooterView is the whole of what the transcript contributes to the screen when
// the alternate screen is off: a capped preview of the reply in flight, plus the
// working indicator. Everything settled has already been printed.
//
// GORILLA FIX: the returned string is ALWAYS exactly FooterReservedRows tall,
// padded with blank lines at the top when the preview is shorter than the
// reserved space. This keeps the frame height constant across all render cycles
// so bubbletea's cursor-up erase never over-reaches into the printed scrollback.
func (m *messagesCmp) FooterView() string {
	preview := m.livePreview()
	working := m.working()

	content := ""
	if preview != "" && strings.TrimSpace(working) != "" {
		content = lipgloss.JoinVertical(lipgloss.Top, preview, working)
	} else if preview != "" {
		content = preview
	} else if strings.TrimSpace(working) != "" {
		content = working
	}

	// Pad to a fixed height so the frame never shrinks between renders.
	// Without this, the transition from "streaming: 7 rows" to "idle: 0 rows"
	// causes bubbletea to walk the cursor 7 rows too far when erasing, landing
	// inside the printed scrollback and wiping it.
	contentRows := lipgloss.Height(content)
	if content == "" {
		contentRows = 0
	}
	padding := FooterReservedRows - contentRows
	if padding > 0 {
		pad := strings.Repeat("\n", padding-1) // JoinVertical adds its own \n
		if content == "" {
			content = lipgloss.JoinVertical(lipgloss.Top, pad)
		} else {
			content = lipgloss.JoinVertical(lipgloss.Top, pad, content)
		}
	}
	return content
}

// tailLines returns at most n trailing lines of s.
func tailLines(s string, n int) string {
	lines := strings.Split(strings.TrimRight(s, "\n"), "\n")
	if len(lines) > n {
		lines = lines[len(lines)-n:]
	}
	return strings.Join(lines, "\n")
}
