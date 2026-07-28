package export

import (
	"strings"
	"testing"
	"time"

	"github.com/opencode-ai/opencode/internal/message"
	"github.com/opencode-ai/opencode/internal/session"
)

// Timestamps in the real database are UNIX SECONDS, though the migration
// comments say milliseconds in three places. This is the value that proves it:
// stored as seconds it is a plausible 2026 date, divided by 1000 it is 1970.
const realStoredTimestamp int64 = 1785228225

func TestStoredTimestampsAreReadAsSecondsNotMilliseconds(t *testing.T) {
	got := at(realStoredTimestamp).UTC().Year()
	if got == 1970 {
		t.Fatal("timestamps are being divided by 1000 somewhere — the schema comment says milliseconds and it is wrong; every exported time would read as January 1970")
	}
	if got != 2026 {
		t.Errorf("stored timestamp read as year %d, expected 2026", got)
	}
}

func fixture() (session.Session, []message.Message) {
	base := realStoredTimestamp
	sess := session.Session{ID: "ses_abc123", Title: "Fix the login overlay"}

	user := message.Message{
		ID: "m1", Role: message.User, SessionID: sess.ID, CreatedAt: base,
		Parts: []message.ContentPart{message.TextContent{Text: "escape does not dismiss the message"}},
	}
	assistant := message.Message{
		ID: "m2", Role: message.Assistant, SessionID: sess.ID, CreatedAt: base + 12,
		Model: "local.z-ai/glm-5.2",
		Parts: []message.ContentPart{
			message.ReasoningContent{Thinking: "The clear is in the keys.Quit branch, so esc never reaches it."},
			message.TextContent{Text: "Found it — the handler is in the wrong branch."},
			message.ToolCall{ID: "call_01", Name: "view", Input: `{"file_path":"/tmp/tui.go"}`},
		},
	}
	toolResult := message.Message{
		ID: "m3", Role: message.Tool, SessionID: sess.ID, CreatedAt: base + 15,
		Parts: []message.ContentPart{
			message.ToolResult{ToolCallID: "call_01", Name: "view", Content: "line 1\nline 2"},
		},
	}
	return sess, []message.Message{user, assistant, toolResult}
}

// The defect that made the old export useless for diagnosis: it switched on the
// message role and handled only User and Assistant, so every role "tool" message
// — every OUTCOME — was silently discarded. You could see what the agent decided
// and never what came back.
func TestToolResultsAppearInTheTranscript(t *testing.T) {
	sess, msgs := fixture()
	out := Render(sess, msgs, time.Unix(realStoredTimestamp+100, 0))

	for _, want := range []string{"Tool result", "call_01", "line 1", "line 2"} {
		if !strings.Contains(out, want) {
			t.Errorf("the transcript is missing %q — a record of decisions without outcomes cannot be used to reconstruct what happened:\n%s", want, out)
		}
	}
}

// A failed tool call must be impossible to miss: flagged inline AND counted in
// the header, so it is visible without reading the whole file.
func TestToolErrorsAreFlaggedInlineAndSummarised(t *testing.T) {
	sess, msgs := fixture()
	msgs = append(msgs, message.Message{
		ID: "m4", Role: message.Tool, SessionID: sess.ID, CreatedAt: realStoredTimestamp + 20,
		Parts: []message.ContentPart{
			message.ToolResult{ToolCallID: "call_02", Name: "bash", Content: "permission denied", IsError: true},
		},
	})
	out := Render(sess, msgs, time.Unix(realStoredTimestamp+100, 0))

	if !strings.Contains(out, "ERROR") {
		t.Error("a failed tool call is not marked inline")
	}
	// Match with markdown emphasis stripped, so a formatting tweak does not fail
	// a test that is about the content.
	if !strings.Contains(plain(header(out)), "Tool calls that failed: 1") {
		t.Errorf("the header does not count tool failures:\n%s", header(out))
	}
}

// Every message needs an absolute time, and an offset from the session start —
// the offset is what lets a transcript be lined up against a build log.
func TestEveryMessageCarriesAnAbsoluteTimeAndAnOffset(t *testing.T) {
	sess, msgs := fixture()
	out := Render(sess, msgs, time.Unix(realStoredTimestamp+100, 0))

	if n := strings.Count(out, "2026-07-28"); n < 3 {
		t.Errorf("expected a date on all 3 messages, found %d:\n%s", n, out)
	}
	// First message is the origin, the later ones are offset from it.
	for _, want := range []string{"(+00:00:00)", "(+00:00:12)", "(+00:00:15)"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing session offset %s — without it you cannot tell how far into a five-hour session something happened", want)
		}
	}
}

