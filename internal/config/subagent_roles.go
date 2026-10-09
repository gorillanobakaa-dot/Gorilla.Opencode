package config

import "fmt"

// GORILLA OVERRIDE (2026-10-09): role agents for the `agent` tool's helpers.
//
// The agent tool spawns three kinds of helper: explore (AgentTask), plan
// (AgentPlan) and coder (AgentSubCoder). The provider layer looks an agent up
// by name in cfg.Agents and refuses one that is missing ("agent X not found"),
// and the system prompt is chosen by the same name. So a role needs its own
// entry, but no config.json written before 2026-10-09 has one.
//
// That is the AgentResearch trap (see researchAgentName): a feature that looks
// implemented and fails on every existing install. The answer here is to
// DERIVE a missing entry from the role's parent at spawn time:
//
//	subcoder <- coder   it edits, so it runs on the model the user chose to
//	                    edit with, never on the cheap helper model
//	plan     <- task    read-only, so it runs where the explore helper runs
//
// A derived entry lives in memory only and is re-derived on every spawn, so it
// follows its parent when the user moves the coder. A user who wants a role
// elsewhere writes an entry in config.json, and from then on it is theirs.
var roleAgentParents = map[AgentName]AgentName{
	AgentSubCoder: AgentCoder,
	AgentPlan:     AgentTask,
}

// derivedRoleAgents records the entries in cfg.Agents that were derived rather
// than configured, with the value written. An entry counts as derived only
// while it still holds exactly that value, so anything else that writes the
// entry makes it a choice without having to remember to say so. Keyed to the
// *Config it was built for, so a reloaded config (tests do this) never
// inherits the previous one's bookkeeping.
var (
	derivedRoleAgents    = map[AgentName]Agent{}
	derivedRoleAgentsFor *Config
)

func isDerivedRoleAgent(name AgentName) bool {
	if cfg == nil || derivedRoleAgentsFor != cfg {
		return false
	}
	want, ok := derivedRoleAgents[name]
	cur, present := cfg.Agents[name]
	return ok && present && cur == want
}

func forgetDerivedRoleAgent(name AgentName) {
	if derivedRoleAgentsFor == cfg {
		delete(derivedRoleAgents, name)
	}
}

// RoleAgentParent names the agent a role agent is derived from, if it is one.
func RoleAgentParent(name AgentName) (AgentName, bool) {
	p, ok := roleAgentParents[name]
	return p, ok
}

// DeriveRoleAgent makes sure cfg.Agents holds an entry for a role agent before
// a helper is built from it. A configured entry is left exactly as it is. A
// missing or previously derived one is (re)copied from the parent — model,
// max tokens and reasoning effort — so it always matches what the parent is on
// NOW. Returns an error when the parent itself is not configured, because a
// helper on a guessed model is worse than a refusal the model can read.
//
// Called for non-role agents it is a no-op, so callers need not special-case.
func DeriveRoleAgent(name AgentName) error {
	if cfg == nil {
		return fmt.Errorf("config not loaded")
	}
	parent, isRole := roleAgentParents[name]
	if !isRole {
		return nil
	}
	if _, present := cfg.Agents[name]; present && !isDerivedRoleAgent(name) {
		return nil // configured: the user's, left exactly as it is
	}
	if derivedRoleAgentsFor != cfg {
		derivedRoleAgents = map[AgentName]Agent{}
		derivedRoleAgentsFor = cfg
	}
	from, ok := cfg.Agents[parent]
	if !ok || from.Model == "" {
		return fmt.Errorf("agent %s has no entry and its parent agent %s is not configured", name, parent)
	}
	if cfg.Agents == nil {
		cfg.Agents = make(map[AgentName]Agent)
	}
	derived := Agent{Model: from.Model, MaxTokens: from.MaxTokens, ReasoningEffort: from.ReasoningEffort}
	// Written only when it changed: cfg.Agents is read without a lock by the
	// UI, so the fewer writes the better.
	if cur, ok := cfg.Agents[name]; !ok || cur != derived {
		cfg.Agents[name] = derived
	}
	derivedRoleAgents[name] = derived
	return nil
}
