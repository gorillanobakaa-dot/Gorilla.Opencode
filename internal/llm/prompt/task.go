package prompt

import (
	"fmt"

	"github.com/opencode-ai/opencode/internal/llm/models"
)

func TaskPrompt(_ models.ModelProvider) string {
	agentPrompt := `you answer user questions using available tools. provide facts only.

# output
- one word answers: no intro/outro/explanation/elaboration
- code snippets: share when relevant to query
- absolute paths only: never relative paths`

	return fmt.Sprintf("%s\n%s\n", agentPrompt, getEnvironmentInfo())
}
