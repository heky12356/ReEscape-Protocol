package webaccess

import (
	"context"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type HTTPFetchClient struct {
	client    *http.Client
	maxBytes  int64
	maxChars  int
	userAgent string
	validate  func(context.Context, string) (*url.URL, error)
}

func NewHTTPFetchClient(transport http.RoundTripper, timeout time.Duration, maxBytes int64, maxChars int, userAgent string) *HTTPFetchClient {
	if transport == nil {
		transport = http.DefaultTransport
	}
	if timeout <= 0 {
		timeout = 8 * time.Second
	}
	if maxBytes <= 0 {
		maxBytes = 1 << 20
	}
	if maxChars <= 0 {
		maxChars = 6000
	}
	if strings.TrimSpace(userAgent) == "" {
		userAgent = "ReEscapeProtocolBot/1.0"
	}

	fetcher := &HTTPFetchClient{
		maxBytes:  maxBytes,
		maxChars:  maxChars,
		userAgent: strings.TrimSpace(userAgent),
		validate:  ValidateOutboundURL,
	}
	fetcher.client = &http.Client{
		Timeout:   timeout,
		Transport: transport,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 5 {
				return fmt.Errorf("too many redirects")
			}
			_, err := fetcher.validate(req.Context(), req.URL.String())
			return err
		},
	}
	return fetcher
}

func (c *HTTPFetchClient) Fetch(ctx context.Context, req FetchRequest) (FetchResponse, error) {
	rawURL := strings.TrimSpace(req.URL)
	if rawURL == "" {
		return FetchResponse{}, fmt.Errorf("url is required")
	}
	validate := c.validate
	if validate == nil {
		validate = ValidateOutboundURL
	}
	parsed, err := validate(ctx, rawURL)
	if err != nil {
		return FetchResponse{}, err
	}

	maxChars := req.MaxChars
	if maxChars <= 0 || maxChars > c.maxChars {
		maxChars = c.maxChars
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, parsed.String(), nil)
	if err != nil {
		return FetchResponse{}, err
	}
	httpReq.Header.Set("Accept", "text/html,text/plain,application/json,application/xml;q=0.8,*/*;q=0.1")
	httpReq.Header.Set("User-Agent", c.userAgent)

	resp, err := c.client.Do(httpReq)
	if err != nil {
		return FetchResponse{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return FetchResponse{}, fmt.Errorf("fetch returned status %d", resp.StatusCode)
	}

	contentType := resp.Header.Get("Content-Type")
	if !isAllowedTextContentType(contentType) {
		return FetchResponse{}, fmt.Errorf("unsupported content-type: %s", contentType)
	}

	body, bodyTruncated, err := readLimited(resp.Body, c.maxBytes)
	if err != nil {
		return FetchResponse{}, err
	}
	bodyText := string(body)
	title := ""
	content := bodyText
	if isHTMLContentType(contentType) {
		title = extractHTMLTitle(bodyText)
		content = htmlToText(bodyText)
	} else {
		content = normalizeWhitespace(bodyText)
	}

	content, charTruncated := truncateRunes(content, maxChars)
	if title == "" {
		title = strings.TrimSpace(resp.Request.URL.Hostname())
	}

	return FetchResponse{
		URL:         resp.Request.URL.String(),
		Title:       title,
		Content:     content,
		ContentType: normalizedMediaType(contentType),
		FetchedAt:   time.Now().UTC(),
		Truncated:   bodyTruncated || charTruncated,
	}, nil
}

func readLimited(reader io.Reader, maxBytes int64) ([]byte, bool, error) {
	if maxBytes <= 0 {
		maxBytes = 1 << 20
	}
	data, err := io.ReadAll(io.LimitReader(reader, maxBytes+1))
	if err != nil {
		return nil, false, err
	}
	if int64(len(data)) <= maxBytes {
		return data, false, nil
	}
	return data[:maxBytes], true, nil
}

func isAllowedTextContentType(contentType string) bool {
	mediaType := normalizedMediaType(contentType)
	if mediaType == "" {
		return false
	}
	return strings.HasPrefix(mediaType, "text/") ||
		mediaType == "application/json" ||
		mediaType == "application/xml" ||
		mediaType == "application/xhtml+xml" ||
		mediaType == "application/rss+xml" ||
		mediaType == "application/atom+xml"
}

func isHTMLContentType(contentType string) bool {
	mediaType := normalizedMediaType(contentType)
	return mediaType == "text/html" || mediaType == "application/xhtml+xml"
}

func normalizedMediaType(contentType string) string {
	mediaType, _, err := mime.ParseMediaType(strings.TrimSpace(contentType))
	if err != nil {
		return strings.ToLower(strings.TrimSpace(contentType))
	}
	return strings.ToLower(strings.TrimSpace(mediaType))
}
