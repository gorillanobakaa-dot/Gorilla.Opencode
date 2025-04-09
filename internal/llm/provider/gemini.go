package provider

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/google/generative-ai-go/genai"
	"github.com/google/uuid"
	"github.com/kujtimiihoxha/termai/internal/llm/models"
	"github.com/kujtimiihoxha/termai/internal/llm/tools"
	"github.com/kujtimiihoxha/termai/internal/message"
	"google.golang.org/api/iterator"
	"google.golang.org/api/option"
)

type geminiProvider struct {
	client        *genai.Client
	model         models.Model
	maxTokens     int32
	apiKey        string
	systemMessage string
}

type GeminiOption func(*geminiProvider)

func NewGeminiProvider(ctx context.Context, opts ...GeminiOption) (Provider, error) {
	provider := &geminiProvider{
		maxTokens: 5000,
	}

	for _, opt := range opts {
		opt(provider)
	}

	if provider.systemMessage == "" {
		return nil, errors.New("system message is required")
	}

	client, err := genai.NewClient(ctx, option.WithAPIKey(provider.apiKey))
	if err != nil {
		return nil, err
	}
	provider.client = client

	return provider, nil
}

func WithGeminiSystemMessage(message string) GeminiOption {
	return func(p *geminiProvider) {
		p.systemMessage = message
	}
}

func WithGeminiMaxTokens(maxTokens int32) GeminiOption {
	return func(p *geminiProvider) {
		p.maxTokens = maxTokens
	}
}

func WithGeminiModel(model models.Model) GeminiOption {
	return func(p *geminiProvider) {
		p.model = model
	}
}

func WithGeminiKey(apiKey string) GeminiOption {
	return func(p *geminiProvider) {
		p.apiKey = apiKey
	}
}

func (p *geminiProvider) Close() {
	if p.client != nil {
		p.client.Close()
	}
}

func (p *geminiProvider) convertToGeminiHistory(messages []message.Message) []*genai.Content {
	var history []*genai.Content

	for _, msg := range messages {
		switch msg.Role {
		case message.User:
			history = append(history, &genai.Content{
				Parts: []genai.Part{genai.Text(msg.Content().String())},
				Role:  "user",
			})
		case message.Assistant:
			content := &genai.Content{
				Role:  "model",
				Parts: []genai.Part{},
			}

			if msg.Content().String() != "" {
				content.Parts = append(content.Parts, genai.Text(msg.Content().String()))
			}

			if len(msg.ToolCalls()) > 0 {
				for _, call := range msg.ToolCalls() {
					args, _ := parseJsonToMap(call.Input)
					content.Parts = append(content.Parts, genai.FunctionCall{
						Name: call.Name,
						Args: args,
					})
				}
			}

			history = append(history, content)
		case message.Tool:
			for _, result := range msg.ToolResults() {
				response := map[string]interface{}{"result": result.Content}
				parsed, err := parseJsonToMap(result.Content)
				if err == nil {
					response = parsed
				}
				var toolCall message.ToolCall
				for _, msg := range messages {
					if msg.Role == message.Assistant {
						for _, call := range msg.ToolCalls() {
							if call.ID == result.ToolCallID {
								toolCall = call
								break
							}
						}
					}
				}

				history = append(history, &genai.Content{
					Parts: []genai.Part{genai.FunctionResponse{
						Name:     toolCall.Name,
						Response: response,
					}},
					Role: "function",
				})
			}
		}
	}

	return history
}

func (p *geminiProvider) extractTokenUsage(resp *genai.GenerateContentResponse) TokenUsage {
	if resp == nil || resp.UsageMetadata == nil {
		return TokenUsage{}
	}

	return TokenUsage{
		InputTokens:         int64(resp.UsageMetadata.PromptTokenCount),
		OutputTokens:        int64(resp.UsageMetadata.CandidatesTokenCount),
		CacheCreationTokens: 0, // Not directly provided by Gemini
		CacheReadTokens:     int64(resp.UsageMetadata.CachedContentTokenCount),
	}
}

func (p *geminiProvider) SendMessages(ctx context.Context, messages []message.Message, tools []tools.BaseTool) (*ProviderResponse, error) {
	model := p.client.GenerativeModel(p.model.APIModel)
	model.SetMaxOutputTokens(p.maxTokens)

	model.SystemInstruction = genai.NewUserContent(genai.Text(p.systemMessage))

	if len(tools) > 0 {
		declarations := p.convertToolsToGeminiFunctionDeclarations(tools)
		for _, declaration := range declarations {
			model.Tools = append(model.Tools, &genai.Tool{FunctionDeclarations: []*genai.FunctionDeclaration{declaration}})
		}
	}

	chat := model.StartChat()
	chat.History = p.convertToGeminiHistory(messages[:len(messages)-1]) // Exclude last message

	lastUserMsg := messages[len(messages)-1]
	resp, err := chat.SendMessage(ctx, genai.Text(lastUserMsg.Content().String()))
	if err != nil {
		return nil, err
	}

	var content string
	var toolCalls []message.ToolCall

	if len(resp.Candidates) > 0 && resp.Candidates[0].Content != nil {
		for _, part := range resp.Candidates[0].Content.Parts {
			switch p := part.(type) {
			case genai.Text:
				content = string(p)
			case genai.FunctionCall:
				id := "call_" + uuid.New().String()
				args, _ := json.Marshal(p.Args)
				toolCalls = append(toolCalls, message.ToolCall{
					ID:    id,
					Name:  p.Name,
					Input: string(args),
					Type:  "function",
				})
			}
		}
	}

	tokenUsage := p.extractTokenUsage(resp)

	return &ProviderResponse{
		Content:   content,
		ToolCalls: toolCalls,
		Usage:     tokenUsage,
	}, nil
}

