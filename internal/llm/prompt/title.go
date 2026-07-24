package prompt

import "github.com/opencode-ai/opencode/internal/llm/models"

func TitlePrompt(_ models.ModelProvider) string {
	return `generate title from user's first message.

# constraints
- max 50 chars: strict character limit
- one line: no line breaks
- no quotes/colons: plain text only
- direct summary: no meta-text like "Title:" or "Summary:"
- entire output becomes title: no additional text`
}
