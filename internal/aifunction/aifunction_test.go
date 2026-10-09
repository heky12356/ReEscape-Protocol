package aifunction

import (
	"context"
	"errors"
	"testing"
)

func TestRetryableAIErrorClassification(t *testing.T) {
	if !isRetryableAIError(errors.New("upstream returned 503")) {
		t.Fatal("503 should be retryable")
	}
	if isRetryableAIError(context.Canceled) {
		t.Fatal("context cancellation should not be retryable")
	}
}

func TestBackoffDelayIsBounded(t *testing.T) {
	for attempt := 1; attempt < 20; attempt++ {
		delay := backoffDelay(attempt)
		if delay < baseRetryDelay/2 || delay > maxRetryDelay {
			t.Fatalf("attempt %d delay = %s, outside bounds", attempt, delay)
		}
	}
}

func TestTokenBucketWaitHonorsContext(t *testing.T) {
	limiter := &tokenBucketLimiter{tokens: make(chan struct{}, 1)}
	limiter.tokens <- struct{}{}
	if err := limiter.Wait(context.Background()); err != nil {
		t.Fatalf("first token: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := limiter.Wait(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("wait error = %v, want context canceled", err)
	}
}
