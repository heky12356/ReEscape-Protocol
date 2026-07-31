package memory

import (
	"context"
	"encoding/json"
	"strings"

	"project-yume/internal/service"
	"project-yume/internal/tools"
)

type GetMemoryContextTool struct{}

type getMemoryContextInput struct {
	Query string `json:"query"`
}

func NewGetMemoryContextTool() *GetMemoryContextTool {
	return &GetMemoryContextTool{}
}

func (t *GetMemoryContextTool) Name() string {
	return "get_memory_context"
}

func (t *GetMemoryContextTool) Description() string {
	return "按当前消息或指定 query 读取短期上下文、用户画像、事实记忆和情感模式。"
}

func (t *GetMemoryContextTool) Schema() tools.Schema {
	return tools.Schema{
		Type: "object",
		Properties: map[string]tools.Property{
			"query": {
				Type:        "string",
				Description: "用于检索相关事实记忆的查询文本；为空时使用当前用户消息。",
			},
		},
		AdditionalProperties: false,
	}
}

func (t *GetMemoryContextTool) ReadOnly() bool {
	return true
}

func (t *GetMemoryContextTool) Execute(ctx context.Context, turn tools.TurnView, input json.RawMessage) (tools.ToolResult, error) {
	select {
	case <-ctx.Done():
		return tools.ToolResult{}, ctx.Err()
	default:
	}

	var args getMemoryContextInput
	if len(input) > 0 {
		if err := json.Unmarshal(input, &args); err != nil {
			return tools.ToolResult{}, err
		}
	}
	query := strings.TrimSpace(args.Query)
	if query == "" {
		query = turn.Message()
	}

	promptMemory := service.BuildPromptMemory(turn.UserID(), turn.SessionID(), query)
	content := service.FormatPromptMemory(promptMemory)
	if strings.TrimSpace(content) == "" {
		content = "未找到可用记忆上下文。"
	}

	return tools.ToolResult{
		Content: content,
		Data:    promptMemory,
	}, nil
}
