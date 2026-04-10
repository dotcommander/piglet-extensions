package repomap

import (
	sdk "github.com/dotcommander/piglet/sdk"
)

const Version = "0.2.0"

// codeChangingTools lists tool names that modify source code.
var codeChangingTools = map[string]bool{
	"write_file":    true,
	"edit_file":     true,
	"bash":          true,
	"notebook_edit": true,
	"multi_edit":    true,
}

// inventoryParams defines parameters for the repomap_inventory tool.
var inventoryParams = map[string]any{
	"type": "object",
	"properties": map[string]any{
		"action": map[string]any{
			"type":        "string",
			"enum":        []string{"scan", "query"},
			"description": "scan: rebuild inventory from disk. query: filter existing inventory.",
		},
		"filter": map[string]any{
			"type":        "string",
			"description": "Filter expression for query (e.g. 'lines>100', 'path=internal/')",
		},
	},
	"required": []string{"action"},
}

// repomapToolParams is shared between repomap_show and repomap_refresh tools.
var repomapToolParams = map[string]any{
	"type": "object",
	"properties": map[string]any{
		"verbose": map[string]any{
			"type":        "boolean",
			"description": "Show all symbols grouped by category (default: false)",
		},
		"detail": map[string]any{
			"type":        "boolean",
			"description": "Show all symbols with full signatures (default: false)",
		},
	},
}

// Register wires the repomap extension into a shared SDK extension.
func Register(e *sdk.Extension) {
	s := newRepomapState()
	s.registerOnInit(e)
	s.registerEventHandler(e)
	s.registerTools(e)
}
