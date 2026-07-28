package chat

import "github.com/opencode-ai/opencode/internal/session"

func sessionForBanner() session.Session {
	return session.Session{Cost: 0.05, PromptTokens: 10_600, CompletionTokens: 87}
}
