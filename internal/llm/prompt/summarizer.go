package prompt

import (
	_ "embed"

	"github.com/opencode-ai/opencode/internal/llm/models"
)

// GORILLA OVERRIDE: the summariser prompt lived as an inline Go string literal,
// so it could not be replaced without editing and recompiling — unlike the coder
// prompt, which was already embedded. All four base prompts (coder, summarizer,
// task, title) now share the same "embedded .txt + strings.TrimSpace" pattern,
// which is the prerequisite for the user-editable override layer in plan 04.
// Zero behaviour change: strings.TrimSpace of "...decisions made\n" equals
// "...decisions made", the exact bytes the literal returned.

//go:embed summarizer.txt
var baseSummarizerPrompt string

// BaseSummarizerPrompt is the shipped default, exported so the override layer
// and tests can compare against it.
func BaseSummarizerPrompt() string { return normaliseNewlines(baseSummarizerPrompt) }

func SummarizerPrompt(_ models.ModelProvider) string { return BaseSummarizerPrompt() }
