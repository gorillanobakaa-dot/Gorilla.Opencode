package chat

import "testing"

// The failure this guards, reported twice: a model caught in a tool loop leaves
// the user unable to change model or trim the tool set until it finishes. The
// allowlist exists so the commands that FIX a runaway stay reachable while it
// is running.
func TestCommandsThatFixARunawayStayReachableWhileBusy(t *testing.T) {
	mustWork := []string{
		"tasks", "kill", "agents", // stop the helpers
		"context", "loadout", "tokens", // trim the tools it is looping on
		"help", "commands",
	}
	for _, c := range mustWork {
		if !controlCommands[c] {
			t.Errorf("/%s must work while the agent is busy: it is one of the ways out of a runaway", c)
		}
	}
}

// These hand the terminal to a full-screen picker via tea.Exec. Running one
// while a stream is writing to the same terminal corrupts the display, so they
// must NOT be on the allowlist however convenient it would be.
func TestTerminalHandoverCommandsAreNotOnTheAllowlist(t *testing.T) {
	for _, c := range []string{"providers", "provider", "switch", "connection", "conn", "link"} {
		if controlCommands[c] {
			t.Errorf("/%s releases the terminal with tea.Exec; allowing it mid-stream corrupts the display", c)
		}
	}
	// Choosing a model calls CoderAgent.Update() and swaps the provider under a
	// request already in flight. Opening the dialog looks harmless; the
	// consequence is not.
	for _, c := range []string{"model", "models"} {
		if controlCommands[c] {
			t.Errorf("/%s mutates agent state mid-request; esc first, then switch", c)
		}
	}
}
