package agent

import (
	"context"
	"strings"
	"testing"

	"github.com/opencode-ai/opencode/internal/message"
)

// A session left with a tool call that never got a result (the program was
// killed mid-tool) must be sendable again. Providers reject that history.
func TestAToolCallWithNoResultIsAnsweredBeforeTheHistoryIsSent(t *testing.T) {
	call := func(id string) message.ToolCall { return message.ToolCall{ID: id, Name: "bash", Finished: true} }
	msgs := []message.Message{
		{Role: message.User, Parts: []message.ContentPart{message.TextContent{Text: "go"}}},
		{Role: message.Assistant, SessionID: "s", Parts: []message.ContentPart{call("a"), call("b"), call("c")}},
		{Role: message.Tool, Parts: []message.ContentPart{message.ToolResult{ToolCallID: "a", Content: "done"}}},
		// b and c never came back.
		{Role: message.User, Parts: []message.ContentPart{message.TextContent{Text: "continue"}}},
		{Role: message.Assistant, Parts: []message.ContentPart{call("d")}}, // the very last thing: also orphaned
	}
	out := answerOrphanedToolCalls(msgs)

	answered := map[string]string{}
	for i, m := range out {
		for _, r := range m.ToolResults() {
			answered[r.ToolCallID] = r.Content
			// A result must come after its call and before the next user turn.
			if i == 0 || (out[i-1].Role != message.Assistant && out[i-1].Role != message.Tool) {
				t.Errorf("result for %s is not directly after its call", r.ToolCallID)
			}
		}
	}
	for _, id := range []string{"a", "b", "c", "d"} {
		if _, ok := answered[id]; !ok {
			t.Errorf("tool call %s still has no result; the provider will refuse this history", id)
		}
	}
	if answered["a"] != "done" {
		t.Errorf("a real result was replaced: %q", answered["a"])
	}
	if !strings.Contains(answered["b"], "never returned") || !strings.Contains(answered["b"], "may or may not") {
		t.Errorf("the synthetic result does not tell the truth about what is known: %q", answered["b"])
	}
	if len(msgs) != 5 {
		t.Error("the stored history was modified; only the copy being sent may be")
	}
	// A sound history comes back unchanged.
	sound := msgs[:3]
	sound[2] = message.Message{Role: message.Tool, Parts: []message.ContentPart{
		message.ToolResult{ToolCallID: "a"}, message.ToolResult{ToolCallID: "b"}, message.ToolResult{ToolCallID: "c"}}}
	if got := answerOrphanedToolCalls(sound); len(got) != 3 {
		t.Errorf("a complete history grew from 3 to %d messages", len(got))
	}
}

// Arguments the tools can mend must REACH the tools. Before 2026-10-05 this
// check refused them first, so the repair never ran in the loop.
func TestRepairableArgumentsAreNotRefusedAndTruncatedOnesAreNotBlamedOnTransport(t *testing.T) {
	for name, input := range map[string]string{
		"sound":                  `{"file_path":"a.go"}`,
		"unescaped Windows path": `{"file_path":"C:\Users\me\project\main.go"}`,
		"real newline in string": "{\"content\":\"line one\nline two\"}",
		"empty":                  "",
	} {
		if why := corruptedToolInput(input); why != "" {
			t.Errorf("%s: refused (%s); the tool-side repair handles this", name, why)
		}
	}

	// Cut off by the output limit: half a call.
	cut := corruptedToolInput(`{"file_path":"a.go","content":"package main\n\nfunc ma`)
	if !strings.HasPrefix(cut, notValidJSON) {
		t.Fatalf("a truncated call was not recognised: %q", cut)
	}
	advice := corruptedToolAdvice(cut)
	if strings.Contains(advice, "again unchanged") && !strings.Contains(advice, "will fail") {
		t.Errorf("a truncated call is told to resend unchanged, which loops: %q", advice)
	}
	if !strings.Contains(advice, "SMALLER") {
		t.Errorf("the advice does not say what to do instead: %q", advice)
	}

	// Real transport damage keeps the transport wording.
	damaged := corruptedToolInput("{\"\ufffd\ufffdcommand\":\"\"}")
	if damaged == "" || !strings.Contains(corruptedToolAdvice(damaged), "again unchanged") {
		t.Errorf("replacement characters: reason=%q", damaged)
	}
}

// Cancel asks the run to stop. It does not declare the session free: only the
// run that holds the session releases it.
func TestCancelDoesNotFreeASessionThatIsStillRunning(t *testing.T) {
	a := &agent{}
	cancelled := false
	run := &activeRun{cancel: context.CancelFunc(func() { cancelled = true })}
	a.activeRequests.Store("s", run)

	a.Cancel("s")
	if !cancelled {
		t.Fatal("Cancel did not cancel the run")
	}
	if !a.IsSessionBusy("s") || !a.IsBusy() {
		t.Fatal("the session reads as idle while its run has not finished: a second run could start beside it")
	}

	// An older run finishing must not remove a newer run's entry.
	newer := &activeRun{cancel: func() {}}
	a.activeRequests.Store("s", newer)
	a.activeRequests.CompareAndDelete("s", run) // what the old run does when it ends
	if !a.IsSessionBusy("s") {
		t.Fatal("an old run removed the new run's entry")
	}
	a.activeRequests.CompareAndDelete("s", newer)
	if a.IsSessionBusy("s") || a.IsBusy() {
		t.Fatal("the session is still busy after its own run released it")
	}
}
