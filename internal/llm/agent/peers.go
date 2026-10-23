// GORILLA (2026-10-10): where session messaging (internal/peers) meets the
// agent. Two things happen here and nowhere else:
//
//  1. Busy and idle. The main conversation's turns are what peers see as this
//     session being busy; the end of a turn is what sends a waiting idle
//     notice.
//  2. Delivery. Messages received since the last turn are put in front of the
//     user's text, each inside a fence that names the sender and says it is not
//     the user's instruction and grants no permission. (Until 2026-10-23 the
//     turn was also marked as having read untrusted content; that mark is
//     gone.) Every front end (the full interface, plain
//     mode, editor mode) sends its prompt through agent.Run, so this is the one
//     shared place.
//
// What delivery does NOT do is the point: it never answers a permission
// question, never switches on auto-approve, never changes a setting. Those
// are reached only by the person's own keys and commands; a message is text in
// the conversation and nothing else. TestAPeerMessageGrantsNothing holds that.
package agent

import (
	"github.com/opencode-ai/opencode/internal/peers"
)

// peerSetBusy tells peers whether this session is in a turn.
func peerSetBusy(busy bool) {
	if s := peers.Active(); s != nil {
		s.SetBusy(busy)
	}
}

// takePeerMessages returns the block that opens this turn's user message and
// the messages themselves, so a turn that fails before the AI sees them can put
// them back. Both empty when nothing has arrived or messaging is not running.
// (Until 2026-10-23 it also returned a taint reason; see agent.go.)
func takePeerMessages() (block string, msgs []peers.Message) {
	s := peers.Active()
	if s == nil {
		return "", nil
	}
	msgs = s.TakeForAI()
	if len(msgs) == 0 {
		return "", nil
	}
	return peers.FormatForAI(msgs), msgs
}

// requeuePeerMessages puts taken messages back when the turn failed before the
// AI could see them.
func requeuePeerMessages(msgs []peers.Message) {
	if s := peers.Active(); s != nil && len(msgs) > 0 {
		s.Requeue(msgs)
	}
}

// withPeerBlock puts the fenced messages in front of the user's own text.
func withPeerBlock(block, content string) string {
	if block == "" {
		return content
	}
	return block + content
}
