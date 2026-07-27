// GORILLA OVERRIDE: this file did not exist upstream. It lets individual slash
// commands be switched off, persisted in ConfigBase()/commands.json.
//
// Honest scoping: slash commands cost ZERO model tokens. They are TUI-local
// dispatch and never appear in the system prompt or a tool schema. Turning one
// off declutters the palette and the unknown-command hint; it does not reduce
// network traffic or spend. The token-bearing surfaces are tools (measured
// 200-850 each) and prompt sections.
//
// Core commands cannot be disabled. Switching off the command that re-enables
// things is a trap with no way out short of hand-editing JSON.
package config

import (
	"encoding/json"
	"os"
	"sort"
	"sync"
)

const commandsFileName = "commands.json"

var (
	commandsOnce  sync.Once
	commandsState map[string]bool // id -> disabled
	commandsMu    sync.RWMutex
)

func commandsPath() string { return ConfigBase() + "/" + commandsFileName }

func initCommands() {
	commandsOnce.Do(func() {
		commandsState = map[string]bool{}
		if data, err := os.ReadFile(commandsPath()); err == nil {
			var saved map[string]bool
			if json.Unmarshal(data, &saved) == nil {
				for k, v := range saved {
					commandsState[k] = v
				}
			}
		}
	})
}

// CommandEnabled reports whether a command should be offered.
//
// Unknown ids are ENABLED, matching LoadoutEnabled's rule: a command added in a
// later version is never silently missing because an old commands.json does not
// mention it.
func CommandEnabled(id string) bool {
	initCommands()
	commandsMu.RLock()
	defer commandsMu.RUnlock()
	return !commandsState[id]
}

// SetCommandDisabled switches a command off or on and persists.
func SetCommandDisabled(id string, disabled bool) {
	initCommands()
	commandsMu.Lock()
	if disabled {
		commandsState[id] = true
	} else {
		delete(commandsState, id) // absent == enabled; keeps the file minimal
	}
	commandsMu.Unlock()
	saveCommands()
}

// ToggleCommand flips a command's state and returns whether it is now enabled.
func ToggleCommand(id string) bool {
	enabled := CommandEnabled(id)
	SetCommandDisabled(id, enabled)
	return !enabled
}

// DisabledCommands lists the switched-off ids, sorted. Used by /reset to report
// how many differ from default before changing anything.
func DisabledCommands() []string {
	initCommands()
	commandsMu.RLock()
	defer commandsMu.RUnlock()
	var out []string
	for id, disabled := range commandsState {
		if disabled {
			out = append(out, id)
		}
	}
	sort.Strings(out)
	return out
}

// ResetCommands re-enables everything.
func ResetCommands() {
	initCommands()
	commandsMu.Lock()
	commandsState = map[string]bool{}
	commandsMu.Unlock()
	saveCommands()
}

func saveCommands() {
	commandsMu.RLock()
	data, _ := json.MarshalIndent(commandsState, "", " ")
	commandsMu.RUnlock()
	_ = writeSecretFile(commandsPath(), data)
}
