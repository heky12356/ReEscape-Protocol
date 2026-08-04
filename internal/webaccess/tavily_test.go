package webaccess

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestTavilyClientSearchSendsExpectedRequestAndParsesResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("unexpected method: %s", r.Method)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer test-key" {
			t.Fatalf("unexpected authorization header: %q", got)
		}
		if got := r.Header.Get("Content-Type"); got != "application/json" {
			t.Fatalf("unexpected content-type header: %q", got)
		}

		var body tavilySearchRequest
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode request body: %v", err)
		}
		if body.Query != "anime" {
			t.Fatalf("unexpected query: %q", body.Query)
		}
		if body.MaxResults != 5 {
			t.Fatalf("unexpected max_results: %d", body.MaxResults)
		}
		if body.SearchDepth != "basic" {
			t.Fatalf("unexpected search_depth: %q", body.SearchDepth)
		}
		if body.Topic != "general" {
			t.Fatalf("unexpected topic: %q", body.Topic)
		}
		if body.TimeRange != "month" {
			t.Fatalf("unexpected time_range: %q", body.TimeRange)
		}
		if body.IncludeAnswer || body.IncludeRawContent || body.IncludeImages || body.IncludeFavicon || body.SafeSearch {
			t.Fatalf("unexpected boolean options: %+v", body)
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprint(w, `{
			"query": "anime",
			"answer": "optional answer",
			"results": [
				{
					"title": "Result A",
					"url": "https://example.com/a",
					"content": "Snippet A",
					"score": 0.9,
					"published_date": "2026-08-01"
				}
			],
			"usage": {"credits": 1}
		}`)
	}))
	defer server.Close()

	client := NewTavilyClient(server.URL, "test-key", time.Second, TavilyOptions{})
	resp, err := client.Search(context.Background(), SearchRequest{Query: "anime", MaxResults: 5, Recency: "month"})
	if err != nil {
		t.Fatalf("search failed: %v", err)
	}
	if resp.Provider != "tavily" {
		t.Fatalf("unexpected provider: %q", resp.Provider)
	}
	if resp.Query != "anime" {
		t.Fatalf("unexpected query: %q", resp.Query)
	}
	if resp.Answer != "optional answer" {
		t.Fatalf("unexpected answer: %q", resp.Answer)
	}
	if got := resp.Usage["credits"]; got == nil {
		t.Fatalf("expected usage credits, got %+v", resp.Usage)
	}
	if len(resp.Results) != 1 {
		t.Fatalf("expected one result, got %d", len(resp.Results))
	}
	result := resp.Results[0]
	if result.Title != "Result A" || result.URL != "https://example.com/a" || result.Snippet != "Snippet A" || result.Source != "example.com" || result.PublishedAt != "2026-08-01" {
		t.Fatalf("unexpected result: %+v", result)
	}
}

func TestTavilyClientSearchClassifiesStatusErrors(t *testing.T) {
	tests := []struct {
		status int
		want   string
	}{
		{status: http.StatusUnauthorized, want: "tavily unauthorized: check WEB_SEARCH_API_KEY"},
		{status: http.StatusTooManyRequests, want: "tavily rate limited"},
		{status: 432, want: "tavily plan usage limit exceeded"},
		{status: 433, want: "tavily pay-as-you-go limit exceeded"},
		{status: http.StatusBadGateway, want: "tavily server error"},
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("status_%d", tt.status), func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(tt.status)
				_, _ = fmt.Fprint(w, `{"error":"limited"}`)
			}))
			defer server.Close()

			client := NewTavilyClient(server.URL, "test-key", time.Second, TavilyOptions{})
			_, err := client.Search(context.Background(), SearchRequest{Query: "anime"})
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("expected error containing %q, got %v", tt.want, err)
			}
		})
	}
}

func TestTavilyClientSearchReturnsDecodeErrorForNonJSONResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = fmt.Fprint(w, `<!doctype html><html><body>not json</body></html>`)
	}))
	defer server.Close()

	client := NewTavilyClient(server.URL, "test-key", time.Second, TavilyOptions{})
	_, err := client.Search(context.Background(), SearchRequest{Query: "anime"})
	if err == nil || !strings.Contains(err.Error(), "decode tavily response") || !strings.Contains(err.Error(), "not json") {
		t.Fatalf("expected decode error with preview, got %v", err)
	}
}
