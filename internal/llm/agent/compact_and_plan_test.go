package agent

// GORILLA FIX (2026-11-21): unit tests for repeated /compact slicing and
// structured /plan mode wire-directive injection.

import (
	"strings"
	"testing"

	"github.com/opencode-ai/opencode/internal/config"
	"github.com/opencode-ai/opencode/internal/message"
)

func TestSliceFromSummaryDropsAlreadyCompactedHistory(t *testing.T) {
	msgs := []message.Message{
		{ID: "m1", Role: message.User, Parts: []message.ContentPart{message.TextContent{Text: "ancient"}}},
		{ID: "m2", Role: message.Assistant, Parts: []message.ContentPart{message.TextContent{Text: "old answer"}}},
		{ID: "sum1", Role: message.Assistant, Parts: []message.ContentPart{message.TextContent{Text: "summary 1"}}},
		{ID: "m3", Role: message.User, Parts: []message.ContentPart{message.TextContent{Text: "next question"}}},
	}

	got := sliceFromSummary(msgs, "sum1")
	if len(got) != 2 {
		t.Fatalf("len(sliceFromSummary) = %d, want 2", len(got))
	}
	if got[0].ID != "sum1" || got[0].Role != message.User {
		t.Fatalf("first message = (%q, %q), want (sum1, user)", got[0].ID, got[0].Role)
	}
	// Original slice must not be mutated in place.
	if msgs[2].Role != message.Assistant {
		t.Fatalf("sliceFromSummary mutated caller's original slice role to %q", msgs[2].Role)
	}
}

func TestWithPlanDirectiveInjectsOnlyInPlanModeWithoutMutatingHistory(t *testing.T) {
	if _, err := config.Load(t.TempDir(), false); err != nil {
		t.Fatalf("config.Load: %v", err)
	}
	orig := []message.Message{
		{ID: "u1", Role: message.User, Parts: []message.ContentPart{message.TextContent{Text: "refactor auth"}}},
	}

	if err := config.SetPermissionMode(config.PermissionAsk); err != nil {
		t.Fatalf("SetPermissionMode(ask): %v", err)
	}
	gotAsk := withPlanDirective(config.AgentCoder, orig)
	if len(gotAsk[0].Parts) != 1 {
		t.Fatalf("ask mode unexpectedly injected plan directive: %d parts", len(gotAsk[0].Parts))
	}

	if err := config.SetPermissionMode(config.PermissionPlan); err != nil {
		t.Fatalf("SetPermissionMode(plan): %v", err)
	}
	gotPlan := withPlanDirective(config.AgentCoder, orig)
	if len(gotPlan[0].Parts) != 2 {
		t.Fatalf("plan mode did not append directive: got %d parts", len(gotPlan[0].Parts))
	}
	tc, ok := gotPlan[0].Parts[1].(message.TextContent)
	if !ok || !strings.Contains(tc.Text, "STRUCTURED PLAN MODE") {
		t.Fatalf("injected part missing plan directive: %#v", gotPlan[0].Parts[1])
	}
	if len(orig[0].Parts) != 1 {
		t.Fatalf("withPlanDirective mutated caller's message Parts")
	}
	// Helpers/summarizer must not receive the coder plan directive.
	gotHelper := withPlanDirective(config.AgentTask, orig)
	if len(gotHelper[0].Parts) != 1 {
		t.Fatalf("non-coder agent received plan directive")
	}
}
