package provider

import (
	"context"
	"fmt"

	"github.com/opencode-ai/opencode/internal/config"
	"github.com/opencode-ai/opencode/internal/llm/models"
	"github.com/opencode-ai/opencode/internal/llm/tools"
	"github.com/opencode-ai/opencode/internal/message"
)

type EventType string

// retryCeiling is how many attempts a turn gets before it gives up and says so.
//
// GORILLA OVERRIDE (2026-08-20): this was `const maxRetries = 5`, a single number
// for every connection. It now comes from the active connection profile, so a
// 1-9 KB/s satellite link retries twice and a fibre line retries five times.
//
// It is a FUNCTION, not a variable, deliberately: the profile can change
// mid-session via /connection, and a package-level variable captured at init
// would silently keep serving the old value — the same class of stale-state bug
// as an installed binary lagging the repo build.
//
// Retrying is not free on the links this program is for. Every attempt
// re-uploads the whole conversation, so on a metered plan a retry ceiling is a
// spending limit. The upload budget in uploadbudget.go is the other half of
// that and counts bytes; this counts attempts.
func retryCeiling() int {
	if n := config.CurrentConnProfile().MaxRetries; n > 0 {
		return n
	}
	return 5
}

const (
	EventContentStart  EventType = "content_start"
	EventToolUseStart  EventType = "tool_use_start"
	EventToolUseDelta  EventType = "tool_use_delta"
	EventToolUseStop   EventType = "tool_use_stop"
	EventContentDelta  EventType = "content_delta"
	EventThinkingDelta EventType = "thinking_delta"
	EventContentStop   EventType = "content_stop"
	EventComplete      EventType = "complete"
	EventError         EventType = "error"
	EventWarning       EventType = "warning"
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
	FinishReason message.FinishReason
}

type ProviderEvent struct {
	Type EventType

	Content  string
	Thinking string
	Response *ProviderResponse
	ToolCall *message.ToolCall
	Error    error
}
type Provider interface {
	SendMessages(ctx context.Context, messages []message.Message, tools []tools.BaseTool) (*ProviderResponse, error)

	StreamResponse(ctx context.Context, messages []message.Message, tools []tools.BaseTool) <-chan ProviderEvent

	Model() models.Model
}

type providerClientOptions struct {
	apiKey        string
	model         models.Model
	maxTokens     int64
	systemMessage string

	anthropicOptions []AnthropicOption
	openaiOptions    []OpenAIOption
	geminiOptions    []GeminiOption
}

type ProviderClientOption func(*providerClientOptions)

type ProviderClient interface {
	send(ctx context.Context, messages []message.Message, tools []tools.BaseTool) (*ProviderResponse, error)
	stream(ctx context.Context, messages []message.Message, tools []tools.BaseTool) <-chan ProviderEvent
}

type baseProvider[C ProviderClient] struct {
	options providerClientOptions
	client  C
}

