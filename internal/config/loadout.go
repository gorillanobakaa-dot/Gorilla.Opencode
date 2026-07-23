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

	"github.com/opencode-ai/opencode/internal/llm/models"
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
	// GORILLA OVERRIDE: env estimate was 150 when the block was a recursive
	// 1000-file tree dump (real cost often 10k–30k). After the shallow
	// project_summary refactor it really is ~100–200 tokens; calibrate
	// still overwrites this at startup with a measured value.
	{"prompt.env", "Environment info", "agent won't be told your cwd, OS, top-level files, or short git status", 150, true, false},
	{"prompt.lsp", "LSP info", "agent won't be told which language servers are active", 100, true, false},
}

// lowBandwidthOff lists components switched OFF by ApplyLowBandwidthLoadout.
// Critical tools (bash/edit/write/view) stay on; optional network/LSP/extra
// edit surfaces and the LSP prompt blurb drop. Env stays ON — it is cheap
// after the shallow project_summary change and still useful on remote links.
var lowBandwidthOff = map[string]bool{
	"tool.patch":       true,
	"tool.fetch":       true,
	"tool.diagnostics": true,
	"tool.agent":       true,
	"tool.sourcegraph": true,
	"prompt.lsp":       true,
}

const loadoutFileName = "loadout.json"

// GORILLA OVERRIDE: token figures start as estimates but are replaced at
// startup with REAL measured values (agent.CalibrateLoadout serialises
// each tool's actual schema and the actual system prompt). This is why
// the menu total matches what the model really receives, and why turning
// a tool off drops the number by its true cost.
var (
	basePromptTokens = 3000 // measured system prompt; always on, not switchable
	tokenOverride    = map[string]int{}
	tokenOverrideMu  sync.RWMutex
)

// SetBasePromptTokens records the measured base system-prompt token count.
func SetBasePromptTokens(n int) {
	tokenOverrideMu.Lock()
	basePromptTokens = n
	tokenOverrideMu.Unlock()
}

// SetLoadoutTokens records a component's measured token cost.
func SetLoadoutTokens(id string, n int) {
	tokenOverrideMu.Lock()
	tokenOverride[id] = n
	tokenOverrideMu.Unlock()
}

// ComponentTokens returns the measured cost if known, else the estimate.
func ComponentTokens(c LoadoutComponent) int {
	tokenOverrideMu.RLock()
	defer tokenOverrideMu.RUnlock()
	if v, ok := tokenOverride[c.ID]; ok {
		return v
	}
	return c.Tokens
}

var (
	loadoutOnce  sync.Once
	loadoutState map[string]bool // id -> enabled (only entries that differ or are set)
	loadoutMu    sync.RWMutex
)

// GORILLA OVERRIDE: Gorilla-specific config lives under a "gorilla-opencode"
// directory, matching the desktop launch key file — NOT under "opencode",
// which is a different app's (SST opencode) config directory. Using appName
// ("opencode") polluted that other app's dir and split our own config.
const gorillaConfigDir = "gorilla-opencode"

func loadoutConfigBase() string {
	if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" {
		return filepath.Join(xdg, gorillaConfigDir)
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", gorillaConfigDir)
}

func loadoutPath() string {
	// One-time migration: move a loadout.json left in the old
	// (~/.config/opencode) location into the correct dir.
	newPath := filepath.Join(loadoutConfigBase(), loadoutFileName)
	if _, err := os.Stat(newPath); os.IsNotExist(err) {
		var oldPath string
		if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" {
			oldPath = filepath.Join(xdg, appName, loadoutFileName)
		} else {
			home, _ := os.UserHomeDir()
			oldPath = filepath.Join(home, ".config", appName, loadoutFileName)
		}
		if data, err := os.ReadFile(oldPath); err == nil {
			_ = os.MkdirAll(loadoutConfigBase(), 0o755)
			if os.WriteFile(newPath, data, 0o600) == nil {
				_ = os.Remove(oldPath)
			}
		}
	}
	return newPath
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

// ApplyLowBandwidthLoadout turns off optional tools/blocks that are not
// required for core edit/build loops. Intended for metered, satellite, or
// high-latency links. Persists like any other loadout change. Returns the
// new active token estimate (after current calibration overrides).
func ApplyLowBandwidthLoadout() int {
	initLoadout()
	loadoutMu.Lock()
	for _, c := range LoadoutComponents {
		if lowBandwidthOff[c.ID] {
			loadoutState[c.ID] = false
		} else {
			// Keep shipped defaults for everything else (including critical tools).
			loadoutState[c.ID] = c.Default
		}
	}
	loadoutMu.Unlock()
	saveLoadout()
	return LoadoutActiveTokens()
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
	tokenOverrideMu.RLock()
	total := basePromptTokens
	tokenOverrideMu.RUnlock()
	for _, c := range LoadoutComponents {
		if LoadoutEnabled(c.ID) {
			total += ComponentTokens(c)
		}
	}
	return total
}

// LoadoutBaseTokens is the fixed, non-switchable overhead (base prompt).
func LoadoutBaseTokens() int {
	tokenOverrideMu.RLock()
	defer tokenOverrideMu.RUnlock()
	return basePromptTokens
}

// LoadoutCost is the money side of the token counter: what the fixed
// per-turn context (LoadoutActiveTokens) actually costs, priced at the
// active coder model's INPUT rate. Same formula the agent uses to bill a
// real turn (CostPer1MIn/1e6 * inputTokens), so the numbers line up.
//
//   - dollars:   cost of one turn's fixed overhead in USD.
//   - per1MIn:   the model's input price per 1M tokens (0 = free/flat/OAuth).
//   - modelName: human name of the active model (for the label).
//   - priced:    false when we have no model or no price table entry, so the
//     UI can say "unpriced" instead of a misleading $0.00.
//
// On a free or flat-rate tier (per1MIn == 0) dollars is genuinely 0 — that
// is the real bill, not missing data; priced stays true.
func LoadoutCost() (dollars, per1MIn float64, modelName string, priced bool) {
	tokens := LoadoutActiveTokens()
	if cfg == nil {
		return 0, 0, "", false
	}
	agent, ok := cfg.Agents[AgentCoder]
	if !ok {
		return 0, 0, "", false
	}
	m, ok := models.SupportedModels[agent.Model]
	if !ok {
		// Unknown/custom model: tokens are real but we can't price them.
		return 0, 0, string(agent.Model), false
	}
	name := m.Name
	if name == "" {
		name = string(m.ID)
	}
	return float64(tokens) / 1e6 * m.CostPer1MIn, m.CostPer1MIn, name, true
}
