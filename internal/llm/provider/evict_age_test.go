package provider

import (
	"fmt"
	"strings"
	"testing"

	"github.com/opencode-ai/opencode/internal/message"
)

// big is a result comfortably over the size floor.
func big(prefix string) string {
	return prefix + strings.Repeat("x", evictAgeMinContent)
}

func call(id, name, input string) message.Message {
	return message.Message{
		Role:  message.Assistant,
		Parts: []message.ContentPart{message.ToolCall{ID: id, Name: name, Input: input}},
	}
}

func result(id, name, content string) message.Message {
	return message.Message{
		Role:  message.Tool,
		Parts: []message.ContentPart{message.ToolResult{ToolCallID: id, Name: name, Content: content}},
	}
}

// assistantTurns appends n assistant messages, which is how age is counted.
func assistantTurns(n int) []message.Message {
	out := make([]message.Message, 0, n)
	for i := 0; i < n; i++ {
		out = append(out, message.Message{
			Role:  message.Assistant,
			Parts: []message.ContentPart{message.TextContent{Text: fmt.Sprintf("turn %d", i)}},
		})
	}
	return out
}

func contentOf(t *testing.T, msgs []message.Message, callID string) string {
	t.Helper()
	for _, m := range msgs {
		for _, p := range m.Parts {
			if tr, ok := p.(message.ToolResult); ok && tr.ToolCallID == callID {
				return tr.Content
			}
		}
	}
	t.Fatalf("no result for %s", callID)
	return ""
}

// The case this exists for: a result read once, long ago, never read again.
// supersedeStaleReads never touches it, so before this it was carried at full
// price for the rest of the session.
func TestAnIdleReadIsDroppedWithAStubThatSaysSo(t *testing.T) {
	msgs := []message.Message{
		call("c1", "view", `{"path":"old.go"}`),
		result("c1", "view", big("OLD FILE ")),
	}
	// A different file, so c1 is never superseded.
	msgs = append(msgs, call("c2", "view", `{"path":"other.go"}`), result("c2", "view", big("OTHER ")))
	msgs = append(msgs, assistantTurns(evictAfterAssistantTurns+1)...)

	got := contentOf(t, evictAgedReads(msgs), "c1")
	if strings.Contains(got, "OLD FILE") {
		t.Error("an idle read survived; supersession never covers this case, which is why age eviction exists")
	}
	// The stub must be actionable, not merely shorter. A model that cannot see
	// that something was removed fills the gap from memory.
	for _, want := range []string{"dropped to save context", "session store", "run view again"} {
		if !strings.Contains(got, want) {
			t.Errorf("stub is missing %q; it reads:\n%s", want, got)
		}
	}
}

// Rule 2. Recent working context is never disturbed, whatever else is true.
func TestRecentResultsAreNeverTouched(t *testing.T) {
	msgs := []message.Message{
		call("c1", "view", `{"path":"a.go"}`),
		result("c1", "view", big("RECENT ")),
	}
	msgs = append(msgs, assistantTurns(evictAfterAssistantTurns-1)...)

	if got := contentOf(t, evictAgedReads(msgs), "c1"); !strings.Contains(got, "RECENT") {
		t.Errorf("a result %d turns old was evicted; the window is %d",
			evictAfterAssistantTurns-1, evictAfterAssistantTurns)
	}
}

// THE ANTI-HANDICAP TEST. A file the model is genuinely still working on gets
// re-read inside the window, and that fresh copy is untouchable. This, not any
// per-file exemption, is what stops eviction breaking a long edit session.
//
// The obvious alternative, "always keep the newest read of each target", was
// tried and removed: a file read once is the newest read of itself, so that rule
// made the whole function inert. See the note in evict_age.go.
func TestAReReadInsideTheWindowProtectsTheWorkingSet(t *testing.T) {
	path := `{"path":"working.go"}`
	msgs := []message.Message{
		call("c1", "view", path), result("c1", "view", big("STALE COPY ")),
	}
	msgs = append(msgs, assistantTurns(evictAfterAssistantTurns*5)...)
	// The model comes back to the file: a fresh read, recent, therefore safe.
	msgs = append(msgs, call("c2", "view", path), result("c2", "view", big("LIVE COPY ")))

	out := evictAgedReads(msgs)
	if !strings.Contains(contentOf(t, out, "c2"), "LIVE COPY") {
		t.Error("the fresh read was evicted; the recency window is what protects the working set")
	}
	if strings.Contains(contentOf(t, out, "c1"), "STALE COPY") {
		t.Error("the ancient copy survived even though a live one exists further down")
	}
}

// A lone ancient read IS evicted, and that is the point rather than a hazard.
// It is the case supersession can never reach, and the stub tells the model to
// re-read rather than to guess.
func TestALoneAncientReadIsEvictedBecauseThatIsThePoint(t *testing.T) {
	msgs := []message.Message{
		call("c1", "view", `{"path":"read-once.go"}`),
		result("c1", "view", big("READ ONCE ")),
	}
	msgs = append(msgs, assistantTurns(evictAfterAssistantTurns+1)...)

	if got := contentOf(t, evictAgedReads(msgs), "c1"); strings.Contains(got, "READ ONCE") {
		t.Error("a read-once-then-idle result survived. If this passes while the " +
			"feature does nothing, the guard has been reintroduced: see the removed " +
			"newest-per-key rule in evict_age.go.")
	}
}

