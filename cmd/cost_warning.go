package cmd

import (
	"fmt"
	"strings"
	"time"

	"github.com/opencode-ai/opencode/internal/llm/cost"
	"github.com/opencode-ai/opencode/internal/llm/models"
)

// costWarningModels are the Claude models the warning prices, cheapest first.
// Their rates come from internal/llm/cost/prices.json (dated, sourced); a model
// missing there is skipped rather than given a guessed price.
var costWarningModels = []struct{ api, name string }{
	{"claude-haiku-5-5", "Claude Haiku 5.5"},
	{"claude-sonnet-5", "Claude Sonnet 5"},
	{"claude-sonnet-5-5", "Claude Sonnet 5.5"},
	{"claude-opus-5-5", "Claude Opus 5.5"},
	{"claude-fable-5-1", "Claude Fable 5.1"},
}

// paidKeyProviders are the portal rows that bill a card for every message.
var paidKeyProviders = map[string]models.ModelProvider{
	"anthropic":  models.ProviderAnthropic,
	"openai":     models.ProviderOpenAI,
	"xai":        models.ProviderXAI,
	"deepseek":   models.ProviderDeepSeek,
	"openrouter": models.ProviderOpenRouter,
}

// costWarning is the screen a person must answer "I understand" to before a
// paid key is used (owner, 2026-10-28). Plain and hard, no insults: the
// readers include children, and a warning that reads as a joke is skipped.
// The first line is drawn white on red.
func costWarning(p models.ModelProvider) string {
	var b strings.Builder
	b.WriteString("API KEY: YOUR CARD PAYS FOR EVERY MESSAGE.\n")
	b.WriteString("Every time you press Enter, money leaves your account. Every message pays again for everything " +
		"already in the conversation. A short reply adds little. A file the AI reads adds its whole size to every " +
		"message after it. A pause of more than 5 minutes makes the next message cost full price again.\n")
	table := costTable(p)
	if table != "" {
		b.WriteString("\n" + table)
	} else {
		b.WriteString("\nGorilla OpenCode has no price list for this provider. What a message costs is on the " +
			"provider's pricing page, and it grows the same way.\n")
	}
	b.WriteString("\nFree options, no card: Google Antigravity, ChatGPT sign-in, NVIDIA NIM, Groq, Cloudflare.")
	return b.String()
}

// costTable prices each known model at three sizes. Empty when the provider
// has no prices in the table.
func costTable(p models.ModelProvider) string {
	if p != models.ProviderAnthropic {
		return ""
	}
	now := time.Now()
	var rows []string
	for _, m := range costWarningModels {
		in, out, ok := cost.ListRates(string(p), m.api, now)
		if !ok {
			continue
		}
		rows = append(rows, fmt.Sprintf("  %-18s %12s %12s %12s", m.name,
			money(cost.MessageCost(in, out, cost.ShortConversationTokens)),
			money(cost.MessageCost(in, out, cost.LongConversationTokens)),
			money(cost.WorkingHourCost(in, out))))
	}
	if len(rows) == 0 {
		return ""
	}
	head := fmt.Sprintf("Up to, at list price (estimate):\n  %-18s %12s %12s %12s\n", "model",
		"new chat", "long chat", "AI 1 hour")
	return head + strings.Join(rows, "\n") + "\n" +
		"  per message at the start of a conversation and in a long one, and an hour of the AI working on its own.\n"
}

// money prints a dollar figure with the precision its size needs.
func money(v float64) string {
	switch {
	case v < 0.1:
		return fmt.Sprintf("$%.3f", v)
	case v < 10:
		return fmt.Sprintf("$%.2f", v)
	}
	return fmt.Sprintf("$%.0f", v)
}
