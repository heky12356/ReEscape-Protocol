package scheduler

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"strings"
	"time"

	domainintent "project-yume/internal/domain/intent"
	"project-yume/internal/eventlog"
	"project-yume/internal/state"
	"project-yume/internal/tools"
)

const (
	maxScheduleMetaKeys     = 10
	maxScheduleSummaryRunes = 120
	minScheduleLeadTime     = time.Minute
	defaultCancelDelay      = 24 * time.Hour
	defaultCancelResumeHour = 9
)

type UpdateProactiveScheduleTool struct{}

type updateProactiveScheduleInput struct {
	Action       string         `json:"action"`
	ScheduledAt  string         `json:"scheduled_at"`
	DelayMinutes int            `json:"delay_minutes"`
	Summary      string         `json:"summary"`
	Reason       string         `json:"reason"`
	Meta         map[string]any `json:"meta"`
}

func NewUpdateProactiveScheduleTool() *UpdateProactiveScheduleTool {
	return &UpdateProactiveScheduleTool{}
}

func (t *UpdateProactiveScheduleTool) Name() string {
	return "update_proactive_schedule"
}

func (t *UpdateProactiveScheduleTool) Description() string {
	return "设置、推迟或取消当前会话下一次主动触达计划；用于用户明确约定稍后继续、提醒、暂停主动联系等场景。"
}

func (t *UpdateProactiveScheduleTool) Schema() tools.Schema {
	return tools.Schema{
		Type: "object",
		Properties: map[string]tools.Property{
			"action": {
				Type:        "string",
				Enum:        []string{"set", "delay", "cancel"},
				Description: "操作类型：set 设置绝对时间，delay 推迟分钟数，cancel 取消或暂停近期主动触达。",
			},
			"scheduled_at": {
				Type:        "string",
				Description: "RFC3339 时间；action=set 时必填。",
			},
			"delay_minutes": {
				Type:        "integer",
				Description: "推迟分钟数；action=delay 时必填。",
			},
			"summary": {
				Type:        "string",
				Description: "简短计划摘要，例如“晚上九点继续聊项目计划”。",
			},
			"reason": {
				Type:        "string",
				Description: "设置原因，必须来自当前对话中的明确约定或用户请求。",
			},
			"meta": {
				Type:        "object",
				Description: "可选审计信息，例如 topic、source、scope。",
			},
		},
		Required:             []string{"action", "reason"},
		AdditionalProperties: false,
	}
}

func (t *UpdateProactiveScheduleTool) ReadOnly() bool {
	return false
}