// Reasoning is what explains how a conclusion was reached. It is the whole point
// of keeping a record rather than just an answer.
func TestReasoningIsRendered(t *testing.T) {
	sess, msgs := fixture()
	out := Render(sess, msgs, time.Unix(realStoredTimestamp+100, 0))

	if !strings.Contains(out, "Reasoning") || !strings.Contains(out, "keys.Quit branch") {
		t.Errorf("the model's reasoning is absent from the transcript:\n%s", out)
	}
}

// Which model produced which answer. A session can span several after a /model
// switch, and it is usually the first question asked of a transcript.
func TestModelIsAttributedPerMessageAndSummarised(t *testing.T) {
	sess, msgs := fixture()
	out := Render(sess, msgs, time.Unix(realStoredTimestamp+100, 0))

	if !strings.Contains(out, "local.z-ai/glm-5.2") {
		t.Error("the answering model is not named on the message")
	}
	if !strings.Contains(header(out), "Models used:") {
		t.Errorf("the header does not summarise which models served the session:\n%s", header(out))
	}
}

// An interrupted turn must say so. "canceled" is often the single most important
// fact in a transcript — it means the output is incomplete by design, not broken.
func TestAbnormalEndingsAreReported(t *testing.T) {
	sess, msgs := fixture()
	msgs = append(msgs, message.Message{
		ID: "m5", Role: message.Assistant, SessionID: sess.ID, CreatedAt: realStoredTimestamp + 30,
		Parts: []message.ContentPart{
			message.TextContent{Text: "partial answer"},
			message.Finish{Reason: message.FinishReasonCanceled},
		},
	})
	out := Render(sess, msgs, time.Unix(realStoredTimestamp+100, 0))

	if !strings.Contains(out, "Ended: canceled") {
		t.Errorf("an interrupted turn is not marked:\n%s", out)
	}
	if !strings.Contains(header(out), "Abnormal endings:") {
		t.Errorf("the header does not surface abnormal endings:\n%s", header(out))
	}
}

// Order is the timeline. A caller handing over rows in any other order must not
// be able to produce a misleading record.
func TestMessagesAreSortedChronologicallyRegardlessOfInputOrder(t *testing.T) {
	sess, msgs := fixture()
	shuffled := []message.Message{msgs[2], msgs[0], msgs[1]}

	out := Render(sess, shuffled, time.Unix(realStoredTimestamp+100, 0))

	iUser := strings.Index(out, "] User")
	iAssistant := strings.Index(out, "] Assistant")
	iTool := strings.Index(out, "] Tool result")
	if !(iUser < iAssistant && iAssistant < iTool) {
		t.Errorf("out of order: user=%d assistant=%d tool=%d", iUser, iAssistant, iTool)
	}
}

// Malformed tool input must survive verbatim. Discarding what was actually sent
// because it failed to parse defeats the purpose of the record.
func TestUnparseableToolInputIsPreservedVerbatim(t *testing.T) {
	const broken = `{"file_path": "/tmp/x.go", TRUNCATED`
	if got := prettyJSON(broken); got != broken {
		t.Errorf("malformed input was altered:\n  in:  %s\n  out: %s", broken, got)
	}
	// Valid JSON is reformatted for reading.
	if got := prettyJSON(`{"b":2,"a":1}`); !strings.Contains(got, "\n") {
		t.Errorf("valid JSON was not pretty-printed: %s", got)
	}
}

func TestEmptySessionDoesNotPanic(t *testing.T) {
	out := Render(session.Session{ID: "ses_empty"}, nil, time.Unix(realStoredTimestamp, 0))
	if !strings.Contains(out, "Untitled session") {
		t.Errorf("an empty session produced no usable header:\n%s", out)
	}
}

func TestHumanDurationSpansHours(t *testing.T) {
	cases := map[time.Duration]string{
		0:                            "00:00:00",
		45 * time.Second:             "00:00:45",
		90 * time.Second:             "00:01:30",
		5 * time.Hour:                "05:00:00",
		5*time.Hour + 61*time.Second: "05:01:01",
		-1 * time.Second:             "00:00:00",
	}
	for d, want := range cases {
		if got := humanDuration(d); got != want {
			t.Errorf("humanDuration(%v) = %s, want %s", d, got, want)
		}
	}
}

