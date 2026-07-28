package chat

import (
	"strings"
	"testing"
	"time"

	"github.com/opencode-ai/opencode/internal/config"
	"github.com/opencode-ai/opencode/internal/message"
)

// The duration shown after a turn was wrong by a factor of 1000.
//
// Both inputs are UNIX SECONDS — messages.created_at is written by a SQLite
// trigger using strftime('%s'), and the Finish part's Time is seconds too — but
// the code divided by 1000 as though they were milliseconds. A 45-second turn
// therefore displayed as "45ms", which made every model look instantaneous and
// made the number useless for noticing a slow one.
//
// The cause was a comment: three places in the initial migration described these
// columns as milliseconds. They never were.
func TestTurnDurationIsReadAsSecondsNotMilliseconds(t *testing.T) {
	const start int64 = 1785228225

	cases := map[int64]string{
		start:       "<1s",
		start + 1:   "1s",
		start + 45:  "45s",
		start + 90:  "1.5m",
		start + 600: "10.0m",
	}
	for end, want := range cases {
		if got := formatTimestampDiff(start, end); got != want {
			t.Errorf("formatTimestampDiff(+%ds) = %q, want %q — a 45-second turn must not read as 45ms", end-start, got, want)
		}
	}
}

// A clock that runs backwards (clock adjustment, or a Finish time never set and
// left at zero) must not render a negative duration.
func TestTurnDurationNeverGoesNegative(t *testing.T) {
	const start int64 = 1785228225
	if got := formatTimestampDiff(start, start-500); got != "<1s" {
		t.Errorf("a backwards interval rendered as %q", got)
	}
	// Finish.Time is genuinely 0 in real stored data when a turn never finished.
	if got := formatTimestampDiff(start, 0); got != "<1s" {
		t.Errorf("an unset finish time rendered as %q", got)
	}
}

func TestMessageTimeFormat(t *testing.T) {
	const ts int64 = 1785228225
	want := time.Unix(ts, 0).Format("15:04:05")
	if got := messageTime(ts); got != want {
		t.Errorf("messageTime = %q, want %q", got, want)
	}
	// An unset timestamp must render nothing rather than 1970.
	if got := messageTime(0); got != "" {
		t.Errorf("messageTime(0) = %q, want empty — a zero stamp would print 01:00:00 from the epoch", got)
	}
}

// Reasoning is shown as a quote block, matching the shape /export uses, so it
// reads as an aside rather than as the answer.
func TestReasoningQuoteIsAMarkdownBlockquote(t *testing.T) {
	got := reasoningQuote("first line\nsecond line")

	if !strings.Contains(got, "**thinking**") {
		t.Error("the block is not labelled, so it reads as part of the answer")
	}
	for _, l := range strings.Split(got, "\n") {
		if l != "" && !strings.HasPrefix(l, ">") {
			t.Errorf("line %q is not quoted, so it renders as ordinary answer text", l)
		}
	}
	if !strings.Contains(got, "> first line") || !strings.Contains(got, "> second line") {
		t.Errorf("a multi-line thought was not fully quoted:\n%s", got)
	}
	// Nothing to say means nothing rendered — not an empty labelled box.
	if reasoningQuote("   \n  ") != "" {
		t.Error("whitespace-only reasoning produced a block")
	}
}

// withExtra flips one extra for a single test and restores it.
func withExtra(t *testing.T, id string, on bool) {
	t.Helper()
	if _, err := config.Load(t.TempDir(), false); err != nil {
		t.Fatalf("config.Load: %v", err)
	}
	before := config.ExtraEnabled(id)
	if err := config.SetExtra(id, on); err != nil {
		t.Fatalf("SetExtra(%s): %v", id, err)
	}
	t.Cleanup(func() { config.SetExtra(id, before) })
}

func userMsg(text string, at int64) message.Message {
	return message.Message{
		ID:        "m1",
		Role:      message.User,
		CreatedAt: at,
		Parts:     []message.ContentPart{message.TextContent{Text: text}},
	}
}

// The timestamp toggle must actually change what is rendered. Before this, the
// switch existed in config and nothing read it.
func TestTimestampToggleChangesWhatIsRendered(t *testing.T) {
	const at int64 = 1785228225
	stamp := time.Unix(at, 0).Format("15:04:05")

	withExtra(t, "extras-timestamps-show", true)
	on := renderUserMessage(userMsg("hello", at), false, 60, 0).content
	if !strings.Contains(on, stamp) {
		t.Errorf("the time %q is absent with timestamps ON:\n%s", stamp, on)
	}

	withExtra(t, "extras-timestamps-show", false)
	off := renderUserMessage(userMsg("hello", at), false, 60, 0).content
	if strings.Contains(off, stamp) {
		t.Errorf("the time is still shown with timestamps OFF:\n%s", off)
	}
	// The message itself must survive either way.
	if !strings.Contains(off, "hello") {
		t.Errorf("the message text was lost when timestamps were disabled:\n%s", off)
	}
}
