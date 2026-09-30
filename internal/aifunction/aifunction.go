package aifunction

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"net"
	"strings"
	"sync"
	"time"

	"project-yume/internal/config"
	"project-yume/internal/utils"

	openai "github.com/sashabaranov/go-openai"
)

const (
	defaultAITimeoutSeconds = 30
	defaultMaxRetryCount    = 3
	defaultRateLimitRPM     = 20
	baseRetryDelay          = 500 * time.Millisecond
	maxRetryDelay           = 8 * time.Second
)

var (
	aiLimiterOnce sync.Once
	aiLimiter     *tokenBucketLimiter
	aiLimiterMu   sync.Mutex
)

type aiTraceMeta struct {
	RequestID string
	Kind      string
}

type tokenBucketLimiter struct {
	tokens chan struct{}
}

func newTokenBucketLimiter(ratePerMinute int) *tokenBucketLimiter {
	if ratePerMinute <= 0 {
		return nil
	}

	limiter := &tokenBucketLimiter{
		tokens: make(chan struct{}, ratePerMinute),
	}

	for i := 0; i < ratePerMinute; i++ {
		limiter.tokens <- struct{}{}
	}

	interval := time.Minute / time.Duration(ratePerMinute)
	ticker := time.NewTicker(interval)

	go func() {
		for range ticker.C {
			select {
			case limiter.tokens <- struct{}{}:
			default:
			}
		}
	}()

	return limiter
}

func (l *tokenBucketLimiter) Wait(ctx context.Context) error {
	if l == nil {
		return nil
	}

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-l.tokens:
		return nil
	}
}

func getAILimiter() *tokenBucketLimiter {
	aiLimiterMu.Lock()
	defer aiLimiterMu.Unlock()

	aiLimiterOnce.Do(func() {
		rateLimit := config.GetConfig().AiRateLimit
		if rateLimit <= 0 {
			rateLimit = defaultRateLimitRPM
		}
		aiLimiter = newTokenBucketLimiter(rateLimit)
	})
	return aiLimiter
}

func ResetRateLimiter() {
	aiLimiterMu.Lock()
	defer aiLimiterMu.Unlock()
	aiLimiter = nil
	aiLimiterOnce = sync.Once{}
}

func backoffDelay(attempt int) time.Duration {
	if attempt <= 1 {
		return baseRetryDelay
	}

	delay := baseRetryDelay * time.Duration(1<<(attempt-1))
	if delay > maxRetryDelay {
		delay = maxRetryDelay
	}

	half := delay / 2
	jitter := time.Duration(rand.Int63n(int64(half + 1)))
	return half + jitter
}

func isRetryableAIError(err error) bool {
	if err == nil {
		return false
	}

	if errors.Is(err, context.Canceled) {
		return false
	}

	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}

	var netErr net.Error
	if errors.As(err, &netErr) {
		return true
	}

	msg := strings.ToLower(err.Error())
	retryableHints := []string{
		"timeout",
		"too many requests",
		"rate limit",
		"temporarily unavailable",
		"connection reset",
		"connection refused",
		"eof",
		"429",
		"500",
		"502",
		"503",
		"504",
	}

	for _, hint := range retryableHints {
		if strings.Contains(msg, hint) {
			return true
		}
	}

	return false
}

func createChatCompletionWithPolicy(ctx context.Context, request openai.ChatCompletionRequest, trace aiTraceMeta) (openai.ChatCompletionResponse, error) {
	cfg := config.GetConfig()
	if ctx == nil {
		ctx = context.Background()
	}

	timeoutSeconds := cfg.AiTimeout
	if timeoutSeconds <= 0 {
		timeoutSeconds = defaultAITimeoutSeconds
	}

	maxRetryCount := cfg.AiRetryCount
	if maxRetryCount < 0 {
		maxRetryCount = 0
	}
	if maxRetryCount > defaultMaxRetryCount {
		maxRetryCount = defaultMaxRetryCount
	}

	attempts := maxRetryCount + 1
	timeout := time.Duration(timeoutSeconds) * time.Second

	var lastErr error
	var resp openai.ChatCompletionResponse

	for attempt := 1; attempt <= attempts; attempt++ {
		utils.LogAIRaw(trace.Kind, "request", trace.RequestID, attempt, buildAIRawRequestPayload(request), nil)

		limiter := getAILimiter()
		if limiter != nil {
			waitCtx, cancelWait := context.WithTimeout(ctx, timeout)
			err := limiter.Wait(waitCtx)
			cancelWait()
			if err != nil {
				utils.LogAIRaw(trace.Kind, "error", trace.RequestID, attempt, nil, err)
				return openai.ChatCompletionResponse{}, fmt.Errorf("wait rate limiter failed: %w", err)
			}
		}

		attemptCtx, cancelAttempt := context.WithTimeout(ctx, timeout)
		client := getClient()
		if client == nil {
			cancelAttempt()
			err := fmt.Errorf("ai client is not initialized")
			utils.LogAIRaw(trace.Kind, "error", trace.RequestID, attempt, nil, err)
			return openai.ChatCompletionResponse{}, err
		}
		resp, lastErr = client.CreateChatCompletion(attemptCtx, request)
		cancelAttempt()

		if lastErr == nil {
			utils.LogAIRaw(trace.Kind, "response", trace.RequestID, attempt, buildAIRawResponsePayload(resp), nil)
			return resp, nil
		}
		utils.LogAIRaw(trace.Kind, "error", trace.RequestID, attempt, nil, lastErr)

		if attempt >= attempts || !isRetryableAIError(lastErr) {
			break
		}

		delay := backoffDelay(attempt)
		utils.Warn("AI request failed, retrying (%d/%d) after %v: %v", attempt, attempts, delay, lastErr)
		select {
		case <-ctx.Done():
			return openai.ChatCompletionResponse{}, ctx.Err()
		case <-time.After(delay):
		}
	}

	return openai.ChatCompletionResponse{}, fmt.Errorf("chat completion failed after %d attempts: %w", attempts, lastErr)
}

