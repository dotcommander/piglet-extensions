// Package webfetch provides web fetch and search capabilities via Jina AI readers.
package webfetch

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const (
	DefaultReaderBase = "https://r.jina.ai/"
	DefaultSearchBase = "https://s.jina.ai/"
	maxBodyBytes      = 100 * 1024 // 100KB
	fetchTimeout      = 30 * time.Second
	searchTimeout     = 15 * time.Second
	userAgent         = "piglet/1.0"
)

// SearchResult holds a single search result.
type SearchResult struct {
	Title       string `json:"title"`
	URL         string `json:"url"`
	Description string `json:"description"`
}

// Client performs web fetch and search operations.
// The zero value is not usable; use New or Default.
type Client struct {
	readerBase string
	searchBase string
	http       *http.Client
}

// New creates a Client with custom base URLs (used in tests to point at httptest servers).
func New(readerBase, searchBase string) *Client {
	return &Client{
		readerBase: readerBase,
		searchBase: searchBase,
		http:       &http.Client{},
	}
}

// Default returns a Client configured for the live Jina endpoints.
func Default() *Client {
	return New(DefaultReaderBase, DefaultSearchBase)
}

// Fetch retrieves content from the given URL.
// If raw is false, content is fetched via the Jina reader (returns clean markdown).
// If raw is true, the URL is fetched directly.
// Response bodies are capped at 100KB; a truncation note is appended if exceeded.
func (c *Client) Fetch(ctx context.Context, rawURL string, raw bool) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, fetchTimeout)
	defer cancel()

	fetchURL := rawURL
	if !raw {
		fetchURL = c.readerBase + rawURL
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fetchURL, nil)
	if err != nil {
		return "", fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("User-Agent", userAgent)

	resp, err := c.http.Do(req)
	if err != nil {
		return "", fmt.Errorf("fetch %s: %w", fetchURL, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return "", fmt.Errorf("fetch %s: HTTP %d", fetchURL, resp.StatusCode)
	}

	limited := io.LimitReader(resp.Body, maxBodyBytes+1)
	data, err := io.ReadAll(limited)
	if err != nil {
		return "", fmt.Errorf("read body: %w", err)
	}

	content := string(data)
	if len(data) > maxBodyBytes {
		content = content[:maxBodyBytes] + "\n\n[Content truncated at 100KB]"
	}

	return content, nil
}

// jinaSearchResponse is the JSON envelope returned by s.jina.ai.
type jinaSearchResponse struct {
	Data []jinaSearchItem `json:"data"`
}

type jinaSearchItem struct {
	Title       string `json:"title"`
	URL         string `json:"url"`
	Description string `json:"description"`
	Content     string `json:"content"`
}

// Search queries the Jina search endpoint and returns up to limit results.
func (c *Client) Search(ctx context.Context, query string, limit int) ([]SearchResult, error) {
	if limit <= 0 {
		limit = 5
	}

	ctx, cancel := context.WithTimeout(ctx, searchTimeout)
	defer cancel()

	searchURL := c.searchBase + url.PathEscape(query)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, searchURL, nil)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Accept", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("search %q: %w", query, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("search %q: HTTP %d", query, resp.StatusCode)
	}

	var envelope jinaSearchResponse
	if err := json.NewDecoder(resp.Body).Decode(&envelope); err != nil {
		return nil, fmt.Errorf("parse response: %w", err)
	}

	items := envelope.Data
	if len(items) > limit {
		items = items[:limit]
	}

	results := make([]SearchResult, 0, len(items))
	for _, item := range items {
		desc := item.Description
		if desc == "" {
			// Fall back to a content snippet when description is absent.
			desc = strings.TrimSpace(item.Content)
			if len(desc) > 200 {
				desc = desc[:200] + "…"
			}
		}
		results = append(results, SearchResult{
			Title:       item.Title,
			URL:         item.URL,
			Description: desc,
		})
	}

	return results, nil
}

// FormatResults renders a slice of SearchResults as a markdown list.
func FormatResults(results []SearchResult) string {
	if len(results) == 0 {
		return "No results found."
	}
	var b strings.Builder
	for i, r := range results {
		fmt.Fprintf(&b, "%d. **%s**\n   %s\n   %s\n\n", i+1, r.Title, r.URL, r.Description)
	}
	return strings.TrimRight(b.String(), "\n")
}