func NewProvider(providerName models.ModelProvider, opts ...ProviderClientOption) (Provider, error) {
	clientOptions := providerClientOptions{}
	for _, o := range opts {
		o(&clientOptions)
	}
	switch providerName {
	case models.ProviderAnthropic:
		return &baseProvider[AnthropicClient]{
			options: clientOptions,
			client:  newAnthropicClient(clientOptions),
		}, nil
	case models.ProviderOpenAI:
		return &baseProvider[OpenAIClient]{
			options: clientOptions,
			client:  newOpenAIClient(clientOptions),
		}, nil
	case models.ProviderGemini:
		return &baseProvider[GeminiClient]{
			options: clientOptions,
			client:  newGeminiClient(clientOptions),
		}, nil
	case models.ProviderGROQ:
		clientOptions.openaiOptions = append(clientOptions.openaiOptions,
			WithOpenAIBaseURL("https://api.groq.com/openai/v1"),
		)
		return &baseProvider[OpenAIClient]{
			options: clientOptions,
			client:  newOpenAIClient(clientOptions),
		}, nil
	// GORILLA OVERRIDE: native Cerebras (OpenAI-compatible, wafer-scale).
	case models.ProviderCerebras:
		clientOptions.openaiOptions = append(clientOptions.openaiOptions,
			WithOpenAIBaseURL("https://api.cerebras.ai/v1"),
		)
		return &baseProvider[OpenAIClient]{
			options: clientOptions,
			client:  newOpenAIClient(clientOptions),
		}, nil
	// GORILLA OVERRIDE: Gemini via "Login with Google" (Code Assist free
	// tier). Not OpenAI-compatible; its own hand-built envelope client.
	case models.ProviderGeminiCA:
		return &baseProvider[CodeAssistClient]{
			options: clientOptions,
			client:  newCodeAssistClient(clientOptions),
		}, nil
	// GORILLA OVERRIDE: Antigravity free tier (Claude/GPT-OSS/Gemini). See
	// internal/llm/provider/antigravity.go.
	case models.ProviderAntigravity:
		return &baseProvider[AntigravityClient]{
			options: clientOptions,
			client:  newAntigravityClient(clientOptions),
		}, nil
	// GORILLA OVERRIDE: OpenAI models via a personal ChatGPT sign-in — free
	// plan included, no API key. Speaks the Responses API, not Chat
	// Completions, so it cannot reuse OpenAIClient with a base URL the way
	// GROQ/xAI/DeepSeek do. See internal/llm/provider/chatgpt.go.
	case models.ProviderChatGPT:
		return &baseProvider[ChatGPTClient]{
			options: clientOptions,
			client:  newChatGPTClient(clientOptions),
		}, nil
	case models.ProviderOpenRouter:
		clientOptions.openaiOptions = append(clientOptions.openaiOptions,
			WithOpenAIBaseURL("https://openrouter.ai/api/v1"),
			WithOpenAIExtraHeaders(map[string]string{
				"HTTP-Referer": "opencode.ai",
				"X-Title":      "OpenCode",
			}),
		)
		return &baseProvider[OpenAIClient]{
			options: clientOptions,
			client:  newOpenAIClient(clientOptions),
		}, nil
	case models.ProviderXAI:
		clientOptions.openaiOptions = append(clientOptions.openaiOptions,
			WithOpenAIBaseURL("https://api.x.ai/v1"),
		)
		return &baseProvider[OpenAIClient]{
			options: clientOptions,
			client:  newOpenAIClient(clientOptions),
		}, nil
	// GORILLA OVERRIDE: DeepSeek (OpenAI-compatible at api.deepseek.com).
	case models.ProviderDeepSeek:
		clientOptions.openaiOptions = append(clientOptions.openaiOptions,
			WithOpenAIBaseURL("https://api.deepseek.com/v1"),
		)
		return &baseProvider[OpenAIClient]{
			options: clientOptions,
			client:  newOpenAIClient(clientOptions),
		}, nil
	case models.ProviderLocal:
		// GORILLA OVERRIDE: route each local model to the endpoint it was
		// discovered from (NIM, Ollama, ...), not a single shared env var.
		baseURL, _, _ := models.LocalRouteFor(clientOptions.model.ID)
		clientOptions.openaiOptions = append(clientOptions.openaiOptions,
			WithOpenAIBaseURL(baseURL),
		)
		return &baseProvider[OpenAIClient]{
			options: clientOptions,
			client:  newOpenAIClient(clientOptions),
		}, nil
	case models.ProviderMock:
		// TODO: implement mock client for test
		panic("not implemented")
	}
	return nil, fmt.Errorf("provider not supported: %s", providerName)
}

func (p *baseProvider[C]) cleanMessages(messages []message.Message) (cleaned []message.Message) {
	// Collapse the content of read-only tool results that a later identical call
	// has superseded, before dropping empties. Shapes the wire only; the session
	// store is untouched. See supersede.go.
	messages = supersedeStaleReads(messages)
	// Then drop results that have simply gone idle: locally reproducible reads
	// that are many turns old and are not the newest read of their target. See
	// evict_age.go. Order matters only for tidiness, not correctness:
	// supersession leaves a short stub, which falls under the size floor here,
	// so a message cannot be stubbed twice.
	messages = evictAgedReads(messages)
	for _, msg := range messages {
		// The message has no content
		if len(msg.Parts) == 0 {
			continue
		}
		cleaned = append(cleaned, msg)
	}
	return
}

