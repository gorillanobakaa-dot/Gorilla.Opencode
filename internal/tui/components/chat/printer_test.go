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

// The live preview must cost the SAME whether the reply is short or enormous.
//
// It runs on every frame while a reply streams, so any cost that scales with the
// answer's length means the interface slows down as the answer grows — reported as
// "it gets sluggish". The original version called the full Markdown renderer over
// the whole growing reply to display six lines of it: measured at 0.96ms/348KB per
// frame at 50 words and 21ms/3.4MB at 3200 words.
//
// This asserts the shape of the cost, not a wall-clock figure, because a timing
// threshold would be flaky on a loaded machine while the SHAPE is the actual
// property that matters.
func TestLivePreviewCostDoesNotGrowWithTheReply(t *testing.T) {
	preview := func(words int) (string, int) {
		body := strings.Repeat("the quick brown fox jumps over the lazy dog ", words/9+1)
		m := printerFor(t, 100, streamingAssistant("m1", body, 1785228225))
		out := m.livePreview()
		return out, len(out)
	}

	// Compare two replies that BOTH exceed the row cap. Comparing a short reply
	// against a long one measures nothing: a 50-word reply does not fill six rows,
	// so its preview is legitimately smaller. Once the cap is reached, more input
	// must produce no more output — that is the property.
	long, longLen := preview(4_000)
	huge, hugeLen := preview(40_000)

	if strings.TrimSpace(long) == "" || strings.TrimSpace(huge) == "" {
		t.Fatal("no preview rendered, so the comparison below is vacuous")
	}
	if hugeLen != longLen {
		t.Errorf("a 40,000-word reply previews as %d bytes but a 4,000-word one as %d; "+
			"both exceed the %d-row cap, so the output must be identical in size. A "+
			"difference means the whole reply is being processed, which is what made "+
			"the interface slow down as answers grew", hugeLen, longLen, livePreviewRows)
	}

	// And a short reply must still render, or the cheap path has broken the feature.
	if short, _ := preview(50); strings.TrimSpace(short) == "" {
		t.Error("a short reply previews as nothing")
	}

	// Output size alone cannot see the defect: the preview is capped to six rows
	// either way, so an implementation that processes the entire reply and then
	// throws almost all of it away produces identical output at enormous cost. That
	// is exactly what the original did. Allocation count is what distinguishes them
	// — measured at 88 per call when the input is bounded, against 15,281 and 52,368
	// for 800- and 3200-word replies when it is not.
	allocs := func(words int) float64 {
		body := strings.Repeat("the quick brown fox jumps over the lazy dog ", words/9+1)
		m := printerFor(t, 100, streamingAssistant("m1", body, 1785228225))
		return testing.AllocsPerRun(20, func() { _ = m.livePreview() })
	}
	small, big := allocs(4_000), allocs(40_000)
	if big > small*2 {
		t.Errorf("previewing a 40,000-word reply allocates %.0f objects against %.0f for "+
			"a 4,000-word one. The cost is scaling with the reply, so the interface "+
			"slows down as answers grow — bound the input BEFORE wrapping it", big, small)
	}

	for _, out := range []string{long, huge} {
		if got := lipgloss.Height(out); got > livePreviewRows {
			t.Errorf("preview is %d rows, cap is %d", got, livePreviewRows)
		}
	}
}

// A model that thinks before answering must not look like a model that has hung.
func TestLivePreviewFallsBackToReasoningBeforeAnyAnswer(t *testing.T) {
	msg := message.Message{
		ID: "m1", Role: message.Assistant, CreatedAt: 1785228225,
		Parts: []message.ContentPart{message.ReasoningContent{Thinking: "weighing the options"}},
	}
	m := printerFor(t, 100, msg)

	if out := m.livePreview(); !strings.Contains(out, "weighing") {
		t.Errorf("with reasoning but no answer yet the preview is %q; a thinking model "+
			"would be indistinguishable from a stalled one", out)
	}
}