// Rule 1. Non-reproducible and side-effectful tools are never evicted, however
// old. fetch and websearch cost the user real bandwidth to re-run and may
// return something different or nothing; bash and edit cannot be regenerated by
// repeating the call at all.
func TestOnlyLocallyReproducibleToolsAreEvicted(t *testing.T) {
	for _, tool := range []string{"fetch", "websearch", "bash", "edit", "write", "patch"} {
		msgs := []message.Message{
			call("c1", tool, `{"a":1}`), result("c1", tool, big("PRECIOUS ")),
			// A second unrelated read so the function does not early-return.
			call("c2", "view", `{"path":"x.go"}`), result("c2", "view", big("X ")),
		}
		msgs = append(msgs, assistantTurns(evictAfterAssistantTurns*3)...)

		if got := contentOf(t, evictAgedReads(msgs), "c1"); !strings.Contains(got, "PRECIOUS") {
			t.Errorf("%s output was evicted. Re-running it is not free and may not "+
				"reproduce: network cost on a metered link, or a side effect that "+
				"cannot be repeated.", tool)
		}
	}
}

// Rule 3. Errors are usually the thing being worked on, and a small result
// saves nothing.
func TestErrorsAndSmallResultsAreLeftAlone(t *testing.T) {
	msgs := []message.Message{
		call("c1", "view", `{"path":"a.go"}`),
		{Role: message.Tool, Parts: []message.ContentPart{
			message.ToolResult{ToolCallID: "c1", Name: "view", Content: big("BOOM "), IsError: true},
		}},
		call("c2", "find", `{"q":"z"}`), result("c2", "find", "tiny"),
		call("c3", "view", `{"path":"b.go"}`), result("c3", "view", big("B ")),
	}
	msgs = append(msgs, assistantTurns(evictAfterAssistantTurns+2)...)

	out := evictAgedReads(msgs)
	if !strings.Contains(contentOf(t, out, "c1"), "BOOM") {
		t.Error("an error result was evicted; errors are usually the thing being worked on")
	}
	if contentOf(t, out, "c2") != "tiny" {
		t.Error("a result below the size floor was stubbed, which costs more than it saves")
	}
}

// The pairing must survive. An orphaned tool_result is a hard API error, so the
// count of results and their IDs must be identical before and after.
func TestPairingIsPreserved(t *testing.T) {
	msgs := []message.Message{
		call("c1", "view", `{"path":"a.go"}`), result("c1", "view", big("A ")),
		call("c2", "find", `{"q":"b"}`), result("c2", "find", big("B ")),
		call("c3", "view", `{"path":"c.go"}`), result("c3", "view", big("C ")),
	}
	msgs = append(msgs, assistantTurns(evictAfterAssistantTurns+1)...)

	out := evictAgedReads(msgs)
	if len(out) != len(msgs) {
		t.Fatalf("message count changed: %d -> %d", len(msgs), len(out))
	}
	for _, id := range []string{"c1", "c2", "c3"} {
		if contentOf(t, out, id) == "" {
			t.Errorf("result %s vanished; an orphaned tool_call is a hard API error", id)
		}
	}
}

// The stored messages must not be mutated: this shapes the wire only, and the
// session store stays intact (the never-delete rule).
func TestTheStoredMessagesAreNotMutated(t *testing.T) {
	original := big("ORIGINAL ")
	msgs := []message.Message{
		call("c1", "view", `{"path":"a.go"}`), result("c1", "view", original),
		call("c2", "view", `{"path":"b.go"}`), result("c2", "view", big("B ")),
	}
	msgs = append(msgs, assistantTurns(evictAfterAssistantTurns+1)...)

	out := evictAgedReads(msgs)
	if strings.Contains(contentOf(t, out, "c1"), "ORIGINAL") {
		t.Fatal("precondition failed: c1 should have been evicted in this fixture")
	}
	if got := contentOf(t, msgs, "c1"); got != original {
		t.Error("the INPUT slice was mutated. This must shape only what goes on the " +
			"wire; the transcript on disk stays whole.")
	}
}

// Supersession runs first and leaves a short stub. That stub is below the size
// floor here, so a result can never be stubbed twice into nonsense.
func TestASupersededResultIsNotStubbedTwice(t *testing.T) {
	same := `{"path":"same.go"}`
	msgs := []message.Message{
		call("c1", "view", same), result("c1", "view", big("FIRST ")),
		call("c2", "view", same), result("c2", "view", big("SECOND ")),
	}
	msgs = append(msgs, assistantTurns(evictAfterAssistantTurns+1)...)

	out := evictAgedReads(supersedeStaleReads(msgs))
	got := contentOf(t, out, "c1")
	if strings.Count(got, "[") > 1 {
		t.Errorf("result was stubbed twice:\n%s", got)
	}
	if !strings.Contains(got, "superseded") {
		t.Errorf("the supersession stub should be the one that stuck, got:\n%s", got)
	}
}

// No evictable tools in the conversation means the slice comes back untouched,
// so the common cheap path stays cheap.
func TestNoEvictableToolsIsANoOp(t *testing.T) {
	msgs := []message.Message{
		call("c1", "bash", `{"cmd":"ls"}`), result("c1", "bash", big("OUT ")),
	}
	msgs = append(msgs, assistantTurns(evictAfterAssistantTurns*4)...)

	out := evictAgedReads(msgs)
	if &out[0] != &msgs[0] && !strings.Contains(contentOf(t, out, "c1"), "OUT") {
		t.Error("a conversation with no evictable tools was altered")
	}
}
