package agent

import (
	"strings"
	"testing"

	"github.com/opencode-ai/opencode/internal/llm/models"
	"github.com/opencode-ai/opencode/internal/llm/provider"
	"github.com/opencode-ai/opencode/internal/message"
)

func toolMsg(content string) message.Message {
	return message.Message{Role: message.Tool, Parts: []message.ContentPart{
		message.ToolResult{ToolCallID: "c1", Content: content},
	}}
}

// The refused request of 2026-10-24, from LM Studio's log: the call that ended
// read 13,595 prompt tokens and wrote 994, almost all reasoning; the find
// result was 1,500 bytes. On Gemma 4's 20,224-token window it fits.
func TestTheRefusedGemmaRequestFitsWhenMeasured(t *testing.T) {
	prev := provider.TokenUsage{InputTokens: 13595, OutputTokens: 994, ReasoningTokens: 950}
	n, ok := MeasuredNextRequest(prev, toolMsg(strings.Repeat("x", 1500)))
	if !ok {
		t.Fatal("a reported usage was not used")
	}
	if n < 13595+44 || n > 14300 {
		t.Errorf("next request %d tokens, want ~14.1K (prompt + 44 written + ~430 of tool output)", n)
	}
	gemma := models.Model{Name: "Gemma 4", ContextWindow: 20224}
	if msg := ContextOverflowMessage(n, gemma); msg != "" {
		t.Errorf("a request the server would have taken was refused:\n%s", msg)
	}
}

func TestNoUsageMeansNoMeasurement(t *testing.T) {
	if _, ok := MeasuredNextRequest(provider.TokenUsage{}, toolMsg("x")); ok {
		t.Error("a server that reported nothing was treated as having measured")
	}
}

// Old reasoning is not sent, so it must not be counted.
func TestReasoningIsNotCountedInTheEstimate(t *testing.T) {
	a := message.Message{Role: message.Assistant, Parts: []message.ContentPart{
		message.TextContent{Text: "ok"},
		message.ReasoningContent{Thinking: strings.Repeat("think ", 2000)},
	}}
	b := message.Message{Role: message.Assistant, Parts: []message.ContentPart{
		message.TextContent{Text: "ok"},
	}}
	if x, y := EstimateRequestTokens("", []message.Message{a}, nil), EstimateRequestTokens("", []message.Message{b}, nil); x != y {
		t.Errorf("12,000 characters of reasoning added %d tokens to a request that does not carry them", x-y)
	}
}

// Earlier turns' outputs go; this turn's stay whole; the stored messages are
// not touched.
func TestOnlyEarlierTurnsLoseTheirToolOutput(t *testing.T) {
	big := strings.Repeat("y", 5000)
	small := "short result"
	history := []message.Message{
		{Role: message.User, Parts: []message.ContentPart{message.TextContent{Text: "q1"}}},
		toolMsg(big),
		toolMsg(small),
		{Role: message.User, Parts: []message.ContentPart{message.TextContent{Text: "q2"}}},
		toolMsg(big),
	}
	turnStart := 3
	out, dropped := elideOldToolResults(history, turnStart)
	if dropped != 1 {
		t.Errorf("dropped %d, want 1 (the big old one; the small one costs nothing)", dropped)
	}
	if got := out[1].ToolResults()[0].Content; !strings.Contains(got, "5000 characters") {
		t.Errorf("old output became %q, want a stub naming its size", got)
	}
	if out[2].ToolResults()[0].Content != small {
		t.Error("a small old output was replaced for no gain")
	}
	if out[4].ToolResults()[0].Content != big {
		t.Error("this turn's own tool output was cut")
	}
	if history[1].ToolResults()[0].Content != big {
		t.Error("the stored message was changed, not only the copy that is sent")
	}
}
