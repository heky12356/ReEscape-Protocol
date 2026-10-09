package service

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"strconv"
	"strings"
	"sync"
	"time"

	"project-yume/internal/assets"
	"project-yume/internal/connect"
	"project-yume/internal/domain/intent"
	"project-yume/internal/eventlog"
	"project-yume/internal/model"
	"project-yume/internal/state"
	"project-yume/internal/utils"

	"github.com/gorilla/websocket"
)

type DeliveryStatus string

const (
	DeliveryPending   DeliveryStatus = "pending"
	DeliverySending   DeliveryStatus = "sending"
	DeliveryDelivered DeliveryStatus = "delivered"
	DeliveryFailed    DeliveryStatus = "failed"
	DeliveryCancelled DeliveryStatus = "cancelled"
	DeliverySkipped   DeliveryStatus = "skipped"
)

const (
	DeliveryResultEmpty     = "empty"
	DeliveryResultDelivered = "delivered"
	DeliveryResultPartial   = "partial"
	DeliveryResultFailed    = "failed"
	DeliveryResultCancelled = "cancelled"
)

type DeliveryItemResult struct {
	Index       int
	Kind        string
	Status      DeliveryStatus
	MessageID   int64
	StartedAt   time.Time
	DeliveredAt time.Time
	Error       string
	AssetID     string
}

type DeliveryResult struct {
	DeliveryID     string
	TurnID         string
	SourceTurnID   string
	SessionID      string
	UserID         int64
	IntentID       string
	OpenLoopID     string
	Status         string
	FirstCommitted bool
	Items          []DeliveryItemResult
	DeliveredText  string
	// DeliveredContent is suitable for assistant history and contains [图片]
	// placeholders instead of local paths or OneBot CQ payloads.
	DeliveredContent string
	DeliveredCount   int
	FailedCount      int
	CancelledCount   int
	StartedAt        time.Time
	FinishedAt       time.Time
	Retryable        bool
	Error            string
}

type DeliveryRequest struct {
	TurnID       string
	SourceTurnID string
	SessionID    string
	UserID       int64
	IntentID     string
	OpenLoopID   string
	Reply        string
	Proactive    bool
	Cancel       <-chan struct{}
	EventStore   eventlog.Store
}

// ApplyDeliveryToIntent records the business outcome of a delivery without
// letting the scheduler decide whether an intent is complete. A partial,
// failed, or cancelled delivery remains pending so it can be retried or
// explicitly handled later.
func ApplyDeliveryToIntent(result DeliveryResult, at time.Time) error {
	if result.IntentID == "" {
		return nil
	}
	if at.IsZero() {
		at = result.FinishedAt
	}
	if at.IsZero() {
		at = time.Now()
	}
	status := intent.StatusPending
	if result.Status == DeliveryResultDelivered {
		status = intent.StatusCompleted
	}
	_, err := state.GetManager().IntentStore().Transition(result.IntentID, status, at)
	return err
}

// DeliverTurnReply wraps one delivery with session-level cancellation and
// interrupted-reply tracking. Callers that have a session should use this
// entry point instead of the compatibility SendMsg wrapper.
func DeliverTurnReply(ctx context.Context, conn *websocket.Conn, request DeliveryRequest) DeliveryResult {
	if ctx == nil {
		ctx = context.Background()
	}
	if request.TurnID == "" {
		request.TurnID = request.SessionID
	}
	deliveryCtx, deliveryCancel, _ := state.GetManager().BeginActiveDelivery(ctx, request.SessionID, request.TurnID, request.TurnID)
	defer func() {
		deliveryCancel()
		state.GetManager().EndActiveDelivery(request.SessionID, request.TurnID)
	}()

	result := DeliverReply(deliveryCtx, conn, request)
	if result.FirstCommitted {
		state.GetManager().MarkDeliveryCommitted(request.SessionID, request.TurnID)
	}
	updateInterruptedReply(request.SessionID, request.TurnID, request.Reply, result)
	return result
}

// deliveryDelayFn is a variable so callers/tests can replace the humanized
// inter-message delay. Production defaults to the historical 1-3 seconds.
var (
	deliveryDelayFn = func() time.Duration {
		return time.Duration(rand.Intn(2000)+1000) * time.Millisecond
	}
	deliveryDelayMu sync.RWMutex
)

func setDeliveryDelayFn(fn func() time.Duration) func() {
	deliveryDelayMu.Lock()
	previous := deliveryDelayFn
	if fn != nil {
		deliveryDelayFn = fn
	}
	deliveryDelayMu.Unlock()
	return func() {
		deliveryDelayMu.Lock()
		deliveryDelayFn = previous
		deliveryDelayMu.Unlock()
	}
}

func nextDeliveryDelay() time.Duration {
	deliveryDelayMu.RLock()
	fn := deliveryDelayFn
	deliveryDelayMu.RUnlock()
	if fn == nil {
		return 0
	}
	return fn()
}

