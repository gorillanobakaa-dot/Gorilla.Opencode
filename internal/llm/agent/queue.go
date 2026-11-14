package agent

// GORILLA OVERRIDE (2026-11-13): a message typed while the agent is working
// used to be refused (ErrSessionBusy) and the text was lost or had to be
// retyped. It is now queued and handed to the model at the next tool
// boundary (after the tool results of the running step), or right after the
// turn if no tool step is left. Esc never loses it either: whatever is still
// queued when a turn is cancelled or fails is returned by TakeQueued.

import (
	"context"
	"strings"
	"sync"
)

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
	// GORILLA OVERRIDE (2026-11-14): "stop" typed while the agent works stops
	// the run at once; it is not queued behind the run it is meant to end.
	if IsStopWord(text) {
		a.Cancel(sessionID)
		return true
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

// IsStopWord: the whole message is a request to stop.
func IsStopWord(text string) bool {
	switch strings.ToLower(strings.Trim(strings.TrimSpace(text), ".!? ")) {
	case "stop", "halt", "abort", "cancel", "stop it", "stop now", "oprește", "opreste", "stai":
		return true
	}
	return false
}

// SteerPrefix marks a message typed while the agent worked. The model reads it
// at the next tool boundary and must answer it before going on.
const SteerPrefix = "[OWNER STEER, typed while you were working. Answer this FIRST: change your next action to follow it, " +
	"and drop the old plan where it conflicts. Do not carry on as before without acknowledging it.]\n"

// toolSlot is the tool call a session is running right now, so Esc can stop
// that one call and leave the run alive.
type toolSlot struct {
	cancel      context.CancelFunc
	interrupted bool
}

var (
	toolMu    sync.Mutex
	toolSlots = map[string]*toolSlot{}
)

func setRunningTool(sessionID string, cancel context.CancelFunc) {
	toolMu.Lock()
	toolSlots[sessionID] = &toolSlot{cancel: cancel}
	toolMu.Unlock()
}

// finishRunningTool clears the slot and says whether Esc stopped the tool.
func finishRunningTool(sessionID string) bool {
	toolMu.Lock()
	defer toolMu.Unlock()
	s := toolSlots[sessionID]
	delete(toolSlots, sessionID)
	return s != nil && s.interrupted
}

// InterruptTool stops the tool call running for the session (Esc). It returns
// false when no tool is running (the model is writing): the caller then cancels
// the whole run as before. The run continues: the model is told the owner
// stopped the tool and asks whether to continue or change.
func InterruptTool(sessionID string) bool {
	toolMu.Lock()
	defer toolMu.Unlock()
	s := toolSlots[sessionID]
	if s == nil || s.cancel == nil {
		return false
	}
	s.interrupted = true
	s.cancel()
	return true
}

// InterruptedToolResult is what the model reads after Esc stopped its tool.
const InterruptedToolResult = "The owner pressed Esc: this tool call was STOPPED before it finished (its output, if any, is partial). " +
	"Do not run anything else yet. Ask the owner one short question: continue with the previous plan, or change it? " +
	"Then wait for the answer."
