// GORILLA OVERRIDE: this file did not exist upstream. It implements the
// "context loadout" — a Slackware-style, total-control view of everything
// this agent sends to the model on EVERY turn, with the approximate token
// cost of each piece and the ability to switch any of it off.
//
// Philosophy (see PHILOSOPHY.md): radical transparency and total user
// control. You can see exactly what you are paying for just to say "yo",
// and you can strip it to the bone. Past a point that lobotomises the
// agent — that is your call, and a one-key reset brings the defaults back.
package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
)

// LoadoutComponent is one switchable piece of per-turn context.
type LoadoutComponent struct {
	ID       string // stable key, also used by the tool/prompt gates
	Name     string // human label
	Tradeoff string // what you lose if you turn it OFF
	Tokens   int    // approximate tokens this adds to EVERY turn
	Default  bool   // shipped-on by default
	Critical bool   // turning this off significantly lobotomises the agent
}

// LoadoutComponents is the registry. Token figures are approximate
// (measured from description/schema sizes, ~4 chars/token) and exist to
// inform a decision, not to bill anyone.
var LoadoutComponents = []LoadoutComponent{
	{"tool.bash", "Bash tool", "agent can't run shell commands (build, test, git, run anything)", 1200, true, true},
	{"tool.edit", "Edit tool", "agent can't modify files in place", 1300, true, true},
	{"tool.write", "Write tool", "agent can't create or overwrite files", 350, true, true},
	{"tool.view", "View tool", "agent can't read file contents", 500, true, true},
	{"tool.ls", "Ls tool", "agent can't list directories", 450, true, false},
	{"tool.grep", "Grep tool", "agent can't search file contents", 600, true, false},
	{"tool.glob", "Glob tool", "agent can't find files by name pattern", 400, true, false},
	{"tool.patch", "Patch tool", "agent loses multi-hunk patch edits (edit/write still work)", 900, true, false},
	{"tool.fetch", "Fetch tool", "agent can't fetch URLs", 300, true, false},
	{"tool.diagnostics", "Diagnostics tool", "agent can't read LSP errors/warnings", 400, true, false},
	{"tool.agent", "Sub-agent tool", "agent can't spawn read-only search sub-agents", 200, true, false},
	{"tool.sourcegraph", "Sourcegraph tool", "agent can't search public code on the web", 1000, false, false},
	{"prompt.env", "Environment info", "agent won't be told your cwd, OS, or git status", 150, true, false},
	{"prompt.lsp", "LSP info", "agent won't be told which language servers are active", 100, true, false},
}

const (
	loadoutFileName    = "loadout.json"
	systemPromptTokens = 3000 // the base coder prompt; always sent, not switchable
)

var (
	loadoutOnce  sync.Once
	loadoutState map[string]bool // id -> enabled (only entries that differ or are set)
	loadoutMu    sync.RWMutex
)

func loadoutPath() string {
	if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" {
		return filepath.Join(xdg, appName, loadoutFileName)
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", appName, loadoutFileName)
}

func initLoadout() {
	loadoutOnce.Do(func() {
		loadoutState = map[string]bool{}
		// start from defaults
		for _, c := range LoadoutComponents {
			loadoutState[c.ID] = c.Default
		}
		// overlay persisted overrides
		if data, err := os.ReadFile(loadoutPath()); err == nil {
			var saved map[string]bool
			if json.Unmarshal(data, &saved) == nil {
				for k, v := range saved {
					loadoutState[k] = v
				}
			}
		}
	})
}

// LoadoutEnabled reports whether a component is currently on. Unknown ids
// default to enabled so a new component is never silently dropped.
func LoadoutEnabled(id string) bool {
	initLoadout()
	loadoutMu.RLock()
	defer loadoutMu.RUnlock()
	v, ok := loadoutState[id]
	if !ok {
		return true
	}
	return v
}

// ToggleLoadout flips a component and persists.
func ToggleLoadout(id string) {
	initLoadout()
	loadoutMu.Lock()
	loadoutState[id] = !loadoutState[id]
	loadoutMu.Unlock()
	saveLoadout()
}

// ResetLoadout restores every component to its shipped default.
func ResetLoadout() {
	initLoadout()
	loadoutMu.Lock()
	for _, c := range LoadoutComponents {
		loadoutState[c.ID] = c.Default
	}
	loadoutMu.Unlock()
	saveLoadout()
}

func saveLoadout() {
	loadoutMu.RLock()
	data, _ := json.MarshalIndent(loadoutState, "", " ")
	loadoutMu.RUnlock()
	path := loadoutPath()
	_ = os.MkdirAll(filepath.Dir(path), 0o755)
	_ = os.WriteFile(path, data, 0o600)
}

// LoadoutActiveTokens is the approximate per-turn overhead of everything
// currently switched on, including the always-present base system prompt.
func LoadoutActiveTokens() int {
	total := systemPromptTokens
	for _, c := range LoadoutComponents {
		if LoadoutEnabled(c.ID) {
			total += c.Tokens
		}
	}
	return total
}

// LoadoutBaseTokens is the fixed, non-switchable overhead (base prompt).
func LoadoutBaseTokens() int { return systemPromptTokens }