// DeliverReply sends every parsed outbound unit in order and returns a result
// even when delivery is only partially successful or is cancelled.
func DeliverReply(ctx context.Context, conn *websocket.Conn, request DeliveryRequest) DeliveryResult {
	if ctx == nil {
		ctx = context.Background()
	}
	started := time.Now()
	items := ParseOutboundMessages(request.Reply)
	result := DeliveryResult{
		DeliveryID:   request.TurnID,
		TurnID:       request.TurnID,
		SourceTurnID: request.SourceTurnID,
		SessionID:    request.SessionID,
		UserID:       request.UserID,
		IntentID:     request.IntentID,
		OpenLoopID:   request.OpenLoopID,
		Items:        make([]DeliveryItemResult, len(items)),
		StartedAt:    started,
	}
	for i, item := range items {
		result.Items[i] = DeliveryItemResult{Index: item.Index, Kind: item.Kind, Status: DeliveryPending, AssetID: item.AssetID}
	}
	if len(items) == 0 {
		result.Status = DeliveryResultEmpty
		result.FinishedAt = time.Now()
		return result
	}

	for i, item := range items {
		if isDeliveryCancelled(ctx, request.Cancel) {
			markRemainingDeliveryItems(result.Items, i, DeliveryCancelled, "delivery cancelled")
			break
		}
		if err := waitBeforeDeliveryWithCancel(ctx, request.Cancel, nextDeliveryDelay()); err != nil {
			status := DeliveryCancelled
			if !isDeliveryCancelled(ctx, request.Cancel) {
				status = DeliveryFailed
			}
			markRemainingDeliveryItems(result.Items, i, status, err.Error())
			if status == DeliveryFailed && result.Error == "" {
				result.Error = err.Error()
			}
			break
		}

		itemResult := &result.Items[i]
		itemResult.Status = DeliverySending
		itemResult.StartedAt = time.Now()
		oneBotText := item.OneBotText
		if item.Kind == "image" {
			asset, err := assets.LookupImageAsset(item.AssetID)
			if err != nil {
				itemResult.Status = DeliveryFailed
				itemResult.Error = fmt.Sprintf("image asset %q: %v", item.AssetID, err)
				result.Error = itemResult.Error
				markRemainingDeliveryItems(result.Items, i+1, DeliverySkipped, "not attempted after delivery failure")
				break
			}
			fileValue, err := assets.ResolveImageAssetCQFile(asset)
			if err != nil {
				itemResult.Status = DeliveryFailed
				itemResult.Error = fmt.Sprintf("image asset %q: %v", item.AssetID, err)
				result.Error = itemResult.Error
				markRemainingDeliveryItems(result.Items, i+1, DeliverySkipped, "not attempted after delivery failure")
				break
			}
			oneBotText = fmt.Sprintf("[CQ:image,file=%s]", fileValue)
		}
		messageID, err := sendDeliveryItem(ctx, conn, request.UserID, oneBotText)
		if err != nil {
			if isDeliveryCancelled(ctx, request.Cancel) {
				itemResult.Status = DeliveryCancelled
				itemResult.Error = "delivery cancelled"
				markRemainingDeliveryItems(result.Items, i+1, DeliveryCancelled, "delivery cancelled")
			} else {
				itemResult.Status = DeliveryFailed
				itemResult.Error = err.Error()
				if result.Error == "" {
					result.Error = err.Error()
				}
				markRemainingDeliveryItems(result.Items, i+1, DeliverySkipped, "not attempted after delivery failure")
			}
			break
		}

		itemResult.Status = DeliveryDelivered
		itemResult.DeliveredAt = time.Now()
		itemResult.MessageID = messageID
		result.FirstCommitted = true
	}

	result.DeliveredCount, result.FailedCount, result.CancelledCount = countDeliveryItems(result.Items)
	result.DeliveredText, result.DeliveredContent = deliveredContent(result.Items, items)
	result.Status = deliveryResultStatus(result.Items, result.DeliveredCount, result.FailedCount, result.CancelledCount)
	result.Retryable = result.Status == DeliveryResultFailed || result.Status == DeliveryResultPartial
	result.FinishedAt = time.Now()
	return result
}

func sendDeliveryItem(ctx context.Context, conn *websocket.Conn, userID int64, message string) (int64, error) {
	if err := contextErr(ctx); err != nil {
		return 0, err
	}
	resp, err := connect.CallAPI(conn, "send_private_msg", model.UserMessageParams{User_id: userID, Message: message})
	if err != nil {
		utils.Error("send private message failed: %v", err)
		return 0, err
	}
	return parseMessageID(resp.Data), nil
}

// deliverItem sends one already-expanded outbound message. DeliverReply is
// responsible for pacing and aggregate state; this helper keeps the per-item
// behavior available for focused service tests and future retry paths.
func deliverItem(ctx context.Context, conn *websocket.Conn, userID int64, item OutboundMessage) DeliveryItemResult {
	result := DeliveryItemResult{Index: item.Index, Kind: item.Kind, AssetID: item.AssetID, Status: DeliverySending, StartedAt: time.Now()}
	message := item.OneBotText
	if item.Kind == "image" {
		asset, err := assets.LookupImageAsset(item.AssetID)
		if err == nil {
			var fileValue string
			fileValue, err = assets.ResolveImageAssetCQFile(asset)
			if err == nil {
				message = fmt.Sprintf("[CQ:image,file=%s]", fileValue)
			}
		}
		if err != nil {
			result.Status = DeliveryFailed
			result.Error = err.Error()
			return result
		}
	}
	messageID, err := sendDeliveryItem(ctx, conn, userID, message)
	if err != nil {
		if contextErr(ctx) != nil {
			result.Status = DeliveryCancelled
		} else {
			result.Status = DeliveryFailed
		}
		result.Error = err.Error()
		return result
	}
	result.Status = DeliveryDelivered
	result.MessageID = messageID
	result.DeliveredAt = time.Now()
	return result
}

