package export

// GORILLA OVERRIDE (2026-08-18): tests for the handoff brief.
//
// The brief is read by a model that was not there. Everything dangerous about
// it follows from that one fact: if it reads as settled fact, "someone was
// working on this" becomes "this is done", and a half-finished change gets
// built on or committed. So the tests concentrate on what the brief must never
// let a reader assume.

import (
	"strings"
	"testing"

	"github.com/opencode-ai/opencode/internal/message"
	"github.com/opencode-ai/opencode/internal/session"
)

func handoffFixture() (session.Session, []message.Message) {
	base := realStoredTimestamp
	sess := session.Session{ID: "s1", Title: "Fix the VA-API decode path"}

	return sess, []message.Message{
		{
			ID: "m1", Role: message.User, SessionID: "s1", CreatedAt: base,
			Parts: []message.ContentPart{message.TextContent{Text: "make hardware decode work in the browser"}},
		},
		{
			ID: "m2", Role: message.Assistant, SessionID: "s1", CreatedAt: base + 10,
			Model: "local.z-ai/glm-5.2",
			Parts: []message.ContentPart{
				message.ReasoningContent{Thinking: "probably the codec gate"},
				message.TextContent{Text: "Patching the gate now."},
				message.ToolCall{ID: "c1", Name: "edit", Input: `{"file_path":"/home/x/gate.cpp"}`},
				message.ToolCall{ID: "c2", Name: "bash", Input: `{"command":"make -j4 2>&1 | tail"}`},
			},
		},
		{
			ID: "m3", Role: message.Tool, SessionID: "s1", CreatedAt: base + 20,
			Parts: []message.ContentPart{
				message.ToolResult{ToolCallID: "c2", Name: "bash", Content: "gate.cpp:41: error: no member named 'vaapi'", IsError: true},
			},
		},
		{
			ID: "m4", Role: message.User, SessionID: "s1", CreatedAt: base + 30,
			Parts: []message.ContentPart{message.TextContent{Text: "no — do not patch the encoder, check SharedArrayBuffer first"}},
		},
		{
			ID: "m5", Role: message.Assistant, SessionID: "s1", CreatedAt: base + 40,
			Model: "local.z-ai/glm-5.2",
			Parts: []message.ContentPart{message.TextContent{Text: "Right, checking the header now."}},
		},
	}
}

func TestBriefCarriesEveryInstructionVerbatimAndInOrder(t *testing.T) {
	sess, msgs := handoffFixture()
	brief, stats := Handoff(sess, msgs, nil, 0)

	if stats.Instructions != 2 {
		t.Errorf("counted %d instructions, want 2", stats.Instructions)
	}
	// The correction is the most valuable line in the session — it records where
	// the previous attempt went wrong — and it must survive verbatim.
	if !strings.Contains(brief, "do not patch the encoder, check SharedArrayBuffer first") {
		t.Error("the user's correction is missing; the next model will repeat the mistake")
	}
	if !strings.Contains(brief, "make hardware decode work in the browser") {
		t.Error("the original goal is missing")
	}
	first := strings.Index(brief, "make hardware decode")
	second := strings.Index(brief, "do not patch the encoder")
	if first > second {
		t.Error("instructions are out of order; a later correction must come after what it corrects")
	}
	if !strings.Contains(brief, "later instruction wins") {
		t.Error("the brief does not say how to resolve conflicting instructions")
	}
}

func TestBriefReportsWhatWasDoneAndWhatFailed(t *testing.T) {
	sess, msgs := handoffFixture()
	brief, stats := Handoff(sess, msgs, nil, 0)

	if stats.FilesTouched != 1 || stats.Commands != 1 || stats.Failures != 1 {
		t.Errorf("stats: %d files, %d commands, %d failures; want 1, 1, 1",
			stats.FilesTouched, stats.Commands, stats.Failures)
	}
	for _, want := range []string{
		"/home/x/gate.cpp",
		"make -j4",
		"no member named 'vaapi'",
		"Right, checking the header now.", // where it stopped
	} {
		if !strings.Contains(brief, want) {
			t.Errorf("the brief is missing %q", want)
		}
	}

	// The failure must be readable BEFORE the hopeful last line, or the reader
	// finishes on "checking the header now" and assumes it went fine.
	if strings.Index(brief, "no member named") > strings.Index(brief, "Right, checking the header") {
		t.Error("the failure is reported after the last thing said; it will be read as already resolved")
	}
}

// The property that makes this safe to hand to a different model.
func TestBriefNeverPresentsItselfAsVerifiedFact(t *testing.T) {
	sess, msgs := handoffFixture()
	brief, _ := Handoff(sess, msgs, nil, 0)

	for _, want := range []string{
		"not a claim that any of it is correct",
		"Whether any of the work above is **correct**",
		"Whether the work is **finished**",
		"Check before you redo",
	} {
		if !strings.Contains(brief, want) {
			t.Errorf("the brief is missing its honesty clause %q — a reader could take it as settled", want)
		}
	}
}

