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
	defaultFetchMaxChars = 6000
	hardFetchMaxChars    = 20000
)

type FetchTool struct {
	client      webaccess.FetchClient
	maxCharsCap int
}

type fetchInput struct {
	URL      string `json:"url"`
	Reason   string `json:"reason"`
	MaxChars int    `json:"max_chars"`
}

func NewFetchTool(client webaccess.FetchClient, maxCharsCap int) *FetchTool {
	if maxCharsCap <= 0 {
		maxCharsCap = defaultFetchMaxChars
	}
	if maxCharsCap > hardFetchMaxChars {
		maxCharsCap = hardFetchMaxChars
	}
	return &FetchTool{
		client:      client,
		maxCharsCap: maxCharsCap,
	}
}

func (t *FetchTool) Name() string {
	return "web_fetch"
}

func (t *FetchTool) Description() string {
	return "读取指定 URL 的网页正文。适合用户提供链接，或搜索结果需要阅读全文确认时使用。"
}

func (t *FetchTool) Schema() tools.Schema {
	return tools.Schema{
		Type: "object",
		Properties: map[string]tools.Property{
			"url": {
				Type:        "string",
				Description: "要读取的 http/https URL。",
			},
			"reason": {
				Type:        "string",
				Description: "为什么当前问题需要读取该网页；用于审计，不要使用角色口吻。",
			},
			"max_chars": {
				Type:        "integer",
				Description: "返回正文字符上限，默认使用系统配置。",
			},
		},
		Required:             []string{"url", "reason"},
		AdditionalProperties: false,
	}
}

func (t *FetchTool) ReadOnly() bool {
	return true
}

func (t *FetchTool) Execute(ctx context.Context, turn tools.TurnView, input json.RawMessage) (tools.ToolResult, error) {
	select {
	case <-ctx.Done():
		return tools.ToolResult{}, ctx.Err()
	default:
	}
	if t.client == nil {
		return tools.ToolResult{}, fmt.Errorf("web fetch client is not configured")
	}

	var args fetchInput
	if len(input) > 0 {
		if err := json.Unmarshal(input, &args); err != nil {
			return tools.ToolResult{}, err
		}
	}
	args.URL = strings.TrimSpace(args.URL)
	args.Reason = strings.TrimSpace(args.Reason)
	if args.URL == "" {
		return tools.ToolResult{}, fmt.Errorf("url is required")
	}
	if args.Reason == "" {
		return tools.ToolResult{}, fmt.Errorf("reason is required")
	}

	maxChars := args.MaxChars
	if maxChars <= 0 || maxChars > t.maxCharsCap {
		maxChars = t.maxCharsCap
	}

	resp, err := t.client.Fetch(ctx, webaccess.FetchRequest{
		URL:      args.URL,
		MaxChars: maxChars,
	})
	if err != nil {
		return tools.ToolResult{}, err
	}

	return tools.ToolResult{
		Content: formatFetchContent(resp),
		Data:    resp,
	}, nil
}

func formatFetchContent(resp webaccess.FetchResponse) string {
	var builder strings.Builder
	builder.WriteString("网页正文：")
	if strings.TrimSpace(resp.Title) != "" {
		builder.WriteString("\n标题：")
		builder.WriteString(strings.TrimSpace(resp.Title))
	}
	builder.WriteString("\nURL：")
	builder.WriteString(strings.TrimSpace(resp.URL))
	builder.WriteString("\n内容：")
	builder.WriteString(strings.TrimSpace(resp.Content))
	if resp.Truncated {
		builder.WriteString("\n[内容已截断]")
	}
	return builder.String()
}
