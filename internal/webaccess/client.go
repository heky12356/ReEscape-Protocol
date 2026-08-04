package webaccess

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"project-yume/internal/config"
)

type SearchClient interface {
	Search(ctx context.Context, req SearchRequest) (SearchResponse, error)
}

type FetchClient interface {
	Fetch(ctx context.Context, req FetchRequest) (FetchResponse, error)
}

func NewSearchClientFromConfig(cfg *config.Config) (SearchClient, error) {
	if cfg == nil {
		return nil, fmt.Errorf("config is nil")
	}

	provider := strings.ToLower(strings.TrimSpace(cfg.WebSearchProvider))
	if provider == "" {
		provider = "searxng"
	}

	timeout := time.Duration(cfg.WebToolTimeoutMs) * time.Millisecond
	if timeout <= 0 {
		timeout = 8 * time.Second
	}

	switch provider {
	case "searxng":
		endpoint := strings.TrimSpace(cfg.WebSearchEndpoint)
		if endpoint == "" {
			return nil, fmt.Errorf("WEB_SEARCH_ENDPOINT is required for searxng provider")
		}
		return NewSearxNGClient(endpoint, timeout), nil
	case "tavily":
		apiKey := strings.TrimSpace(cfg.WebSearchAPIKey)
		if apiKey == "" {
			return nil, fmt.Errorf("WEB_SEARCH_API_KEY is required for tavily provider")
		}
		endpoint := strings.TrimSpace(cfg.WebSearchEndpoint)
		if endpoint == "" {
			endpoint = defaultTavilySearchEndpoint
		}
		return NewTavilyClient(endpoint, apiKey, timeout, TavilyOptions{
			SearchDepth:       cfg.TavilySearchDepth,
			Topic:             cfg.TavilyTopic,
			IncludeAnswer:     cfg.TavilyIncludeAnswer,
			IncludeRawContent: cfg.TavilyIncludeRawContent,
			SafeSearch:        cfg.TavilySafeSearch,
		}), nil
	case "brave", "serper":
		return nil, fmt.Errorf("%s provider not implemented", provider)
	default:
		return nil, fmt.Errorf("unsupported web search provider: %s", provider)
	}
}

func NewFetchClientFromConfig(cfg *config.Config) FetchClient {
	timeout := 8 * time.Second
	maxBytes := int64(1 << 20)
	maxChars := 6000
	userAgent := "ReEscapeProtocolBot/1.0"

	if cfg != nil {
		if cfg.WebToolTimeoutMs > 0 {
			timeout = time.Duration(cfg.WebToolTimeoutMs) * time.Millisecond
		}
		if cfg.WebFetchMaxBytes > 0 {
			maxBytes = int64(cfg.WebFetchMaxBytes)
		}
		if cfg.WebFetchMaxChars > 0 {
			maxChars = cfg.WebFetchMaxChars
		}
		if strings.TrimSpace(cfg.WebFetchUserAgent) != "" {
			userAgent = strings.TrimSpace(cfg.WebFetchUserAgent)
		}
	}

	return NewHTTPFetchClient(http.DefaultTransport, timeout, maxBytes, maxChars, userAgent)
}
