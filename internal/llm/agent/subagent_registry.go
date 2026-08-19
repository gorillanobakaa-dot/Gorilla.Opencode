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

	"github.com/opencode-ai/opencode/internal/permission"
	"github.com/opencode-ai/opencode/internal/pubsub"
)

// SubAgentState is where a helper is in its life.
//
// GORILLA OVERRIDE 2026-08-14: the registry used to have no state at all —
// being IN it meant "running", and a helper waiting for a concurrency slot was
// simply not in it yet. That is how a user who asked for 10 helpers saw four in
// /tasks and reasonably concluded the feature was broken. Six were alive and
// queued, and nothing in the app could see them.
//
// Worse, they could not be killed. KillAllSubAgents walks the registry, so the
// Nuclear Option cancelled the four holding slots, which released those slots,
// which let the next four start. Pressing it did not stop a research run.
//
// A helper is now registered the moment it EXISTS, carrying its state, so the
// list always shows the whole run and every row is killable whether or not it
// has started.
type SubAgentState int

const (
	SubAgentQueued  SubAgentState = iota // alive, waiting for a concurrency slot
	SubAgentRunning                      // holding a slot, model working
	SubAgentDone                         // finished and returned a result
	SubAgentFailed                       // errored out (rate limit, provider, timeout)
	SubAgentKilled                       // cancelled by the user
)

// Marker is the two-glyph status badge: a gorilla plus a signal.
//
// The gorilla is the constant that makes a helper row scannable at a glance;
// the second glyph carries the state. Both are exactly two cells wide, so every
// marker is four cells and rows stay aligned — a variable-width badge would put
// a line over the terminal width, which is the documented root cause of the
// footer-drift bug in CLAUDE.md.
//
// The glyph is NEVER the only carrier. Label() ships beside it in every render,
// because the reference machine is a 2012 laptop on Debian whose terminal font
// may have no emoji coverage at all — and a user who sees four identical boxes
// must still be able to read what is happening.
func (s SubAgentState) Marker() string {
	switch s {
	case SubAgentQueued:
		return "\U0001F98D\U0001F7E1" // gorilla + yellow circle: waiting its turn
	case SubAgentRunning:
		return "\U0001F98D\U0001F7E2" // gorilla + green circle: working now
	case SubAgentDone:
		return "\U0001F98D\U0001F535" // gorilla + blue circle: finished, result in
	case SubAgentFailed:
		return "\U0001F98D\U0001F534" // gorilla + red circle: it broke
	case SubAgentKilled:
		return "\U0001F98D\U0001F6D1" // gorilla + stop sign: you stopped it
	}
	return "\U0001F98D\U000026AA"
}

// Label is the word for the state. Always rendered next to Marker.
func (s SubAgentState) Label() string {
	switch s {
	case SubAgentQueued:
		return "QUEUED"
	case SubAgentRunning:
		return "RUNNING"
	case SubAgentDone:
		return "DONE"
	case SubAgentFailed:
		return "FAILED"
	case SubAgentKilled:
		return "KILLED"
	}
	return "UNKNOWN"
}

// Live reports whether this helper is still consuming or about to consume
// quota. Only live helpers count toward the status-bar total and only they are
// worth killing.
func (s SubAgentState) Live() bool {
	return s == SubAgentQueued || s == SubAgentRunning
}

// SubAgentInfo is a snapshot of one live helper agent. Safe to copy/share with
// the UI (carries no cancel func or locks).
type SubAgentInfo struct {
	ID              string        // short, stable handle shown in /tasks (e.g. "a3")
	SessionID       string        // the helper's own task session
	ParentSessionID string        // the coder session that spawned it
	ToolCallID      string        // the agent tool call that created it
	Prompt          string        // the task the helper was given
	StartedAt       time.Time     // spawn time, for elapsed display
	State           SubAgentState // queued / running / done / failed / killed
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
	return RegisterSubAgentState(sessionID, parentSessionID, toolCallID, prompt, SubAgentRunning, cancel)
}

