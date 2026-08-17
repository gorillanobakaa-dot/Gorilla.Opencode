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
		for _, m := range renderAssistantMessage(msg, msgIndex, allMessages, svc, "", false, width, 0, true) {
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
// ScrollbackSettled is ScrollbackReady plus one more condition: everything the
// message will DISPLAY must exist too.
//
// GORILLA FIX (2026-08-17): an assistant message reports finished the moment
// its stream ends — but its tool RESULTS arrive later, in a separate tool
// message. Printing at IsFinished meant every tool-using turn rendered its
// calls with the response==nil branch, so the terminal's permanent history
// read "Waiting for response..." forever and what the tool actually returned
// was never visible anywhere. Observed live on the crocodile search
// (2026-08-17, v0.1.88~test1): the find call and its 32 KB result were in the
// database and absent from the screen. Scrollback cannot be reprinted, so the
// only correct moment to print a tool-using turn is after its results exist.
//
// The exception list is the anti-stall belt (see the 2026-07-30 corpse below):
// a turn that ended abnormally — canceled, errored, permission-denied — may
// legitimately never get results for every call, and the transcript must not
// wait forever behind it. Those print immediately; their tool calls render
// with whatever state they reached, which is the truthful account of that turn.
func ScrollbackSettled(msg message.Message, all []message.Message) bool {
	if !ScrollbackReady(msg) {
		return false
	}
	if msg.Role != message.Assistant {
		return true
	}
	switch msg.FinishReason() {
	case message.FinishReasonCanceled, message.FinishReasonError, message.FinishReasonPermissionDenied:
		return true
	}
	for _, tc := range msg.ToolCalls() {
		if findToolResponse(tc.ID, all) == nil {
			return false
		}
	}
	return true
}

func ScrollbackReady(msg message.Message) bool {
	switch msg.Role {
	case message.User:
		return true
	case message.Assistant:
		return msg.IsFinished()
	default:
		// GORILLA FIX: tool and system messages are SETTLED, not unsettled.
		//
		// This returned false, and printPending BREAKS on the first message that
		// is not ready — so the first tool result halted the transcript
		// permanently. Every later message, including the model's finished
		// answer, was generated, stored in the database and never shown.
		//
		// Observed 2026-07-30: a bash call returned in two seconds, the model
		// answered in full, and the screen sat on "Waiting for response..." for
		// fifteen minutes. It looked exactly like the provider had hung. Nothing
		// had hung — the printer had stopped, and every tool-using conversation
		// had been silently truncated at its first tool call.
		//
		// "Ready" here means "will not change again", not "has something to
		// show". A tool result is complete the moment it exists, exactly like a
		// user message. It still renders to nothing — RenderForScrollback
		// returns "" for these roles because their output reaches the screen
		// through the assistant message that owns them — and printPending skips
		// empty output while marking it printed, so the loop moves past it
		// instead of stopping dead.
		return true
	}
}
