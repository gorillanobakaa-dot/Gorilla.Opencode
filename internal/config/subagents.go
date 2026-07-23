// GORILLA OVERRIDE: this file did not exist upstream. It stores the
// user-adjustable "helper-leash" ("Dial 2" in the context menu): how many
// helper AIs (sub-agents) the main agent may summon.
//
// Background (see LESSON_How_Tool_Use_And_Billing_Work.md, Parts 5–6): the
// upstream design places no cap on sub-agent spawns and its tool description
// even encourages the model to "launch multiple agents concurrently." Spawns
// can't recurse (depth is 1) and run sequentially (no stampede), but each helper
// is a full nested request loop — so on a low-RPM / metered tier an unleashed
// main agent can grind through the budget. This dial hands that control to the
// user, from UNLIMITED down to the Gorilla Nuclear Option (helpers disabled).
package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
)

const subAgentsFileName = "subagents.json"

// Sentinel values for the leash.
const (
	SubAgentsUnlimited = -1 // no cap
	SubAgentsNuclear   = 0  // helpers fully disabled ("Gorilla Nuclear Option")
)

// SubAgentPresets is the ladder the /context dial steps through, richest/most-
// powerful first (UNLIMITED) down to the Nuclear Option (0 = disabled).
var SubAgentPresets = []int{SubAgentsUnlimited, 100, 50, 25, 12, 6, 3, 1, SubAgentsNuclear}

// DefaultMaxSubAgents preserves upstream behaviour out of the box (unlimited);
// the user leashes it if their tier needs it. (The low-bandwidth loadout preset
// already disables the helper tool entirely.)
const DefaultMaxSubAgents = SubAgentsUnlimited

var (
	maxSubAgents  = DefaultMaxSubAgents
	subAgentsOnce sync.Once
	subAgentsMu   sync.RWMutex
)

func subAgentsPath() string {
	return filepath.Join(loadoutConfigBase(), subAgentsFileName)
}

type subAgentsFile struct {
	MaxSubAgents int `json:"max_sub_agents"`
}

func initSubAgents() {
	subAgentsOnce.Do(func() {
		if data, err := os.ReadFile(subAgentsPath()); err == nil {
			var f subAgentsFile
			if json.Unmarshal(data, &f) == nil && f.MaxSubAgents >= SubAgentsNuclear-1 {
				subAgentsMu.Lock()
				maxSubAgents = f.MaxSubAgents
				subAgentsMu.Unlock()
			}
		}
	})
}

// MaxSubAgents returns the current leash: -1 unlimited, 0 disabled, else the cap.
func MaxSubAgents() int {
	initSubAgents()
	subAgentsMu.RLock()
	defer subAgentsMu.RUnlock()
	return maxSubAgents
}

// SetMaxSubAgents sets and persists the leash. Values below the Nuclear sentinel
// are clamped to unlimited.
func SetMaxSubAgents(n int) {
	initSubAgents()
	if n < SubAgentsNuclear {
		n = SubAgentsUnlimited
	}
	subAgentsMu.Lock()
	maxSubAgents = n
	subAgentsMu.Unlock()
	saveSubAgents(n)
}

// StepMaxSubAgents moves one rung along SubAgentPresets. dir>0 = more helpers
// (up the ladder toward UNLIMITED), dir<0 = fewer (toward Nuclear). Snaps to the
// nearest preset first.
func StepMaxSubAgents(dir int) int {
	cur := MaxSubAgents()
	idx := nearestSubAgentPresetIdx(cur)
	// SubAgentPresets is high→low (UNLIMITED first), so "more" (dir>0) = lower index.
	if dir > 0 {
		idx--
	} else if dir < 0 {
		idx++
	}
	if idx < 0 {
		idx = 0
	}
	if idx >= len(SubAgentPresets) {
		idx = len(SubAgentPresets) - 1
	}
	SetMaxSubAgents(SubAgentPresets[idx])
	return SubAgentPresets[idx]
}

// nearestSubAgentPresetIdx maps a raw value to a preset index. UNLIMITED (-1)
// ranks above everything; otherwise nearest by magnitude.
func nearestSubAgentPresetIdx(v int) int {
	if v <= SubAgentsUnlimited {
		return 0
	}
	best, bestIdx := 1<<30, len(SubAgentPresets)-1
	for i, p := range SubAgentPresets {
		if p < 0 {
			continue // skip UNLIMITED for magnitude comparison
		}
		d := p - v
		if d < 0 {
			d = -d
		}
		if d < best {
			best, bestIdx = d, i
		}
	}
	return bestIdx
}

func saveSubAgents(n int) {
	_ = os.MkdirAll(loadoutConfigBase(), 0o755)
	data, _ := json.MarshalIndent(subAgentsFile{MaxSubAgents: n}, "", " ")
	_ = os.WriteFile(subAgentsPath(), data, 0o600)
}
