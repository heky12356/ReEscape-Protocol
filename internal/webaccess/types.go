package webaccess

import "time"

type SearchRequest struct {
	Query      string
	MaxResults int
	Recency    string
}

type SearchResponse struct {
	Provider string         `json:"provider"`
	Query    string         `json:"query"`
	Results  []SearchResult `json:"results"`
	Answer   string         `json:"answer,omitempty"`
	Usage    map[string]any `json:"usage,omitempty"`
}

type SearchResult struct {
	Title       string `json:"title"`
	URL         string `json:"url"`
	Snippet     string `json:"snippet"`
	Source      string `json:"source"`
	PublishedAt string `json:"published_at"`
}

type FetchRequest struct {
	URL      string
	MaxChars int
}

type FetchResponse struct {
	URL         string    `json:"url"`
	Title       string    `json:"title"`
	Content     string    `json:"content"`
	ContentType string    `json:"content_type"`
	FetchedAt   time.Time `json:"fetched_at"`
	Truncated   bool      `json:"truncated"`
}
