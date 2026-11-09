package config

import (
	"os"

	"github.com/opencode-ai/opencode/internal/llm/models"
)

// GORILLA (2026-11-09, plan P22, owner idea I-24): a model on THIS machine reads
// the whole prompt before it says a word, and nothing else is paid for that
// reading. Measured on the L15 with a 35B MoE behind LM Studio: 5m15s to the first
// word on a 16k-token prompt. Every optional tool schema in the prompt is time
// the owner waits, so for a model served on a loopback address the optional
// tools are deferred (tool_search finds them on demand) unless the owner says
// otherwise.
//
// The rule, not a second mechanism: this only answers the question "is tool
// deferral on?" for a local coder model whose loadout row is still at its
// default (off). A row the owner switched on stays on. Opting out of the whole
// thing: GORILLA_LOCAL_TRIM=0 (the program never reads or sends anything for
// this; it is one environment variable).
//
// Deferral already refuses to cost more than it saves (agent.deferralStillPays),
// keeps the stable tools first so a discovery does not break the cached prefix
// (tools.VisibleTools), and is shown in /context, so nothing here is hidden.

// LocalTrimDisabledEnv switches the automatic trim off when set to "0".
const LocalTrimDisabledEnv = "GORILLA_LOCAL_TRIM"

// LocalTrimActive reports whether the configured coder model runs on this
// machine and the automatic trim has not been switched off.
func LocalTrimActive() bool {
	if os.Getenv(LocalTrimDisabledEnv) == "0" {
		return false
	}
	cfg := Get()
	if cfg == nil {
		return false
	}
	id := cfg.Agents[AgentCoder].Model
	return id != "" && models.IsOnMachine(id)
}
