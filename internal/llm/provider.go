package llm

import (
	"context"

	"project-yume/internal/aifunction"

	openai "github.com/sashabaranov/go-openai"
)

// Provider is the boundary around an external model service. Provider
// implementations own timeout, retry, rate limiting, and raw request logging.
type Provider interface {
	Name() string
	Complete(context.Context, openai.ChatCompletionRequest) (openai.ChatCompletionResponse, error)
}

type AIFunctionProvider struct{}

func NewAIFunctionProvider() *AIFunctionProvider { return &AIFunctionProvider{} }

func (p *AIFunctionProvider) Name() string { return "aifunction" }

func (p *AIFunctionProvider) Complete(ctx context.Context, request openai.ChatCompletionRequest) (openai.ChatCompletionResponse, error) {
	return aifunction.Chat(ctx, request)
}
