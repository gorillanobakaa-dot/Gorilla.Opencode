package chat

import "testing"

// The kill switch must be reachable while work is running. /tasks was refused
// with "Agent is working, please wait..." — the exact moment it exists for.
func TestControlCommandsIncludeTheKillSwitch(t *testing.T) {
	for _, name := range []string{"tasks", "task", "agents", "kill"} {
		if !controlCommands[name] {
			t.Errorf("/%s must work while the agent is busy; it is how a user stops helpers", name)
		}
	}
}

// Anything that SENDS to the model must still be gated, or a second request
// interleaves with one already in flight.
func TestSendingCommandsAreStillGated(t *testing.T) {
	for _, name := range []string{"init", "compact", "clear", "model", "models", "connect", "export"} {
		if controlCommands[name] {
			t.Errorf("/%s reaches the model or mutates session state; it must stay behind the busy guard", name)
		}
	}
}
