package chat

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/opencode-ai/opencode/internal/message"
)

// printerFor builds a component in scrollback mode without an app, which is what
// lets these tests run: printPending only reaches app.Messages through
// RenderForScrollback, and a nil service renders everything that does not need it.
// userMsgID exists because the shared userMsg helper hardcodes the ID "m1". Two
// user messages built with it collide, and a colliding ID is silently skipped by
// the printed-once check — which quietly disabled the ordering assertion below
// until a mutation check exposed it.
func userMsgID(id, text string, at int64) message.Message {
	m := userMsg(text, at)
	m.ID = id
	return m
}

func printerFor(t *testing.T, width int, msgs ...message.Message) *messagesCmp {
	t.Helper()
	return &messagesCmp{
		width:         width,
		scrollback:    true,
		printed:       make(map[string]bool),
		cachedContent: make(map[string]cacheItem),
		messages:      msgs,
	}
}

// THE failure mode. Every pubsub update carries the whole message, so a printer
// that emits on each update would paste the reply again, longer every time — and
// printed output cannot be taken back. Six updates of a growing reply must
// produce exactly one print, when it finishes.
func TestStreamingRepliesArePrintedOnceWhenTheyFinish(t *testing.T) {
	const at int64 = 1785228225
	m := printerFor(t, 80)

	total := 0
	for i, partial := range []string{"The", "The ans", "The answer i", "The answer is fo", "The answer is forty"} {
		m.messages = []message.Message{streamingAssistant("m1", partial, at)}
		got := m.printPending()
		total += len(got)
		if len(got) != 0 {
			t.Errorf("update %d (%q) printed %d time(s); a reply that is still "+
				"arriving must not be printed, because every update carries the "+
				"whole message and printing cannot be undone", i, partial, len(got))
		}
	}

	m.messages = []message.Message{finishedAssistant("m1", "The answer is forty two", at)}
	final := m.printPending()
	if len(final) != 1 {
		t.Fatalf("the finished reply printed %d times, want exactly 1", len(final))
	}
	total += len(final)
	if total != 1 {
		t.Errorf("%d prints across the whole stream; want exactly 1", total)
	}

	// And it must not be printed again on any later update.
	if again := m.printPending(); len(again) != 0 {
		t.Errorf("printed %d more time(s) after finishing; the transcript would "+
			"contain the reply twice", len(again))
	}
}

// Printed output cannot be reordered. If a later message is emitted while an
// earlier one is still streaming, the transcript is permanently out of sequence —
// there is no re-render that could repair it, unlike a viewport.
func TestPrintingStopsAtTheFirstUnsettledMessage(t *testing.T) {
	const at int64 = 1785228225
	m := printerFor(t, 80,
		userMsgID("u1", "first question", at),
		streamingAssistant("a1", "still going", at+1),
		userMsgID("u2", "second question", at+2),
	)

	cmds := m.printPending()
	if len(cmds) != 1 {
		t.Fatalf("printed %d messages; want 1 — only the settled user message that "+
			"comes before the in-flight reply", len(cmds))
	}
	if !m.printed["u1"] {
		t.Error("the settled first message was not marked printed, so it would print again")
	}
	if m.printed["a1"] {
		t.Error("an unfinished reply was marked as printed, so it would never be printed at all")
	}
	// The one that matters: u2 is settled and would render fine, but it comes AFTER
	// a reply that has not finished. Emitting it now puts it permanently ahead of
	// that reply, and printed output cannot be reordered.
	if m.printed["u2"] {
		t.Error("a later message was printed while an earlier reply was still " +
			"streaming; the transcript order is now permanently wrong, and unlike a " +
			"viewport there is no re-render that could repair it")
	}
}

// Session switches must forget the record without trying to unprint. The old
// session's text stays in the terminal — that is the history the whole change
// exists to provide — but the new session's messages must all print.
func TestForgettingPrintedDoesNotUnprintAnything(t *testing.T) {
	const at int64 = 1785228225
	m := printerFor(t, 80, finishedAssistant("m1", "old session reply", at))

	if got := m.printPending(); len(got) != 1 {
		t.Fatalf("setup: printed %d, want 1", len(got))
	}
	m.forgetPrinted()
	if len(m.printed) != 0 {
		t.Errorf("forgetPrinted left %d entries behind", len(m.printed))
	}

	m.messages = []message.Message{finishedAssistant("m2", "new session reply", at)}
	if got := m.printPending(); len(got) != 1 {
		t.Errorf("after a session switch the new message printed %d times, want 1", len(got))
	}
}

// Nothing may be printed before the width is known: the text would be wrapped by
// the terminal at a width we did not choose, and it cannot be recalled.
func TestNothingIsPrintedBeforeTheWidthIsKnown(t *testing.T) {
	const at int64 = 1785228225
	m := printerFor(t, 0, finishedAssistant("m1", "the answer", at))

	if got := m.printPending(); len(got) != 0 {
		t.Errorf("printed %d message(s) at width 0; output would be wrapped "+
			"unpredictably and cannot be taken back", len(got))
	}
	if m.printed["m1"] {
		t.Error("marked printed at width 0, so the message would never be printed once " +
			"the real width arrived")
	}

	m.width = 80
	if got := m.printPending(); len(got) != 1 {
		t.Errorf("printed %d once the width was known, want 1", len(got))
	}
}

// With the alternate screen on, this component must behave exactly as before: the
// viewport owns the transcript and nothing is printed.
func TestNothingIsPrintedWhenTheAlternateScreenIsUsed(t *testing.T) {
	const at int64 = 1785228225
	m := printerFor(t, 80, finishedAssistant("m1", "the answer", at))
	m.scrollback = false

	if got := m.printPending(); len(got) != 0 {
		t.Errorf("printed %d message(s) while drawing on the alternate screen, where "+
			"tea.Println is discarded anyway", len(got))
	}
}

