package provider

import (
	"context"
	"errors"

	"github.com/kujtimiihoxha/termai/internal/llm/models"
	"github.com/kujtimiihoxha/termai/internal/llm/tools"
	"github.com/kujtimiihoxha/termai/internal/message"
	"github.com/openai/openai-go"
	"github.com/openai/openai-go/option"
)

type openaiProvider struct {
	client        openai.Client
	model         models.Model
	maxTokens     int64
	baseURL       string
	apiKey        string
	systemMessage string
}

type OpenAIOption func(*openaiProvider)

func NewOpenAIProvider(opts ...OpenAIOption) (Provider, error) {
	provider := &openaiProvider{
		maxTokens: 5000,
	}

	for _, opt := range opts {
		opt(provider)
	}

	clientOpts := []option.RequestOption{
		option.WithAPIKey(provider.apiKey),
	}
	if provider.baseURL != "" {
		clientOpts = append(clientOpts, option.WithBaseURL(provider.baseURL))
	}

	provider.client = openai.NewClient(clientOpts...)
	if provider.systemMessage == "" {
		return nil, errors.New("system message is required")
	}

	return provider, nil
}

func WithOpenAISystemMessage(message string) OpenAIOption {
	return func(p *openaiProvider) {
		p.systemMessage = message
	}
}

func WithOpenAIMaxTokens(maxTokens int64) OpenAIOption {
	return func(p *openaiProvider) {
		p.maxTokens = maxTokens
	}
}

func WithOpenAIModel(model models.Model) OpenAIOption {
	return func(p *openaiProvider) {
		p.model = model
	}
}

func WithOpenAIBaseURL(baseURL string) OpenAIOption {
	return func(p *openaiProvider) {
		p.baseURL = baseURL
	}
}

func WithOpenAIKey(apiKey string) OpenAIOption {
	return func(p *openaiProvider) {
		p.apiKey = apiKey
	}
}

func (p *openaiProvider) convertToOpenAIMessages(messages []message.Message) []openai.ChatCompletionMessageParamUnion {
	var chatMessages []openai.ChatCompletionMessageParamUnion

	chatMessages = append(chatMessages, openai.SystemMessage(p.systemMessage))

	for _, msg := range messages {
		switch msg.Role {
		case message.User:
			chatMessages = append(chatMessages, openai.UserMessage(msg.Content().String()))

		case message.Assistant:
			assistantMsg := openai.ChatCompletionAssistantMessageParam{
				Role: "assistant",
			}

			if msg.Content().String() != "" {
				assistantMsg.Content = openai.ChatCompletionAssistantMessageParamContentUnion{
					OfString: openai.String(msg.Content().String()),
				}
			}

			if len(msg.ToolCalls()) > 0 {
				assistantMsg.ToolCalls = make([]openai.ChatCompletionMessageToolCallParam, len(msg.ToolCalls()))
				for i, call := range msg.ToolCalls() {
					assistantMsg.ToolCalls[i] = openai.ChatCompletionMessageToolCallParam{
						ID:   call.ID,
						Type: "function",
						Function: openai.ChatCompletionMessageToolCallFunctionParam{
							Name:      call.Name,
							Arguments: call.Input,
						},
					}
				}
			}

			chatMessages = append(chatMessages, openai.ChatCompletionMessageParamUnion{
				OfAssistant: &assistantMsg,
			})

		case message.Tool:
			for _, result := range msg.ToolResults() {
				chatMessages = append(chatMessages,
					openai.ToolMessage(result.Content, result.ToolCallID),
				)
			}
		}
	}

	return chatMessages
}

func (p *openaiProvider) convertToOpenAITools(tools []tools.BaseTool) []openai.ChatCompletionToolParam {
	openaiTools := make([]openai.ChatCompletionToolParam, len(tools))

	for i, tool := range tools {
		info := tool.Info()
		openaiTools[i] = openai.ChatCompletionToolParam{
			Function: openai.FunctionDefinitionParam{
				Name:        info.Name,
				Description: openai.String(info.Description),
				Parameters: openai.FunctionParameters{
					"type":       "object",
					"properties": info.Parameters,
					"required":   info.Required,
				},
			},
		}
	}

	return openaiTools
}

