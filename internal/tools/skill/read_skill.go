package skill

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"project-yume/internal/eventlog"
	skillpkg "project-yume/internal/skill"
	"project-yume/internal/tools"
)

type ReadSkillTool struct{}

type readSkillInput struct {
	Name   string `json:"name"`
	Reason string `json:"reason"`
}

func NewReadSkillTool() *ReadSkillTool {
	return &ReadSkillTool{}
}

func (t *ReadSkillTool) Name() string {
	return "read_skill"
}

func (t *ReadSkillTool) Description() string {
	return "读取指定 skill 的完整 SKILL.md 正文和只读元数据，不读取扩展资源。"
}

func (t *ReadSkillTool) Schema() tools.Schema {
	return tools.Schema{
		Type: "object",
		Properties: map[string]tools.Property{
			"name": {
				Type:        "string",
				Description: "skill frontmatter 中的 name。",
			},
			"reason": {
				Type:        "string",
				Description: "为什么需要读取完整 skill；用于审计，不要使用角色口吻。",
			},
		},
		Required:             []string{"name"},
		AdditionalProperties: false,
	}
}

func (t *ReadSkillTool) ReadOnly() bool {
	return true
}

func (t *ReadSkillTool) Execute(ctx context.Context, turn tools.TurnView, input json.RawMessage) (tools.ToolResult, error) {
	select {
	case <-ctx.Done():
		return tools.ToolResult{}, ctx.Err()
	default:
	}

	var args readSkillInput
	if len(input) > 0 {
		if err := json.Unmarshal(input, &args); err != nil {
			return tools.ToolResult{}, err
		}
	}
	args.Name = strings.TrimSpace(args.Name)
	args.Reason = strings.TrimSpace(args.Reason)
	if args.Name == "" {
		return tools.ToolResult{}, fmt.Errorf("name is required")
	}

	pkg, ok := skillpkg.GetManager().Get(args.Name)
	if !ok {
		return tools.ToolResult{}, fmt.Errorf("skill not found: %s", args.Name)
	}
	resources, err := skillpkg.ListResources(pkg)
	if err != nil {
		return tools.ToolResult{}, err
	}
	alreadyLoaded := false
	if tracker, ok := turn.(interface {
		IsSkillLoaded(string) bool
		MarkSkillRead(string)
	}); ok {
		alreadyLoaded = tracker.IsSkillLoaded(args.Name)
		tracker.MarkSkillRead(args.Name)
	}
	payload := map[string]any{
		"name":        pkg.Name,
		"description": pkg.Description,
		"body":        pkg.Body,
		"resources":   resources,
	}
	content := formatReadSkillContent(pkg, resources, alreadyLoaded)
	result := tools.ToolResult{
		Content: content,
		Data:    payload,
	}
	if !alreadyLoaded {
		result.Events = append(result.Events, skillToolEvent(turn, "skill_model_selected", map[string]any{
			"skill":  pkg.Name,
			"reason": args.Reason,
		}))
	}
	return result, nil
}

func formatReadSkillContent(pkg skillpkg.Package, resources []string, alreadyLoaded bool) string {
	lines := []string{
		fmt.Sprintf("【Skill: %s】", pkg.Name),
		"description: " + pkg.Description,
	}
	if alreadyLoaded {
		lines = append(lines, "该 skill 已在当前 turn 加载，不重复返回正文。")
	} else {
		lines = append(lines, "", strings.TrimSpace(pkg.Body))
	}
	if len(resources) > 0 {
		lines = append(lines, "", "【Available Resources】")
		for _, resource := range resources {
			lines = append(lines, "- "+resource)
		}
	}
	return strings.TrimSpace(strings.Join(lines, "\n"))
}

func skillToolEvent(turn tools.TurnView, eventType string, data map[string]any) eventlog.Event {
	event := eventlog.Event{
		Type:      eventType,
		Actor:     "runtime",
		Data:      data,
		CreatedAt: time.Now(),
	}
	if turn != nil {
		event.SessionID = turn.SessionID()
		event.UserID = turn.UserID()
	}
	return event
}
