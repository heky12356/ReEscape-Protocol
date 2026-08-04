package webaccess

import (
	"strings"
	"testing"

	"project-yume/internal/config"
)

func TestNewSearchClientFromConfigRequiresTavilyAPIKey(t *testing.T) {
	_, err := NewSearchClientFromConfig(&config.Config{
		WebSearchProvider: "tavily",
	})
	if err == nil || !strings.Contains(err.Error(), "WEB_SEARCH_API_KEY is required for tavily provider") {
		t.Fatalf("expected tavily api key error, got %v", err)
	}
}

func TestNewSearchClientFromConfigUsesDefaultTavilyEndpoint(t *testing.T) {
	client, err := NewSearchClientFromConfig(&config.Config{
		WebSearchProvider: "tavily",
		WebSearchAPIKey:   "test-key",
		WebToolTimeoutMs:  1000,
	})
	if err != nil {
		t.Fatalf("create client failed: %v", err)
	}
	tavily, ok := client.(*TavilyClient)
	if !ok {
		t.Fatalf("expected TavilyClient, got %T", client)
	}
	if tavily.endpoint != defaultTavilySearchEndpoint {
		t.Fatalf("expected default endpoint %q, got %q", defaultTavilySearchEndpoint, tavily.endpoint)
	}
}

func TestNewSearchClientFromConfigRequiresSearxNGEndpoint(t *testing.T) {
	_, err := NewSearchClientFromConfig(&config.Config{
		WebSearchProvider: "searxng",
	})
	if err == nil || !strings.Contains(err.Error(), "WEB_SEARCH_ENDPOINT is required for searxng provider") {
		t.Fatalf("expected searxng endpoint error, got %v", err)
	}
}

func TestNewSearchClientFromConfigRejectsUnsupportedProvider(t *testing.T) {
	_, err := NewSearchClientFromConfig(&config.Config{
		WebSearchProvider: "unknown",
	})
	if err == nil || !strings.Contains(err.Error(), "unsupported web search provider") {
		t.Fatalf("expected unsupported provider error, got %v", err)
	}
}
