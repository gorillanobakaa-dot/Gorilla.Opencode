package agent

// GORILLA OVERRIDE (2026-11-13): a message typed while the agent is working
// used to be refused (ErrSessionBusy) and the text was lost or had to be
// retyped. It is now queued and handed to the model at the next tool
// boundary (after the tool results of the running step), or right after the
// turn if no tool step is left. Esc never loses it either: whatever is still
// queued when a turn is cancelled or fails is returned by TakeQueued.

import "sync"

var (
	queueMu sync.Mutex
	queued  = map[string][]string{}
)

// QueueUserMessage queues text for a session that is mid-turn. It returns
// false when the session is idle: the caller then sends the text normally.
func QueueUserMessage(a Service, sessionID, text string) bool {
	if text == "" || !a.IsSessionBusy(sessionID) {
		return false
	}
	queueMu.Lock()
	queued[sessionID] = append(queued[sessionID], text)
	queueMu.Unlock()
	return true
}

// TakeQueued removes and returns what is queued for the session.
func TakeQueued(sessionID string) []string {
	queueMu.Lock()
	defer queueMu.Unlock()
	q := queued[sessionID]
	delete(queued, sessionID)
	return q
}

// QueuedCount says how many messages wait for the session.
func QueuedCount(sessionID string) int {
	queueMu.Lock()
	defer queueMu.Unlock()
	return len(queued[sessionID])
}
