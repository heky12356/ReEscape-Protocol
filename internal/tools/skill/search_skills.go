package skill

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	skillpkg "project-yume/internal/skill"
	"project-yume/internal/tools"
)

type SearchSkillsTool struct{}

type searchSkillsInput struct {
	Query  string `json:"query"`
	Limit  int    `json:"limit"`
	Reason string `json:"reason"`
}

func NewSearchSkillsTool() *SearchSkillsTool {
	return &SearchSkillsTool{}
}

func (t *SearchSkillsTool) Name() string {
	return "search_skills"
}

func (t *SearchSkillsTool) Description() string {
	return "按当前任务搜索可能相关的标准 skill 包，只返回名称、描述和匹配原因。"
}

func (t *SearchSkillsTool) Schema() tools.Schema {
	return tools.Schema{
		Type: "object",
		Properties: map[string]tools.Property{
			"query": {
				Type:        "string",
				Description: "当前任务或用户消息关键词。",
			},
			"limit": {
				Type:        "integer",
				Description: "最多返回多少个 skill，默认 3，最大 8。",
			},
			"reason": {
				Type:        "string",
				Description: "为什么需要搜索 skill；用于审计，不要使用角色口吻。",
			},
		},
		Required:             []string{"query", "reason"},
		AdditionalProperties: false,
	}
}

func (t *SearchSkillsTool) ReadOnly() bool {
	return true
}

func (t *SearchSkillsTool) Execute(ctx context.Context, turn tools.TurnView, input json.RawMessage) (tools.ToolResult, error) {
	select {
	case <-ctx.Done():
		return tools.ToolResult{}, ctx.Err()
	default:
	}

	var args searchSkillsInput
	if len(input) > 0 {
		if err := json.Unmarshal(input, &args); err != nil {
			return tools.ToolResult{}, err
		}
	}
	args.Query = strings.TrimSpace(args.Query)
	args.Reason = strings.TrimSpace(args.Reason)
	if args.Query == "" {
		return tools.ToolResult{}, fmt.Errorf("query is required")
	}
	if args.Reason == "" {
		return tools.ToolResult{}, fmt.Errorf("reason is required")
	}
	if args.Limit <= 0 {
		args.Limit = 3
	}
	if args.Limit > 8 {
		args.Limit = 8
	}

	matches := skillpkg.GetManager().Search(args.Query, args.Limit)
	results := make([]map[string]any, 0, len(matches))
	lines := []string{"【Skill Search Results】"}
	for _, match := range matches {
		results = append(results, map[string]any{
			"name":           match.Name,
			"description":    match.Description,
			"score":          match.Score,
			"confidence":     match.Confidence,
			"reason":         match.Reason,
			"suggested_read": match.Score >= skillpkg.DefaultCandidateMinScore,
		})
		lines = append(lines, fmt.Sprintf(
			"- %s: score=%d confidence=%.2f reason=%s",
			match.Name,
			match.Score,
			match.Confidence,
			match.Reason,
		))
	}
	if len(matches) == 0 {
		lines = append(lines, "未找到相关 skill。")
	}
	payload := map[string]any{"matches": results}
	return tools.ToolResult{
		Content: strings.Join(lines, "\n"),
		Data:    payload,
	}, nil
}
