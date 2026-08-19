package plain

import (
	"bufio"
	"bytes"
	"strings"
	"testing"

	"github.com/opencode-ai/opencode/internal/config"
	"github.com/opencode-ai/opencode/internal/message"
)

// newRenderer builds just enough of a Session to exercise the rendering path.
// The app is not needed: render/stream/toolCalls/toolResults touch only the
// writer and the seen-maps.
func newRenderer() (*Session, *bytes.Buffer) {
	var buf bytes.Buffer
	return &Session{
		out:              &buf,
		in:               bufio.NewScanner(strings.NewReader("")),
		printed:          map[string]int{},
		reasoningPrinted: map[string]int{},
		toolNames:        map[string]string{},
	}, &buf
}

func withExtras(t *testing.T, state map[string]bool) {
	t.Helper()
	if _, err := config.Load(t.TempDir(), false); err != nil {
		t.Fatalf("config.Load: %v", err)
	}
	for id, on := range state {
		before := config.ExtraEnabled(id)
		if err := config.SetExtra(id, on); err != nil {
			t.Fatalf("SetExtra(%s): %v", id, err)
		}
		id := id
		t.Cleanup(func() { config.SetExtra(id, before) })
	}
}

// The whole point of this mode: ordinary selectable text. A single escape byte
// would mean the terminal is being drawn on rather than written to, and the
// output would not survive a copy into a text editor.
func TestOutputContainsNoTerminalEscapes(t *testing.T) {
	withExtras(t, map[string]bool{
		"extras-reasoning-show": true, "extras-toolcalls-show": true, "extras-timestamps-show": true,
	})
	s, buf := newRenderer()

	s.render(message.Message{
		ID: "m1", Role: message.Assistant, Model: "local.z-ai/glm-5.2",
		Parts: []message.ContentPart{
			message.ReasoningContent{Thinking: "working it out"},
			message.TextContent{Text: "the answer"},
			message.ToolCall{ID: "c1", Name: "view", Input: `{"file_path":"/x"}`},
		},
	})
	s.render(message.Message{
		ID: "m2", Role: message.Tool,
		Parts: []message.ContentPart{message.ToolResult{ToolCallID: "c1", Content: "file body"}},
	})

	if n := bytes.Count(buf.Bytes(), []byte{0x1b}); n != 0 {
		t.Errorf("%d escape byte(s) in the output — this mode exists so the text can be selected and copied:\n%q", n, buf.String())
	}
}

// Every update carries the WHOLE message, not a delta. Without tracking how much
// has been written, the reply is re-printed from the beginning on every token.
func TestStreamingPrintsOnlyTheNewSuffix(t *testing.T) {
	withExtras(t, map[string]bool{"extras-reasoning-show": false, "extras-timestamps-show": false})
	s, buf := newRenderer()

	for _, text := range []string{"Hel", "Hello", "Hello wor", "Hello world"} {
		s.render(message.Message{
			ID: "m1", Role: message.Assistant,
			Parts: []message.ContentPart{message.TextContent{Text: text}},
		})
	}

	out := buf.String()
	if n := strings.Count(out, "Hello world"); n != 1 {
		t.Errorf("the final text appears %d times; each update re-printed the whole message:\n%q", n, out)
	}
	if strings.Count(out, "assistant") != 1 {
		t.Errorf("the heading was printed more than once:\n%q", out)
	}
}

// A tool result stores no name — only the call does. Without pairing, a failure
// reads "<- unknown tool (ERROR)" and you cannot tell what broke. This is the same
// defect that showed up in the export renderer against real data.
func TestToolResultBorrowsTheCallName(t *testing.T) {
	withExtras(t, map[string]bool{"extras-toolcalls-show": true, "extras-timestamps-show": false})
	s, buf := newRenderer()

	s.render(message.Message{
		ID: "m1", Role: message.Assistant,
		Parts: []message.ContentPart{message.ToolCall{ID: "c9", Name: "bash", Input: `{"command":"ls"}`}},
	})
	s.render(message.Message{
		// Exactly as stored: no Name on the result.
		ID: "m2", Role: message.Tool,
		Parts: []message.ContentPart{message.ToolResult{ToolCallID: "c9", Content: "boom", IsError: true}},
	})

	out := buf.String()
	if !strings.Contains(out, "<- bash (ERROR)") {
		t.Errorf("the result did not borrow \"bash\" from the call it answers:\n%s", out)
	}
	if strings.Contains(out, "unknown tool") {
		t.Errorf("a pairable result rendered as unknown:\n%s", out)
	}
}

