package webaccess

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const maxSearxNGResponseBytes = 2 << 20

type SearxNGClient struct {
	endpoint string
	client   *http.Client
}

func NewSearxNGClient(endpoint string, timeout time.Duration) *SearxNGClient {
	if timeout <= 0 {
		timeout = 8 * time.Second
	}
	return &SearxNGClient{
		endpoint: strings.TrimSpace(endpoint),
		client: &http.Client{
			Timeout: timeout,
		},
	}
}

func (c *SearxNGClient) Search(ctx context.Context, req SearchRequest) (SearchResponse, error) {
	query := strings.TrimSpace(req.Query)
	if query == "" {
		return SearchResponse{}, fmt.Errorf("query is required")
	}
	if strings.TrimSpace(c.endpoint) == "" {
		return SearchResponse{}, fmt.Errorf("searxng endpoint is required")
	}

	endpointURL, err := url.Parse(c.endpoint)
	if err != nil {
		return SearchResponse{}, fmt.Errorf("invalid searxng endpoint: %w", err)
	}
	values := endpointURL.Query()
	values.Set("q", query)
	values.Set("format", "json")
	if recency := searxngTimeRange(req.Recency); recency != "" {
		values.Set("time_range", recency)
	}
	endpointURL.RawQuery = values.Encode()

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, endpointURL.String(), nil)
	if err != nil {
		return SearchResponse{}, err
	}
	httpReq.Header.Set("Accept", "application/json")

	resp, err := c.client.Do(httpReq)
	if err != nil {
		return SearchResponse{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, maxSearxNGResponseBytes))
		return SearchResponse{}, fmt.Errorf("searxng returned status %d content-type %q body %q", resp.StatusCode, resp.Header.Get("Content-Type"), responsePreview(body))
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxSearxNGResponseBytes+1))
	if err != nil {
		return SearchResponse{}, fmt.Errorf("read searxng response: %w", err)
	}
	if len(body) > maxSearxNGResponseBytes {
		body = body[:maxSearxNGResponseBytes]
	}
	if !looksLikeJSONResponse(resp.Header.Get("Content-Type"), body) {
		return SearchResponse{}, fmt.Errorf("searxng returned non-json response content-type %q body %q", resp.Header.Get("Content-Type"), responsePreview(body))
	}

	var parsed struct {
		Results []struct {
			Title         string `json:"title"`
			URL           string `json:"url"`
			Content       string `json:"content"`
			PublishedDate string `json:"publishedDate"`
			Engine        string `json:"engine"`
		} `json:"results"`
	}
	if err := json.NewDecoder(bytes.NewReader(body)).Decode(&parsed); err != nil {
		return SearchResponse{}, fmt.Errorf("decode searxng response content-type %q body %q: %w", resp.Header.Get("Content-Type"), responsePreview(body), err)
	}

	maxResults := req.MaxResults
	if maxResults <= 0 {
		maxResults = 5
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
		source := strings.TrimSpace(item.Engine)
		if source == "" {
			source = hostFromURL(itemURL)
		}
		results = append(results, SearchResult{
			Title:       strings.TrimSpace(item.Title),
			URL:         itemURL,
			Snippet:     strings.TrimSpace(item.Content),
			Source:      source,
			PublishedAt: strings.TrimSpace(item.PublishedDate),
		})
	}

	return SearchResponse{
		Provider: "searxng",
		Query:    query,
		Results:  results,
	}, nil
}

func searxngTimeRange(recency string) string {
	switch strings.ToLower(strings.TrimSpace(recency)) {
	case "day", "month", "year":
		return strings.ToLower(strings.TrimSpace(recency))
	default:
		return ""
	}
}

func looksLikeJSONResponse(contentType string, body []byte) bool {
	mediaType := normalizedMediaType(contentType)
	if mediaType == "application/json" || strings.HasSuffix(mediaType, "+json") {
		return true
	}
	trimmed := bytes.TrimSpace(body)
	return len(trimmed) > 0 && (trimmed[0] == '{' || trimmed[0] == '[')
}

func responsePreview(body []byte) string {
	preview := strings.Join(strings.Fields(string(body)), " ")
	runes := []rune(preview)
	if len(runes) > 300 {
		return string(runes[:300]) + "..."
	}
	return preview
}

func hostFromURL(raw string) string {
	parsed, err := url.Parse(raw)
	if err != nil {
		return ""
	}
	return parsed.Hostname()
}