func (p *geminiProvider) StreamResponse(ctx context.Context, messages []message.Message, tools []tools.BaseTool) (<-chan ProviderEvent, error) {
	model := p.client.GenerativeModel(p.model.APIModel)
	model.SetMaxOutputTokens(p.maxTokens)

	model.SystemInstruction = genai.NewUserContent(genai.Text(p.systemMessage))

	if len(tools) > 0 {
		declarations := p.convertToolsToGeminiFunctionDeclarations(tools)
		for _, declaration := range declarations {
			model.Tools = append(model.Tools, &genai.Tool{FunctionDeclarations: []*genai.FunctionDeclaration{declaration}})
		}
	}

	chat := model.StartChat()
	chat.History = p.convertToGeminiHistory(messages[:len(messages)-1]) // Exclude last message

	lastUserMsg := messages[len(messages)-1]

	iter := chat.SendMessageStream(ctx, genai.Text(lastUserMsg.Content().String()))

	eventChan := make(chan ProviderEvent)

	go func() {
		defer close(eventChan)

		var finalResp *genai.GenerateContentResponse
		currentContent := ""
		toolCalls := []message.ToolCall{}

		for {
			resp, err := iter.Next()
			if err == iterator.Done {
				break
			}
			if err != nil {
				eventChan <- ProviderEvent{
					Type:  EventError,
					Error: err,
				}
				return
			}

			finalResp = resp

			if len(resp.Candidates) > 0 && resp.Candidates[0].Content != nil {
				for _, part := range resp.Candidates[0].Content.Parts {
					switch p := part.(type) {
					case genai.Text:
						newText := string(p)
						eventChan <- ProviderEvent{
							Type:    EventContentDelta,
							Content: newText,
						}
						currentContent += newText
					case genai.FunctionCall:
						id := "call_" + uuid.New().String()
						args, _ := json.Marshal(p.Args)
						newCall := message.ToolCall{
							ID:    id,
							Name:  p.Name,
							Input: string(args),
							Type:  "function",
						}

						isNew := true
						for _, existing := range toolCalls {
							if existing.Name == newCall.Name && existing.Input == newCall.Input {
								isNew = false
								break
							}
						}

						if isNew {
							toolCalls = append(toolCalls, newCall)
						}
					}
				}
			}
		}

		tokenUsage := p.extractTokenUsage(finalResp)

		eventChan <- ProviderEvent{
			Type: EventComplete,
			Response: &ProviderResponse{
				Content:      currentContent,
				ToolCalls:    toolCalls,
				Usage:        tokenUsage,
				FinishReason: string(finalResp.Candidates[0].FinishReason.String()),
			},
		}
	}()

	return eventChan, nil
}

func (p *geminiProvider) convertToolsToGeminiFunctionDeclarations(tools []tools.BaseTool) []*genai.FunctionDeclaration {
	declarations := make([]*genai.FunctionDeclaration, len(tools))

	for i, tool := range tools {
		info := tool.Info()
		declarations[i] = &genai.FunctionDeclaration{
			Name:        info.Name,
			Description: info.Description,
			Parameters: &genai.Schema{
				Type:       genai.TypeObject,
				Properties: convertSchemaProperties(info.Parameters),
				Required:   info.Required,
			},
		}
	}

	return declarations
}

func convertSchemaProperties(parameters map[string]interface{}) map[string]*genai.Schema {
	properties := make(map[string]*genai.Schema)

	for name, param := range parameters {
		properties[name] = convertToSchema(param)
	}

	return properties
}

func convertToSchema(param interface{}) *genai.Schema {
	schema := &genai.Schema{Type: genai.TypeString}

	paramMap, ok := param.(map[string]interface{})
	if !ok {
		return schema
	}

	if desc, ok := paramMap["description"].(string); ok {
		schema.Description = desc
	}

	typeVal, hasType := paramMap["type"]
	if !hasType {
		return schema
	}

	typeStr, ok := typeVal.(string)
	if !ok {
		return schema
	}

	schema.Type = mapJSONTypeToGenAI(typeStr)

	switch typeStr {
	case "array":
		schema.Items = processArrayItems(paramMap)
	case "object":
		if props, ok := paramMap["properties"].(map[string]interface{}); ok {
			schema.Properties = convertSchemaProperties(props)
		}
	}

	return schema
}

func processArrayItems(paramMap map[string]interface{}) *genai.Schema {
	items, ok := paramMap["items"].(map[string]interface{})
	if !ok {
		return nil
	}

	return convertToSchema(items)
}

func mapJSONTypeToGenAI(jsonType string) genai.Type {
	switch jsonType {
	case "string":
		return genai.TypeString
	case "number":
		return genai.TypeNumber
	case "integer":
		return genai.TypeInteger
	case "boolean":
		return genai.TypeBoolean
	case "array":
		return genai.TypeArray
	case "object":
		return genai.TypeObject
	default:
		return genai.TypeString // Default to string for unknown types
	}
}

func parseJsonToMap(jsonStr string) (map[string]interface{}, error) {
	var result map[string]interface{}
	err := json.Unmarshal([]byte(jsonStr), &result)
	return result, err
}
