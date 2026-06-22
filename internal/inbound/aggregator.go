package inbound

import (
	"context"
	"fmt"
	"strings"
	"time"

	"project-yume/internal/config"
	"project-yume/internal/model"
	"project-yume/internal/state"
)

const defaultAggregationSweepInterval = 250 * time.Millisecond

type MessageAggregator struct {
	buckets map[string]*aggregationBucket
}

type aggregationBucket struct {
	firstSeen time.Time
	lastSeen  time.Time
	messages  []model.Msg
	seenKeys  map[string]struct{}
}

func NewMessageAggregator() *MessageAggregator {
	return &MessageAggregator{
		buckets: make(map[string]*aggregationBucket),
	}
}

func (a *MessageAggregator) Run(ctx context.Context, in <-chan model.Msg, out chan<- model.Msg) {
	defer close(out)

	ticker := time.NewTicker(defaultAggregationSweepInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			a.flushAll(ctx, out)
			return
		case msg, ok := <-in:
			if !ok {
				a.flushAll(ctx, out)
				return
			}
			a.handleMessage(ctx, msg, out)
		case <-ticker.C:
			a.flushExpired(ctx, out)
		}
	}
}

func (a *MessageAggregator) handleMessage(ctx context.Context, msg model.Msg, out chan<- model.Msg) {
	sessionID := state.BuildSessionID(msg.User_id, msg.Group_id, msg.Type)

	if !shouldAggregate(msg) {
		if strings.TrimSpace(msg.Message) == "exit();" {
			a.flushBucket(ctx, sessionID, out)
		}
		a.send(ctx, out, msg)
		return
	}

	now := time.Now()
	idleWindow, maxWindow, maxMessages := currentAggregationLimits()
	bucket := a.buckets[sessionID]

	if bucket != nil {
		if now.Sub(bucket.lastSeen) > idleWindow || now.Sub(bucket.firstSeen) > maxWindow {
			a.flushBucket(ctx, sessionID, out)
			bucket = nil
		}
	}

	if bucket == nil {
		bucket = &aggregationBucket{
			firstSeen: now,
			lastSeen:  now,
			messages:  make([]model.Msg, 0, maxMessages),
			seenKeys:  make(map[string]struct{}),
		}
		a.buckets[sessionID] = bucket
	}

	if !bucket.add(msg) {
		return
	}

	bucket.lastSeen = now
	if len(bucket.messages) >= maxMessages {
		a.flushBucket(ctx, sessionID, out)
	}
}

func (a *MessageAggregator) flushExpired(ctx context.Context, out chan<- model.Msg) {
	now := time.Now()
	idleWindow, maxWindow, _ := currentAggregationLimits()

	for sessionID, bucket := range a.buckets {
		if now.Sub(bucket.lastSeen) > idleWindow || now.Sub(bucket.firstSeen) > maxWindow {
			a.flushBucket(ctx, sessionID, out)
		}
	}
}

func (a *MessageAggregator) flushAll(ctx context.Context, out chan<- model.Msg) {
	for sessionID := range a.buckets {
		a.flushBucket(ctx, sessionID, out)
	}
}

func (a *MessageAggregator) flushBucket(ctx context.Context, sessionID string, out chan<- model.Msg) {
	bucket, ok := a.buckets[sessionID]
	if !ok || len(bucket.messages) == 0 {
		delete(a.buckets, sessionID)
		return
	}

	aggregated := bucket.build()
	delete(a.buckets, sessionID)
	a.send(ctx, out, aggregated)
}

func (a *MessageAggregator) send(ctx context.Context, out chan<- model.Msg, msg model.Msg) {
	select {
	case out <- msg:
	case <-ctx.Done():
	}
}

func currentAggregationLimits() (time.Duration, time.Duration, int) {
	cfg := config.GetConfig()

	idleWindow := time.Duration(cfg.MessageAggregateIdleWindowMs) * time.Millisecond
	if idleWindow <= 0 {
		idleWindow = 2 * time.Second
	}

	maxWindow := time.Duration(cfg.MessageAggregateMaxWindowMs) * time.Millisecond
	if maxWindow <= 0 {
		maxWindow = 10 * time.Second
	}
	if maxWindow < idleWindow {
		maxWindow = idleWindow
	}

	maxMessages := cfg.MessageAggregateMaxMessages
	if maxMessages <= 0 {
		maxMessages = 5
	}

	return idleWindow, maxWindow, maxMessages
}

func shouldAggregate(msg model.Msg) bool {
	cfg := config.GetConfig()
	if msg.Type != 1 {
		return false
	}
	if msg.User_id != cfg.TargetId {
		return false
	}
	if strings.TrimSpace(msg.Message) == "" {
		return false
	}
	if strings.TrimSpace(msg.Message) == "exit();" {
		return false
	}
	return true
}

func (b *aggregationBucket) add(msg model.Msg) bool {
	key := aggregationKey(msg)
	if key != "" {
		if _, exists := b.seenKeys[key]; exists {
			return false
		}
		b.seenKeys[key] = struct{}{}
	}

	b.messages = append(b.messages, msg)
	return true
}

func (b *aggregationBucket) build() model.Msg {
	first := b.messages[0]
	last := b.messages[len(b.messages)-1]

	segments := make([]string, 0, len(b.messages))
	messageIDs := make([]int64, 0, len(b.messages))
	parts := make([]model.MessagePart, 0, len(b.messages))
	for _, msg := range b.messages {
		segments = append(segments, msg.Message)
		messageIDs = append(messageIDs, msg.MessageID)
		parts = append(parts, msg.Parts...)
	}

	return model.Msg{
		Message:     strings.Join(segments, "\n"),
		Parts:       parts,
		User_id:     first.User_id,
		Group_id:    first.Group_id,
		MessageID:   last.MessageID,
		MessageIDs:  messageIDs,
		RawSegments: segments,
		Aggregated:  len(b.messages) > 1,
		StartTime:   first.Time,
		EndTime:     last.Time,
		Time:        last.Time,
		Type:        first.Type,
	}
}

func aggregationKey(msg model.Msg) string {
	if msg.MessageID != 0 {
		return fmt.Sprintf("msg:%d", msg.MessageID)
	}

	raw := strings.TrimSpace(msg.Message)
	if raw == "" {
		return ""
	}

	return fmt.Sprintf("fallback:%d:%d:%d:%s", msg.Type, msg.User_id, msg.Group_id, raw)
}
