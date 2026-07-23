// GORILLA OVERRIDE: this file did not exist upstream. It makes sub-agent
// (helper) spawns *transparent and killable*, in line with the Gorilla policy
// that the user must always be able to SEE what agents are running on their
// behalf and STOP them — one by one, or all at once.
//
// Upstream, the `agent` tool spawns a helper synchronously inside a throwaway
// NewAgent instance with its own private activeRequests map, so the main coder
// agent had no way to enumerate or cancel a running helper. This registry is
// the missing shared, process-wide view: every live helper registers here with
// its own cancel func, so the TUI can list them (/tasks), kill a single one, or
// invoke the Nuclear Option and kill them all.
package agent

import (
	"context"
	"sort"
	"sync"
	"time"

	"github.com/opencode-ai/opencode/internal/pubsub"
)

// SubAgentInfo is a snapshot of one live helper agent. Safe to copy/share with
// the UI (carries no cancel func or locks).
type SubAgentInfo struct {
	ID              string    // short, stable handle shown in /tasks (e.g. "a3")
	SessionID       string    // the helper's own task session
	ParentSessionID string    // the coder session that spawned it
	ToolCallID      string    // the agent tool call that created it
	Prompt          string    // the task the helper was given
	StartedAt       time.Time // spawn time, for elapsed display
}

type subAgentEntry struct {
	info   SubAgentInfo
	cancel context.CancelFunc
}

var (
	subAgentRegMu sync.Mutex
	subAgentReg   = map[string]*subAgentEntry{}
	subAgentSeq   int
	// subAgentBroker fans out spawn (Created) / exit (Deleted) events so the
	// TUI can show live state (status-bar count, /tasks list, spawn toast).
	subAgentBroker = pubsub.NewBroker[SubAgentInfo]()
)

// SubAgentSubscribe matches the setupSubscriber signature so the TUI can wire it
// alongside the other service subscriptions in cmd/root.go.
func SubAgentSubscribe(ctx context.Context) <-chan pubsub.Event[SubAgentInfo] {
	return subAgentBroker.Subscribe(ctx)
}

// RegisterSubAgent records a newly-spawned helper and returns its handle plus a
// cancel func that also removes it from the registry. Call UnregisterSubAgent
// (or the returned func) when the helper finishes.
func RegisterSubAgent(sessionID, parentSessionID, toolCallID, prompt string, cancel context.CancelFunc) SubAgentInfo {
	subAgentRegMu.Lock()
	subAgentSeq++
	id := shortHandle(subAgentSeq)
	info := SubAgentInfo{
		ID:              id,
		SessionID:       sessionID,
		ParentSessionID: parentSessionID,
		ToolCallID:      toolCallID,
		Prompt:          prompt,
		StartedAt:       time.Now(),
	}
	subAgentReg[id] = &subAgentEntry{info: info, cancel: cancel}
	subAgentRegMu.Unlock()

	subAgentBroker.Publish(pubsub.CreatedEvent, info)
	return info
}

// UnregisterSubAgent removes a helper from the registry (called when it exits
// normally). Publishing a Deleted event lets the UI refresh live.
func UnregisterSubAgent(id string) {
	subAgentRegMu.Lock()
	entry, ok := subAgentReg[id]
	if ok {
		delete(subAgentReg, id)
	}
	subAgentRegMu.Unlock()

	if ok {
		subAgentBroker.Publish(pubsub.DeletedEvent, entry.info)
	}
}

// ListSubAgents returns a snapshot of all live helpers, oldest first.
func ListSubAgents() []SubAgentInfo {
	subAgentRegMu.Lock()
	out := make([]SubAgentInfo, 0, len(subAgentReg))
	for _, e := range subAgentReg {
		out = append(out, e.info)
	}
	subAgentRegMu.Unlock()

	sort.Slice(out, func(i, j int) bool { return out[i].StartedAt.Before(out[j].StartedAt) })
	return out
}

// ActiveSubAgentCount is the cheap read used by the status bar every frame.
func ActiveSubAgentCount() int {
	subAgentRegMu.Lock()
	n := len(subAgentReg)
	subAgentRegMu.Unlock()
	return n
}

// KillSubAgent cancels a single helper by its handle. Returns false if the
// handle is unknown (already finished/killed). The entry is removed here so a
// second kill is a no-op; the helper's own defer will publish the Deleted event.
func KillSubAgent(id string) (SubAgentInfo, bool) {
	subAgentRegMu.Lock()
	entry, ok := subAgentReg[id]
	if ok {
		delete(subAgentReg, id)
	}
	subAgentRegMu.Unlock()

	if !ok {
		return SubAgentInfo{}, false
	}
	entry.cancel()
	subAgentBroker.Publish(pubsub.DeletedEvent, entry.info)
	return entry.info, true
}

// KillAllSubAgents is the Nuclear Option: cancel every live helper. Returns how
// many were killed.
func KillAllSubAgents() int {
	subAgentRegMu.Lock()
	entries := make([]*subAgentEntry, 0, len(subAgentReg))
	for id, e := range subAgentReg {
		entries = append(entries, e)
		delete(subAgentReg, id)
	}
	subAgentRegMu.Unlock()

	for _, e := range entries {
		e.cancel()
		subAgentBroker.Publish(pubsub.DeletedEvent, e.info)
	}
	return len(entries)
}

// shortHandle turns a spawn sequence number into a compact, typeable id:
// a1, a2, … a9, then a10, a11, … Stable for the life of the process.
func shortHandle(seq int) string {
	return "a" + itoa(seq)
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}
