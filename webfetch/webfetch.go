// Package webfetch provides web fetch and search capabilities via multiple providers.
package webfetch

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"
)

const (
	maxBodyBytes       = 100 * 1024 // 100KB
	defaultSearchLimit = 5
	fetchTimeout       = 30 * time.Second
	searchTimeout      = 15 * time.Second
	userAgent          = "piglet/1.0"

	// noiseElements are HTML tags stripped when extracting page text.
	noiseElements = "script, style, noscript, nav, header, footer, iframe, svg"

	cacheNSFetch   = "webfetch"
	cacheNSSearch  = "webfetch_search"
	cacheTTLFetch  = 24 * time.Hour
	cacheTTLSearch = time.Hour
)

const truncationNote = "\n\n[Content truncated at 100KB]"

// truncateBody truncates content to maxBodyBytes and appends truncationNote.
func truncateBody(content string) string {
	if len(content) <= maxBodyBytes {
		return content
	}
	return content[:maxBodyBytes] + truncationNote
}

// buildFetchResult formats a fetched page into the standard output format
// with optional title/URL header and truncation at maxBodyBytes.
func buildFetchResult(title, rawURL, body string) string {
	var sb strings.Builder
	if title != "" {
		fmt.Fprintf(&sb, "Title: %s\n\nURL Source: %s\n\n", title, rawURL)
	}
	sb.WriteString(body)
	content := sb.String()
	if len(content) > maxBodyBytes {
		content = content[:maxBodyBytes] + truncationNote
	}
	return content
}

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

func (e *HTTPError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("request %s: %v", e.URL, e.Err)
	}
	return fmt.Sprintf("request %s: HTTP %d", e.URL, e.StatusCode)
}

func (e *HTTPError) Unwrap() error {
	return e.Err
}

// llmRefusalPrefixes detects LLM responses that are refusals rather than content.
var llmRefusalPrefixes = []string{
	"i am unable to access",
	"i cannot access",
	"i'm unable to access",
	"i can't access",
	"i do not have the ability to access",
	"i don't have access to",
	"as an ai",
	"i cannot browse",
	"i'm not able to access",
	"i cannot visit",
	"i cannot fetch",
	"i cannot open",
	"i cannot retrieve",
	"i'm not able to browse",
}

// isLLMRefusal returns true if the content looks like an LLM refusal.
func isLLMRefusal(content string) bool {
	s := strings.TrimSpace(content)
	if len(s) > 100 {
		s = s[:100]
	}
	lower := strings.ToLower(s)
	for _, prefix := range llmRefusalPrefixes {
		if strings.HasPrefix(lower, prefix) {
			return true
		}
	}
	return false
}

// isRecoverable returns true if the error might succeed with a different provider.
func isRecoverable(err error) bool {
	if err == nil {
		return false
	}

	var httpErr *HTTPError
	if errors.As(err, &httpErr) {
		if httpErr.StatusCode == 0 {
			return true
		}
		if httpErr.StatusCode >= 200 && httpErr.StatusCode < 300 {
			return true
		}
		if httpErr.StatusCode >= 400 && httpErr.StatusCode < 500 &&
			httpErr.StatusCode != 401 && httpErr.StatusCode != 403 &&
			httpErr.StatusCode != 429 && httpErr.StatusCode != 451 {
			return false
		}
		return true
	}

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
type Client struct {
	fetchProviders  []FetchProvider
	searchProviders []SearchProvider
	http            *http.Client
	github          *GitHubClient
	storage         *Storage
	ddgSearchURL    string
}

// New creates a Client with custom provider lists (used for testing).
func New(fetchProviders []FetchProvider, searchProviders []SearchProvider, github *GitHubClient) *Client {
	return &Client{
		fetchProviders:  fetchProviders,
		searchProviders: searchProviders,
		http: &http.Client{
			Timeout: fetchTimeout,
		},
		github:       github,
		storage:      NewStorage(),
		ddgSearchURL: DefaultDuckDuckGoConfig().SearchURL,
	}
}

// NewWithConfig creates a Client with providers based on the given config.
func NewWithConfig(cfg *Config) *Client {
	var fetchProviders []FetchProvider
	var searchProviders []SearchProvider

	exaKey := firstNonEmpty(cfg.ExaAPIKey, os.Getenv("EXA_API_KEY"))
	braveKey := firstNonEmpty(cfg.BraveAPIKey, os.Getenv("BRAVE_API_KEY"))
	geminiKey := firstNonEmpty(cfg.GeminiAPIKey, os.Getenv("GEMINI_API_KEY"))
	perplexityKey := firstNonEmpty(cfg.PerplexityAPIKey, os.Getenv("PERPLEXITY_API_KEY"))
	jinaKey := firstNonEmpty(cfg.JinaAPIKey, os.Getenv("JINA_API_KEY"))

	jina := NewJinaProvider(jinaKey, cfg.Jina)
	gemini := NewGeminiProvider(geminiKey, cfg.Gemini)
	perplexity := NewPerplexityProvider(perplexityKey, cfg.Perplexity)

	fetchProviders = append(fetchProviders, NewCollyProvider())
	fetchProviders = append(fetchProviders, jina)
	fetchProviders = append(fetchProviders, NewRodProvider())
	if ab := NewAgentBrowserProvider(); ab != nil {
		fetchProviders = append(fetchProviders, ab)
	}
	if gemini != nil {
		fetchProviders = append(fetchProviders, gemini)
	}
	if perplexity != nil {
		fetchProviders = append(fetchProviders, perplexity)
	}

	if brave := NewBraveProvider(braveKey, cfg.Brave); brave != nil {
		searchProviders = append(searchProviders, brave)
	}
	if exa := NewExaProvider(exaKey, cfg.Exa); exa != nil {
		searchProviders = append(searchProviders, exa)
	}
	if gemini != nil {
		searchProviders = append(searchProviders, gemini)
	}
	if perplexity != nil {
		searchProviders = append(searchProviders, perplexity)
	}
	searchProviders = append(searchProviders, jina)

	var github *GitHubClient
	if cfg.GitHub.Enabled {
		github = NewGitHubClient(&cfg.GitHub)
	}

	c := New(fetchProviders, searchProviders, github)
	c.ddgSearchURL = cfg.DuckDuckGo.SearchURL
	return c
}

// Default returns a Client with only Jina provider (no API keys required).
func Default() *Client {
	cfg := &Config{
		GitHub: GitHubConfig{
			Enabled:        true,
			SkipLargeRepos: true,
		},
	}
	cfg.applyDefaults()
	return NewWithConfig(cfg)
}

// NewForTest creates a Client with mock Jina providers for testing.
func NewForTest(readerBase, searchBase string) *Client {
	jina := NewJinaProviderWithBase(readerBase, searchBase, "")
	return New([]FetchProvider{jina}, []SearchProvider{jina}, nil)
}

// firstNonEmpty returns the first non-empty string.
func firstNonEmpty(s ...string) string {
	for _, v := range s {
		if v != "" {
			return v
		}
	}
	return ""
}
