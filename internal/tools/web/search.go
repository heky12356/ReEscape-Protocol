package web

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"project-yume/internal/tools"
	"project-yume/internal/webaccess"
)

const (
	defaultSearchMaxResults = 5
	hardSearchMaxResults    = 10
	maxSearchQueryLength    = 300
)

type SearchTool struct {
	client            webaccess.SearchClient
	defaultMaxResults int
}

type searchInput struct {
	Query      string `json:"query"`
	Reason     string `json:"reason"`
	MaxResults int    `json:"max_results"`
	Recency    string `json:"recency"`
}

func NewSearchTool(client webaccess.SearchClient, defaultMaxResults int) *SearchTool {
	if defaultMaxResults <= 0 {
		defaultMaxResults = defaultSearchMaxResults
	}
	if defaultMaxResults > hardSearchMaxResults {
		defaultMaxResults = hardSearchMaxResults
	}
	return &SearchTool{
		client:            client,
		defaultMaxResults: defaultMaxResults,
	}
}

func (t *SearchTool) Name() string {
	return "web_search"
}

func (t *SearchTool) Source() string { return "web" }

func (t *SearchTool) Description() string {
	return "搜索外部网页信息，适合最新消息、陌生实体、近期事件、法规、价格、公告或需要来源核实时使用。"
}

func (t *SearchTool) Schema() tools.Schema {
	return tools.Schema{
		Type: "object",
		Properties: map[string]tools.Property{
			"query": {
				Type:        "string",
				Description: "搜索关键词，最多 300 字符。",
			},
			"reason": {
				Type:        "string",
				Description: "为什么当前问题需要搜索；用于审计，不要使用角色口吻。",
			},
			"max_results": {
				Type:        "integer",
				Description: "最多返回多少条结果，默认使用系统配置，最大 10。",
			},
			"recency": {
				Type:        "string",
				Description: "可选时间范围。",
				Enum:        []string{"any", "day", "week", "month", "year"},
			},
		},
		Required:             []string{"query", "reason"},
		AdditionalProperties: false,
	}
}

func (t *SearchTool) ReadOnly() bool {
	return true
}

func (t *SearchTool) Execute(ctx context.Context, turn tools.TurnView, input json.RawMessage) (tools.ToolResult, error) {
	select {
	case <-ctx.Done():
		return tools.ToolResult{}, ctx.Err()
	default:
	}
	if t.client == nil {
		return tools.ToolResult{}, fmt.Errorf("web search client is not configured")
	}

	var args searchInput
	if len(input) > 0 {
		if err := json.Unmarshal(input, &args); err != nil {
			return tools.ToolResult{}, err
		}
	}
	args.Query = strings.TrimSpace(args.Query)
	args.Reason = strings.TrimSpace(args.Reason)
	args.Recency = strings.ToLower(strings.TrimSpace(args.Recency))
	if args.Query == "" {
		return tools.ToolResult{}, fmt.Errorf("query is required")
	}
	if len([]rune(args.Query)) > maxSearchQueryLength {
		return tools.ToolResult{}, fmt.Errorf("query must be at most %d characters", maxSearchQueryLength)
	}
	if args.Reason == "" {
		return tools.ToolResult{}, fmt.Errorf("reason is required")
	}
	if args.Recency == "" {
		args.Recency = "any"
	}
	switch args.Recency {
	case "any", "day", "week", "month", "year":
	default:
		return tools.ToolResult{}, fmt.Errorf("recency must be one of any/day/week/month/year")
	}

	maxResults := args.MaxResults
	if maxResults <= 0 {
		maxResults = t.defaultMaxResults
	}
	if maxResults > hardSearchMaxResults {
		maxResults = hardSearchMaxResults
	}

	resp, err := t.client.Search(ctx, webaccess.SearchRequest{
		Query:      args.Query,
		MaxResults: maxResults,
		Recency:    args.Recency,
	})
	if err != nil {
		return tools.ToolResult{}, err
	}
	if len(resp.Results) > maxResults {
		resp.Results = resp.Results[:maxResults]
	}

	return tools.ToolResult{
		Content: formatSearchContent(resp),
		Data:    resp,
	}, nil
}

func formatSearchContent(resp webaccess.SearchResponse) string {
	if len(resp.Results) == 0 {
		return "搜索结果为空。"
	}
	var builder strings.Builder
	builder.WriteString("搜索结果：")
	for i, result := range resp.Results {
		builder.WriteString(fmt.Sprintf("\n%d. %s", i+1, fallbackText(result.Title, result.URL)))
		if strings.TrimSpace(result.Snippet) != "" {
			builder.WriteString(" - ")
			builder.WriteString(strings.TrimSpace(result.Snippet))
		}
		builder.WriteString(" (")
		builder.WriteString(strings.TrimSpace(result.URL))
		builder.WriteString(")")
	}
	return builder.String()
}

func fallbackText(value, fallback string) string {
	if strings.TrimSpace(value) != "" {
		return strings.TrimSpace(value)
	}
	return strings.TrimSpace(fallback)
}
