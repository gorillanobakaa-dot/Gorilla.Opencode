package agent

import (
	"testing"
	"time"

	"github.com/opencode-ai/opencode/internal/llm/provider"
)

// fakeNow is a clock the test moves by hand.
type fakeNow struct{ t time.Time }

func (f *fakeNow) now() time.Time          { return f.t }
func (f *fakeNow) advance(d time.Duration) { f.t = f.t.Add(d) }

// The 2026-10-24 Gorilla session, first call, from the LM Studio log: 257 s
// reading 12,592 prompt tokens, then 310 tokens in 78.7 s.
func TestTheClockSplitsReadingFromWriting(t *testing.T) {
	f := &fakeNow{t: time.Unix(1_700_000_000, 0)}
	c := newStreamClockAt(f.now)
	f.advance(257 * time.Second)
	c.see(provider.ProviderEvent{Type: provider.EventThinkingDelta, Content: "The"})
	f.advance(78700 * time.Millisecond)
	c.see(provider.ProviderEvent{Type: provider.EventContentDelta, Content: "late"})

	s := c.speed(provider.TokenUsage{InputTokens: 12592, OutputTokens: 310, ReasoningTokens: 250})
	if s.FirstMS != 257_000 {
		t.Errorf("first word after %d ms, want 257000: reasoning is the first token", s.FirstMS)
	}
	if s.StreamMS != 78_700 {
		t.Errorf("stream %d ms, want 78700", s.StreamMS)
	}
	if r := s.TokensPerSecond(); r < 3.9 || r > 3.95 {
		t.Errorf("%.2f tok/s, want ~3.93 as LM Studio measured", r)
	}
	if want := "first word 4m17s, 3.9 tok/s, 310 tokens (250 thinking)"; s.Summary() != want {
		t.Errorf("summary %q, want %q", s.Summary(), want)
	}
}

// Error and warning events are not tokens: they must not stop the reading clock.
func TestOnlyTokensStopTheReadingClock(t *testing.T) {
	f := &fakeNow{t: time.Unix(1_700_000_000, 0)}
	c := newStreamClockAt(f.now)
	f.advance(5 * time.Second)
	c.see(provider.ProviderEvent{Type: provider.EventWarning})
	f.advance(5 * time.Second)
	c.see(provider.ProviderEvent{Type: provider.EventToolUseStart})
	if s := c.speed(provider.TokenUsage{}); s.FirstMS != 10_000 {
		t.Errorf("first token after %d ms, want 10000", s.FirstMS)
	}
}

// A call that streamed nothing spent all its time waiting; no rate is invented.
func TestACallWithNoTokensHasNoRate(t *testing.T) {
	f := &fakeNow{t: time.Unix(1_700_000_000, 0)}
	c := newStreamClockAt(f.now)
	f.advance(3 * time.Second)
	s := c.speed(provider.TokenUsage{})
	if s.FirstMS != 3000 || s.StreamMS != 0 || s.TokensPerSecond() != 0 {
		t.Errorf("speed %+v: want 3000 ms waiting, no stream, no rate", s)
	}
	if want := "first word 3.0s"; s.Summary() != want {
		t.Errorf("summary %q, want %q", s.Summary(), want)
	}
}
