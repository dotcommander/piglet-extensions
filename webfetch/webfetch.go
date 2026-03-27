// Package webfetch provides web fetch and search capabilities via multiple providers.
package webfetch

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/dotcommander/piglet-extensions/cache"
)

const (
	maxBodyBytes  = 100 * 1024 // 100KB
	fetchTimeout  = 30 * time.Second
	searchTimeout = 15 * time.Second
	userAgent     = "piglet/1.0"

	cacheNSFetch  = "webfetch"
	cacheNSSearch = "webfetch_search"
	cacheTTLFetch = 24 * time.Hour
	cacheTTLSearch = time.Hour
)

// SearchResult holds a single search result.
type SearchResult struct {
	Title       string `json:"title"`
	URL         string `json:"url"`
	Description string `json:"description"`
}

// HTTPError represents an HTTP error with status code and URL.
type HTTPError struct {
	URL        string
	StatusCode int
	Err        error
}

// Error implements the error interface.
func (e *HTTPError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("request %s: %v", e.URL, e.Err)
	}
	return fmt.Sprintf("request %s: HTTP %d", e.URL, e.StatusCode)
}

// Unwrap returns the underlying error.
func (e *HTTPError) Unwrap() error {
	return e.Err
}

// isRecoverable returns true if the error might succeed with a different provider.
func isRecoverable(err error) bool {
	if err == nil {
		return false
	}

	var httpErr *HTTPError
	if errors.As(err, &httpErr) {
		// Network errors (status 0) are recoverable
		if httpErr.StatusCode == 0 {
			return true
		}
		// 4xx client errors (except 401, 403, 429) are not recoverable
		if httpErr.StatusCode >= 400 && httpErr.StatusCode < 500 && httpErr.StatusCode != 401 && httpErr.StatusCode != 403 && httpErr.StatusCode != 429 {
			return false
		}
		// 5xx server errors and 429 rate limits are recoverable
		return true
	}

	// Other errors (parsing, etc.) might be recoverable
	return true
}

// FetchProvider defines the interface for content fetching.
type FetchProvider interface {
	Name() string
	Fetch(ctx context.Context, rawURL string) (string, error)
}

// SearchProvider defines the interface for web search.
type SearchProvider interface {
	Name() string
	Search(ctx context.Context, query string, limit int) ([]SearchResult, error)
}

// Client performs web fetch and search operations with fallback support.
// The zero value is not usable; use New, NewWithConfig, Default, or NewForTest.
type Client struct {
	fetchProviders  []FetchProvider
	searchProviders []SearchProvider
	http            *http.Client
	github          *GitHubClient
	storage         *Storage
}

// New creates a Client with custom provider lists (used for testing).
func New(fetchProviders []FetchProvider, searchProviders []SearchProvider, github *GitHubClient) *Client {
	return &Client{
		fetchProviders:  fetchProviders,
		searchProviders: searchProviders,
		http: &http.Client{
			Timeout: fetchTimeout,
		},
		github:  github,
		storage: NewStorage(),
	}
}

// NewWithConfig creates a Client with providers based on the given config.
// Providers are added in order: Jina (always) -> Perplexity (if API key) -> Gemini (if API key).
func NewWithConfig(cfg *Config) *Client {
	var fetchProviders []FetchProvider
	var searchProviders []SearchProvider

	// Always add Jina as the primary provider
	jinaKey := cfg.JinaAPIKey
	if jinaKey == "" {
		jinaKey = os.Getenv("JINA_API_KEY")
	}
	jina := NewJinaProvider(jinaKey)
	fetchProviders = append(fetchProviders, jina)
	searchProviders = append(searchProviders, jina)

	// Add Perplexity if API key is configured
	if perplexity := NewPerplexityProvider(cfg.PerplexityAPIKey); perplexity != nil {
		fetchProviders = append(fetchProviders, perplexity)
		searchProviders = append(searchProviders, perplexity)
	}

	// Add Gemini if API key is configured
	if gemini := NewGeminiProvider(cfg.GeminiAPIKey); gemini != nil {
		fetchProviders = append(fetchProviders, gemini)
		searchProviders = append(searchProviders, gemini)
	}

	// Add Brave if API key is configured (search only)
	braveKey := cfg.BraveAPIKey
	if braveKey == "" {
		braveKey = os.Getenv("BRAVE_API_KEY")
	}
	if brave := NewBraveProvider(braveKey); brave != nil {
		searchProviders = append(searchProviders, brave)
	}

	// Add Exa if API key is configured
	exaKey := cfg.ExaAPIKey
	if exaKey == "" {
		exaKey = os.Getenv("EXA_API_KEY")
	}
	if exa := NewExaProvider(exaKey); exa != nil {
		fetchProviders = append(fetchProviders, exa)
		searchProviders = append(searchProviders, exa)
	}

	// Create GitHub client if enabled
	var github *GitHubClient
	if cfg.GitHub.Enabled {
		github = NewGitHubClient(&cfg.GitHub)
	}

	return New(fetchProviders, searchProviders, github)
}

