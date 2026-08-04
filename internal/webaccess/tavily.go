package webaccess

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const (
	defaultTavilySearchEndpoint = "https://api.tavily.com/search"
	maxTavilyResponseBytes      = 2 << 20
)

type TavilyOptions struct {
	SearchDepth       string
	Topic             string
	IncludeAnswer     bool
	IncludeRawContent bool
	SafeSearch        bool
}

type TavilyClient struct {
	endpoint string
	apiKey   string
	client   *http.Client
	options  TavilyOptions
}

func NewTavilyClient(endpoint string, apiKey string, timeout time.Duration, options TavilyOptions) *TavilyClient {
	if strings.TrimSpace(endpoint) == "" {
		endpoint = defaultTavilySearchEndpoint
	}
	if timeout <= 0 {
		timeout = 8 * time.Second
	}
	options.SearchDepth = normalizeTavilySearchDepth(options.SearchDepth)
	options.Topic = normalizeTavilyTopic(options.Topic)

	return &TavilyClient{
		endpoint: strings.TrimSpace(endpoint),
		apiKey:   strings.TrimSpace(apiKey),
		client: &http.Client{
			Timeout: timeout,
		},
		options: options,
	}
}

func (c *TavilyClient) Search(ctx context.Context, req SearchRequest) (SearchResponse, error) {
	query := strings.TrimSpace(req.Query)
	if query == "" {
		return SearchResponse{}, fmt.Errorf("query is required")
	}
	if strings.TrimSpace(c.apiKey) == "" {
		return SearchResponse{}, fmt.Errorf("tavily api key is required")
	}
	if strings.TrimSpace(c.endpoint) == "" {
		return SearchResponse{}, fmt.Errorf("tavily endpoint is required")
	}

	maxResults := req.MaxResults
	if maxResults <= 0 {
		maxResults = 5
	}

	body, err := json.Marshal(tavilySearchRequest{
		Query:             query,
		SearchDepth:       c.options.SearchDepth,
		Topic:             c.options.Topic,
		MaxResults:        maxResults,
		TimeRange:         tavilyTimeRange(req.Recency),
		IncludeAnswer:     c.options.IncludeAnswer,
		IncludeRawContent: c.options.IncludeRawContent,
		IncludeImages:     false,
		IncludeFavicon:    false,
		SafeSearch:        c.options.SafeSearch,
	})
	if err != nil {
		return SearchResponse{}, fmt.Errorf("encode tavily request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint, bytes.NewReader(body))
	if err != nil {
		return SearchResponse{}, err
	}
	httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "application/json")

	resp, err := c.client.Do(httpReq)
	if err != nil {
		return SearchResponse{}, err
	}
	defer resp.Body.Close()

	responseBody, err := io.ReadAll(io.LimitReader(resp.Body, maxTavilyResponseBytes+1))
	if err != nil {
		return SearchResponse{}, fmt.Errorf("read tavily response: %w", err)
	}
	if len(responseBody) > maxTavilyResponseBytes {
		responseBody = responseBody[:maxTavilyResponseBytes]
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return SearchResponse{}, tavilyStatusError(resp.StatusCode, resp.Header.Get("Content-Type"), responseBody)
	}
	if !looksLikeJSONResponse(resp.Header.Get("Content-Type"), responseBody) {
		return SearchResponse{}, fmt.Errorf("decode tavily response content-type %q body %q: non-json response", resp.Header.Get("Content-Type"), responsePreview(responseBody))
	}

	var parsed tavilySearchResponse
	if err := json.NewDecoder(bytes.NewReader(responseBody)).Decode(&parsed); err != nil {
		return SearchResponse{}, fmt.Errorf("decode tavily response content-type %q body %q: %w", resp.Header.Get("Content-Type"), responsePreview(responseBody), err)
	}

	results := make([]SearchResult, 0, min(maxResults, len(parsed.Results)))
	for _, item := range parsed.Results {
		if len(results) >= maxResults {
			break
		}
		itemURL := strings.TrimSpace(item.URL)
		if itemURL == "" {
			continue
		}
		results = append(results, SearchResult{
			Title:       strings.TrimSpace(item.Title),
			URL:         itemURL,
			Snippet:     strings.TrimSpace(item.Content),
			Source:      hostFromURL(itemURL),
			PublishedAt: strings.TrimSpace(item.PublishedDate),
		})
	}

	responseQuery := strings.TrimSpace(parsed.Query)
	if responseQuery == "" {
		responseQuery = query
	}

	return SearchResponse{
		Provider: "tavily",
		Query:    responseQuery,
		Results:  results,
		Answer:   strings.TrimSpace(parsed.Answer),
		Usage:    parsed.Usage,
	}, nil
}

type tavilySearchRequest struct {
	Query             string `json:"query"`
	SearchDepth       string `json:"search_depth,omitempty"`
	Topic             string `json:"topic,omitempty"`
	MaxResults        int    `json:"max_results,omitempty"`
	TimeRange         string `json:"time_range,omitempty"`
	IncludeAnswer     bool   `json:"include_answer"`
	IncludeRawContent bool   `json:"include_raw_content"`
	IncludeImages     bool   `json:"include_images"`
	IncludeFavicon    bool   `json:"include_favicon"`
	SafeSearch        bool   `json:"safe_search"`
}

type tavilySearchResponse struct {
	Query   string         `json:"query"`
	Answer  string         `json:"answer"`
	Results []tavilyResult `json:"results"`
	Usage   map[string]any `json:"usage"`
}

type tavilyResult struct {
	Title         string  `json:"title"`
	URL           string  `json:"url"`
	Content       string  `json:"content"`
	Score         float64 `json:"score"`
	RawContent    string  `json:"raw_content"`
	PublishedDate string  `json:"published_date"`
}

func tavilyTimeRange(recency string) string {
	switch strings.ToLower(strings.TrimSpace(recency)) {
	case "day", "week", "month", "year":
		return strings.ToLower(strings.TrimSpace(recency))
	default:
		return ""
	}
}

func normalizeTavilySearchDepth(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "advanced":
		return "advanced"
	default:
		return "basic"
	}
}

func normalizeTavilyTopic(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "news":
		return "news"
	default:
		return "general"
	}
}

func tavilyStatusError(statusCode int, contentType string, body []byte) error {
	detail := fmt.Sprintf(" content-type %q body %q", contentType, responsePreview(body))
	switch statusCode {
	case http.StatusUnauthorized:
		return fmt.Errorf("tavily unauthorized: check WEB_SEARCH_API_KEY%s", detail)
	case http.StatusTooManyRequests:
		return fmt.Errorf("tavily rate limited: retry later%s", detail)
	case 432:
		return fmt.Errorf("tavily plan usage limit exceeded%s", detail)
	case 433:
		return fmt.Errorf("tavily pay-as-you-go limit exceeded%s", detail)
	default:
		if statusCode >= 500 {
			return fmt.Errorf("tavily server error: returned status %d%s", statusCode, detail)
		}
		return fmt.Errorf("tavily returned status %d%s", statusCode, detail)
	}
}
