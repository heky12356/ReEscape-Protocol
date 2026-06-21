package inbound

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"project-yume/internal/handler"
)

type DedupeStage struct {
	mu   sync.Mutex
	seen map[string]time.Time
	ttl  time.Duration
}

func NewDedupeStage(ttl time.Duration) *DedupeStage {
	if ttl <= 0 {
		ttl = 5 * time.Minute
	}
	return &DedupeStage{
		seen: make(map[string]time.Time),
		ttl:  ttl,
	}
}

func (s *DedupeStage) Name() string {
	return "dedupe"
}

func (s *DedupeStage) Process(ctx *handler.MessageContext) error {
	key := dedupeKey(ctx)
	if key == "" {
		return nil
	}

	now := time.Now()

	s.mu.Lock()
	defer s.mu.Unlock()

	for existingKey, seenAt := range s.seen {
		if now.Sub(seenAt) > s.ttl {
			delete(s.seen, existingKey)
		}
	}

	if seenAt, ok := s.seen[key]; ok && now.Sub(seenAt) <= s.ttl {
		return skip("duplicate message")
	}

	s.seen[key] = now
	return nil
}

func dedupeKey(ctx *handler.MessageContext) string {
	if ctx.MessageID != 0 {
		return fmt.Sprintf("msg:%d", ctx.MessageID)
	}

	raw := strings.TrimSpace(ctx.RawMessage)
	if raw == "" {
		return ""
	}

	return fmt.Sprintf("fallback:%d:%d:%d:%s", ctx.ChatType, ctx.UserID, ctx.GroupID, raw)
}
