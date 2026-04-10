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

//go:embed defaults/gemini-fetch-prompt.txt
var rawGeminiFetchPrompt string

//go:embed defaults/gemini-search-prompt.txt
var rawGeminiSearchPrompt string

var (
	geminiPromptOnce         sync.Once
	activeGeminiFetchPrompt  string
	activeGeminiSearchPrompt string
)

func loadGeminiPrompts() {
	geminiPromptOnce.Do(func() {
		activeGeminiFetchPrompt = xdg.LoadOrCreateExt("webfetch", "gemini-fetch-prompt.txt", strings.TrimSpace(rawGeminiFetchPrompt))
		activeGeminiSearchPrompt = xdg.LoadOrCreateExt("webfetch", "gemini-search-prompt.txt", strings.TrimSpace(rawGeminiSearchPrompt))
	})
}

// GeminiProvider implements FetchProvider and SearchProvider using Google Gemini API.
type GeminiProvider struct {
	apiBase string
	model   string
	apiKey  string
	http    *http.Client
}

// NewGeminiProvider creates a GeminiProvider with the given API key and endpoint config.
// Returns nil if apiKey is empty.
func NewGeminiProvider(apiKey string, cfg GeminiConfig) *GeminiProvider {
	if apiKey == "" {
		return nil
	}

	return &GeminiProvider{
		apiBase: cfg.APIBase,
		model:   cfg.Model,
		apiKey:  apiKey,
		http: &http.Client{
			Timeout: fetchTimeout,
		},
	}
}

// Name returns the provider name for logging.
func (g *GeminiProvider) Name() string {
	return "gemini"
}

type geminiRequest struct {
	Contents []geminiContent `json:"contents"`
}

type geminiContent struct {
	Parts []geminiPart `json:"parts"`
}

type geminiPart struct {
	Text string `json:"text"`
}

type geminiResponse struct {
	Candidates []geminiCandidate `json:"candidates"`
	Error      *geminiError      `json:"error,omitempty"`
}

type geminiCandidate struct {
	Content geminiContent `json:"content"`
}

type geminiError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Status  string `json:"status"`
}

// Fetch retrieves content by asking Gemini to summarize the URL.
func (g *GeminiProvider) Fetch(ctx context.Context, rawURL string) (string, error) {
	loadGeminiPrompts()
	apiURL := fmt.Sprintf("%s/models/%s:generateContent?key=%s", g.apiBase, g.model, g.apiKey)

	reqBody := geminiRequest{
		Contents: []geminiContent{
			{Parts: []geminiPart{{Text: fmt.Sprintf(activeGeminiFetchPrompt, rawURL)}}},
		},
	}

	respData, err := llmPost(g.http, apiURL, nil, reqBody)
	if err != nil {
		return "", err
	}

	var geminiResp geminiResponse
	if err := json.Unmarshal(respData, &geminiResp); err != nil {
		return "", fmt.Errorf("parse response: %w", err)
	}

	if geminiResp.Error != nil {
		return "", fmt.Errorf("gemini error [%d]: %s", geminiResp.Error.Code, geminiResp.Error.Message)
	}

	if len(geminiResp.Candidates) == 0 || len(geminiResp.Candidates[0].Content.Parts) == 0 {
		return "", fmt.Errorf("no response from gemini")
	}

	content := geminiResp.Candidates[0].Content.Parts[0].Text

	if isLLMRefusal(content) {
		return "", &HTTPError{
			URL:        rawURL,
			StatusCode: 204,
			Err:        fmt.Errorf("gemini refused to fetch: content appears to be a refusal"),
		}
	}

	slog.Debug("gemini fetch completed", "url", rawURL)
	return truncateBody(content), nil
}

// Search queries Gemini for search results.
func (g *GeminiProvider) Search(ctx context.Context, query string, limit int) ([]SearchResult, error) {
	loadGeminiPrompts()

	if limit <= 0 {
		limit = defaultSearchLimit
	}

	apiURL := fmt.Sprintf("%s/models/%s:generateContent?key=%s", g.apiBase, g.model, g.apiKey)

	reqBody := geminiRequest{
		Contents: []geminiContent{
			{Parts: []geminiPart{{Text: fmt.Sprintf(activeGeminiSearchPrompt, query, limit)}}},
		},
	}

	respData, err := llmPost(g.http, apiURL, nil, reqBody)
	if err != nil {
		return nil, err
	}

	var geminiResp geminiResponse
	if err := json.Unmarshal(respData, &geminiResp); err != nil {
		return nil, fmt.Errorf("parse response: %w", err)
	}

	if geminiResp.Error != nil {
		return nil, fmt.Errorf("gemini error [%d]: %s", geminiResp.Error.Code, geminiResp.Error.Message)
	}

	if len(geminiResp.Candidates) == 0 || len(geminiResp.Candidates[0].Content.Parts) == 0 {
		return nil, fmt.Errorf("no response from gemini")
	}

	content := geminiResp.Candidates[0].Content.Parts[0].Text
	results := parseGeminiSearchResults(content, limit)

	slog.Debug("gemini search completed", "query", query, "results", len(results))
	return results, nil
}

// parseGeminiSearchResults extracts structured results from Gemini text response.
func parseGeminiSearchResults(content string, limit int) []SearchResult {
	return parseLLMSearchResults("Gemini Search Results", "https://gemini.google.com", content, limit)
}
