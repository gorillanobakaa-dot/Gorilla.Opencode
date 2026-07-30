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
	"github.com/muesli/reflow/wordwrap"
	"github.com/opencode-ai/opencode/internal/message"
	"github.com/opencode-ai/opencode/internal/tui/styles"
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

// FooterReservedRows is the number of rows the footer ALWAYS occupies.
//
// GORILLA OVERRIDE: this used to be eight, because the footer carried a
// six-row rolling preview of the reply in flight. That preview is gone. It was
// a second scrolling region competing with the terminal's own, it could not be
// scrolled back to, and everything it showed is now printed into the scrollback
// as it arrives — which is what the terminal is for. What remains is the
// working indicator, and that is one row.
//
// The height is still FIXED rather than merely bounded, and that part is not
// taste. Bubbletea erases its previous frame by walking the cursor UP by the
// logical row count of the last View() it drew; a frame that shrinks between
// renders makes the erase over-reach into output already printed above it and
// wipe it. That was the 2026-07-30 "text vanishes" bug. Pad, never shrink.
const FooterReservedRows = 1

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

// Markers printed around a block of streamed reasoning.
//
// GORILLA OVERRIDE: reasoning is now written into the terminal as it arrives,
// mixed into the same scrollback as everything else. Without a visible frame
// there is no way to tell where the model stopped thinking and started
// answering, so the block is delimited explicitly. These are printed to the
// TERMINAL and never sent to a model, so they cost nothing in tokens.
const (
	thinkingOpenMarker  = "🦍🦍🦍 thinking"
	thinkingCloseMarker = "🦍🦍🦍 done thinking (hard job...) 💪"
)

// completeReasoningLines splits reasoning into the lines that will never change
// again.
//
// Reasoning is append-only: once a newline has arrived, the line before it is
// final. The text after the last newline is still being written and must not be
// printed, because printing cannot be taken back. Returns the settled lines.
func completeReasoningLines(thinking string) []string {
	idx := strings.LastIndex(thinking, "\n")
	if idx < 0 {
		return nil
	}
	return strings.Split(thinking[:idx], "\n")
}

// reasoningColor is what streamed thinking is printed in.
//
// GORILLA OVERRIDE: this deliberately does NOT use the theme's TextMuted. Muted
// is the right weight for text inside a rendered pane, but reasoning is now
// printed as ordinary terminal output on a black background, where it came out
// too dull to read comfortably. Cyan reads clearly against black and is instantly
// distinguishable from the answer.
//
// Still adaptive rather than a bare #00FFFF: pure cyan on a light background is
// close to invisible, and this text is the whole point of the feature. The dark
// value is the one that matters here; change either freely.
var reasoningColor = lipgloss.AdaptiveColor{Dark: "#00FFFF", Light: "#00707A"}

// styleReasoning renders reasoning so it reads as working-out, not as the answer.
func styleReasoning(s string) string {
	return styles.BaseStyle().Foreground(reasoningColor).Render(s)
}

// wrapReasoning breaks a reasoning line at word boundaries to fit the terminal.
//
// GORILLA FIX: a model emits one paragraph as ONE line with no newlines in it,
// often several hundred characters. Printed raw, the terminal hard-wraps it at
// the last column and splits words down the middle — "avail/able",
// "developm/ent", "the g/rep tool". The text was all there and readable only
// with effort, which defeats the point of printing it at all.
//
// Wrapped with wordwrap rather than lipgloss's Width(): Width() also PADS every
// line out to the full width, and trailing spaces on hundreds of reasoning lines
// end up in the user's clipboard the moment they select any of it. Scrollback
// text is meant to be copied, so it must not carry invisible padding.
func wrapReasoning(s string, width int) []string {
	if width <= 0 {
		return []string{s}
	}
	return strings.Split(wordwrap.String(s, width), "\n")
}