// A session where nothing failed must not be described as a session where
// everything worked.
func TestNoRecordedFailuresIsNotReportedAsSuccess(t *testing.T) {
	sess := session.Session{ID: "s", Title: "quiet run"}
	msgs := []message.Message{{
		ID: "m1", Role: message.User, CreatedAt: realStoredTimestamp,
		Parts: []message.ContentPart{message.TextContent{Text: "do the thing"}},
	}}
	brief, _ := Handoff(sess, msgs, nil, 0)
	if !strings.Contains(brief, "not the same as everything") {
		t.Error("an absence of recorded failures is being presented as success")
	}
}

// A session cut off mid-turn has no closing message. Saying nothing there would
// leave the reader to invent an ending.
func TestASessionCutOffMidTurnSaysSo(t *testing.T) {
	sess := session.Session{ID: "s", Title: "power cut"}
	msgs := []message.Message{{
		ID: "m1", Role: message.User, CreatedAt: realStoredTimestamp,
		Parts: []message.ContentPart{message.TextContent{Text: "build the kernel"}},
	}}
	brief, _ := Handoff(sess, msgs, nil, 0)
	if !strings.Contains(brief, "cut off mid-turn") {
		t.Error("a session that ended with no assistant message does not say so")
	}
}

// A research run's work is in its lanes. A brief from the orchestrator alone
// describes the wrapper.
func TestHelperSessionsContributeWorkAndFailuresButNotInstructions(t *testing.T) {
	sess, msgs := handoffFixture()
	branches := []Branch{{
		Session: session.Session{ID: "call_a-local", Title: "Research: local"},
		Messages: []message.Message{
			{
				ID: "h1", Role: message.User, CreatedAt: realStoredTimestamp,
				// A GENERATED prompt. It must never appear as a user instruction.
				Parts: []message.ContentPart{message.TextContent{Text: "You are helper 1 of 10 in a research investigation."}},
			},
			{
				ID: "h2", Role: message.Assistant, CreatedAt: realStoredTimestamp + 1,
				Parts: []message.ContentPart{
					message.ToolCall{ID: "h-c1", Name: "write", Input: `{"file_path":"/home/x/notes.md"}`},
				},
			},
			{
				ID: "h3", Role: message.Tool, CreatedAt: realStoredTimestamp + 2,
				Parts: []message.ContentPart{
					message.ToolResult{ToolCallID: "h-c1", Name: "web_search", Content: "rate limited", IsError: true},
				},
			},
		},
	}}

	brief, stats := Handoff(sess, msgs, branches, 0)

	if !strings.Contains(brief, "/home/x/notes.md") {
		t.Error("a file written by a helper is missing; the lanes are where the work happens")
	}
	if !strings.Contains(brief, "Research: local") || !strings.Contains(brief, "rate limited") {
		t.Error("a helper's failure is missing, and it is not attributed to the helper")
	}
	if strings.Contains(brief, "You are helper 1 of 10") {
		t.Error("a generated helper prompt was presented as a user instruction")
	}
	if stats.Instructions != 2 {
		t.Errorf("helper prompts inflated the instruction count to %d", stats.Instructions)
	}
	if !strings.Contains(brief, "1 helper sessions") && !strings.Contains(brief, "helper sessions") {
		t.Error("the brief does not say the count spans helper sessions")
	}
}

// Truncation must never cost the instructions or the guidance — the two parts
// that decide whether the next model does the right thing.
func TestTruncationKeepsTheGoalAndTheGuidance(t *testing.T) {
	sess, msgs := handoffFixture()
	// A wall of commands, so the middle section is what has to give.
	noise := make([]message.Message, 0, 200)
	for i := range 200 {
		noise = append(noise, message.Message{
			ID: string(rune('a' + i%26)), Role: message.Assistant, CreatedAt: realStoredTimestamp + int64(50+i),
			Parts: []message.ContentPart{message.ToolCall{
				ID: "x", Name: "bash", Input: `{"command":"` + strings.Repeat("echo padding; ", 40) + `"}`,
			}},
		})
	}
	full, _ := Handoff(sess, append(msgs, noise...), nil, 0)
	small, stats := Handoff(sess, append(msgs, noise...), nil, 3000)

	if !stats.Truncated {
		t.Fatalf("a %d-character brief was not truncated to a 3000-character budget", len(full))
	}
	if len(small) > 3200 {
		t.Errorf("budget was 3000 characters, produced %d", len(small))
	}
	if !strings.Contains(small, "do not patch the encoder") {
		t.Error("truncation dropped the user's correction — the most valuable line in the brief")
	}
	if !strings.Contains(small, "Check before you redo") {
		t.Error("truncation dropped the guidance that stops the next model repeating the work")
	}
	if !strings.Contains(small, "dropped to fit") && !strings.Contains(small, "TRUNCATED") {
		t.Error("content was dropped silently; the reader must be told the brief is partial")
	}
}
