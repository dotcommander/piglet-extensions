package webfetch

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
)

// BraveProvider implements SearchProvider using the Brave Search API.
// Brave is search-only — no fetch/reader capability.
type BraveProvider struct {
	searchURL string
	apiKey    string
	http      *http.Client
}

// NewBraveProvider creates a BraveProvider with the given API key and endpoint config.
// Returns nil if apiKey is empty.
func NewBraveProvider(apiKey string, cfg BraveConfig) *BraveProvider {
	if apiKey == "" {
		return nil
	}
	return &BraveProvider{
		searchURL: cfg.SearchURL,
		apiKey:    apiKey,
		http:      &http.Client{Timeout: fetchTimeout},
	}
}

func (b *BraveProvider) Name() string { return "brave" }

func (b *BraveProvider) Search(ctx context.Context, query string, limit int) ([]SearchResult, error) {
	if limit <= 0 {
		limit = defaultSearchLimit
	}

	ctx, cancel := context.WithTimeout(ctx, searchTimeout)
	defer cancel()

	searchURL := fmt.Sprintf("%s?q=%s&count=%d", b.searchURL, url.QueryEscape(query), limit)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, searchURL, nil)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("X-Subscription-Token", b.apiKey)
	req.Header.Set("Accept", "application/json")

	var envelope braveSearchResponse
	if err := doJSONRequest(b.http, req, &envelope); err != nil {
		return nil, err
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
