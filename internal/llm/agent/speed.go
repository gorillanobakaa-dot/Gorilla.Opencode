package agent

// GORILLA (2026-10-24): the speed of every model call, measured from the
// stream and kept on the answer. See message/speed.go for why.

import (
	"context"
	"sync"
	"time"

	"github.com/opencode-ai/opencode/internal/llm/provider"
	"github.com/opencode-ai/opencode/internal/message"
)

// streamClock times one call: started when the request is made, first when
// the first token of any kind arrives.
type streamClock struct {
	start, first time.Time
	now          func() time.Time
}

func newStreamClock() *streamClock { return newStreamClockAt(time.Now) }

func newStreamClockAt(now func() time.Time) *streamClock {
	return &streamClock{start: now(), now: now}
}

// see notes the first token. Reasoning counts: the server has finished reading
// the prompt the moment it emits anything at all.
func (c *streamClock) see(ev provider.ProviderEvent) {
	if !c.first.IsZero() {
		return
	}
	switch ev.Type {
	case provider.EventThinkingDelta, provider.EventContentDelta, provider.EventToolUseStart:
		c.first = c.now()
	}
}

// speed closes the measurement. A call that streamed nothing (a tool call
// delivered whole at the end, a non-streaming backend) has its first token at
// the end: all of its time was waiting.
func (c *streamClock) speed(u provider.TokenUsage) message.Speed {
	end := c.now()
	first := c.first
	if first.IsZero() {
		first = end
	}
	return message.Speed{
		FirstMS:         first.Sub(c.start).Milliseconds(),
		StreamMS:        end.Sub(first).Milliseconds(),
		PromptTokens:    u.InputTokens + u.CacheReadTokens + u.CacheCreationTokens,
		OutputTokens:    u.OutputTokens,
		ReasoningTokens: u.ReasoningTokens,
	}
}

// speedUsage turns a recorded speed back into the usage it was built from, for
// MeasuredNextRequest. PromptTokens is the whole prompt, so it goes in Input.
func speedUsage(s message.Speed) provider.TokenUsage {
	return provider.TokenUsage{
		InputTokens:     s.PromptTokens,
		OutputTokens:    s.OutputTokens,
		ReasoningTokens: s.ReasoningTokens,
	}
}

var (
	lastSpeedMu sync.RWMutex
	lastSpeed   = map[string]message.Speed{}
)

// LastSpeed is the speed of the newest call in a session, for the status bar.
func LastSpeed(sessionID string) (message.Speed, bool) {
	lastSpeedMu.RLock()
	defer lastSpeedMu.RUnlock()
	s, ok := lastSpeed[sessionID]
	return s, ok
}

// noteLastSpeed keeps the newest call's speed for the status bar.
func noteLastSpeed(sessionID string, s message.Speed) {
	lastSpeedMu.Lock()
	lastSpeed[sessionID] = s
	lastSpeedMu.Unlock()
}

type speedKey struct{}

// withSpeed carries one call's speed to the place that finishes its message.
func withSpeed(ctx context.Context, s message.Speed) context.Context {
	return context.WithValue(ctx, speedKey{}, s)
}

// withCallSpeed puts the carried speed on msg's finish part. Called after
// AddFinish (which replaces the part) and before the Update that publishes it.
func withCallSpeed(ctx context.Context, msg *message.Message) {
	if s, ok := ctx.Value(speedKey{}).(message.Speed); ok {
		msg.SetSpeed(s)
	}
}
