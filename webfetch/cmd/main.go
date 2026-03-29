// Webfetch extension binary. Provides web_fetch and web_search tools.
// Communicates with piglet host via JSON-RPC over stdin/stdout.
package main

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/dotcommander/piglet-extensions/webfetch"
	sdk "github.com/dotcommander/piglet/sdk"
)

func main() {
	cfg, err := webfetch.LoadConfig()
	if err != nil {
		slog.Error("failed to load config", "error", err)
	}

	client := webfetch.NewWithConfig(cfg)
	e := sdk.New("webfetch", "0.2.0")

	e.RegisterPromptSection(sdk.PromptSectionDef{
		Title:   "Web Access",
		Content: "You have web_fetch (read URLs) and web_search (search the web) tools available. Only use these when the user explicitly requests web access, asks a question requiring current information beyond your knowledge, or provides a URL to read.",
		Order:   85,
	})

	e.RegisterTool(sdk.ToolDef{
		Name:        "web_fetch",
		Description: "Fetch a URL and return its text content. By default uses a reader that converts HTML to clean markdown. Set raw=true to return the raw HTML.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"url": map[string]any{
					"type":        "string",
					"description": "URL to fetch.",
				},
				"raw": map[string]any{
					"type":        "boolean",
					"description": "If true, return raw HTML instead of extracted text. Default: false.",
				},
			},
			"required": []string{"url"},
		},
		PromptHint: "Fetch and read web pages",
		Execute: func(ctx context.Context, args map[string]any) (*sdk.ToolResult, error) {
			rawURL, _ := args["url"].(string)
			if rawURL == "" {
				return sdk.ErrorResult("url is required"), nil
			}
			raw, _ := args["raw"].(bool)

			content, err := client.Fetch(ctx, rawURL, raw)
			if err != nil {
				return sdk.ErrorResult(fmt.Sprintf("fetch failed: %s", err)), nil
			}
			return sdk.TextResult(content), nil
		},
	})

	e.RegisterTool(sdk.ToolDef{
		Name:        "web_search",
		Description: "Search the web and return results as a markdown list with titles, URLs, and snippets.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"query": map[string]any{
					"type":        "string",
					"description": "Search query.",
				},
				"limit": map[string]any{
					"type":        "integer",
					"description": "Maximum number of results to return. Default: 5.",
				},
			},
			"required": []string{"query"},
		},
		PromptHint: "Search the web for information",
		Execute: func(ctx context.Context, args map[string]any) (*sdk.ToolResult, error) {
			query, _ := args["query"].(string)
			if query == "" {
				return sdk.ErrorResult("query is required"), nil
			}
			limit := 5
			if v, ok := args["limit"].(float64); ok && int(v) > 0 {
				limit = int(v)
			}

			results, err := client.Search(ctx, query, limit)
			if err != nil {
				return sdk.ErrorResult(fmt.Sprintf("search failed: %s", err)), nil
			}
			return sdk.TextResult(webfetch.FormatResults(results)), nil
		},
	})

	e.RegisterTool(sdk.ToolDef{
		Name:        "webfetch_get_stored",
		Description: "Retrieve cached content from a previous web_fetch or web_search call. Useful when content was truncated in the original response.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"url": map[string]any{
					"type":        "string",
					"description": "URL to retrieve (for fetch results).",
				},
				"query": map[string]any{
					"type":        "string",
					"description": "Query to retrieve (for search results).",
				},
			},
		},
		PromptHint: "Retrieve cached web content",
		Execute: func(ctx context.Context, args map[string]any) (*sdk.ToolResult, error) {
			url, _ := args["url"].(string)
			query, _ := args["query"].(string)

			if url != "" {
				content := client.GetStorage().GetFetch(url)
				if content == "" {
					return sdk.ErrorResult("URL not found in cache"), nil
				}
				return sdk.TextResult(content), nil
			}
			if query != "" {
				results := client.GetStorage().GetSearch(query)
				if results == nil {
					return sdk.ErrorResult("Query not found in cache"), nil
				}
				return sdk.TextResult(webfetch.FormatResults(results)), nil
			}
			return sdk.ErrorResult("url or query is required"), nil
		},
	})

	e.Run()
}