// Default returns a Client with only Jina provider (no API keys required).
func Default() *Client {
	return NewWithConfig(&Config{
		GitHub: GitHubConfig{
			Enabled:        true,
			SkipLargeRepos: true,
		},
	})
}

// NewForTest creates a Client with mock Jina providers for testing.
func NewForTest(readerBase, searchBase string) *Client {
	jina := NewJinaProviderWithBase(readerBase, searchBase, "")
	return New([]FetchProvider{jina}, []SearchProvider{jina}, nil)
}

// Fetch retrieves content from the given URL with provider fallback.
// If raw is false, content is fetched via reader providers (returns clean markdown).
// If raw is true, the URL is fetched directly.
func (c *Client) Fetch(ctx context.Context, rawURL string, raw bool) (string, error) {
	if cached, ok := cache.Get(cacheNSFetch, rawURL); ok {
		return cached, nil
	}

	// Raw mode fetches directly without provider fallback
	if raw {
		content, err := c.fetchRaw(ctx, rawURL)
		if err != nil {
			return "", err
		}
		// Store full content before returning
		c.storage.StoreFetch(rawURL, content)
		if err := cache.Set(cacheNSFetch, rawURL, content, cacheTTLFetch); err != nil {
			slog.Debug("cache write failed", "url", rawURL, "error", err)
		}
		return content, nil
	}

	// Try GitHub first if enabled
	if c.github != nil {
		result, err := c.github.Fetch(ctx, rawURL)
		if err != nil {
			return "", err
		}
		if result != nil {
			content := FormatGitHubResult(result)
			c.storage.StoreFetch(rawURL, content)
			if err := cache.Set(cacheNSFetch, rawURL, content, cacheTTLFetch); err != nil {
			slog.Debug("cache write failed", "url", rawURL, "error", err)
		}
			return content, nil
		}
	}

	// Try each provider in order
	var lastErr error
	for _, provider := range c.fetchProviders {
		content, err := provider.Fetch(ctx, rawURL)
		if err == nil {
			// Store full content before returning
			c.storage.StoreFetch(rawURL, content)
			if err := cache.Set(cacheNSFetch, rawURL, content, cacheTTLFetch); err != nil {
			slog.Debug("cache write failed", "url", rawURL, "error", err)
		}
			return content, nil
		}

		slog.Debug("fetch provider failed",
			"provider", provider.Name(),
			"url", rawURL,
			"error", err)

		lastErr = err

		// If error is not recoverable, don't try next provider
		if !isRecoverable(err) {
			break
		}
	}

	return "", fmt.Errorf("all fetch providers failed: %w", lastErr)
}

// fetchRaw fetches the URL directly without any provider.
func (c *Client) fetchRaw(ctx context.Context, rawURL string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, fetchTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return "", fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("User-Agent", userAgent)

	resp, err := c.http.Do(req)
	if err != nil {
		return "", &HTTPError{URL: rawURL, StatusCode: 0, Err: err}
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return "", &HTTPError{URL: rawURL, StatusCode: resp.StatusCode}
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

// Search queries providers and returns up to limit results with fallback.
func (c *Client) Search(ctx context.Context, query string, limit int) ([]SearchResult, error) {
	cacheKey := fmt.Sprintf("%s:%d", query, limit)
	if cached, ok := cache.Get(cacheNSSearch, cacheKey); ok {
		var results []SearchResult
		if err := json.Unmarshal([]byte(cached), &results); err == nil {
			return results, nil
		}
	}

	// Try each provider in order
	var lastErr error
	for _, provider := range c.searchProviders {
		results, err := provider.Search(ctx, query, limit)
		if err == nil {
			// Store results before returning
			c.storage.StoreSearch(query, results)
			if data, err := json.Marshal(results); err == nil {
				if err := cache.Set(cacheNSSearch, cacheKey, string(data), cacheTTLSearch); err != nil {
					slog.Debug("cache write failed", "query", query, "error", err)
				}
			}
			return results, nil
		}

		slog.Debug("search provider failed",
			"provider", provider.Name(),
			"query", query,
			"error", err)

		lastErr = err

		// If error is not recoverable, don't try next provider
		if !isRecoverable(err) {
			break
		}
	}

	// Fallback: fetch a DuckDuckGo search page via reader providers
	slog.Debug("search providers exhausted, falling back to web fetch", "query", query)
	ddgURL := "https://html.duckduckgo.com/html/?q=" + url.QueryEscape(query)
	content, err := c.Fetch(ctx, ddgURL, false)
	if err != nil {
		return nil, fmt.Errorf("all search providers failed (fetch fallback also failed): %w", lastErr)
	}

	return []SearchResult{{
		Title:       "Web search results for: " + query,
		URL:         ddgURL,
		Description: content,
	}}, nil
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

// GetStorage returns the session storage for cached results.
func (c *Client) GetStorage() *Storage {
	return c.storage
}
