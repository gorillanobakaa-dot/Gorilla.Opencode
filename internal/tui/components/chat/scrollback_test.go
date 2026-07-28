package chat

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/opencode-ai/opencode/internal/message"
)

func finishedAssistant(id, text string, at int64) message.Message {
	return message.Message{
		ID:        id,
		Role:      message.Assistant,
		CreatedAt: at,
		Parts: []message.ContentPart{
			message.TextContent{Text: text},
			message.Finish{Reason: message.FinishReasonEndTurn, Time: at + 3},
		},
	}
}

func streamingAssistant(id, text string, at int64) message.Message {
	return message.Message{
		ID:        id,
		Role:      message.Assistant,
		CreatedAt: at,
		Parts:     []message.ContentPart{message.TextContent{Text: text}},
	}
}

// Printing to the terminal cannot be undone. The pane may re-render a message on
// every token because it owns its buffer; scrollback has no such luxury, so a
// message is only printable once it will not change again. Getting this wrong
// does not look like a crash — it looks like a reply pasted into the transcript
// six times, growing each time.
func TestOnlySettledMessagesAreReadyForScrollback(t *testing.T) {
	const at int64 = 1785228225

	if !ScrollbackReady(userMsg("hello", at)) {
		t.Error("a user message is not considered settled; it exists in full the " +
			"moment it is created and would never be printed")
	}
	if ScrollbackReady(streamingAssistant("m2", "partial ans", at)) {
		t.Error("a still-streaming assistant message is considered settled; printing " +
			"it would emit the reply again on every token, since each update carries " +
			"the whole message")
	}
	if !ScrollbackReady(finishedAssistant("m2", "the answer", at)) {
		t.Error("a finished assistant message is not considered settled, so nothing " +
			"would ever reach the scrollback")
	}
	// A tool message reaches the pane only through the assistant message that owns
	// it. Printing it in its own right would duplicate it.
	if ScrollbackReady(message.Message{ID: "m3", Role: message.Tool}) {
		t.Error("a tool message is printable on its own; its content is already " +
			"rendered inside the assistant message that called it")
	}
}

// The point of reusing the pane's renderer is that scrollback and pane cannot
// drift. If this file ever grows its own formatter, this test is what should stop
// it: the printed text must be the pane's text.
func TestScrollbackRenderMatchesThePaneRenderer(t *testing.T) {
	const (
		at    int64 = 1785228225
		width       = 80
	)
	msg := userMsg("a question worth asking", at)

	got := RenderForScrollback(msg, 0, []message.Message{msg}, nil, width)
	want := strings.TrimRight(renderUserMessage(msg, false, width, 0).content, "\n")

	if got != want {
		t.Errorf("scrollback rendering diverged from the transcript pane.\n got: %q\nwant: %q", got, want)
	}
	if strings.TrimSpace(got) == "" {
		t.Fatal("rendered nothing, so the comparison above is vacuous")
	}
}

// Every printed line must fit the terminal, because the terminal will wrap
// anything that does not — and a wrapped line makes the inline renderer's own
// line arithmetic wrong for everything drawn afterwards.
func TestScrollbackRenderRespectsWidth(t *testing.T) {
	const at int64 = 1785228225
	long := strings.Repeat("unbreakable", 40) // no spaces: nothing to wrap on

	for _, width := range []int{40, 60, 100} {
		msg := finishedAssistant("m1", long, at)
		out := RenderForScrollback(msg, 0, []message.Message{msg}, nil, width)
		if strings.TrimSpace(out) == "" {
			t.Fatalf("width %d: rendered nothing", width)
		}
		for i, line := range strings.Split(out, "\n") {
			if w := lipgloss.Width(line); w > width {
				t.Errorf("width %d: line %d is %d columns wide; the terminal would wrap "+
					"it and every later line would be misplaced", width, i, w)
			}
		}
	}
}

// A non-positive width means the size is not known yet. Rendering then would
// produce garbage that cannot be recalled, so it must produce nothing at all.
func TestScrollbackRenderRefusesUnknownWidth(t *testing.T) {
	const at int64 = 1785228225
	msg := finishedAssistant("m1", "text", at)

	for _, width := range []int{0, -1} {
		if got := RenderForScrollback(msg, 0, []message.Message{msg}, nil, width); got != "" {
			t.Errorf("width %d rendered %q; printing at an unknown width writes "+
				"unrecallable garbage into the user's terminal", width, got)
		}
	}
}

// A trailing newline would double-space the whole transcript, because tea.Println
// supplies the line break itself.
func TestScrollbackRenderHasNoTrailingBlankLine(t *testing.T) {
	const at int64 = 1785228225
	msg := finishedAssistant("m1", "the answer", at)

	got := RenderForScrollback(msg, 0, []message.Message{msg}, nil, 80)
	if got == "" {
		t.Fatal("rendered nothing")
	}
	if strings.HasSuffix(got, "\n") {
		t.Error("render ends with a newline; tea.Println adds one, so every message " +
			"would be followed by a blank line")
	}
}