// emitReasoning prints reasoning lines for a message that is still arriving,
// opening the block on first sight.
//
// GORILLA OVERRIDE: this is the whole point of scrollback mode applied to
// thinking. Previously reasoning existed only inside a six-row preview that was
// overwritten every frame and never kept, so a model that thought for a minute
// gave you sixty seconds of scrolling text you could not read and could not
// scroll back to. Now each settled line is printed once, permanently, exactly
// like every other line of the conversation.
func (m *messagesCmp) emitReasoning(msg message.Message, upto []string) []tea.Cmd {
	// Lazily initialised: a nil map here would panic mid-transcript, after part
	// of the conversation had been written to the terminal and could not be
	// withdrawn. Same reasoning as messagesService above — degrade, never die.
	if m.reasonedLines == nil {
		m.reasonedLines = make(map[string]int)
	}
	if m.reasoningOpened == nil {
		m.reasoningOpened = make(map[string]bool)
	}

	var cmds []tea.Cmd
	if !m.reasoningOpened[msg.ID] {
		m.reasoningOpened[msg.ID] = true
		cmds = append(cmds, tea.Println(""), tea.Println(styleReasoning(thinkingOpenMarker)))
		// Match the gap the closing marker gets, WITHOUT relying on the provider.
		// Nemotron's reasoning happens to begin with a newline, which is why this
		// end already looked right; another provider's would not, and spacing
		// that changes with the backend is spacing nobody can rely on. Only add
		// the blank when the reasoning does not already start with one, so the
		// two cases converge on the same result instead of doubling up.
		if len(upto) == 0 || strings.TrimSpace(upto[0]) != "" {
			cmds = append(cmds, tea.Println(""))
		}
	}
	from := m.reasonedLines[msg.ID]
	if from >= len(upto) {
		return cmds
	}
	for _, line := range upto[from:] {
		// The watermark counts SOURCE lines, not printed ones, so wrapping one
		// source line into several printed lines cannot desynchronise it.
		for _, wrapped := range wrapReasoning(line, m.width) {
			cmds = append(cmds, tea.Println(styleReasoning(wrapped)))
		}
	}
	m.reasonedLines[msg.ID] = len(upto)
	return cmds
}

// streamReasoning emits whatever reasoning has settled into complete lines for a
// message still in flight.
func (m *messagesCmp) streamReasoning(msg message.Message) []tea.Cmd {
	thinking := msg.ReasoningContent().Thinking
	if strings.TrimSpace(thinking) == "" {
		return nil
	}
	return m.emitReasoning(msg, completeReasoningLines(thinking))
}

// flushReasoning closes out a message's reasoning as it settles: the trailing
// partial line is now final, so it is printed, and the block is closed.
//
// It also covers the provider that delivers all its reasoning at once on finish,
// having streamed none of it — the watermark is still 0, so everything is
// printed here. That is why the finished render must ALWAYS drop the reasoning
// quote (see RenderForScrollback): by this point it has been printed either way,
// and printing it again would duplicate the whole block.
func (m *messagesCmp) flushReasoning(msg message.Message) []tea.Cmd {
	thinking := strings.TrimRight(msg.ReasoningContent().Thinking, "\n")
	if strings.TrimSpace(thinking) == "" {
		return nil
	}
	cmds := m.emitReasoning(msg, strings.Split(thinking, "\n"))
	// GORILLA OVERRIDE: blank lines on BOTH sides of the closing marker.
	//
	// Three different kinds of text meet at this point — the model's private
	// working-out, the boundary marker, and the answer actually addressed to the
	// reader — and they were stacked with no separation at all. The opening
	// marker already gets a gap after it because reasoning streams tend to start
	// with a newline; the closing one got nothing, so the answer began on the
	// line immediately below the marker that exists to say it had ended.
	//
	// One blank line each side, not more: this is a transcript that scrolls, and
	// every decorative row is a row of real content pushed off the screen. The
	// gap is emitted HERE rather than left to the model's trailing newlines,
	// which are stripped above, so the spacing is the same for every provider.
	cmds = append(cmds,
		tea.Println(""),
		tea.Println(styleReasoning(thinkingCloseMarker)),
		tea.Println(""),
	)
	delete(m.reasonedLines, msg.ID)
	delete(m.reasoningOpened, msg.ID)
	return cmds
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
			// Not settled, but its reasoning may be: print the lines that can
			// no longer change, then stop so nothing overtakes this message.
			cmds = append(cmds, m.streamReasoning(msg)...)
			break
		}
		cmds = append(cmds, m.flushReasoning(msg)...)
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

// FooterView is the whole of what the transcript contributes to the screen when
// the alternate screen is off: the working indicator, and nothing else.
// Everything else has already been printed into the terminal.
//
// The returned string is ALWAYS exactly FooterReservedRows tall — padded, never
// shrunk — so bubbletea's cursor-up erase cannot over-reach into the printed
// scrollback above it.
func (m *messagesCmp) FooterView() string {
	content := ""
	if working := m.working(); strings.TrimSpace(working) != "" {
		content = working
	}

	rows := lipgloss.Height(content)
	if content == "" {
		rows = 0
	}
	if padding := FooterReservedRows - rows; padding > 0 {
		content = strings.TrimSuffix(strings.Repeat("\n", padding), "\n") + content
	}
	return content
}
