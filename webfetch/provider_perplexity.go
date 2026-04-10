package webfetch

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"sync"

	"github.com/dotcommander/piglet-extensions/internal/xdg"
)

//go:embed defaults/fetch-prompt.txt
var rawFetchPrompt string

//go:embed defaults/search-prompt.txt
var rawSearchPrompt string

var (
	perplexityPromptOnce sync.Once
	activeFetchPrompt    string
	activeSearchPrompt   string
)

func loadPerplexityPrompts() {
	perplexityPromptOnce.Do(func() {
		activeFetchPrompt = xdg.LoadOrCreateExt("webfetch", "fetch-prompt.txt", strings.TrimSpace(rawFetchPrompt))
		activeSearchPrompt = xdg.LoadOrCreateExt("webfetch", "search-prompt.txt", strings.TrimSpace(rawSearchPrompt))
	})
}

// PerplexityProvider implements FetchProvider and SearchProvider using Perplexity API.
type PerplexityProvider struct {
	apiURL      string
	model       string
	apiKey      string
	http        *http.Client
	rateLimiter *rateLimiter
}

// NewPerplexityProvider creates a PerplexityProvider with the given API key and endpoint config.
// Returns nil if apiKey is empty.
func NewPerplexityProvider(apiKey string, cfg PerplexityConfig) *PerplexityProvider {
	if apiKey == "" {
		return nil
	}

	return &PerplexityProvider{
		apiURL: cfg.APIURL,
		model:  cfg.Model,
		apiKey: apiKey,
		http: &http.Client{
			Timeout: fetchTimeout,
		},
		rateLimiter: newRateLimiter(PerplexityRateLimitInterval),
	}
}

// Name returns the provider name for logging.
func (p *PerplexityProvider) Name() string {
	return "perplexity"
}

type perplexityRequest struct {
	Model    string              `json:"model"`
	Messages []perplexityMessage `json:"messages"`
}

type perplexityMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type perplexityResponse struct {
	Choices []perplexityChoice `json:"choices"`
	Error   *perplexityError   `json:"error,omitempty"`
}

type perplexityChoice struct {
	Message perplexityMessage `json:"message"`
}

type perplexityError struct {
	Message string `json:"message"`
	Type    string `json:"type"`
}

// Fetch retrieves content by asking Perplexity to summarize the URL.
func (p *PerplexityProvider) Fetch(ctx context.Context, rawURL string) (string, error) {
	loadPerplexityPrompts()
	p.rateLimiter.Wait()

	reqBody := perplexityRequest{
		Model: p.model,
		Messages: []perplexityMessage{
			{Role: "user", Content: fmt.Sprintf(activeFetchPrompt, rawURL)},
		},
	}

	headers := map[string]string{"Authorization": "Bearer " + p.apiKey}
	respData, err := llmPost(p.http, p.apiURL, headers, reqBody)
	if err != nil {
		return "", err
	}

	var perplexResp perplexityResponse
	if err := json.Unmarshal(respData, &perplexResp); err != nil {
		return "", fmt.Errorf("parse response: %w", err)
	}

	if perplexResp.Error != nil {
		return "", fmt.Errorf("perplexity error: %s", perplexResp.Error.Message)
	}

	if len(perplexResp.Choices) == 0 {
		return "", fmt.Errorf("no response from perplexity")
	}

	content := perplexResp.Choices[0].Message.Content

	if isLLMRefusal(content) {
		return "", &HTTPError{
			URL:        rawURL,
			StatusCode: 204,
			Err:        fmt.Errorf("perplexity refused to fetch: content appears to be a refusal"),
		}
	}

	slog.Debug("perplexity fetch completed", "url", rawURL)
	return truncateBody(content), nil
}

// Search queries Perplexity for search results.
func (p *PerplexityProvider) Search(ctx context.Context, query string, limit int) ([]SearchResult, error) {
	loadPerplexityPrompts()

	if limit <= 0 {
		limit = defaultSearchLimit
	}

	p.rateLimiter.Wait()

	reqBody := perplexityRequest{
		Model: p.model,
		Messages: []perplexityMessage{
			{Role: "user", Content: fmt.Sprintf(activeSearchPrompt, query, limit)},
		},
	}

	headers := map[string]string{"Authorization": "Bearer " + p.apiKey}
	respData, err := llmPost(p.http, p.apiURL, headers, reqBody)
	if err != nil {
		return nil, err
	}

	var perplexResp perplexityResponse
	if err := json.Unmarshal(respData, &perplexResp); err != nil {
		return nil, fmt.Errorf("parse response: %w", err)
	}

	if perplexResp.Error != nil {
		return nil, fmt.Errorf("perplexity error: %s", perplexResp.Error.Message)
	}

	if len(perplexResp.Choices) == 0 {
		return nil, fmt.Errorf("no response from perplexity")
	}

	content := perplexResp.Choices[0].Message.Content
	results := parsePerplexitySearchResults(content, limit)

	slog.Debug("perplexity search completed", "query", query, "results", len(results))
	return results, nil
}

// parsePerplexitySearchResults extracts structured results from Perplexity text response.
func parsePerplexitySearchResults(content string, limit int) []SearchResult {
	return parseLLMSearchResults("Perplexity Search Results", "https://perplexity.ai", content, limit)
}

// parseLLMSearchResults wraps unstructured LLM text as a single SearchResult.
func parseLLMSearchResults(title, url, content string, limit int) []SearchResult {
	if limit <= 0 {
		limit = 1
	}
	results := []SearchResult{
		{
			Title:       title,
			URL:         url,
			Description: truncateString(content, 500),
		},
	}
	return results[:min(len(results), limit)]
}

func truncateString(s string, maxRunes int) string {
	runes := []rune(s)
	if len(runes) <= maxRunes {
		return s
	}
	return string(runes[:maxRunes]) + "…"
}
