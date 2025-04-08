package provider

import (
	"context"

	"github.com/kujtimiihoxha/termai/internal/llm/tools"
	"github.com/kujtimiihoxha/termai/internal/message"
)

// EventType represents the type of streaming event
type EventType string

const (
	EventContentStart  EventType = "content_start"
	EventContentDelta  EventType = "content_delta"
	EventThinkingDelta EventType = "thinking_delta"
	EventContentStop   EventType = "content_stop"
	EventComplete      EventType = "complete"
	EventError         EventType = "error"
	EventWarning       EventType = "warning"
	EventInfo          EventType = "info"
)

type TokenUsage struct {
	InputTokens         int64
	OutputTokens        int64
	CacheCreationTokens int64
	CacheReadTokens     int64
}

type ProviderResponse struct {
	Content      string
	ToolCalls    []message.ToolCall
	Usage        TokenUsage
	FinishReason string
}

type ProviderEvent struct {
	Type     EventType
	Content  string
	Thinking string
	ToolCall *message.ToolCall
	Error    error
	Response *ProviderResponse

	// Used for giving users info on e.x retry
	Info string
}

type Provider interface {
	SendMessages(ctx context.Context, messages []message.Message, tools []tools.BaseTool) (*ProviderResponse, error)

	StreamResponse(ctx context.Context, messages []message.Message, tools []tools.BaseTool) (<-chan ProviderEvent, error)
}
