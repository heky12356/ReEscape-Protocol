package webaccess

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

func TestHTTPFetchClientFetchesHTML(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("User-Agent"); got != "TestBot/1.0" {
			t.Fatalf("unexpected user agent: %s", got)
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = fmt.Fprint(w, `<html><head><title>Example Title</title><script>bad()</script></head><body><h1>Hello</h1><p>World</p></body></html>`)
	}))
	defer server.Close()

	client := newTestFetchClient(1024, 200)
	resp, err := client.Fetch(context.Background(), FetchRequest{URL: server.URL, MaxChars: 200})
	if err != nil {
		t.Fatalf("fetch html failed: %v", err)
	}
	if resp.Title != "Example Title" {
		t.Fatalf("unexpected title: %q", resp.Title)
	}
	if resp.Content != "Example Title Hello World" {
		t.Fatalf("unexpected content: %q", resp.Content)
	}
	if resp.ContentType != "text/html" {
		t.Fatalf("unexpected content type: %q", resp.ContentType)
	}
	if resp.Truncated {
		t.Fatalf("did not expect truncation")
	}
}

func TestHTTPFetchClientFetchesPlainTextAndTruncates(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		_, _ = fmt.Fprint(w, "alpha beta gamma")
	}))
	defer server.Close()

	client := newTestFetchClient(1024, 8)
	resp, err := client.Fetch(context.Background(), FetchRequest{URL: server.URL, MaxChars: 8})
	if err != nil {
		t.Fatalf("fetch text failed: %v", err)
	}
	if resp.Content != "alpha be" {
		t.Fatalf("unexpected truncated content: %q", resp.Content)
	}
	if !resp.Truncated {
		t.Fatalf("expected truncation")
	}
}

func TestHTTPFetchClientTruncatesByMaxBytes(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		_, _ = fmt.Fprint(w, "1234567890abcdef")
	}))
	defer server.Close()

	client := newTestFetchClient(5, 100)
	resp, err := client.Fetch(context.Background(), FetchRequest{URL: server.URL, MaxChars: 100})
	if err != nil {
		t.Fatalf("fetch text failed: %v", err)
	}
	if resp.Content != "12345" {
		t.Fatalf("unexpected byte-limited content: %q", resp.Content)
	}
	if !resp.Truncated {
		t.Fatalf("expected truncation")
	}
}

func TestHTTPFetchClientRejectsNonTextContentType(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write([]byte{0x89, 0x50, 0x4e, 0x47})
	}))
	defer server.Close()

	client := newTestFetchClient(1024, 100)
	_, err := client.Fetch(context.Background(), FetchRequest{URL: server.URL, MaxChars: 100})
	if err == nil || !strings.Contains(err.Error(), "unsupported content-type") {
		t.Fatalf("expected unsupported content-type error, got %v", err)
	}
}

func TestHTTPFetchClientRejectsUnsafeRedirect(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/blocked", http.StatusFound)
	}))
	defer server.Close()

	client := newTestFetchClient(1024, 100)
	client.validate = func(ctx context.Context, raw string) (*url.URL, error) {
		parsed, err := url.Parse(raw)
		if err != nil {
			return nil, err
		}
		if strings.HasSuffix(parsed.Path, "/blocked") {
			return nil, fmt.Errorf("blocked redirect")
		}
		return parsed, nil
	}

	_, err := client.Fetch(context.Background(), FetchRequest{URL: server.URL, MaxChars: 100})
	if err == nil || !strings.Contains(err.Error(), "blocked redirect") {
		t.Fatalf("expected redirect safety error, got %v", err)
	}
}

func TestHTTPFetchClientTimeout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(100 * time.Millisecond)
		w.Header().Set("Content-Type", "text/plain")
		_, _ = fmt.Fprint(w, "slow")
	}))
	defer server.Close()

	client := NewHTTPFetchClient(http.DefaultTransport, 10*time.Millisecond, 1024, 100, "TestBot/1.0")
	client.validate = allowAnyURL
	_, err := client.Fetch(context.Background(), FetchRequest{URL: server.URL, MaxChars: 100})
	if err == nil {
		t.Fatalf("expected timeout error")
	}
}

func newTestFetchClient(maxBytes int64, maxChars int) *HTTPFetchClient {
	client := NewHTTPFetchClient(http.DefaultTransport, time.Second, maxBytes, maxChars, "TestBot/1.0")
	client.validate = allowAnyURL
	return client
}

func allowAnyURL(ctx context.Context, raw string) (*url.URL, error) {
	return url.Parse(raw)
}
