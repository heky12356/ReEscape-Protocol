package intent

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"project-yume/internal/domain/openloop"
	"project-yume/internal/eventlog"
	"project-yume/internal/state"
	"project-yume/internal/tools"
)

type UpdateOpenLoopTool struct{}
type updateOpenLoopInput struct {
	Action      string `json:"action"`
	OpenLoopID  string `json:"open_loop_id"`
	Kind        string `json:"kind"`
	Description string `json:"description"`
	DueAt       string `json:"due_at"`
}

func NewUpdateOpenLoopTool() *UpdateOpenLoopTool { return &UpdateOpenLoopTool{} }
func (t *UpdateOpenLoopTool) Name() string       { return "update_open_loop" }
func (t *UpdateOpenLoopTool) Description() string {
	return "创建、关闭或延期当前会话中的未闭合事项；没有明确时间的后续承诺应记录为 OpenLoop。"
}
func (t *UpdateOpenLoopTool) Schema() tools.Schema {
	return tools.Schema{Type: "object", Properties: map[string]tools.Property{
		"action": {Type: "string", Enum: []string{"create", "resolve", "defer"}}, "open_loop_id": {Type: "string"},
		"kind": {Type: "string", Enum: []string{"unanswered_question", "unresolved_problem", "assistant_follow_up", "pending_user_thread"}}, "description": {Type: "string"}, "due_at": {Type: "string"},
	}, Required: []string{"action"}, AdditionalProperties: false}
}
func (t *UpdateOpenLoopTool) ReadOnly() bool { return false }
func (t *UpdateOpenLoopTool) Execute(ctx context.Context, turn tools.TurnView, input json.RawMessage) (tools.ToolResult, error) {
	select {
	case <-ctx.Done():
		return tools.ToolResult{}, ctx.Err()
	default:
	}
	var args updateOpenLoopInput
	if err := json.Unmarshal(input, &args); err != nil {
		return tools.ToolResult{}, err
	}
	args.Action, args.OpenLoopID, args.Kind, args.Description, args.DueAt = strings.TrimSpace(args.Action), strings.TrimSpace(args.OpenLoopID), strings.TrimSpace(args.Kind), strings.TrimSpace(args.Description), strings.TrimSpace(args.DueAt)
	now := turn.ReferenceTime()
	if now.IsZero() {
		now = time.Now()
	}
	store := state.GetManager().OpenLoopStore()
	var item openloop.OpenLoop
	var err error
	switch args.Action {
	case "create":
		if args.Description == "" {
			return tools.ToolResult{}, fmt.Errorf("description is required")
		}
		kind := args.Kind
		if kind == "" {
			kind = string(openloop.KindPendingThread)
		}
		item = openloop.NewExplicit(turn.UserID(), turn.SessionID(), kind, args.Description, turn.RequestID(), now)
		if args.DueAt != "" {
			item.DueAt, err = time.Parse(time.RFC3339, args.DueAt)
			if err != nil {
				return tools.ToolResult{}, fmt.Errorf("due_at must be RFC3339: %w", err)
			}
		}
		item, err = store.Upsert(item)
	case "resolve":
		if args.OpenLoopID == "" {
			return tools.ToolResult{}, fmt.Errorf("open_loop_id is required")
		}
		item, ok := store.Get(args.OpenLoopID)
		if !ok || item.UserID != turn.UserID() {
			return tools.ToolResult{}, fmt.Errorf("open loop not found")
		}
		item, err = store.Close(item.ID, turn.Actor(), now)
	case "defer":
		if args.OpenLoopID == "" || args.DueAt == "" {
			return tools.ToolResult{}, fmt.Errorf("open_loop_id and due_at are required")
		}
		due, parseErr := time.Parse(time.RFC3339, args.DueAt)
		if parseErr != nil {
			return tools.ToolResult{}, fmt.Errorf("due_at must be RFC3339: %w", parseErr)
		}
		item, ok := store.Get(args.OpenLoopID)
		if !ok || item.UserID != turn.UserID() {
			return tools.ToolResult{}, fmt.Errorf("open loop not found")
		}
		item, err = store.Defer(item.ID, due, turn.Actor(), now)
	default:
		return tools.ToolResult{}, fmt.Errorf("action must be create, resolve or defer")
	}
	if err != nil {
		return tools.ToolResult{}, err
	}
	eventType := map[string]string{"create": "open_loop_created", "resolve": "open_loop_resolved", "defer": "open_loop_deferred"}[args.Action]
	event := eventlog.Event{Type: eventType, SessionID: turn.SessionID(), UserID: turn.UserID(), Actor: turn.Actor(), Tool: t.Name(), Message: item.Description, CreatedAt: time.Now(), Data: map[string]any{"open_loop_id": item.ID, "status": item.Status, "intent_id": item.IntentID}}
	return tools.ToolResult{Content: fmt.Sprintf("open_loop_%s id=%s status=%s", args.Action, item.ID, item.Status), Data: item, Events: []eventlog.Event{event}, Mutated: true}, nil
}
