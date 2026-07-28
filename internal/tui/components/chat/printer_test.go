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

// The footer is the only thing drawn in place, and bubbletea erases its previous
// frame by counting logical lines — so a footer taller than the window makes every
// later erase land in the wrong place. That is the bug the startup picker showed.
func TestLivePreviewIsCappedSoTheFooterStaysShort(t *testing.T) {
	const at int64 = 1785228225
	many := strings.Repeat("a line of streamed reply\n", 60)

	m := printerFor(t, 80, streamingAssistant("m1", many, at))
	footer := m.FooterView()
	if strings.TrimSpace(footer) == "" {
		t.Fatal("no preview at all while a reply streams; a long answer could not be watched")
	}

	rows := lipgloss.Height(footer)
	if rows > livePreviewRows+1 {
		t.Errorf("footer is %d rows for a %d-line reply; the cap is %d (+1 for the "+
			"working line). An unbounded footer breaks the renderer's line arithmetic",
			rows, strings.Count(many, "\n"), livePreviewRows)
	}
}

// A settled message must never appear in the footer: it has already been printed,
// so showing it again would duplicate it on screen.
func TestFinishedMessagesAreNotAlsoShownInTheFooter(t *testing.T) {
	const at int64 = 1785228225
	m := printerFor(t, 80, finishedAssistant("m1", "the whole answer", at))

	if preview := m.livePreview(); preview != "" {
		t.Errorf("a finished reply is previewed in the footer as well as printed, so "+
			"it would appear twice: %q", preview)
	}
}
