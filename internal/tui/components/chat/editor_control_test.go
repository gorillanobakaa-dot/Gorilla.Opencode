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
	for _, name := range []string{"init", "compact", "model", "models", "connect", "export"} {
		if controlCommands[name] {
			t.Errorf("/%s reaches the model or mutates session state; it must stay behind the busy guard", name)
		}
	}
}

// GORILLA OVERRIDE (2026-11-21): /yolo, /plan, /compact, and /clear are live
// commands. They stay OUT of controlCommands (the test above) because each has
// explicit mid-turn handling in the TUI (/clear cancels, /compact queues until
// turn end, /yolo and /plan toggle mode without sending to the model).
func TestLiveCommands(t *testing.T) {
	live := []struct{ word, args string }{
		{"yolo", ""}, {"yolo", "off"}, {"yolo", "OFF"}, {"auto", ""}, {"autopilot", "off"},
		{"goal", ""}, {"clear", ""}, {"new", ""},
		{"compact", ""}, {"summarize", ""}, {"summarise", ""},
		{"plan", ""}, {"plan", "off"}, {"plan", "OFF"}, {"plan", "exit"},
	}
	for _, c := range live {
		if !isLiveCommand(c.word, c.args) {
			t.Errorf("/%s %s must work while the agent is busy", c.word, c.args)
		}
	}
	// Task-sending forms still wait for an idle agent.
	for _, c := range []struct{ word, args string }{
		{"yolo", "fix the build"}, {"goal", "ship it"}, {"plan", "refactor auth"},
		{"plan", "go"}, {"model", ""}, {"init", ""},
	} {
		if isLiveCommand(c.word, c.args) {
			t.Errorf("/%s %s sends or reconfigures the model's work; it must stay behind the busy guard", c.word, c.args)
		}
	}
}