// waitBeforeDelivery waits for the configured pacing delay and can be
// interrupted by context cancellation.
func waitBeforeDelivery(ctx context.Context, delay time.Duration) error {
	return waitBeforeDeliveryWithCancel(ctx, nil, delay)
}

func waitBeforeDeliveryWithCancel(ctx context.Context, cancel <-chan struct{}, delay time.Duration) error {
	if delay <= 0 {
		if err := contextErr(ctx); err != nil {
			return err
		}
		select {
		case <-cancel:
			return context.Canceled
		default:
			return nil
		}
	}
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-timer.C:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	case <-cancel:
		return context.Canceled
	}
}

func contextErr(ctx context.Context) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
		return nil
	}
}

func isDeliveryCancelled(ctx context.Context, cancel <-chan struct{}) bool {
	if contextErr(ctx) != nil {
		return true
	}
	select {
	case <-cancel:
		return true
	default:
		return false
	}
}

func markRemainingDeliveryItems(items []DeliveryItemResult, from int, status DeliveryStatus, reason string) {
	for i := from; i < len(items); i++ {
		if items[i].Status != DeliveryPending {
			continue
		}
		items[i].Status = status
		items[i].Error = reason
	}
}

func countDeliveryItems(items []DeliveryItemResult) (delivered, failed, cancelled int) {
	for _, item := range items {
		switch item.Status {
		case DeliveryDelivered:
			delivered++
		case DeliveryFailed:
			failed++
		case DeliveryCancelled:
			cancelled++
		}
	}
	return
}

func deliveryResultStatus(items []DeliveryItemResult, delivered, failed, cancelled int) string {
	if len(items) == 0 {
		return DeliveryResultEmpty
	}
	if delivered == len(items) {
		return DeliveryResultDelivered
	}
	if delivered > 0 {
		return DeliveryResultPartial
	}
	if cancelled > 0 && failed == 0 {
		return DeliveryResultCancelled
	}
	return DeliveryResultFailed
}

func deliveredContent(results []DeliveryItemResult, items []OutboundMessage) (string, string) {
	textParts := make([]string, 0, len(results))
	contentParts := make([]string, 0, len(results))
	for i, result := range results {
		if result.Status != DeliveryDelivered || i >= len(items) {
			continue
		}
		if items[i].Kind == "image" {
			contentParts = append(contentParts, "[图片]")
			continue
		}
		value := strings.TrimSpace(items[i].Content)
		if value != "" {
			textParts = append(textParts, value)
			contentParts = append(contentParts, value)
		}
	}
	return strings.Join(textParts, " "), strings.Join(contentParts, " ")
}

// OneBot message IDs are parsed by this helper for callers that provide an
// API response hook. It accepts both numeric and string encodings.
func parseMessageID(data json.RawMessage) int64 {
	var payload struct {
		MessageID json.RawMessage `json:"message_id"`
	}
	if json.Unmarshal(data, &payload) != nil || len(payload.MessageID) == 0 {
		return 0
	}
	var number int64
	if json.Unmarshal(payload.MessageID, &number) == nil {
		return number
	}
	var value string
	if json.Unmarshal(payload.MessageID, &value) == nil {
		number, _ = strconv.ParseInt(strings.TrimSpace(value), 10, 64)
	}
	return number
}

func updateInterruptedReply(sessionID, turnID, original string, result DeliveryResult) {
	if result.Status == DeliveryResultDelivered {
		state.GetManager().ClearInterruptedReply(sessionID, "resumed")
		return
	}
	if !result.FirstCommitted {
		return
	}

	items := ParseOutboundMessages(original)
	delivered := make([]string, 0)
	undelivered := make([]string, 0)
	for index, item := range items {
		if index >= len(result.Items) {
			break
		}
		value := item.Content
		if item.Kind == "image" {
			value = "[图片]"
		}
		if strings.TrimSpace(value) == "" {
			continue
		}
		if result.Items[index].Status == DeliveryDelivered {
			delivered = append(delivered, value)
		} else {
			undelivered = append(undelivered, value)
		}
	}
	if len(undelivered) == 0 {
		return
	}
	state.GetManager().SetInterruptedReply(sessionID, state.InterruptedReply{
		SourceTurnID:        turnID,
		DeliveredSegments:   delivered,
		UndeliveredSegments: undelivered,
		Summary:             fmt.Sprintf("上一轮回复已有 %d 段成功发送，后续内容未发送。未发送内容：%s", len(delivered), strings.Join(undelivered, " ")),
		Status:              "pending",
	})
}
