package prompt

import (
	_ "embed"
	"fmt"

	"github.com/opencode-ai/opencode/internal/llm/models"
)

// GORILLA OVERRIDE (2026-10-09): prompts for the agent tool's `plan` and
// `coder` roles. Same embed pattern as task.go: the instruction text lives in a
// file, the environment block is composed at call time.

//go:embed plan.txt
var basePlanPrompt string

// BasePlanPrompt is the shipped planning instruction fragment (no env block).
func BasePlanPrompt() string { return normaliseNewlines(basePlanPrompt) }

// PlanPrompt is the system prompt of a role=plan helper.
func PlanPrompt(_ models.ModelProvider) string {
	return fmt.Sprintf("%s\n%s\n", BasePlanPrompt(), getEnvironmentInfo())
}

// SubCoderNote is appended to the coder prompt for a role=coder helper. The
// helper is the coder in every other respect — same tools minus the helper
// spawners, same model — but nobody reads its turns except the parent agent,
// and it gets exactly one reply to make. Without this it ends the way the
// coder ends a turn with a person: "done", which tells the parent nothing it
// can check.
const SubCoderNote = `# sub-agent
you are a helper started by a parent agent, not talking to the user. you report back once, in your final message, and cannot be asked a follow-up.
- do the task you were given and nothing beyond it
- your final message must say: every file you changed (absolute paths) and what changed in each; every command you ran and its result; anything you could not do or verify
- permission questions still go to the user; a refusal is final: report it, do not work around it`

// SubCoderPrompt is the system prompt of a role=coder helper.
func SubCoderPrompt(provider models.ModelProvider) string {
	return CoderPrompt(provider) + "\n\n" + SubCoderNote
}
