// GORILLA OVERRIDE: this file did not exist upstream.
//
// It exposes the transcript pane's own message rendering so a FINISHED message
// can be handed to tea.Println and land in the terminal's real scrollback,
// instead of being painted into a screen buffer the terminal keeps no history of.
//
// Why reuse rather than reimplement. The obvious shortcut is a second, plainer
// formatter for scrollback — and it would drift. The pane already knows how to
// show reasoning as a quote, how to lay tool calls out, which of the display
// extras are on, and how to stamp a time; a parallel formatter means every one of
// those decisions has two homes and eventually two answers. So the same
// renderUserMessage / renderAssistantMessage that fills the pane fills the
// scrollback, and the only new logic is joining their output.
package chat

import (
	"strings"

	"github.com/opencode-ai/opencode/internal/message"
)

// RenderForScrollback renders one message exactly as the transcript pane would,
// wrapped to width, ready to be printed into the terminal's own output.
//
// allMessages and svc are needed for the same reason the pane needs them: an
// assistant message's tool results live in sibling messages, and sub-agent task
// output is fetched through the service. Passing nil for svc is allowed and
// simply renders whatever does not require it.
//
// The returned string has no trailing newline: tea.Println adds the line break,
// and adding another here would open a blank line between every message.
func RenderForScrollback(
	msg message.Message,
	msgIndex int,
	allMessages []message.Message,
	svc message.Service,
	width int,
) string {
	if width <= 0 {
		return ""
	}

	var blocks []string
	switch msg.Role {
	case message.User:
		// Never focused: focus is a cursor in a scrollable pane, and printed text
		// has no cursor. Position is only used for click targeting, so it is 0.
		blocks = append(blocks, renderUserMessage(msg, false, width, 0).content)
	case message.Assistant:
		for _, m := range renderAssistantMessage(msg, msgIndex, allMessages, svc, "", false, width, 0) {
			if strings.TrimSpace(m.content) != "" {
				blocks = append(blocks, m.content)
			}
		}
	default:
		// Tool and system messages reach the pane only through the assistant
		// message that owns them, so rendering one on its own would duplicate it.
		return ""
	}

	return strings.TrimRight(strings.Join(blocks, "\n"), "\n")
}

// ScrollbackReady reports whether a message is settled enough to print.
//
// This is the whole correctness question for printing into scrollback: output
// cannot be taken back. The transcript pane can re-render a message on every
// token because it owns its buffer, but a line printed to the terminal is gone —
// so a message may only be printed once it will not change again.
//
// A user message is complete the moment it exists. An assistant message is not
// complete until it reports finished, because until then both its text and its
// tool calls are still arriving.
func ScrollbackReady(msg message.Message) bool {
	switch msg.Role {
	case message.User:
		return true
	case message.Assistant:
		return msg.IsFinished()
	default:
		return false
	}
}