func (p *baseProvider[C]) SendMessages(ctx context.Context, messages []message.Message, tools []tools.BaseTool) (*ProviderResponse, error) {
	// GORILLA OVERRIDE: proactive pace-setter (see ratelimit.go). Space
	// requests under the user's configured RPM cap before sending.
	if err := paceRequest(ctx); err != nil {
		return nil, err
	}
	messages = p.cleanMessages(messages)
	return p.client.send(ctx, messages, tools)
}

func (p *baseProvider[C]) Model() models.Model {
	return p.options.model
}

func (p *baseProvider[C]) StreamResponse(ctx context.Context, messages []message.Message, tools []tools.BaseTool) <-chan ProviderEvent {
	// GORILLA OVERRIDE: proactive pace-setter (see ratelimit.go). If the wait
	// is cancelled, surface it as an error event so the caller's channel-range
	// loop terminates cleanly instead of hitting the provider.
	if err := paceRequest(ctx); err != nil {
		ch := make(chan ProviderEvent, 1)
		ch <- ProviderEvent{Type: EventError, Error: err}
		close(ch)
		return ch
	}
	messages = p.cleanMessages(messages)

	// GORILLA OVERRIDE (2026-08-20): on the slow connection profiles, fetch the
	// reply in ONE piece and hand it to the caller as a single event, instead of
	// streaming it token by token.
	//
	// WHY. A streamed reply wraps every token in its own JSON envelope. Measured
	// on the same question, same model, same 60-token answer: 22,256 bytes
	// streamed against 834 not streamed - 27x. TOKENS ARE IDENTICAL (106 either
	// way), so this costs the provider nothing and saves the USER's metered
	// allowance, which on a satellite plan is real money.
	//
	// The adapter lives here rather than in each client so every provider gets
	// it from one place, and callers keep consuming a channel exactly as before
	// - the TUI never learns which mode it is in.
	if !config.StreamRepliesEnabled() {
		return p.sendAsSingleEvent(ctx, messages, tools)
	}
	return p.client.stream(ctx, messages, tools)
}

// sendAsSingleEvent runs the non-streaming path and reports it on a channel, so
// non-streaming is invisible to every caller.
func (p *baseProvider[C]) sendAsSingleEvent(ctx context.Context, messages []message.Message, tools []tools.BaseTool) <-chan ProviderEvent {
	ch := make(chan ProviderEvent, 2)
	go func() {
		defer close(ch)
		resp, err := p.client.send(ctx, messages, tools)
		if err != nil {
			ch <- ProviderEvent{Type: EventError, Error: err}
			return
		}
		if resp == nil {
			ch <- ProviderEvent{Type: EventError, Error: fmt.Errorf("provider returned no response")}
			return
		}
		// One content event so anything rendering incremental text still sees
		// the text arrive, then the completion event carrying tool calls and
		// usage. Emitting Complete alone would leave content-only consumers
		// blank.
		if resp.Content != "" {
			ch <- ProviderEvent{Type: EventContentDelta, Content: resp.Content}
		}
		ch <- ProviderEvent{Type: EventComplete, Response: resp}
	}()
	return ch
}

func WithAPIKey(apiKey string) ProviderClientOption {
	return func(options *providerClientOptions) {
		options.apiKey = apiKey
	}
}

func WithModel(model models.Model) ProviderClientOption {
	return func(options *providerClientOptions) {
		options.model = model
	}
}

func WithMaxTokens(maxTokens int64) ProviderClientOption {
	return func(options *providerClientOptions) {
		options.maxTokens = maxTokens
	}
}

func WithSystemMessage(systemMessage string) ProviderClientOption {
	return func(options *providerClientOptions) {
		options.systemMessage = systemMessage
	}
}

func WithAnthropicOptions(anthropicOptions ...AnthropicOption) ProviderClientOption {
	return func(options *providerClientOptions) {
		options.anthropicOptions = anthropicOptions
	}
}

func WithOpenAIOptions(openaiOptions ...OpenAIOption) ProviderClientOption {
	return func(options *providerClientOptions) {
		options.openaiOptions = openaiOptions
	}
}

func WithGeminiOptions(geminiOptions ...GeminiOption) ProviderClientOption {
	return func(options *providerClientOptions) {
		options.geminiOptions = geminiOptions
	}
}
