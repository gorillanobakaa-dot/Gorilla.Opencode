package config

import "strings"

// GORILLA (2026-11-23): the work mode is the owner's session-start question
// (Economy / Time-Sensitive / Fast) expressed as the helper leash, so there is
// one dial and one file (subagents.json), not two. It is a named shortcut for
// SetMaxSubAgents; /context still steps the same value.
//
// It is a command (/work) and not a start-up dialog on purpose: the start-up
// offer already had to be taught not to swallow keys typed for the chat
// (tui/init_offer.go), and a second modal at launch would repeat that bug.
// The status of the default is instead one line in /work.

// WorkMode is a named helper limit.
type WorkMode struct {
	Name    string
	Aliases []string
	Max     int
	Layman  string
}

// WorkModes are the owner's three modes. Economy is the default.
var WorkModes = []WorkMode{
	{"economy", []string{"solo", "0"}, 0, "solo, no helper agents, cheapest"},
	{"time-sensitive", []string{"timed", "6"}, 6, "up to 6 helper agents at once"},
	{"fast", []string{"1000"}, 1000, "up to 1000 helper agents, spends the most"},
}

// ParseWorkMode finds a mode by name, alias or number; nil when unknown.
func ParseWorkMode(s string) *WorkMode {
	s = strings.ToLower(strings.TrimSpace(s))
	for i := range WorkModes {
		m := &WorkModes[i]
		if m.Name == s {
			return m
		}
		for _, a := range m.Aliases {
			if a == s {
				return m
			}
		}
	}
	return nil
}

// CurrentWorkMode names the mode the leash matches, or "custom" for any other
// value set in /context.
func CurrentWorkMode() string {
	cur := MaxSubAgents()
	for _, m := range WorkModes {
		if m.Max == cur {
			return m.Name
		}
	}
	return "custom"
}

// SetWorkMode applies a mode to the leash.
func SetWorkMode(m WorkMode) { SetMaxSubAgents(m.Max) }
