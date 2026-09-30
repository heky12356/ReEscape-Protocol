package intent

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	domain "project-yume/internal/domain/intent"
	"project-yume/internal/domain/openloop"
	"project-yume/internal/eventlog"
	"project-yume/internal/state"
	"project-yume/internal/tools"
)

type UpdateIntentTool struct{}
type updateIntentInput struct {
	Action       string `json:"action"`
	IntentID     string `json:"intent_id"`
	Kind         string `json:"kind"`
	Summary      string `json:"summary"`
	ActionName   string `json:"intent_action"`
	DueAt        string `json:"due_at"`
	DelayMinutes int    `json:"delay_minutes"`
	Priority     int    `json:"priority"`
	OpenLoopID   string `json:"open_loop_id"`
}

func NewUpdateIntentTool() *UpdateIntentTool { return &UpdateIntentTool{} }
func (t *UpdateIntentTool) Name() string     { return "update_intent" }
func (t *UpdateIntentTool) Description() string {
	return "创建或更新未来要执行的后续动作；支持创建、延期、取消和完成 Intent，并可绑定 OpenLoop。"
}
func (t *UpdateIntentTool) Schema() tools.Schema {
	return tools.Schema{Type: "object", Properties: map[string]tools.Property{
		"action":        {Type: "string", Enum: []string{"create", "update", "defer", "cancel", "complete"}},
		"intent_id":     {Type: "string"},
		"kind":          {Type: "string", Enum: []string{"proactive_contact", "follow_up", "delivery_retry", "check_open_loop"}},
		"summary":       {Type: "string"},
		"intent_action": {Type: "string", Enum: []string{"proactive_contact", "follow_up", "delivery_retry", "check_open_loop"}},
		"due_at":        {Type: "string", Description: "RFC3339 时间。"},
		"delay_minutes": {Type: "integer"},
		"priority":      {Type: "integer"},
		"open_loop_id":  {Type: "string"},
	}, Required: []string{"action"}, AdditionalProperties: false}
}
func (t *UpdateIntentTool) ReadOnly() bool { return false }