func (p *openaiProvider) extractTokenUsage(usage openai.CompletionUsage) TokenUsage {
	cachedTokens := int64(0)

	cachedTokens = usage.PromptTokensDetails.CachedTokens
	inputTokens := usage.PromptTokens - cachedTokens

	return TokenUsage{
		InputTokens:         inputTokens,
		OutputTokens:        usage.CompletionTokens,
		CacheCreationTokens: 0, // OpenAI doesn't provide this directly
		CacheReadTokens:     cachedTokens,
	}
}

func (p *openaiProvider) SendMessages(ctx context.Context, messages []message.Message, tools []tools.BaseTool) (*ProviderResponse, error) {
	messages = cleanupMessages(messages)
	chatMessages := p.convertToOpenAIMessages(messages)
	openaiTools := p.convertToOpenAITools(tools)

	params := openai.ChatCompletionNewParams{
		Model:     openai.ChatModel(p.model.APIModel),
		Messages:  chatMessages,
		MaxTokens: openai.Int(p.maxTokens),
		Tools:     openaiTools,
	}

	response, err := p.client.Chat.Completions.New(ctx, params)
	if err != nil {
		return nil, err
	}

	content := ""
	if response.Choices[0].Message.Content != "" {
		content = response.Choices[0].Message.Content
	}

	var toolCalls []message.ToolCall
	if len(response.Choices[0].Message.ToolCalls) > 0 {
		toolCalls = make([]message.ToolCall, len(response.Choices[0].Message.ToolCalls))
		for i, call := range response.Choices[0].Message.ToolCalls {
			toolCalls[i] = message.ToolCall{
				ID:    call.ID,
				Name:  call.Function.Name,
				Input: call.Function.Arguments,
				Type:  "function",
			}
		}
	}

	tokenUsage := p.extractTokenUsage(response.Usage)

	return &ProviderResponse{
		Content:   content,
		ToolCalls: toolCalls,
		Usage:     tokenUsage,
	}, nil
}

func (p *openaiProvider) StreamResponse(ctx context.Context, messages []message.Message, tools []tools.BaseTool) (<-chan ProviderEvent, error) {
	messages = cleanupMessages(messages)
	chatMessages := p.convertToOpenAIMessages(messages)
	openaiTools := p.convertToOpenAITools(tools)

	params := openai.ChatCompletionNewParams{
		Model:     openai.ChatModel(p.model.APIModel),
		Messages:  chatMessages,
		MaxTokens: openai.Int(p.maxTokens),
		Tools:     openaiTools,
		StreamOptions: openai.ChatCompletionStreamOptionsParam{
			IncludeUsage: openai.Bool(true),
		},
	}

	stream := p.client.Chat.Completions.NewStreaming(ctx, params)

	eventChan := make(chan ProviderEvent)

	toolCalls := make([]message.ToolCall, 0)
	go func() {
		defer close(eventChan)

		acc := openai.ChatCompletionAccumulator{}
		currentContent := ""

		for stream.Next() {
			chunk := stream.Current()
			acc.AddChunk(chunk)

			if tool, ok := acc.JustFinishedToolCall(); ok {
				toolCalls = append(toolCalls, message.ToolCall{
					ID:    tool.Id,
					Name:  tool.Name,
					Input: tool.Arguments,
					Type:  "function",
				})
			}

			for _, choice := range chunk.Choices {
				if choice.Delta.Content != "" {
					eventChan <- ProviderEvent{
						Type:    EventContentDelta,
						Content: choice.Delta.Content,
					}
					currentContent += choice.Delta.Content
				}
			}
		}

		if err := stream.Err(); err != nil {
			eventChan <- ProviderEvent{
				Type:  EventError,
				Error: err,
			}
			return
		}

		tokenUsage := p.extractTokenUsage(acc.Usage)

		eventChan <- ProviderEvent{
			Type: EventComplete,
			Response: &ProviderResponse{
				Content:   currentContent,
				ToolCalls: toolCalls,
				Usage:     tokenUsage,
			},
		}
	}()

	return eventChan, nil
}
