package memory

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"project-yume/internal/eventlog"
	domainmemory "project-yume/internal/memory"
	"project-yume/internal/tools"
)

type RememberFactTool struct{}

type rememberFactInput struct {
	Predicate     string   `json:"predicate"`
	Object        string   `json:"object"`
	Summary       string   `json:"summary"`
	Tags          []string `json:"tags"`
	Confidence    float64  `json:"confidence"`
	SourceMessage string   `json:"source_message"`
	ExpiresAt     string   `json:"expires_at"`
}

func NewRememberFactTool() *RememberFactTool {
	return &RememberFactTool{}
}

func (t *RememberFactTool) Name() string {
	return "remember_fact"
}

func (t *RememberFactTool) Description() string {
	return "写入或确认当前用户明确表达的长期事实记忆，例如名字、身份、地点、重要关系、稳定偏好、计划；不要用于一次性情绪或推测。"
}

func (t *RememberFactTool) Schema() tools.Schema {
	return tools.Schema{
		Type: "object",
		Properties: map[string]tools.Property{
			"predicate": {
				Type:        "string",
				Description: "事实谓词，例如 name、likes、location、current_plan。",
			},
			"object": {
				Type:        "string",
				Description: "事实对象，例如名字、偏好对象、地点或计划内容。",
			},
			"summary": {
				Type:        "string",
				Description: "给人类和模型阅读的简短事实摘要。",
			},
			"tags": {
				Type:        "array",
				Description: "检索标签。",
				Items: &tools.Property{
					Type: "string",
				},
			},
			"confidence": {
				Type:        "number",
				Description: "0 到 1 的置信度；不确定时不要写入，或使用较低置信度。",
			},
			"source_message": {
				Type:        "string",
				Description: "触发该事实的原始用户消息；为空时使用当前消息。",
			},
			"expires_at": {
				Type:        "string",
				Description: "可选 RFC3339 过期时间，适合今天、明天或近期计划类事实。",
			},
		},
		Required:             []string{"predicate", "object", "summary"},
		AdditionalProperties: false,
	}
}

func (t *RememberFactTool) ReadOnly() bool {
	return false
}

func (t *RememberFactTool) Execute(ctx context.Context, turn tools.TurnView, input json.RawMessage) (tools.ToolResult, error) {
	select {
	case <-ctx.Done():
		return tools.ToolResult{}, ctx.Err()
	default:
	}

	var args rememberFactInput
	if len(input) > 0 {
		if err := json.Unmarshal(input, &args); err != nil {
			return tools.ToolResult{}, err
		}
	}

	args.Predicate = strings.TrimSpace(args.Predicate)
	args.Object = strings.TrimSpace(args.Object)
	args.Summary = strings.TrimSpace(args.Summary)
	args.SourceMessage = strings.TrimSpace(args.SourceMessage)
	if args.Predicate == "" || args.Object == "" || args.Summary == "" {
		return tools.ToolResult{}, fmt.Errorf("predicate, object and summary are required")
	}
	if args.SourceMessage == "" {
		args.SourceMessage = turn.Message()
	}

	var expiresAt *time.Time
	if strings.TrimSpace(args.ExpiresAt) != "" {
		parsed, err := time.Parse(time.RFC3339, strings.TrimSpace(args.ExpiresAt))
		if err != nil {
			return tools.ToolResult{}, fmt.Errorf("expires_at must be RFC3339: %w", err)
		}
		expiresAt = &parsed
	}

	confidence := args.Confidence
	if confidence <= 0 {
		confidence = 0.7
	}

	fact := domainmemory.FactMemory{
		Predicate:     args.Predicate,
		Object:        args.Object,
		Summary:       args.Summary,
		Tags:          args.Tags,
		Confidence:    confidence,
		SourceMessage: args.SourceMessage,
		Status:        domainmemory.FactStatusActive,
		ExpiresAt:     expiresAt,
	}
	domainmemory.GetFactManager().UpsertFacts(turn.UserID(), turn.SessionID(), []domainmemory.FactMemory{fact})

	event := eventlog.Event{
		Type:      "memory_fact_remembered",
		SessionID: turn.SessionID(),
		UserID:    turn.UserID(),
		Actor:     turn.Actor(),
		Tool:      t.Name(),
		Message:   args.Summary,
		Data: map[string]any{
			"predicate":  args.Predicate,
			"object":     args.Object,
			"tags":       args.Tags,
			"confidence": confidence,
		},
	}

	return tools.ToolResult{
		Content: fmt.Sprintf("remembered fact: %s=%s", args.Predicate, args.Object),
		Data:    fact,
		Events:  []eventlog.Event{event},
		Mutated: true,
	}, nil
}