// THE assertion the fixed-height contract actually needs.
//
// TestLivePreviewIsCappedSoTheFooterStaysShort only checks an upper bound, and a
// footer that SHRINKS when streaming ends satisfies an upper bound perfectly — so
// that test passes against the very bug FooterReservedRows exists to kill. What
// breaks the scrollback is not a tall footer, it is a footer whose height CHANGES
// between renders: bubbletea walks the cursor up by the last frame's row count, so
// a frame that was taller and is now shorter erases rows of already-printed
// conversation above it. That is the "replies vanish" symptom.
//
// So: assert the height is IDENTICAL across every state the footer passes through
// in a normal turn, not merely bounded.
func TestFooterHeightIsConstantAcrossEveryStreamingState(t *testing.T) {
	const at int64 = 1785228225

	states := []struct {
		name string
		msgs []message.Message
	}{
		{"idle, no messages at all", nil},
		{"idle, reply already finished and printed", []message.Message{finishedAssistant("m1", "the answer", at)}},
		{"streaming a one-line reply", []message.Message{streamingAssistant("m1", "The", at)}},
		{"streaming a reply far taller than the preview", []message.Message{streamingAssistant("m1", strings.Repeat("a line of streamed reply\n", 60), at)}},
	}

	for _, s := range states {
		m := printerFor(t, 80, s.msgs...)
		if rows := lipgloss.Height(m.FooterView()); rows != FooterReservedRows {
			t.Errorf("%s: footer is %d rows, want exactly FooterReservedRows=%d.\n"+
				"A footer that changes height between renders makes bubbletea's "+
				"cursor-up erase land in the printed scrollback and wipe it.",
				s.name, rows, FooterReservedRows)
		}
	}
}

// reasoningMsg builds an in-flight assistant message carrying only reasoning.
func reasoningMsg(id, thinking string) message.Message {
	return message.Message{
		ID: id, Role: message.Assistant, CreatedAt: 1785228225,
		Parts: []message.ContentPart{message.ReasoningContent{Thinking: thinking}},
	}
}

// Reasoning must reach the terminal WHILE the model is thinking, not after.
//
// This is the whole point of removing the preview pane: a model that thinks for a
// minute used to give a six-row window of text that scrolled past unreadably and
// was never kept. Each settled line must now be printed, once, permanently.
func TestReasoningIsPrintedLineByLineAsItArrives(t *testing.T) {
	m := printerFor(t, 80, reasoningMsg("m1", "first thought\nsecond thought\n"))

	if n := len(m.printPending()); n != 3 {
		t.Fatalf("emitted %d prints for the marker plus two settled lines, want 3", n)
	}
	if got := m.reasonedLines["m1"]; got != 2 {
		t.Errorf("watermark is %d, want 2", got)
	}

	// Nothing new has arrived: printing again would duplicate the block.
	if n := len(m.printPending()); n != 0 {
		t.Errorf("re-emitted %d prints with no new reasoning; every pubsub update "+
			"carries the whole message, so this would reprint the block on every token", n)
	}

	// One more complete line, plus a partial that must NOT be printed yet.
	m.messages = []message.Message{reasoningMsg("m1", "first thought\nsecond thought\nthird thought\npartial")}
	cmds := m.printPending()
	if len(cmds) != 1 {
		t.Errorf("emitted %d prints for one newly-settled line, want 1 (the partial "+
			"line is still being written and cannot be taken back once printed)", len(cmds))
	}
}

// The trailing partial line is final once the message settles, and the block must
// be closed so the answer that follows is distinguishable from the working-out.
func TestReasoningIsFlushedAndClosedWhenTheMessageSettles(t *testing.T) {
	m := printerFor(t, 80, reasoningMsg("m1", "a thought\nand a trailing partial"))
	m.printPending() // streams "a thought" only

	settled := reasoningMsg("m1", "a thought\nand a trailing partial")
	settled.Parts = append(settled.Parts,
		message.TextContent{Text: "the answer"},
		message.Finish{Reason: message.FinishReasonEndTurn})
	m.messages = []message.Message{settled}

	if !ScrollbackReady(settled) {
		t.Fatal("the message did not settle, so this test proves nothing")
	}
	if n := len(m.printPending()); n < 3 {
		t.Errorf("emitted %d prints on settling; want at least 3 (the trailing partial "+
			"line, the closing marker, and the message itself)", n)
	}
	if _, still := m.reasonedLines["m1"]; still {
		t.Error("the watermark outlived the message it tracked; it would leak per turn")
	}
}

// Providers that deliver all reasoning at once on finish, having streamed none,
// must still get the full block — and exactly once.
func TestReasoningDeliveredOnlyAtFinishIsStillPrintedOnce(t *testing.T) {
	msg := reasoningMsg("m1", "thought one\nthought two\nthought three")
	msg.Parts = append(msg.Parts,
		message.TextContent{Text: "the answer"},
		message.Finish{Reason: message.FinishReasonEndTurn})
	m := printerFor(t, 80, msg)

	if n := len(m.printPending()); n < 5 {
		t.Errorf("emitted %d prints, want at least 5 (marker, three lines, close, message); "+
			"a provider that streams no reasoning must not lose it", n)
	}

	// And the finished render must NOT carry the quote as well, or the whole block
	// appears twice — once streamed, once quoted.
	rendered := RenderForScrollback(msg, 0, []message.Message{msg}, nil, 80)
	if strings.Contains(rendered, "thought two") {
		t.Error("the finished render still contains the reasoning, which was already " +
			"printed line by line — the block would appear twice in the scrollback")
	}
}
