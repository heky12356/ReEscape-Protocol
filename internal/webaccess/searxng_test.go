package webaccess

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestSearxNGClientSearchParsesResults(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Query().Get("q"); got != "react web tools" {
			t.Fatalf("unexpected query: %s", got)
		}
		if got := r.URL.Query().Get("format"); got != "json" {
			t.Fatalf("unexpected format: %s", got)
		}
		if got := r.URL.Query().Get("time_range"); got != "week" {
			t.Fatalf("unexpected time_range: %s", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprint(w, `{"results":[{"title":"One","url":"https://example.com/one","content":"First result","engine":"engine-a"},{"title":"Two","url":"https://example.com/two","content":"Second result"}]}`)
	}))
	defer server.Close()

	client := NewSearxNGClient(server.URL, time.Second)
	resp, err := client.Search(context.Background(), SearchRequest{Query: "react web tools", MaxResults: 1, Recency: "week"})
	if err != nil {
		t.Fatalf("search failed: %v", err)
	}
	if resp.Provider != "searxng" || resp.Query != "react web tools" {
		t.Fatalf("unexpected response metadata: %+v", resp)
	}
	if len(resp.Results) != 1 {
		t.Fatalf("expected one result, got %d", len(resp.Results))
	}
	if resp.Results[0].Title != "One" || resp.Results[0].Source != "engine-a" {
		t.Fatalf("unexpected first result: %+v", resp.Results[0])
	}
}

func TestSearxNGClientSearchHandlesEmptyResults(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprint(w, `{"results":[]}`)
	}))
	defer server.Close()

	client := NewSearxNGClient(server.URL, time.Second)
	resp, err := client.Search(context.Background(), SearchRequest{Query: "nothing"})
	if err != nil {
		t.Fatalf("search failed: %v", err)
	}
	if len(resp.Results) != 0 {
		t.Fatalf("expected empty results, got %+v", resp.Results)
	}
}

func TestSearxNGClientSearchReturnsHTTPError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "bad", http.StatusBadGateway)
	}))
	defer server.Close()

	client := NewSearxNGClient(server.URL, time.Second)
	_, err := client.Search(context.Background(), SearchRequest{Query: "bad"})
	if err == nil || !strings.Contains(err.Error(), "status 502") {
		t.Fatalf("expected status error, got %v", err)
	}
}

func TestSearxNGClientSearchReturnsJSONError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprint(w, `{not-json`)
	}))
	defer server.Close()

	client := NewSearxNGClient(server.URL, time.Second)
	_, err := client.Search(context.Background(), SearchRequest{Query: "bad"})
	if err == nil || !strings.Contains(err.Error(), "decode searxng response") {
		t.Fatalf("expected json error, got %v", err)
	}
}

func TestSearxNGClientSearchReturnsNonJSONError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = fmt.Fprint(w, `<!doctype html><html><body>json disabled</body></html>`)
	}))
	defer server.Close()

	client := NewSearxNGClient(server.URL, time.Second)
	_, err := client.Search(context.Background(), SearchRequest{Query: "html"})
	if err == nil || !strings.Contains(err.Error(), "non-json response") || !strings.Contains(err.Error(), "json disabled") {
		t.Fatalf("expected non-json diagnostic error, got %v", err)
	}
}
