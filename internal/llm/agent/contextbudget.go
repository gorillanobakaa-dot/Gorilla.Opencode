// GORILLA OVERRIDE (2026-09-01): know how big the NEXT request is, before
// sending it.
//
// # THE FAILURE THIS EXISTS TO PREVENT
//
// Measured, six times, in one local session (gorilla-opencode.db):
//
//	request (18545 tokens) exceeds the available context size (15104 tokens)
//
// while the footer read "context 9.7K (64%)". Both numbers were correct and they
// were measuring different things. The footer shows
// session.PromptTokens + CompletionTokens, which is the usage the provider
// reported for the LAST COMPLETED RESPONSE. The next request is that history
// PLUS every tool result produced since — and a single `view` of a large file
// adds thousands of tokens in one step. The turn that pushed 9.7K to 18.5K never
// appeared in any counter, because the counter is only updated when a response
// comes back and no response ever came back.
//
// So auto-compaction could not fire: it is checked after a completed response,
// against a number that predates the tool output which caused the overflow. The
// conversation walked over the edge in a place where nothing was looking.
//
// # WHY AN ESTIMATE IS ENOUGH
//
// Counting exactly would need each provider's own tokeniser, which is a
// dependency per vendor and still wrong for local GGUF models whose tokeniser
// lives in the file. The decision here is not "how many tokens exactly" but
// "will this obviously not fit", and for that a cheap character-based estimate
// is sufficient — as long as it is deliberately PESSIMISTIC. Over-estimating
// costs an early compaction; under-estimating costs the whole turn.
package agent

import (
	"strings"
	"time"

	"github.com/opencode-ai/opencode/internal/config"

	"github.com/opencode-ai/opencode/internal/llm/models"
	"github.com/opencode-ai/opencode/internal/llm/tools"
	"github.com/opencode-ai/opencode/internal/message"
)

// charsPerToken is the divisor for the estimate.
//
// The usual rule of thumb is ~4 characters per token for English prose. 3.5 is
// used instead because this conversation is mostly source code, paths and JSON,
// which tokenise far less efficiently than prose — identifiers split, indentation
// becomes runs of single tokens, and punctuation rarely merges. Erring low on
// characters-per-token means erring HIGH on the token count, which is the safe
// direction: an early compaction is an inconvenience, a rejected request is a
// lost turn.
const charsPerToken = 3.5

// replyHeadroom is the share of the window kept free for the model's own answer.
//
// A prompt that exactly fills the context leaves no room to reply, which is the
// same mistake as overflowing it, approached from the other side. Bounded by
// DefaultMaxTokens when the model states one.
const replyHeadroomFraction = 0.15

// estimateTokens approximates how many tokens a string will occupy.
//
// The package already has EstimateTokens (len/4) in research_recover.go, used to
// decide whether to WARN before a recovery run. This one divides by 3.5 instead
// and is used to decide whether to BLOCK a request, so it errs high on purpose:
// an early compaction is an inconvenience, a rejected request is a lost turn.
func estimateTokens(s string) int {
	if s == "" {
		return 0
	}
	return int(float64(len(s))/charsPerToken) + 1
}

// perMessageOverhead is what the wire format costs beyond the visible text:
// role, delimiters, and each provider's message envelope. Small, but it is paid
// on every message and a long conversation has hundreds.
const perMessageOverhead = 4

