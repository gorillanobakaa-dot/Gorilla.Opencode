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

// GORILLA OVERRIDE (2026-11-21): /yolo and /clear are live commands. Forgetting
// /yolo used to cost the whole turn (Esc); /clear was refused while a run was
// going wrong. They stay OUT of controlCommands (the test above) because /clear
// must cancel the turn it abandons, which the TUI handler does.
func TestLiveCommands(t *testing.T) {
	live := []struct{ word, args string }{
		{"yolo", ""}, {"yolo", "off"}, {"yolo", "OFF"}, {"auto", ""}, {"autopilot", "off"},
		{"goal", ""}, {"clear", ""}, {"new", ""},
	}
	for _, c := range live {
		if !isLiveCommand(c.word, c.args) {
			t.Errorf("/%s %s must work while the agent is busy", c.word, c.args)
		}
	}
	// "/yolo <task>" starts new work, so it still waits for an idle agent.
	for _, c := range []struct{ word, args string }{
		{"yolo", "fix the build"}, {"goal", "ship it"}, {"compact", ""}, {"model", ""}, {"init", ""},
	} {
		if isLiveCommand(c.word, c.args) {
			t.Errorf("/%s %s sends or reconfigures the model's work; it must stay behind the busy guard", c.word, c.args)
		}
	}
}
