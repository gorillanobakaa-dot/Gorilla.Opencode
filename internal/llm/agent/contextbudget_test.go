package agent

import (
	"strings"
	"testing"

	"github.com/opencode-ai/opencode/internal/llm/models"
	"github.com/opencode-ai/opencode/internal/message"
)

// The model from the real session: LM Studio reported a 15104-token window.
func qwenLocal() models.Model {
	return models.Model{
		Name:             "Qwen3 Coder",
		ContextWindow:    15104,
		DefaultMaxTokens: 7552,
	}
}

// GORILLA OVERRIDE (2026-09-01): the case that was actually failing.
//
// Six recorded failures in one local session, all of the shape
// "request (18545 tokens) exceeds the available context size (15104 tokens)"
// while the footer showed 9.7K/64%. The footer was reporting the LAST
// response's usage; the request that followed carried tool results the footer
// had never seen.
func TestARequestBiggerThanTheWindowIsRefusedBeforeItIsSent(t *testing.T) {
	if got := ContextOverflowMessage(18545, qwenLocal()); got == "" {
		t.Fatal("an 18545-token request against a 15104-token window was allowed through — " +
			"this is the exact request the provider rejected six times")
	}
}

func TestARequestThatFitsIsNotRefused(t *testing.T) {
	if got := ContextOverflowMessage(4000, qwenLocal()); got != "" {
		t.Errorf("a 4000-token request against a 15104-token window was refused:\n%s", got)
	}
}

// Room must be left for the answer. A prompt that exactly fills the window is
// the same failure approached from the other side.
func TestRoomIsLeftForTheReply(t *testing.T) {
	m := qwenLocal()
	usable := UsableWindow(m)
	if usable >= int(m.ContextWindow) {
		t.Errorf("UsableWindow = %d with a %d window — nothing reserved for the reply",
			usable, m.ContextWindow)
	}
	// Filling the window exactly must be refused, not accepted.
	if ContextOverflowMessage(int(m.ContextWindow), m) == "" {
		t.Error("a request that exactly fills the context was allowed; there would be no room to answer")
	}
}

// A model that declares no window must not be gated on a number we do not have.
// Refusing to work because a local endpoint was vague would be worse than the bug.
func TestAModelWithNoDeclaredWindowIsNeverBlocked(t *testing.T) {
	if got := ContextOverflowMessage(999_999, models.Model{Name: "mystery"}); got != "" {
		t.Errorf("blocked a request for a model that declares no context window:\n%s", got)
	}
}

// The message has to name the fix and both numbers, or it is just a different
// way of saying "no".
func TestTheRefusalNamesTheFixAndTheNumbers(t *testing.T) {
	got := ContextOverflowMessage(18545, qwenLocal())
	for _, want := range []string{"/compact", "/models", "Qwen3 Coder", "Ollama", "LM Studio"} {
		if !strings.Contains(got, want) {
			t.Errorf("the refusal does not mention %q:\n%s", want, got)
		}
	}
	if strings.Contains(got, "{") {
		t.Errorf("raw JSON leaked into a user-facing message:\n%s", got)
	}
}

// Tool RESULTS are what actually overflow a context, and they are the thing no
// counter in the program was measuring.
func TestToolResultsAreCounted(t *testing.T) {
	big := strings.Repeat("x", 40_000) // ~11K tokens of file content

	withResult := []message.Message{{
		Role: message.Tool,
		Parts: []message.ContentPart{
			message.ToolResult{ToolCallID: "1", Content: big},
		},
	}}
	empty := []message.Message{{Role: message.Tool}}

	got := EstimateRequestTokens("", withResult, nil)
	base := EstimateRequestTokens("", empty, nil)
	if got-base < 8000 {
		t.Errorf("a 40KB tool result added only %d tokens to the estimate — tool output is "+
			"the thing that overflows a context and it must be counted", got-base)
	}
}

// The system prompt is re-sent on every request and is substantial. Omitting it
// under-counts by a constant, in the direction that loses a turn.
func TestTheSystemPromptIsCounted(t *testing.T) {
	prompt := strings.Repeat("system instructions. ", 500)
	with := EstimateRequestTokens(prompt, nil, nil)
	without := EstimateRequestTokens("", nil, nil)
	if with <= without {
		t.Error("the system prompt contributes nothing to the estimate")
	}
}

// The estimate must err HIGH. Under-estimating costs the whole turn; over-
// estimating costs an early compaction.
func TestTheEstimateErrsHigh(t *testing.T) {
	// Source code tokenises far worse than prose. A conservative check: the
	// estimate for a block of code should not be below len/4.
	code := strings.Repeat("func (a *agent) doSomething(ctx context.Context) error {\n", 100)
	msgs := []message.Message{{
		Role:  message.User,
		Parts: []message.ContentPart{message.TextContent{Text: code}},
	}}
	got := EstimateRequestTokens("", msgs, nil)
	if got < len(code)/4 {
		t.Errorf("estimate %d is below the len/4 floor (%d) — it errs low, which loses turns",
			got, len(code)/4)
	}
}
