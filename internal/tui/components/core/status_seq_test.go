package core

// GORILLA FIX (2026-08-18): the stale-timer guard. Reported as "footer messages
// vanish quite fast these days" — not the TTL (10s), but an older message's
// clear-timer wiping a newer message before its own time was up.

import (
	"testing"
	"time"

	"github.com/opencode-ai/opencode/internal/tui/util"
)

// A clear armed by an earlier message must NOT dismiss a later one; only the
// clear matching the current message may.
func TestAStaleClearDoesNotWipeANewerMessage(t *testing.T) {
	m := statusCmp{messageTTL: 10 * time.Second}

	// Message A (seq -> 1), then B (seq -> 2). Each Update returns a clear cmd
	// stamped with its own seq; we simulate those timers firing by hand rather
	// than waiting 10 real seconds.
	u, _ := m.Update(util.InfoMsg{Type: util.InfoTypeInfo, Msg: "A"})
	m = u.(statusCmp)
	u, _ = m.Update(util.InfoMsg{Type: util.InfoTypeInfo, Msg: "B"})
	m = u.(statusCmp)

	if m.info.Msg != "B" {
		t.Fatalf("setup: expected B showing, got %q", m.info.Msg)
	}

	// A's timer fires late (seq 1). Before the fix this wiped B; now it is ignored.
	u, _ = m.Update(util.ClearStatusMsg{Seq: 1})
	m = u.(statusCmp)
	if m.info.Msg != "B" {
		t.Errorf("a stale clear from message A wiped the newer message B — this is the "+
			"'messages vanish fast' bug; info=%q", m.info.Msg)
	}

	// B's own timer fires (seq 2). This one matches, so it clears.
	u, _ = m.Update(util.ClearStatusMsg{Seq: 2})
	m = u.(statusCmp)
	if m.info.Msg != "" {
		t.Errorf("the matching clear did not dismiss the message: info=%q", m.info.Msg)
	}
}

// Every message must still schedule SOME clear — the fix must not leave a
// message pinned forever.
func TestEveryMessageStillSchedulesAClear(t *testing.T) {
	m := statusCmp{messageTTL: 10 * time.Second}
	_, cmd := m.Update(util.InfoMsg{Type: util.InfoTypeInfo, Msg: "x"})
	if cmd == nil {
		t.Fatal("no clear scheduled; the message would stay pinned forever")
	}
}
