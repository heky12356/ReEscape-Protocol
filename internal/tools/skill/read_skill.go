package skill

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

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
		Required:             []string{"name", "reason"},
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
	if args.Reason == "" {
		return tools.ToolResult{}, fmt.Errorf("reason is required")
	}

	pkg, ok := skillpkg.GetManager().Get(args.Name)
	if !ok {
		return tools.ToolResult{}, fmt.Errorf("skill not found: %s", args.Name)
	}
	resources, err := skillpkg.ListResources(pkg)
	if err != nil {
		return tools.ToolResult{}, err
	}
	payload := map[string]any{
		"name":        pkg.Name,
		"description": pkg.Description,
		"body":        pkg.Body,
		"resources":   resources,
	}
	content, err := json.Marshal(payload)
	if err != nil {
		return tools.ToolResult{}, err
	}
	return tools.ToolResult{
		Content: string(content),
		Data:    payload,
	}, nil
}