// EstimateRequestTokens approximates the size of the request that would be sent
// for this history, including the system prompt and the tool schemas.
//
// The tool schemas matter and are easy to forget: they are re-sent in full on
// every single request, they are large (this program ships a lot of tools with
// long descriptions, deliberately), and they are invisible in any counter
// derived from message content.
func EstimateRequestTokens(systemPrompt string, msgs []message.Message, toolset []tools.BaseTool) int {
	total := estimateTokens(systemPrompt)

	for _, m := range msgs {
		total += perMessageOverhead
		total += estimateTokens(m.Content().String())
		total += estimateTokens(m.ReasoningContent().String())
		for _, tc := range m.ToolCalls() {
			total += estimateTokens(tc.Name) + estimateTokens(tc.Input) + perMessageOverhead
		}
		for _, tr := range m.ToolResults() {
			total += estimateTokens(tr.Content) + perMessageOverhead
		}
		for _, bc := range m.BinaryContent() {
			// An image is not text and does not divide by charsPerToken. Providers
			// bill these in the high hundreds to low thousands; 1500 is a
			// deliberately pessimistic stand-in rather than a measurement.
			_ = bc
			total += 1500
		}
	}

	for _, t := range toolset {
		info := t.Info()
		total += estimateTokens(info.Name) + estimateTokens(info.Description)
		for name, schema := range info.Parameters {
			total += estimateTokens(name)
			if s, ok := schema.(map[string]any); ok {
				if d, ok := s["description"].(string); ok {
					total += estimateTokens(d)
				}
			}
			total += perMessageOverhead
		}
	}

	return total
}

// UsableWindow is how much of a model's context this request may occupy, leaving
// room for the answer. Zero means the model declares no window and no judgement
// can be made.
func UsableWindow(m models.Model) int {
	if m.ContextWindow <= 0 {
		return 0
	}
	window := int(m.ContextWindow)

	headroom := int(float64(window) * replyHeadroomFraction)
	if m.DefaultMaxTokens > 0 && int(m.DefaultMaxTokens) < headroom {
		// The model states what it will write at most; no need to reserve more.
		headroom = int(m.DefaultMaxTokens)
	}
	if headroom < 512 {
		headroom = 512
	}
	if headroom >= window {
		return 0 // a window too small to hold any answer; do not gate on it
	}
	return window - headroom
}

// ContextOverflowMessage reports the problem in the user's terms, or "" when the
// request fits.
//
// Deliberately the SAME advice the provider-side explanation gives, so someone
// who hits this once from each direction is not told two different things.
func ContextOverflowMessage(estimated int, m models.Model) string {
	usable := UsableWindow(m)
	if usable == 0 || estimated <= usable {
		return ""
	}
	var b strings.Builder
	b.WriteString("This conversation has grown bigger than ")
	if m.Name != "" {
		b.WriteString(m.Name)
	} else {
		b.WriteString("this model")
	}
	b.WriteString(" can hold, so the request was not sent.\n\n")
	b.WriteString("  About ")
	b.WriteString(humanTokens(int64(estimated)))
	b.WriteString(" would be needed; there is room for roughly ")
	b.WriteString(humanTokens(int64(usable)))
	b.WriteString(" once space for the reply is set aside")
	b.WriteString(" (the model holds ")
	b.WriteString(humanTokens(m.ContextWindow))
	b.WriteString(" in total).\n\n")
	b.WriteString("What to do:\n")
	b.WriteString("  - /compact  summarise the conversation so far and carry on\n")
	b.WriteString("  - /new      start a fresh conversation\n")
	b.WriteString("  - /models   switch to a model with a larger context\n")
	b.WriteString("\nIf this model runs on your own machine, you can also raise its context " +
		"length in Ollama or LM Studio and restart it — the limit is a setting, not a licence.")
	return b.String()
}

// titleWaitBudget is how long the title generator waits for the user's own turn
// to finish before giving up on naming the session.
//
// GORILLA OVERRIDE (2026-09-01): scaled to the model, because a fixed 120s is
// two different things on two different machines. On a cloud model a turn is
// seconds and 120s is generous; on a model running locally on a laptop one turn
// can spend six to eight minutes in prompt processing alone, so a fixed budget
// expires every single time and the title fires into a busy server.
//
// A local endpoint gets a much longer budget so the title still gets written,
// just late — and if even that runs out, the caller abandons rather than
// competing with the work the user is actually waiting for.
func titleWaitBudget() time.Duration {
	if isLocalProviderSelected() {
		return 15 * time.Minute
	}
	return 120 * time.Second
}

// isLocalProviderSelected reports whether the coder agent is pointed at a model
// served from this machine.
func isLocalProviderSelected() bool {
	cfg := config.Get()
	if cfg == nil {
		return false
	}
	id := cfg.Agents[config.AgentCoder].Model
	return models.SupportedModels[id].Provider == models.ProviderLocal
}
