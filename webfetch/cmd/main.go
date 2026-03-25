// Webfetch extension binary. Provides web_fetch and web_search tools.
// Communicates with piglet host via JSON-RPC over stdin/stdout.
package main

import (
	"context"
	"fmt"

	"github.com/dotcommander/piglet-extensions/webfetch"
	sdk "github.com/dotcommander/piglet/sdk"
)

func main() {
	client := webfetch.Default()
	e := sdk.New("webfetch", "0.1.0")

	e.RegisterPromptSection(sdk.PromptSectionDef{
		Title:   "Web Access",
		Content: "You have web_fetch (read URLs) and web_search (search the web) tools available.",
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

	e.Run()
}