func (t *UpdateProactiveScheduleTool) Execute(ctx context.Context, turn tools.TurnView, input json.RawMessage) (tools.ToolResult, error) {
	select {
	case <-ctx.Done():
		return tools.ToolResult{}, ctx.Err()
	default:
	}

	var args updateProactiveScheduleInput
	if len(input) > 0 {
		if err := json.Unmarshal(input, &args); err != nil {
			return tools.ToolResult{}, err
		}
	}

	args.Action = strings.TrimSpace(args.Action)
	args.ScheduledAt = strings.TrimSpace(args.ScheduledAt)
	args.Summary = strings.TrimSpace(args.Summary)
	args.Reason = strings.TrimSpace(args.Reason)
	if args.Reason == "" {
		return tools.ToolResult{}, fmt.Errorf("reason is required")
	}

	meta, err := validateScheduleMeta(args.Meta)
	if err != nil {
		return tools.ToolResult{}, err
	}

	referenceTime := turn.ReferenceTime()
	if referenceTime.IsZero() {
		referenceTime = time.Now()
	}
	minimum := referenceTime.Add(minScheduleLeadTime)

	next, err := resolveNextScheduleTime(turn.SessionID(), args, meta, referenceTime)
	if err != nil {
		return tools.ToolResult{}, err
	}
	if next.Before(minimum) {
		return tools.ToolResult{}, fmt.Errorf("scheduled_at must be at least %s", minimum.Format(time.RFC3339))
	}

	summary := normalizeToolSummary(args.Summary)
	if summary == "" && args.Action == "cancel" {
		summary = "用户取消近期主动触达"
	}
	intentSummary := summary
	if intentSummary == "" {
		intentSummary = "主动联系计划"
	}

	schedule := state.ProactiveSchedule{
		SessionID:       turn.SessionID(),
		NextScheduledAt: next,
		Summary:         summary,
		Reason:          args.Reason,
		Meta:            meta,
		Manual:          true,
		UpdatedAt:       time.Now(),
		UpdatedBy:       turn.Actor(),
	}
	intentStore := state.GetManager().IntentStore()
	if turn.UserID() == 0 {
		state.GetManager().SetProactiveSchedule(turn.SessionID(), schedule)
		updated := state.GetManager().GetProactiveSchedule(turn.SessionID())
		return tools.ToolResult{Content: fmt.Sprintf("proactive_schedule_updated action=%s next_scheduled_at=%s", args.Action, formatScheduleTime(updated.NextScheduledAt)), Data: scheduleData(updated), Mutated: true}, nil
	}
	if args.Action == "cancel" {
		if existingID := state.GetManager().GetProactiveSchedule(turn.SessionID()).IntentID; existingID != "" {
			if existing, ok := intentStore.Get(existingID); ok && existing.UserID == turn.UserID() {
				if existing.Status == domainintent.StatusPending || existing.Status == domainintent.StatusClaimed || existing.Status == domainintent.StatusDeferred {
					existing.DueAt = next
					existing.Status = domainintent.StatusDeferred
					existing.UpdatedAt = time.Now()
					_, _ = intentStore.Upsert(existing)
				}
			}
		}
	} else {
		existingID := state.GetManager().GetProactiveSchedule(turn.SessionID()).IntentID
		var scheduleIntent domainintent.Intent
		if existingID != "" {
			if existing, ok := intentStore.Get(existingID); ok && existing.UserID == turn.UserID() {
				if existing.Status == domainintent.StatusPending || existing.Status == domainintent.StatusDeferred || existing.Status == domainintent.StatusClaimed {
					scheduleIntent = existing
					scheduleIntent.DueAt = next
					scheduleIntent.Summary = summary
					scheduleIntent.Status = domainintent.StatusPending
					scheduleIntent.UpdatedAt = time.Now()
					if updated, updateErr := intentStore.Upsert(scheduleIntent); updateErr == nil {
						scheduleIntent = updated
					}
				}
			}
		}
		if scheduleIntent.ID == "" {
			scheduleIntent = domainintent.NewProactiveContact(turn.UserID(), turn.SessionID(), intentSummary, next)
			scheduleIntent.Source = "explicit"
			scheduleIntent.SourceTurnID = turn.RequestID()
			if created, createErr := intentStore.Upsert(scheduleIntent); createErr != nil {
				return tools.ToolResult{}, createErr
			} else {
				scheduleIntent = created
			}
		}
		schedule.IntentID = scheduleIntent.ID
	}
	state.GetManager().SetProactiveSchedule(turn.SessionID(), schedule)

	updated := state.GetManager().GetProactiveSchedule(turn.SessionID())
	data := scheduleData(updated)
	data["action"] = args.Action

	event := eventlog.Event{
		Type:      "proactive_schedule_updated",
		SessionID: turn.SessionID(),
		UserID:    turn.UserID(),
		Actor:     turn.Actor(),
		Tool:      t.Name(),
		Message:   summary,
		CreatedAt: time.Now(),
		Data: map[string]any{
			"action":            args.Action,
			"next_scheduled_at": formatScheduleTime(updated.NextScheduledAt),
			"reason":            args.Reason,
			"meta":              meta,
			"intent_id":         updated.IntentID,
		},
	}

	return tools.ToolResult{
		Content: fmt.Sprintf("proactive_schedule_updated action=%s next_scheduled_at=%s", args.Action, formatScheduleTime(updated.NextScheduledAt)),
		Data:    data,
		Events:  []eventlog.Event{event},
		Mutated: true,
	}, nil
}