func Chat(ctx context.Context, req openai.ChatCompletionRequest) (openai.ChatCompletionResponse, error) {
	if strings.TrimSpace(req.Model) == "" {
		req.Model = config.GetConfig().AiModel
	}
	if req.MaxTokens == 0 {
		req.MaxTokens = config.GetConfig().AiMaxTokens
	}
	if req.Temperature == 0 {
		req.Temperature = config.GetConfig().AiTemperature
	}
	if req.TopP == 0 {
		req.TopP = config.GetConfig().AiTopP
	}
	req.Stream = false

	return createChatCompletionWithPolicy(
		ctx,
		req,
		aiTraceMeta{
			RequestID: utils.NewRequestID("ai"),
			Kind:      "chat",
		},
	)
}

type aiRawRequestPayload struct {
	Model       string                `json:"model"`
	MaxTokens   int                   `json:"max_tokens,omitempty"`
	Temperature float32               `json:"temperature,omitempty"`
	TopP        float32               `json:"top_p,omitempty"`
	N           int                   `json:"n,omitempty"`
	Stream      bool                  `json:"stream"`
	Messages    []aiRawMessagePayload `json:"messages"`
}

type aiRawMessagePayload struct {
	Role         string                    `json:"role"`
	Content      string                    `json:"content,omitempty"`
	MultiContent []aiRawMessagePartPayload `json:"multi_content,omitempty"`
}

type aiRawMessagePartPayload struct {
	Type     string `json:"type"`
	Text     string `json:"text,omitempty"`
	ImageURL string `json:"image_url,omitempty"`
	Detail   string `json:"detail,omitempty"`
}

type aiRawResponsePayload struct {
	ID      string               `json:"id"`
	Model   string               `json:"model"`
	Usage   openai.Usage         `json:"usage"`
	Choices []aiRawChoicePayload `json:"choices"`
}

type aiRawChoicePayload struct {
	Index        int                 `json:"index"`
	FinishReason string              `json:"finish_reason,omitempty"`
	Message      aiRawMessagePayload `json:"message"`
}

func buildAIRawRequestPayload(request openai.ChatCompletionRequest) aiRawRequestPayload {
	payload := aiRawRequestPayload{
		Model:       request.Model,
		MaxTokens:   request.MaxTokens,
		Temperature: request.Temperature,
		TopP:        request.TopP,
		N:           request.N,
		Stream:      request.Stream,
		Messages:    make([]aiRawMessagePayload, 0, len(request.Messages)),
	}

	for _, msg := range request.Messages {
		payload.Messages = append(payload.Messages, convertAIRawMessage(msg))
	}

	return payload
}

func buildAIRawResponsePayload(resp openai.ChatCompletionResponse) aiRawResponsePayload {
	payload := aiRawResponsePayload{
		ID:      resp.ID,
		Model:   resp.Model,
		Usage:   resp.Usage,
		Choices: make([]aiRawChoicePayload, 0, len(resp.Choices)),
	}

	for _, choice := range resp.Choices {
		payload.Choices = append(payload.Choices, aiRawChoicePayload{
			Index:        choice.Index,
			FinishReason: string(choice.FinishReason),
			Message:      convertAIRawMessage(choice.Message),
		})
	}

	return payload
}

func convertAIRawMessage(msg openai.ChatCompletionMessage) aiRawMessagePayload {
	payload := aiRawMessagePayload{
		Role:    msg.Role,
		Content: msg.Content,
	}

	if len(msg.MultiContent) == 0 {
		return payload
	}

	payload.MultiContent = make([]aiRawMessagePartPayload, 0, len(msg.MultiContent))
	for _, part := range msg.MultiContent {
		entry := aiRawMessagePartPayload{
			Type: string(part.Type),
			Text: part.Text,
		}
		if part.ImageURL != nil {
			entry.ImageURL = part.ImageURL.URL
			entry.Detail = string(part.ImageURL.Detail)
		}
		payload.MultiContent = append(payload.MultiContent, entry)
	}

	return payload
}
