package webfetch

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"time"
)

// BraveProvider implements SearchProvider using the Brave Search API.
// Brave is search-only — no fetch/reader capability.
type BraveProvider struct {
	apiKey string
	http   *http.Client
}

// NewBraveProvider creates a BraveProvider. Returns nil if apiKey is empty.
func NewBraveProvider(apiKey string) *BraveProvider {
	if apiKey == "" {
		return nil
	}
	return &BraveProvider{
		apiKey: apiKey,
		http:   &http.Client{Timeout: 30 * time.Second},
	}
}

func (b *BraveProvider) Name() string { return "brave" }

func (b *BraveProvider) Search(ctx context.Context, query string, limit int) ([]SearchResult, error) {
	if limit <= 0 {
		limit = 5
	}

	ctx, cancel := context.WithTimeout(ctx, searchTimeout)
	defer cancel()

	searchURL := fmt.Sprintf("https://api.search.brave.com/res/v1/web/search?q=%s&count=%d", url.QueryEscape(query), limit)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, searchURL, nil)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("X-Subscription-Token", b.apiKey)
	req.Header.Set("Accept", "application/json")

	resp, err := b.http.Do(req)
	if err != nil {
		return nil, &HTTPError{URL: searchURL, StatusCode: 0, Err: err}
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return nil, &HTTPError{URL: searchURL, StatusCode: resp.StatusCode}
	}

	var envelope braveSearchResponse
	if err := json.NewDecoder(resp.Body).Decode(&envelope); err != nil {
		return nil, fmt.Errorf("parse response: %w", err)
	}

	items := envelope.Web.Results
	if len(items) > limit {
		items = items[:limit]
	}

	results := make([]SearchResult, 0, len(items))
	for _, item := range items {
		results = append(results, SearchResult{
			Title:       item.Title,
			URL:         item.URL,
			Description: item.Description,
		})
	}

	slog.Debug("brave search completed", "query", query, "results", len(results))
	return results, nil
}

type braveSearchResponse struct {
	Web braveWebResults `json:"web"`
}

type braveWebResults struct {
	Results []braveWebResult `json:"results"`
}

type braveWebResult struct {
	Title       string `json:"title"`
	URL         string `json:"url"`
	Description string `json:"description"`
}
