// GORILLA OVERRIDE: this file did not exist upstream. It enforces the
// user-configured "helper-leash" (config.MaxSubAgents) at spawn time.
//
// The count is per coder-session and resets at the start of each user turn, so
// the leash bounds the *burst* of helpers one request can trigger — which is
// exactly what drains a low-RPM budget. Sub-agents cannot recurse (their toolset
// omits tool.agent), so every spawn is counted against the one parent session.
package agent

import "sync"

var (
	subAgentSpawnMu    sync.Mutex
	subAgentSpawnCount = map[string]int{}
)

// resetSubAgentSpawns clears the per-turn spawn tally for a coder session.
// Called at the start of each top-level (coder) Run.
func resetSubAgentSpawns(sessionID string) {
	subAgentSpawnMu.Lock()
	delete(subAgentSpawnCount, sessionID)
	subAgentSpawnMu.Unlock()
}

// reserveSubAgentSpawn tries to claim one helper spawn against `limit` for this
// session. limit < 0 means unlimited. Returns ok=false (and the current used
// count) when the leash is exhausted; increments the tally when ok=true.
func reserveSubAgentSpawn(sessionID string, limit int) (ok bool, used int) {
	if limit < 0 {
		return true, 0 // unlimited
	}
	subAgentSpawnMu.Lock()
	defer subAgentSpawnMu.Unlock()
	used = subAgentSpawnCount[sessionID]
	if used >= limit {
		return false, used
	}
	subAgentSpawnCount[sessionID] = used + 1
	return true, used + 1
}