func (t *UpdateIntentTool) Execute(ctx context.Context, turn tools.TurnView, input json.RawMessage) (tools.ToolResult, error) {
	select {
	case <-ctx.Done():
		return tools.ToolResult{}, ctx.Err()
	default:
	}
	var args updateIntentInput
	if err := json.Unmarshal(input, &args); err != nil {
		return tools.ToolResult{}, err
	}
	args.Action, args.IntentID, args.Kind, args.Summary, args.ActionName, args.DueAt, args.OpenLoopID = strings.TrimSpace(args.Action), strings.TrimSpace(args.IntentID), strings.TrimSpace(args.Kind), strings.TrimSpace(args.Summary), strings.TrimSpace(args.ActionName), strings.TrimSpace(args.DueAt), strings.TrimSpace(args.OpenLoopID)
	sm := state.GetManager()
	now := turn.ReferenceTime()
	if now.IsZero() {
		now = time.Now()
	}
	var item domain.Intent
	var err error
	switch args.Action {
	case "create":
		if args.Summary == "" || args.DueAt == "" {
			return tools.ToolResult{}, fmt.Errorf("summary and due_at are required")
		}
		due, parseErr := time.Parse(time.RFC3339, args.DueAt)
		if parseErr != nil {
			return tools.ToolResult{}, fmt.Errorf("due_at must be RFC3339: %w", parseErr)
		}
		kind := domain.Kind(args.Kind)
		if kind == "" {
			kind = domain.KindFollowUp
		}
		if !validKind(kind) {
			return tools.ToolResult{}, fmt.Errorf("unsupported intent kind: %s", kind)
		}
		if args.OpenLoopID != "" {
			loop, ok := sm.GetOpenLoop(args.OpenLoopID)
			if !ok || loop.UserID != turn.UserID() {
				return tools.ToolResult{}, fmt.Errorf("open_loop_id is invalid")
			}
		}
		item = domain.NewProactiveContact(turn.UserID(), turn.SessionID(), args.Summary, due)
		item.Kind, item.Action, item.Priority, item.OpenLoopID, item.Source, item.SourceTurnID = kind, args.ActionName, args.Priority, args.OpenLoopID, "explicit", turn.RequestID()
		if item.Action == "" {
			item.Action = string(kind)
		}
		item.CreatedAt, item.UpdatedAt = now, now
		item, err = sm.UpsertIntent(item)
	case "update", "defer", "cancel", "complete":
		if args.IntentID == "" {
			return tools.ToolResult{}, fmt.Errorf("intent_id is required")
		}
		item, ok := sm.GetIntent(args.IntentID)
		if !ok || item.UserID != turn.UserID() {
			return tools.ToolResult{}, fmt.Errorf("intent not found")
		}
		if args.Action == "update" {
			if args.Summary != "" {
				item.Summary = args.Summary
			}
			if args.Priority != 0 {
				item.Priority = args.Priority
			}
			if args.DueAt != "" {
				item.DueAt, err = time.Parse(time.RFC3339, args.DueAt)
				if err != nil {
					return tools.ToolResult{}, fmt.Errorf("due_at must be RFC3339: %w", err)
				}
			}
			if args.OpenLoopID != "" {
				loop, ok := sm.GetOpenLoop(args.OpenLoopID)
				if !ok || loop.UserID != turn.UserID() {
					return tools.ToolResult{}, fmt.Errorf("open_loop_id is invalid")
				}
				item.OpenLoopID = args.OpenLoopID
			}
			item.UpdatedAt = now
			item, err = sm.UpsertIntent(item)
		} else {
			if args.Action == "defer" {
				if args.DelayMinutes <= 0 {
					return tools.ToolResult{}, fmt.Errorf("delay_minutes must be greater than 0")
				}
				item.DueAt = now.Add(time.Duration(args.DelayMinutes) * time.Minute)
				item, err = sm.TransitionIntent(item.ID, domain.StatusDeferred, now)
				if err == nil {
					item.Status = domain.StatusDeferred
					item.DueAt = now.Add(time.Duration(args.DelayMinutes) * time.Minute)
					item, err = sm.UpsertIntent(item)
				}
			} else {
				status := map[string]domain.Status{"cancel": domain.StatusCancelled, "complete": domain.StatusCompleted}[args.Action]
				item, err = sm.TransitionIntent(item.ID, status, now)
			}
		}
	default:
		return tools.ToolResult{}, fmt.Errorf("action must be create, update, defer, cancel or complete")
	}
	if err != nil {
		return tools.ToolResult{}, err
	}
	eventType := map[string]string{"create": "intent_created", "update": "intent_updated", "defer": "intent_deferred", "cancel": "intent_cancelled", "complete": "intent_completed"}[args.Action]
	event := eventlog.Event{Type: eventType, SessionID: turn.SessionID(), UserID: turn.UserID(), Actor: turn.Actor(), Tool: t.Name(), Message: item.Summary, CreatedAt: time.Now(), Data: map[string]any{"intent_id": item.ID, "status": item.Status, "open_loop_id": item.OpenLoopID}}
	return tools.ToolResult{Content: fmt.Sprintf("intent_%s id=%s status=%s", args.Action, item.ID, item.Status), Data: item, Events: []eventlog.Event{event}, Mutated: true}, nil
}

func validKind(kind domain.Kind) bool {
	switch kind {
	case domain.KindProactiveContact, domain.KindFollowUp, domain.KindDeliveryRetry, domain.KindCheckOpenLoop:
		return true
	default:
		return false
	}
}
func stateIntentStore() domain.Store { return state.GetManager().IntentStore() }

var _ openloop.Store = (*state.StateOpenLoopStore)(nil)
