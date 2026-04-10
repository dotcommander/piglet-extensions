package webfetch

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
)

// ExaProvider implements SearchProvider and FetchProvider using the Exa API.
type ExaProvider struct {
	searchURL   string
	contentsURL string
	apiKey      string
	http        *http.Client
}

// NewExaProvider creates an ExaProvider with the given API key and endpoint config.
// Returns nil if apiKey is empty.
func NewExaProvider(apiKey string, cfg ExaConfig) *ExaProvider {
	if apiKey == "" {
		return nil
	}
	return &ExaProvider{
		searchURL:   cfg.SearchURL,
		contentsURL: cfg.ContentsURL,
		apiKey:      apiKey,
		http:        &http.Client{Timeout: fetchTimeout},
	}
}

func (e *ExaProvider) Name() string { return "exa" }

func (e *ExaProvider) Search(ctx context.Context, query string, limit int) ([]SearchResult, error) {
	if limit <= 0 {
		limit = defaultSearchLimit
	}

	ctx, cancel := context.WithTimeout(ctx, searchTimeout)
	defer cancel()

	body, err := json.Marshal(exaSearchRequest{
		Query:      query,
		NumResults: limit,
		Type:       "auto",
	})
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, e.searchURL, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("x-api-key", e.apiKey)
	req.Header.Set("Content-Type", "application/json")

	var envelope exaSearchResponse
	if err := doJSONRequest(e.http, req, &envelope); err != nil {
		return nil, err
	}

	items := envelope.Results
	if len(items) > limit {
		items = items[:limit]
	}

	results := make([]SearchResult, 0, len(items))
	for _, item := range items {
		desc := strings.TrimSpace(item.Text)
		if len(desc) > 200 {
			desc = desc[:200] + "…"
		}
		results = append(results, SearchResult{
			Title:       item.Title,
			URL:         item.URL,
			Description: desc,
		})
	}

	slog.Debug("exa search completed", "query", query, "results", len(results))
	return results, nil
}

func (e *ExaProvider) Fetch(ctx context.Context, rawURL string) (string, error) {
	body, err := json.Marshal(exaContentsRequest{
		IDs:  []string{rawURL},
		Text: true,
	})
	if err != nil {
		return "", fmt.Errorf("marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, e.contentsURL, bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("x-api-key", e.apiKey)
	req.Header.Set("Content-Type", "application/json")

	data, err := doRequest(e.http, req)
	if err != nil {
		return "", err
	}

	var envelope exaContentsResponse
	if err := json.Unmarshal(data, &envelope); err != nil {
		return "", fmt.Errorf("parse response: %w", err)
	}

	if len(envelope.Results) == 0 {
		return "", fmt.Errorf("no content from exa")
	}

	content := truncateBody(envelope.Results[0].Text)

	slog.Debug("exa fetch completed", "url", rawURL)
	return content, nil
}

type exaSearchRequest struct {
	Query      string `json:"query"`
	NumResults int    `json:"numResults"`
	Type       string `json:"type"`
}

type exaSearchResponse struct {
	Results []exaResult `json:"results"`
}

type exaResult struct {
	Title string `json:"title"`
	URL   string `json:"url"`
	Text  string `json:"text"`
}

type exaContentsRequest struct {
	IDs  []string `json:"ids"`
	Text bool     `json:"text"`
}

type exaContentsResponse struct {
	Results []exaResult `json:"results"`
}