// RegisterSubAgentState is RegisterSubAgent with the starting state named.
// Research helpers register as SubAgentQueued BEFORE they wait on the
// concurrency semaphore, so the whole run is visible and killable from the
// instant it is scheduled rather than only once it starts.
func RegisterSubAgentState(sessionID, parentSessionID, toolCallID, prompt string, state SubAgentState, cancel context.CancelFunc) SubAgentInfo {
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
		State:           state,
	}
	subAgentReg[id] = &subAgentEntry{info: info, cancel: cancel}
	subAgentRegMu.Unlock()

	subAgentBroker.Publish(pubsub.CreatedEvent, info)
	return info
}

// SetSubAgentState moves a helper to a new state and tells the UI.
//
// Kept separate from Unregister so a finished helper can be SHOWN as finished
// instead of vanishing: a row that disappears the moment it completes gives the
// user no way to tell "it answered" from "it was never there", which is exactly
// the ambiguity that made a 10-helper run look like a 4-helper one.
func SetSubAgentState(id string, state SubAgentState) {
	subAgentRegMu.Lock()
	entry, ok := subAgentReg[id]
	if ok {
		entry.info.State = state
	}
	subAgentRegMu.Unlock()

	if ok {
		subAgentBroker.Publish(pubsub.UpdatedEvent, entry.info)
	}
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
// Counts LIVE helpers only — queued and running. A finished row lingers in
// /tasks so the user can see it landed, but it is not still costing them
// anything and must not be counted as if it were.
func ActiveSubAgentCount() int {
	subAgentRegMu.Lock()
	n := 0
	for _, e := range subAgentReg {
		if e.info.State.Live() {
			n++
		}
	}
	subAgentRegMu.Unlock()
	return n
}

// SubAgentStateCounts breaks the registry down by state, for a status line that
// can say "4 running, 6 queued" instead of a single ambiguous number.
func SubAgentStateCounts() map[SubAgentState]int {
	subAgentRegMu.Lock()
	out := map[SubAgentState]int{}
	for _, e := range subAgentReg {
		out[e.info.State]++
	}
	subAgentRegMu.Unlock()
	return out
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
	entry.info.State = SubAgentKilled
	entry.cancel()
	// GORILLA FIX (2026-08-17): cancelling the context is not enough. A tool
	// waiting on a permission prompt is parked on a channel and never looks at
	// ctx, so a killed helper stayed parked until the wait expired. Release it
	// here so "kill" means now.
	permission.CancelForSession(entry.info.SessionID)
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
		e.info.State = SubAgentKilled
		e.cancel()
		permission.CancelForSession(e.info.SessionID)
		subAgentBroker.Publish(pubsub.DeletedEvent, e.info)
	}
	return len(entries)
}

// shortHandle turns a spawn sequence number into a compact, typeable id:
// a1, a2, ... a9, then a10, a11, ... Stable for the life of the process.
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

// HeartbeatState summarises what the helpers are doing, for the "still alive"
// notice. Returns how many are running, how long the longest has been at it,
// and how long since ANY of them last changed state.
//
// GORILLA OVERRIDE (2026-08-17): built after a lane went silent for 23 minutes
// and came back with 19,118 tokens. It was thinking the whole time. Even with
// direct database access the difference between "grinding" and "hung" was not
// visible; a user watching a still screen has no chance at all. On a slow model
// over a slow link — a satellite uplink at single-digit KB/s, which is the
// owner's actual field experience — a healthy run looks exactly like a crash.
func HeartbeatState() (running int, longest time.Duration, quiet time.Duration) {
	subAgentRegMu.Lock()
	defer subAgentRegMu.Unlock()
	now := time.Now()
	newest := time.Time{}
	for _, e := range subAgentReg {
		if e.info.State != SubAgentRunning && e.info.State != SubAgentQueued {
			continue
		}
		running++
		if d := now.Sub(e.info.StartedAt); d > longest {
			longest = d
		}
		if e.info.StartedAt.After(newest) {
			newest = e.info.StartedAt
		}
	}
	if !newest.IsZero() {
		quiet = now.Sub(newest)
	}
	return running, longest, quiet
}
