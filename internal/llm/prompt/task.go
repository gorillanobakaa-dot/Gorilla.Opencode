package prompt

import (
	_ "embed"
	"fmt"

	"github.com/opencode-ai/opencode/internal/llm/models"
)

// GORILLA OVERRIDE: see summarizer.go — same rationale, same pattern. Only the
// INSTRUCTION text moved to the file; the environment block is composed at call
// time from getEnvironmentInfo(), as it always was. The `%s\n%s\n` composition
// is byte-preserved: strings.TrimSpace of the file's trailing newline yields
// exactly what the old `agentPrompt` literal held.

//go:embed task.txt
var baseTaskPrompt string

// BaseTaskPrompt is the shipped default instruction fragment (no env block).
func BaseTaskPrompt() string { return normaliseNewlines(baseTaskPrompt) }

func TaskPrompt(_ models.ModelProvider) string {
	return fmt.Sprintf("%s\n%s\n", BaseTaskPrompt(), getEnvironmentInfo())
}
