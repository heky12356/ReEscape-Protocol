package web

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"project-yume/internal/webaccess"
)

type fakeFetchClient struct {
	req webaccess.FetchRequest
}

func (c *fakeFetchClient) Fetch(ctx context.Context, req webaccess.FetchRequest) (webaccess.FetchResponse, error) {
	c.req = req
	return webaccess.FetchResponse{
		URL:         req.URL,
		Title:       "Example",
		Content:     "Fetched content",
		ContentType: "text/html",
		FetchedAt:   time.Now(),
		Truncated:   false,
	}, nil
}

func TestFetchToolRequiresURL(t *testing.T) {
	tool := NewFetchTool(&fakeFetchClient{}, 6000)
	_, err := tool.Execute(context.Background(), nil, json.RawMessage(`{"url":"","reason":"test"}`))
	if err == nil || !strings.Contains(err.Error(), "url is required") {
		t.Fatalf("expected url error, got %v", err)
	}
}

func TestFetchToolDefaultsAndClampsMaxChars(t *testing.T) {
	client := &fakeFetchClient{}
	tool := NewFetchTool(client, 6000)

	result, err := tool.Execute(context.Background(), nil, json.RawMessage(`{"url":"https://example.com","reason":"test","max_chars":999999}`))
	if err != nil {
		t.Fatalf("execute failed: %v", err)
	}
	if client.req.MaxChars != 6000 {
		t.Fatalf("expected max chars clamp to 6000, got %d", client.req.MaxChars)
	}
	if !strings.Contains(result.Content, "网页正文：") || !strings.Contains(result.Content, "Fetched content") {
		t.Fatalf("unexpected content: %q", result.Content)
	}
	if _, ok := result.Data.(webaccess.FetchResponse); !ok {
		t.Fatalf("expected structured fetch response data, got %T", result.Data)
	}
}

func TestFetchToolUsesDefaultMaxChars(t *testing.T) {
	client := &fakeFetchClient{}
	tool := NewFetchTool(client, 7000)

	_, err := tool.Execute(context.Background(), nil, json.RawMessage(`{"url":"https://example.com","reason":"test"}`))
	if err != nil {
		t.Fatalf("execute failed: %v", err)
	}
	if client.req.MaxChars != 7000 {
		t.Fatalf("expected default max chars 7000, got %d", client.req.MaxChars)
	}
}
