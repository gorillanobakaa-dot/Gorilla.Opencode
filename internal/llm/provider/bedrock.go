package provider

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/kujtimiihoxha/termai/internal/llm/models"
	"github.com/kujtimiihoxha/termai/internal/llm/tools"
	"github.com/kujtimiihoxha/termai/internal/message"
)

type bedrockProvider struct {
	childProvider Provider
	model         models.Model
	maxTokens     int64
	systemMessage string
}

func (b *bedrockProvider) SendMessages(ctx context.Context, messages []message.Message, tools []tools.BaseTool) (*ProviderResponse, error) {
	return b.childProvider.SendMessages(ctx, messages, tools)
}

func (b *bedrockProvider) StreamResponse(ctx context.Context, messages []message.Message, tools []tools.BaseTool) (<-chan ProviderEvent, error) {
	return b.childProvider.StreamResponse(ctx, messages, tools)
}

func NewBedrockProvider(opts ...BedrockOption) (Provider, error) {
	provider := &bedrockProvider{}
	for _, opt := range opts {
		opt(provider)
	}

	// based on the AWS region prefix the model name with, us, eu, ap, sa, etc.
	region := os.Getenv("AWS_REGION")
	if region == "" {
		region = os.Getenv("AWS_DEFAULT_REGION")
	}

	if region == "" {
		return nil, errors.New("AWS_REGION or AWS_DEFAULT_REGION environment variable is required")
	}
	if len(region) < 2 {
		return nil, errors.New("AWS_REGION or AWS_DEFAULT_REGION environment variable is invalid")
	}
	regionPrefix := region[:2]
	provider.model.APIModel = fmt.Sprintf("%s.%s", regionPrefix, provider.model.APIModel)

	if strings.Contains(string(provider.model.APIModel), "anthropic") {
		anthropic, err := NewAnthropicProvider(
			WithAnthropicModel(provider.model),
			WithAnthropicMaxTokens(provider.maxTokens),
			WithAnthropicSystemMessage(provider.systemMessage),
			WithAnthropicBedrock(),
			WithAnthropicDisableCache(),
		)
		provider.childProvider = anthropic
		if err != nil {
			return nil, err
		}
	} else {
		return nil, errors.New("unsupported model for bedrock provider")
	}
	return provider, nil
}

type BedrockOption func(*bedrockProvider)

func WithBedrockSystemMessage(message string) BedrockOption {
	return func(a *bedrockProvider) {
		a.systemMessage = message
	}
}

func WithBedrockMaxTokens(maxTokens int64) BedrockOption {
	return func(a *bedrockProvider) {
		a.maxTokens = maxTokens
	}
}

func WithBedrockModel(model models.Model) BedrockOption {
	return func(a *bedrockProvider) {
		a.model = model
	}
}
