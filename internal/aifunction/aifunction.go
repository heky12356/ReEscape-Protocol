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

func createChatCompletionWithPolicy(request openai.ChatCompletionRequest) (openai.ChatCompletionResponse, error) {
	cfg := config.GetConfig()

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
		limiter := getAILimiter()
		if limiter != nil {
			waitCtx, cancelWait := context.WithTimeout(context.Background(), timeout)
			err := limiter.Wait(waitCtx)
			cancelWait()
			if err != nil {
				return openai.ChatCompletionResponse{}, fmt.Errorf("wait rate limiter failed: %w", err)
			}
		}

		attemptCtx, cancelAttempt := context.WithTimeout(context.Background(), timeout)
		client := getClient()
		if client == nil {
			cancelAttempt()
			return openai.ChatCompletionResponse{}, fmt.Errorf("ai client is not initialized")
		}
		resp, lastErr = client.CreateChatCompletion(attemptCtx, request)
		cancelAttempt()

		if lastErr == nil {
			return resp, nil
		}

		if attempt >= attempts || !isRetryableAIError(lastErr) {
			break
		}

		delay := backoffDelay(attempt)
		utils.Warn("AI request failed, retrying (%d/%d) after %v: %v", attempt, attempts, delay, lastErr)
		time.Sleep(delay)
	}

	return openai.ChatCompletionResponse{}, fmt.Errorf("chat completion failed after %d attempts: %w", attempts, lastErr)
}

func Queryai(prompt string, msg string) (string, error) {
	resp, err := createChatCompletionWithPolicy(
		openai.ChatCompletionRequest{
			Model: config.GetConfig().AiModel,
			Messages: []openai.ChatCompletionMessage{
				{Role: "system", Content: prompt},
				{Role: "user", Content: msg},
			},
			Stream:      false,
			MaxTokens:   config.GetConfig().AiMaxTokens,
			Temperature: config.GetConfig().AiTemperature,
			TopP:        config.GetConfig().AiTopP,
		},
	)
	if err != nil {
		return "", fmt.Errorf("error in Queryai : ChatCompletion error: %v", err)
	}

	if len(resp.Choices) == 0 {
		return "", fmt.Errorf("error in Queryai : empty choices")
	}

	content := utils.CleanThinkTag(resp.Choices[0].Message.Content)
	return content, nil
}

func QueryaiWithChain(conversation []openai.ChatCompletionMessage) (newConversation []openai.ChatCompletionMessage, result []string, err error) {
	resp, err := createChatCompletionWithPolicy(
		openai.ChatCompletionRequest{
			Model:       config.GetConfig().AiModel,
			Messages:    conversation,
			Stream:      false,
			MaxTokens:   config.GetConfig().AiMaxTokens,
			Temperature: config.GetConfig().AiTemperature,
			TopP:        config.GetConfig().AiTopP,
			N:           1,
		},
	)
	if err != nil {
		return nil, nil, fmt.Errorf("error in QueryaiWithChain : ChatCompletion error: %v", err)
	}

	if len(resp.Choices) == 0 {
		return nil, nil, fmt.Errorf("error in QueryaiWithChain : empty choices")
	}

	for _, chs := range resp.Choices {
		content := utils.CleanThinkTag(chs.Message.Content)
		result = append(result, content)
		chs.Message.Content = content
		conversation = append(conversation, chs.Message)
	}
	return conversation, result, nil
}
