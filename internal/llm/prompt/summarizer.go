package prompt

import "github.com/opencode-ai/opencode/internal/llm/models"

func SummarizerPrompt(_ models.ModelProvider) string {
	return `condense conversation history for context continuity.

# include
- completed actions: what was done
- active work: current task and files being modified
- next steps: what needs completion

# format
- factual only: no interpretation or opinion
- compressed: remove filler and redundancy
- preserve key details: file paths, error states, decisions made`
}
