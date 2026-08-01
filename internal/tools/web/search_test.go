package web

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"project-yume/internal/webaccess"
)

type fakeSearchClient struct {
	req webaccess.SearchRequest
}

func (c *fakeSearchClient) Search(ctx context.Context, req webaccess.SearchRequest) (webaccess.SearchResponse, error) {
	c.req = req
	return webaccess.SearchResponse{
		Provider: "fake",
		Query:    req.Query,
		Results: []webaccess.SearchResult{
			{Title: "One", URL: "https://example.com/one", Snippet: "First"},
			{Title: "Two", URL: "https://example.com/two", Snippet: "Second"},
		},
	}, nil
}

func TestSearchToolRequiresQuery(t *testing.T) {
	tool := NewSearchTool(&fakeSearchClient{}, 5)
	_, err := tool.Execute(context.Background(), nil, json.RawMessage(`{"query":"","reason":"test"}`))
	if err == nil || !strings.Contains(err.Error(), "query is required") {
		t.Fatalf("expected query error, got %v", err)
	}
}

func TestSearchToolDefaultsAndClampsMaxResults(t *testing.T) {
	client := &fakeSearchClient{}
	tool := NewSearchTool(client, 3)

	result, err := tool.Execute(context.Background(), nil, json.RawMessage(`{"query":"hello","reason":"test","max_results":99}`))
	if err != nil {
		t.Fatalf("execute failed: %v", err)
	}
	if client.req.MaxResults != hardSearchMaxResults {
		t.Fatalf("expected max results clamp to %d, got %d", hardSearchMaxResults, client.req.MaxResults)
	}
	if !strings.Contains(result.Content, "搜索结果：") || !strings.Contains(result.Content, "https://example.com/one") {
		t.Fatalf("unexpected content: %q", result.Content)
	}
	if _, ok := result.Data.(webaccess.SearchResponse); !ok {
		t.Fatalf("expected structured search response data, got %T", result.Data)
	}
}

func TestSearchToolUsesDefaultMaxResults(t *testing.T) {
	client := &fakeSearchClient{}
	tool := NewSearchTool(client, 4)

	_, err := tool.Execute(context.Background(), nil, json.RawMessage(`{"query":"hello","reason":"test"}`))
	if err != nil {
		t.Fatalf("execute failed: %v", err)
	}
	if client.req.MaxResults != 4 {
		t.Fatalf("expected default max results 4, got %d", client.req.MaxResults)
	}
}
