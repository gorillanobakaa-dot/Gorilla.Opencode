package prompt

import (
	_ "embed"
	"strings"

	"github.com/opencode-ai/opencode/internal/llm/models"
)

// GORILLA OVERRIDE: see summarizer.go — same rationale, same pattern.

//go:embed title.txt
var baseTitlePrompt string

// BaseTitlePrompt is the shipped default.
func BaseTitlePrompt() string { return strings.TrimSpace(baseTitlePrompt) }

func TitlePrompt(_ models.ModelProvider) string { return BaseTitlePrompt() }
