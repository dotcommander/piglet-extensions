package repomap

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"

	"github.com/dotcommander/piglet-extensions/internal/xdg"
	sdk "github.com/dotcommander/piglet/sdk"
)

// registerTools registers all repomap tools.
func (s *repomapState) registerTools(e *sdk.Extension) {
	e.RegisterTool(sdk.ToolDef{
		Name:        "repomap_refresh",
		Description: "Force rebuild the repository map after major file changes.",
		Parameters:  repomapToolParams,
		PromptHint:  "Rebuild the repository map after major file changes",
		Execute: func(ctx context.Context, args map[string]any) (*sdk.ToolResult, error) {
			if s.rm == nil {
				return sdk.ErrorResult("repository map not initialized"), nil
			}
			if err := s.rm.Build(ctx); err != nil {
				return sdk.ErrorResult("rebuild failed: " + err.Error()), nil
			}
			s.setBuilt(true)
			return sdk.TextResult(s.formatOutput(args)), nil
		},
	})

	e.RegisterTool(sdk.ToolDef{
		Name:        "repomap_show",
		Description: "Show the current repository structure map with source code definitions.",
		Parameters:  repomapToolParams,
		PromptHint:  "Show the current repository structure map (default: source lines, verbose/detail for alternatives)",
		Execute: func(ctx context.Context, args map[string]any) (*sdk.ToolResult, error) {
			if s.rm == nil {
				return sdk.TextResult("Repository map not initialized."), nil
			}
			out := s.formatOutput(args)
			if out == "" {
				if !s.isBuilt() {
					return sdk.TextResult("Repository map is still building..."), nil
				}
				return sdk.TextResult("Repository map is empty (no source files found)."), nil
			}
			return sdk.TextResult(out), nil
		},
	})

	e.RegisterTool(sdk.ToolDef{
		Name:        "repomap_inventory",
		Description: "Scan repository files for metrics (lines, imports) and query the inventory.",
		Parameters:  inventoryParams,
		PromptHint:  "Query per-file metrics: lines, imports. Use 'scan' to build, 'query' to filter.",
		Execute: func(ctx context.Context, args map[string]any) (*sdk.ToolResult, error) {
			if s.ext == nil {
				return sdk.TextResult("repomap not initialized"), nil
			}
			action, _ := args["action"].(string)
			filter, _ := args["filter"].(string)

			switch action {
			case "scan":
				inv, err := ScanInventory(ctx, s.ext.CWD())
				if err != nil {
					return sdk.ErrorResult("scan failed: " + err.Error()), nil
				}
				if err := PersistInventory(inv, repomapCacheDir()); err != nil {
					s.ext.Log("warn", "inventory persist failed: "+err.Error())
				}
				header := fmt.Sprintf("Inventory: %d files (scanned %s)\n\n", len(inv.Files), inv.Scanned)
				if inv.Truncated {
					header = fmt.Sprintf("Inventory: %d files — truncated at cap %d (scanned %s)\n\n", len(inv.Files), inventoryFileCap, inv.Scanned)
				}
				return sdk.TextResult(formatInventoryTable(inv.Files, header)), nil
			case "query":
				out, err := QueryInventory(repomapCacheDir(), filter)
				if err != nil {
					return sdk.ErrorResult(err.Error()), nil
				}
				return sdk.TextResult(out), nil
			default:
				return sdk.ErrorResult("unknown action: " + action + " (expected 'scan' or 'query')"), nil
			}
		},
	})

	e.RegisterTool(sdk.ToolDef{
		Name:        "repomap_status",
		Description: "Show repomap extension status: version, build state, and file/symbol counts.",
		Parameters:  map[string]any{"type": "object", "properties": map[string]any{}},
		Execute: func(_ context.Context, _ map[string]any) (*sdk.ToolResult, error) {
			state := "not built"
			if s.isBuilt() {
				state = "built"
			}
			return sdk.TextResult(fmt.Sprintf("repomap v%s\n  State: %s", Version, state)), nil
		},
	})
}

// formatOutput returns the repomap in the requested format.
func (s *repomapState) formatOutput(args map[string]any) string {
	verbose, _ := args["verbose"].(bool)
	detail, _ := args["detail"].(bool)
	switch {
	case detail:
		return s.rm.StringDetail()
	case verbose:
		return s.rm.StringVerbose()
	default:
		return s.rm.StringLines()
	}
}

// turnModifiedCode checks if the turn's tool results include code-changing tools.
func turnModifiedCode(data json.RawMessage) bool {
	var payload struct {
		ToolResults []struct {
			ToolName string `json:"toolName"`
		} `json:"ToolResults"`
	}
	if err := json.Unmarshal(data, &payload); err != nil {
		return false
	}
	for _, tr := range payload.ToolResults {
		if codeChangingTools[tr.ToolName] {
			return true
		}
	}
	return false
}

// pigletConfig mirrors the relevant subset of ~/.config/piglet/config.yaml.
type pigletConfig struct {
	Repomap struct {
		MaxTokens      int `yaml:"maxTokens"`
		MaxTokensNoCtx int `yaml:"maxTokensNoCtx"`
	} `yaml:"repomap"`
}

// loadRepomapConfig reads repomap settings from ~/.config/piglet/config.yaml.
func loadRepomapConfig() Config {
	cfg := DefaultConfig()
	pc := xdg.LoadYAMLExt("repomap", "config.yaml", pigletConfig{})

	if pc.Repomap.MaxTokens > 0 {
		cfg.MaxTokens = pc.Repomap.MaxTokens
	}
	if pc.Repomap.MaxTokensNoCtx > 0 {
		cfg.MaxTokensNoCtx = pc.Repomap.MaxTokensNoCtx
	}

	return cfg
}

// repomapCacheDir returns the repomap cache directory.
func repomapCacheDir() string {
	configDir, err := xdg.ConfigDir()
	if err != nil {
		return ""
	}
	return filepath.Join(configDir, "cache")
}