func resolveNextScheduleTime(sessionID string, args updateProactiveScheduleInput, meta map[string]any, referenceTime time.Time) (time.Time, error) {
	switch args.Action {
	case "set":
		if args.ScheduledAt == "" {
			return time.Time{}, fmt.Errorf("scheduled_at is required for action=set")
		}
		parsed, err := time.Parse(time.RFC3339, args.ScheduledAt)
		if err != nil {
			return time.Time{}, fmt.Errorf("scheduled_at must be RFC3339: %w", err)
		}
		return parsed, nil
	case "delay":
		return resolveDelayScheduleTime(sessionID, args.DelayMinutes, referenceTime)
	case "cancel":
		return resolveCancelScheduleTime(meta, referenceTime), nil
	default:
		return time.Time{}, fmt.Errorf("action must be one of set, delay, cancel")
	}
}

func resolveDelayScheduleTime(sessionID string, delayMinutes int, referenceTime time.Time) (time.Time, error) {
	if delayMinutes <= 0 {
		return time.Time{}, fmt.Errorf("delay_minutes must be greater than 0 for action=delay")
	}
	base := state.GetManager().GetProactiveSchedule(sessionID).NextScheduledAt
	if base.IsZero() || base.Before(referenceTime) {
		base = referenceTime
	}
	return base.Add(time.Duration(delayMinutes) * time.Minute), nil
}

func resolveCancelScheduleTime(meta map[string]any, referenceTime time.Time) time.Time {
	if scope, ok := meta["scope"].(string); ok && strings.EqualFold(strings.TrimSpace(scope), "today") {
		year, month, day := referenceTime.Date()
		location := referenceTime.Location()
		return time.Date(year, month, day+1, defaultCancelResumeHour, 0, 0, 0, location)
	}
	return referenceTime.Add(defaultCancelDelay)
}

func validateScheduleMeta(meta map[string]any) (map[string]any, error) {
	if len(meta) == 0 {
		return nil, nil
	}
	if len(meta) > maxScheduleMetaKeys {
		return nil, fmt.Errorf("meta must contain at most %d keys", maxScheduleMetaKeys)
	}

	result := make(map[string]any, len(meta))
	for key, value := range meta {
		key = strings.TrimSpace(key)
		if key == "" {
			return nil, fmt.Errorf("meta keys must be non-empty")
		}

		normalized, err := normalizeScheduleMetaValue(value)
		if err != nil {
			return nil, fmt.Errorf("meta.%s: %w", key, err)
		}
		result[key] = normalized
	}
	return result, nil
}

func normalizeScheduleMetaValue(value any) (any, error) {
	switch typed := value.(type) {
	case string:
		return typed, nil
	case bool:
		return typed, nil
	case float64:
		if math.IsNaN(typed) || math.IsInf(typed, 0) {
			return nil, fmt.Errorf("number must be finite")
		}
		return typed, nil
	case int:
		return typed, nil
	case int64:
		return typed, nil
	case []string:
		return append([]string(nil), typed...), nil
	case []any:
		values := make([]string, 0, len(typed))
		for _, item := range typed {
			text, ok := item.(string)
			if !ok {
				return nil, fmt.Errorf("array values must be strings")
			}
			values = append(values, text)
		}
		return values, nil
	default:
		return nil, fmt.Errorf("value must be string, number, bool, or []string")
	}
}

func normalizeToolSummary(summary string) string {
	summary = strings.TrimSpace(summary)
	runes := []rune(summary)
	if len(runes) <= maxScheduleSummaryRunes {
		return summary
	}
	return string(runes[:maxScheduleSummaryRunes])
}