// Each tool call and result is announced exactly once, however many updates carry
// it — otherwise a long turn repeats every call on every token.
func TestToolCallsAndResultsArePrintedOnce(t *testing.T) {
	withExtras(t, map[string]bool{"extras-toolcalls-show": true, "extras-timestamps-show": false})
	s, buf := newRenderer()

	m := message.Message{
		ID: "m1", Role: message.Assistant,
		Parts: []message.ContentPart{message.ToolCall{ID: "c1", Name: "ls", Input: "{}"}},
	}
	for i := 0; i < 5; i++ {
		s.render(m)
	}
	if n := strings.Count(buf.String(), "-> ls"); n != 1 {
		t.Errorf("the tool call was announced %d times, want once:\n%s", n, buf.String())
	}
}

// The toggles must actually change the output, in this mode too.
func TestExtrasTogglesChangeTheOutput(t *testing.T) {
	msg := message.Message{
		ID: "m1", Role: message.Assistant,
		Parts: []message.ContentPart{
			message.ReasoningContent{Thinking: "SECRET-WORKING"},
			message.TextContent{Text: "the answer"},
			message.ToolCall{ID: "c1", Name: "bash", Input: "{}"},
		},
	}

	t.Run("all on", func(t *testing.T) {
		withExtras(t, map[string]bool{
			"extras-reasoning-show": true, "extras-toolcalls-show": true, "extras-timestamps-show": true,
		})
		s, buf := newRenderer()
		s.render(msg)
		out := buf.String()
		for _, want := range []string{"SECRET-WORKING", "-> bash", "the answer"} {
			if !strings.Contains(out, want) {
				t.Errorf("missing %q with everything on:\n%s", want, out)
			}
		}
	})

	t.Run("all off", func(t *testing.T) {
		withExtras(t, map[string]bool{
			"extras-reasoning-show": false, "extras-toolcalls-show": false, "extras-timestamps-show": false,
		})
		s, buf := newRenderer()
		s.render(msg)
		out := buf.String()
		if strings.Contains(out, "SECRET-WORKING") {
			t.Errorf("reasoning shown while disabled:\n%s", out)
		}
		if strings.Contains(out, "-> bash") {
			t.Errorf("tool call shown while disabled:\n%s", out)
		}
		// The answer must survive regardless — it is not an "extra".
		if !strings.Contains(out, "the answer") {
			t.Errorf("the answer was lost when the extras were turned off:\n%s", out)
		}
	})
}

// Reasoning and answer are separate blocks. Merging them would send the model's
// private working back as its own prior output on the next turn.
func TestReasoningAndAnswerAreDistinctBlocks(t *testing.T) {
	withExtras(t, map[string]bool{"extras-reasoning-show": true, "extras-timestamps-show": false})
	s, buf := newRenderer()

	s.render(message.Message{
		ID: "m1", Role: message.Assistant,
		Parts: []message.ContentPart{
			message.ReasoningContent{Thinking: "the working"},
			message.TextContent{Text: "the answer"},
		},
	})

	out := buf.String()
	iThinking := strings.Index(out, "thinking:")
	iAssistant := strings.Index(out, "assistant")
	if iThinking < 0 || iAssistant < 0 {
		t.Fatalf("expected both blocks:\n%s", out)
	}
	if iThinking > iAssistant {
		t.Errorf("reasoning printed after the answer it explains:\n%s", out)
	}
}

// A pasted prompt can be far larger than bufio's default 64K line limit — a stack
// trace or a whole file. Silently truncating input would be the worst kind of bug.
func TestLongPastedInputIsNotTruncated(t *testing.T) {
	long := strings.Repeat("x", 900_000)
	s := New(nil, strings.NewReader(long+"\n"), &bytes.Buffer{})

	if !s.in.Scan() {
		t.Fatalf("a %d-character line could not be read: %v", len(long), s.in.Err())
	}
	if got := len(s.in.Text()); got != len(long) {
		t.Errorf("read %d characters of %d — the input was truncated", got, len(long))
	}
}

func TestOneLineCollapsesAndClipsToolInput(t *testing.T) {
	if got := oneLine("{\n  \"a\": 1,\n  \"b\": 2\n}"); strings.Contains(got, "\n") {
		t.Errorf("newlines survived: %q", got)
	}
	long := oneLine(strings.Repeat("y", 500))
	if n := len([]rune(long)); n > 120 {
		t.Errorf("clipped to %d runes, want <=120", n)
	}
	// ASCII marker since 2026-08-19; see internal/tui/styles/ascii.go.
	if !strings.HasSuffix(long, "...") {
		t.Errorf("clipped text does not show it was cut: %q", long)
	}
}
