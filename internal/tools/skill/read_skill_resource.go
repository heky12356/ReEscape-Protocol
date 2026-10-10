package skill

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"project-yume/internal/config"
	"project-yume/internal/eventlog"
	skillpkg "project-yume/internal/skill"
	"project-yume/internal/tools"
)

type ReadSkillResourceTool struct {
	maxBytes int
}

type readSkillResourceInput struct {
	Name   string `json:"name"`
	Path   string `json:"path"`
	Reason string `json:"reason"`
}

func NewReadSkillResourceTool(maxBytes ...int) *ReadSkillResourceTool {
	limit := skillpkg.DefaultResourceMaxBytes
	if len(maxBytes) > 0 && maxBytes[0] > 0 {
		limit = maxBytes[0]
	}
	return &ReadSkillResourceTool{maxBytes: limit}
}

func (t *ReadSkillResourceTool) Name() string {
	return "read_skill_resource"
}

func (t *ReadSkillResourceTool) Source() string { return "skill" }

func (t *ReadSkillResourceTool) Description() string {
	return "读取指定 skill root 内 references、assets 或 scripts 中的文本资源；不会执行脚本。"
}

func (t *ReadSkillResourceTool) Schema() tools.Schema {
	return tools.Schema{
		Type: "object",
		Properties: map[string]tools.Property{
			"name": {
				Type:        "string",
				Description: "skill frontmatter 中的 name。",
			},
			"path": {
				Type:        "string",
				Description: "相对于 skill root 的文本资源路径，例如 references/examples.md。",
			},
			"reason": {
				Type:        "string",
				Description: "为什么需要读取资源；用于审计，不要使用角色口吻。",
			},
		},
		Required:             []string{"name", "path", "reason"},
		AdditionalProperties: false,
	}
}

func (t *ReadSkillResourceTool) ReadOnly() bool {
	return true
}

func (t *ReadSkillResourceTool) Execute(ctx context.Context, turn tools.TurnView, input json.RawMessage) (tools.ToolResult, error) {
	select {
	case <-ctx.Done():
		return tools.ToolResult{}, ctx.Err()
	default:
	}

	var args readSkillResourceInput
	if len(input) > 0 {
		if err := json.Unmarshal(input, &args); err != nil {
			return tools.ToolResult{}, err
		}
	}
	args.Name = strings.TrimSpace(args.Name)
	args.Path = strings.TrimSpace(args.Path)
	args.Reason = strings.TrimSpace(args.Reason)
	if args.Name == "" {
		return tools.ToolResult{}, fmt.Errorf("name is required")
	}
	if args.Path == "" {
		return tools.ToolResult{}, fmt.Errorf("path is required")
	}
	if args.Reason == "" {
		return tools.ToolResult{}, fmt.Errorf("reason is required")
	}
	if access, ok := turn.(interface {
		HasSkillAccess(string) bool
	}); ok && !access.HasSkillAccess(args.Name) {
		return tools.ToolResult{}, fmt.Errorf("skill must be activated or read before accessing resources: %s", args.Name)
	}

	maxBytes := t.maxBytes
	if cfg := config.GetConfig(); cfg != nil && cfg.SkillResourceMaxBytes > 0 {
		maxBytes = cfg.SkillResourceMaxBytes
	}
	content, err := skillpkg.GetManager().ReadResource(args.Name, args.Path, maxBytes)
	if err != nil {
		return tools.ToolResult{}, err
	}
	result := map[string]any{
		"name":         args.Name,
		"path":         args.Path,
		"content":      string(content),
		"scripts_note": "脚本内容仅供理解 skill，不会被执行。",
	}
	data, err := json.Marshal(result)
	if err != nil {
		return tools.ToolResult{}, err
	}
	return tools.ToolResult{
		Content: string(data),
		Data:    result,
		Events: []eventlog.Event{skillToolEvent(turn, "skill_resource_read", map[string]any{
			"skill": args.Name,
			"path":  args.Path,
		})},
	}, nil
}
