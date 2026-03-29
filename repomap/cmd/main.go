// Repomap extension binary. Provides a repository structure map injected into
// the system prompt. Communicates with the piglet host via JSON-RPC over stdin/stdout.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"path/filepath"
	"sync"
	"time"

	"github.com/dotcommander/piglet-extensions/internal/xdg"
	"github.com/dotcommander/piglet-extensions/repomap"
	sdk "github.com/dotcommander/piglet/sdk"
)

var (
	rm      *repomap.Map
	ext     *sdk.Extension
	once    sync.Once
	built   bool
	builtMu sync.RWMutex
)

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

func main() {
	e := sdk.New("repomap", "0.1.0")

	e.OnInit(func(x *sdk.Extension) {
		ext = x
		cfg := loadConfig()
		rm = repomap.New(x.CWD(), cfg)

		// Enable disk caching
		cd := cacheDir()
		rm.SetCacheDir(cd)

		// Try disk cache first — instant startup
		if rm.LoadCache(cd) {
			setBuilt(true)
			x.RegisterPromptSection(sdk.PromptSectionDef{
				Title:   "Repository Map",
				Content: rm.StringLines(),
				Order:   95, // late in prompt stack — volatile content goes after stable sections for cache efficiency
			})
			// Validate in background — rebuild if stale
			go func() {
				if !rm.Stale() {
					return
				}
				buildCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
				defer cancel()
				if err := rm.Build(buildCtx); err != nil {
					if !errors.Is(err, repomap.ErrNotCodeProject) {
						x.Log("warn", "repomap background rebuild: "+err.Error())
					}
				}
			}()
			return
		}

		// No cache — try quick build (5s timeout)
		quickCtx, quickCancel := context.WithTimeout(context.Background(), 5*time.Second)
		buildErr := rm.Build(quickCtx)
		quickCancel()
		if buildErr == nil {
			setBuilt(true)
			x.RegisterPromptSection(sdk.PromptSectionDef{
				Title:   "Repository Map",
				Content: rm.StringLines(),
				Order:   95,
			})
			return
		}
		if errors.Is(buildErr, repomap.ErrNotCodeProject) {
			x.Log("debug", "skipping repomap: no source files found")
			return
		}

		// Slow repo: register empty, build in background
		x.RegisterPromptSection(sdk.PromptSectionDef{
			Title:   "Repository Map",
			Content: "",
			Order:   95,
		})
		go buildInBackground()
	})

	// Rebuild when stale (after turn ends)
	e.RegisterEventHandler(sdk.EventHandlerDef{
		Name:     "repomap-stale-check",
		Priority: 50,
		Events:   []string{"turn_end"},
		Handle: func(ctx context.Context, eventType string, data json.RawMessage) *sdk.Action {
			if rm == nil {
				return nil
			}

			// Force dirty after code-changing tools
			if turnModifiedCode(data) {
				rm.Dirty()
			}

			if !rm.Stale() {
				return nil
			}
			go func() {
				buildCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
				defer cancel()
				if err := rm.Build(buildCtx); err != nil {
					if !errors.Is(err, repomap.ErrNotCodeProject) {
						ext.Log("warn", "repomap rebuild failed: "+err.Error())
					}
				}
			}()
			return nil
		},
	})

	e.RegisterTool(sdk.ToolDef{
		Name:        "repomap_refresh",
		Description: "Force rebuild the repository map after major file changes.",
		Parameters:  repomapToolParams,
		PromptHint:  "Rebuild the repository map after major file changes",
		Execute: func(ctx context.Context, args map[string]any) (*sdk.ToolResult, error) {
			if rm == nil {
				return sdk.ErrorResult("repository map not initialized"), nil
			}
			if err := rm.Build(ctx); err != nil {
				return sdk.ErrorResult("rebuild failed: " + err.Error()), nil
			}
			setBuilt(true)
			return sdk.TextResult(formatOutput(args)), nil
		},
	})

	e.RegisterTool(sdk.ToolDef{
		Name:        "repomap_show",
		Description: "Show the current repository structure map with source code definitions.",
		Parameters:  repomapToolParams,
		PromptHint:  "Show the current repository structure map (default: source lines, verbose/detail for alternatives)",
		Execute: func(ctx context.Context, args map[string]any) (*sdk.ToolResult, error) {
			if rm == nil {
				return sdk.TextResult("Repository map not initialized."), nil
			}
			out := formatOutput(args)
			if out == "" {
				if !isBuilt() {
					return sdk.TextResult("Repository map is still building..."), nil
				}
				return sdk.TextResult("Repository map is empty (no source files found)."), nil
			}
			return sdk.TextResult(out), nil
		},
	})

	e.Run()
}

func buildInBackground() {
	ext.Notify("🗺️ Scanning repository...")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	start := time.Now()
	if err := rm.Build(ctx); err != nil {
		if errors.Is(err, repomap.ErrNotCodeProject) {
			ext.Log("debug", "skipping repomap: no source files found")
		} else {
			ext.Notify("❌ Scan failed")
			ext.Log("warn", "repomap background build failed: "+err.Error())
		}
		return
	}

	elapsed := time.Since(start).Round(time.Millisecond)
	out := rm.StringLines()
	if out == "" {
		ext.Notify("⚠️ No source files found")
		ext.Log("warn", "repomap produced empty output")
		setBuilt(true)
		return
	}

	setBuilt(true)
	ext.Notify("✓ Map ready")
	ext.Log("info", "repomap built in "+elapsed.String())
}

func setBuilt(v bool) {
	builtMu.Lock()
	built = v
	builtMu.Unlock()
}

func isBuilt() bool {
	builtMu.RLock()
	defer builtMu.RUnlock()
	return built
}

// formatOutput returns the repomap in the requested format.
// Default is source lines; verbose/detail are explicit overrides.
func formatOutput(args map[string]any) string {
	verbose, _ := args["verbose"].(bool)
	detail, _ := args["detail"].(bool)
	switch {
	case detail:
		return rm.StringDetail()
	case verbose:
		return rm.StringVerbose()
	default:
		return rm.StringLines()
	}
}

// pigletConfig mirrors the relevant subset of ~/.config/piglet/config.yaml.
type pigletConfig struct {
	Repomap struct {
		MaxTokens      int `yaml:"maxTokens"`
		MaxTokensNoCtx int `yaml:"maxTokensNoCtx"`
	} `yaml:"repomap"`
}

// loadConfig reads repomap settings from ~/.config/piglet/config.yaml.
// Missing file or missing section uses defaults from repomap.DefaultConfig().
func loadConfig() repomap.Config {
	cfg := repomap.DefaultConfig()
	pc := xdg.LoadYAML("config.yaml", pigletConfig{})

	if pc.Repomap.MaxTokens > 0 {
		cfg.MaxTokens = pc.Repomap.MaxTokens
	}
	if pc.Repomap.MaxTokensNoCtx > 0 {
		cfg.MaxTokensNoCtx = pc.Repomap.MaxTokensNoCtx
	}

	return cfg
}

// cacheDir returns the repomap cache directory.
func cacheDir() string {
	configDir, err := xdg.ConfigDir()
	if err != nil {
		return ""
	}
	return filepath.Join(configDir, "cache")
}

// codeChangingTools lists tool names that modify source code.
var codeChangingTools = map[string]bool{
	"write_file":    true,
	"edit_file":     true,
	"bash":          true,
	"notebook_edit": true,
	"multi_edit":    true,
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