// Found by rendering a real session, not by reasoning about the code: stored tool
// RESULTS usually have an empty Name — only the call carries it. Rows came out as
// "← (ERROR)" with no indication of which tool had failed. The result must borrow
// the name of the call it answers.
func TestToolResultBorrowsTheCallNameWhenItHasNone(t *testing.T) {
	sess, _ := fixture()
	base := realStoredTimestamp
	msgs := []message.Message{
		{
			ID: "m1", Role: message.Assistant, SessionID: sess.ID, CreatedAt: base,
			Parts: []message.ContentPart{
				message.ToolCall{ID: "call_xyz", Name: "bash", Input: `{"command":"ls"}`},
			},
		},
		{
			// Exactly as the real database stores it: no Name on the result.
			ID: "m2", Role: message.Tool, SessionID: sess.ID, CreatedAt: base + 3,
			Parts: []message.ContentPart{
				message.ToolResult{ToolCallID: "call_xyz", Content: "boom", IsError: true},
			},
		},
	}

	out := Render(sess, msgs, time.Unix(base+50, 0))

	if strings.Contains(out, "← ** (ERROR)") || strings.Contains(out, "←  (ERROR)") {
		t.Errorf("a failed tool result rendered with no tool name:\n%s", out)
	}
	if !strings.Contains(out, "bash** (ERROR)") {
		t.Errorf("the result did not borrow \"bash\" from the call it answers:\n%s", out)
	}
}

// And when nothing can be recovered, say so rather than rendering a blank.
func TestUnpairableToolResultIsLabelled(t *testing.T) {
	sess, _ := fixture()
	msgs := []message.Message{{
		ID: "m1", Role: message.Tool, SessionID: sess.ID, CreatedAt: realStoredTimestamp,
		Parts: []message.ContentPart{message.ToolResult{ToolCallID: "orphan", Content: "x"}},
	}}
	if out := Render(sess, msgs, time.Unix(realStoredTimestamp, 0)); !strings.Contains(out, "unknown tool") {
		t.Errorf("an unpairable result rendered without a label:\n%s", out)
	}
}

// header returns just the metadata block, for assertions that care about the
// summary rather than the body.
func header(out string) string {
	if i := strings.Index(out, "\n---\n"); i > 0 {
		return out[:i]
	}
	return out
}

// plain strips markdown emphasis so content assertions survive formatting changes.
func plain(s string) string { return strings.ReplaceAll(s, "**", "") }

// The honest form of "reasoning costs extra": report what was actually captured.
//
// Exact reasoning-token counts are NOT available — measured against NVIDIA NIM,
// the usage object carries only prompt/completion/total tokens. So the header
// reports stored characters plus an estimate, and must SAY it is an estimate.
func TestReasoningVolumeIsReportedAsAnEstimateNotAFact(t *testing.T) {
	sess, msgs := fixture()
	out := header(Render(sess, msgs, time.Unix(realStoredTimestamp+100, 0)))

	if !strings.Contains(out, "Reasoning captured:") {
		t.Errorf("the header does not report how much reasoning was generated:\n%s", out)
	}
	if !strings.Contains(out, "estimate") {
		t.Error("the token figure is not marked as an estimate — a derived number presented as exact is worse than an obviously rough one")
	}
	if !strings.Contains(out, "does not report an exact count") {
		t.Error("the header does not explain why the count is approximate, which reads as sloppiness rather than honesty")
	}
	if !strings.Contains(out, "4 chars/token") {
		t.Error("the estimation method is not stated, so the number cannot be checked")
	}
}

// A session with no reasoning must not claim a zero volume — that would read as
// "reasoning ran and cost nothing", which is a different claim entirely.
func TestNoReasoningMeansNoVolumeLine(t *testing.T) {
	sess := session.Session{ID: "s1", Title: "no thinking here"}
	msgs := []message.Message{{
		ID: "m1", Role: message.Assistant, CreatedAt: realStoredTimestamp,
		Parts: []message.ContentPart{message.TextContent{Text: "just an answer"}},
	}}

	if out := header(Render(sess, msgs, time.Unix(realStoredTimestamp, 0))); strings.Contains(out, "Reasoning captured") {
		t.Errorf("a session with no reasoning reported a volume:\n%s", out)
	}
}
