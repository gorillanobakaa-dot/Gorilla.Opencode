package message

import (
	"fmt"
	"strings"
)

// Speed is one model call measured from the stream.
//
// GORILLA (2026-10-24): a local model was "much slower in Gorilla than in LM
// Studio" and nothing on screen could say why. The LM Studio server log, read
// by hand, did: 4.3 minutes reading a 12,592-token prompt before the first
// word, then ~3.9 tokens/s, most of them reasoning. These are those numbers,
// taken from timestamps the app already sees; nothing is sent anywhere.
type Speed struct {
	// FirstMS is from sending the request to the first streamed token of any
	// kind (reasoning, text or tool call): the time the server spent reading
	// the prompt, plus the network.
	FirstMS int64 `json:"first_ms"`
	// StreamMS is from that first token to the end of the stream.
	StreamMS        int64 `json:"stream_ms"`
	PromptTokens    int64 `json:"prompt_tokens,omitempty"`
	OutputTokens    int64 `json:"output_tokens,omitempty"`
	ReasoningTokens int64 `json:"reasoning_tokens,omitempty"`
}

// TokensPerSecond is the generation rate, 0 when it cannot be known (no token
// count from the server, or a stream too short to time).
func (s Speed) TokensPerSecond() float64 {
	if s.OutputTokens <= 1 || s.StreamMS <= 0 {
		return 0
	}
	// The first token arrived at the end of FirstMS, so the stream carried one
	// fewer than the count.
	return float64(s.OutputTokens-1) / (float64(s.StreamMS) / 1000)
}

// Summary is the one line shown under an answer, in the status bar and in the
// export: "first word 4m17s, 3.9 tok/s, 994 tokens (812 thinking)". Plain
// commas: a middle dot is ambiguous-width and can wrap the TUI frame (see
// tui/styles/ascii_test.go).
func (s Speed) Summary() string {
	parts := []string{"first word " + shortDuration(s.FirstMS)}
	if r := s.TokensPerSecond(); r > 0 {
		parts = append(parts, fmt.Sprintf("%.1f tok/s", r))
	}
	if s.OutputTokens > 0 {
		out := fmt.Sprintf("%d tokens", s.OutputTokens)
		if s.ReasoningTokens > 0 {
			out += fmt.Sprintf(" (%d thinking)", s.ReasoningTokens)
		}
		parts = append(parts, out)
	}
	return strings.Join(parts, ", ")
}

// shortDuration: 850ms, 6.8s, 4m17s.
func shortDuration(ms int64) string {
	switch {
	case ms < 1000:
		return fmt.Sprintf("%dms", ms)
	case ms < 60_000:
		return fmt.Sprintf("%.1fs", float64(ms)/1000)
	default:
		sec := ms / 1000
		return fmt.Sprintf("%dm%02ds", sec/60, sec%60)
	}
}

// SetSpeed records how the answer arrived on its finish part. No-op when the
// message is not finished.
func (m *Message) SetSpeed(s Speed) {
	for i, part := range m.Parts {
		if f, ok := part.(Finish); ok {
			f.Speed = &s
			m.Parts[i] = f
			return
		}
	}
}

// Speed is the measured speed of this answer, nil when none was recorded.
func (m *Message) Speed() *Speed {
	if f := m.FinishPart(); f != nil {
		return f.Speed
	}
	return nil
}
