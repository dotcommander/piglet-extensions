package repomap

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"sync"
	"time"

	"github.com/dotcommander/piglet-extensions/internal/xdg"
	sdk "github.com/dotcommander/piglet/sdk"
)

// repomapState holds mutable state for the repomap extension lifecycle.
type repomapState struct {
	rm      *Map
	ext     *sdk.Extension
	built   bool
	builtMu sync.RWMutex
	version string
}

func newRepomapState(version string) *repomapState {
	return &repomapState{version: version}
}

func (s *repomapState) setBuilt(v bool) {
	s.builtMu.Lock()
	s.built = v
	s.builtMu.Unlock()
}

func (s *repomapState) isBuilt() bool {
	s.builtMu.RLock()
	defer s.builtMu.RUnlock()
	return s.built
}

// registerPrompt registers the Repository Map prompt section with the given content.
func (s *repomapState) registerPrompt(ext *sdk.Extension, content string) {
	ext.RegisterPromptSection(sdk.PromptSectionDef{
		Title:   "Repository Map",
		Content: content,
		Order:   95,
	})
}

// buildInBackground runs a full scan in a goroutine, updating the user via notifications.
func (s *repomapState) buildInBackground() {
	s.ext.Notify("Scanning repository...")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	start := time.Now()
	if err := s.rm.Build(ctx); err != nil {
		if errors.Is(err, ErrNotCodeProject) {
			s.ext.Log("debug", "skipping repomap: no source files found")
		} else {
			s.ext.Notify("Scan failed")
			s.ext.Log("warn", "repomap background build failed: "+err.Error())
		}
		return
	}

	elapsed := time.Since(start).Round(time.Millisecond)
	out := s.rm.StringLines()
	if out == "" {
		s.ext.Notify("No source files found")
		s.ext.Log("warn", "repomap produced empty output")
		s.setBuilt(true)
		return
	}

	s.setBuilt(true)
	s.ext.Notify("Map ready")
	s.ext.Log("info", "repomap built in "+elapsed.String())
}

// registerOnInit sets up the OnInit handler for cache-aware startup.
func (s *repomapState) registerOnInit(e *sdk.Extension) {
	e.OnInit(func(x *sdk.Extension) {
		start := time.Now()
		x.Log("debug", "[repomap] OnInit start")

		s.ext = x

		cachedInv := LoadInventory(repomapCacheDir())
		if cachedInv != nil {
			x.Log("debug", fmt.Sprintf("[repomap] inventory cache found: %d files", len(cachedInv.Files)))
		}
		cfg := loadRepomapConfig()
		s.rm = New(x.CWD(), cfg)

		cd := repomapCacheDir()
		s.rm.SetCacheDir(cd)

		// Try disk cache first — instant startup
		if s.rm.LoadCache(cd) {
			x.Log("debug", fmt.Sprintf("[repomap] cache hit (%s)", time.Since(start)))
			s.setBuilt(true)
			s.registerPrompt(x, s.rm.StringLines())
			go func() {
				if !s.rm.Stale() {
					return
				}
				buildCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
				defer cancel()
				if err := s.rm.Build(buildCtx); err != nil {
					if !errors.Is(err, ErrNotCodeProject) {
						x.Log("warn", "repomap background rebuild: "+err.Error())
					}
				}
			}()
			x.Log("debug", fmt.Sprintf("[repomap] OnInit complete (%s)", time.Since(start)))
			return
		}

		x.Log("debug", fmt.Sprintf("[repomap] cache miss — quick build start (%s)", time.Since(start)))

		quickCtx, quickCancel := context.WithTimeout(context.Background(), 5*time.Second)
		buildErr := s.rm.Build(quickCtx)
		quickCancel()
		if buildErr == nil {
			x.Log("debug", fmt.Sprintf("[repomap] quick build done (%s)", time.Since(start)))
			s.setBuilt(true)
			s.registerPrompt(x, s.rm.StringLines())
			x.Log("debug", fmt.Sprintf("[repomap] OnInit complete (%s)", time.Since(start)))
			return
		}
		if errors.Is(buildErr, ErrNotCodeProject) {
			x.Log("debug", "skipping repomap: no source files found")
			x.Log("debug", fmt.Sprintf("[repomap] OnInit complete — not a code project (%s)", time.Since(start)))
			return
		}

		x.Log("debug", fmt.Sprintf("[repomap] quick build timed out — continuing in background (%s)", time.Since(start)))

		s.registerPrompt(x, "")
		x.Log("debug", fmt.Sprintf("[repomap] OnInit complete (%s)", time.Since(start)))
		go s.buildInBackground()
	})
}

// registerEventHandler registers the turn_end handler for stale detection.
func (s *repomapState) registerEventHandler(e *sdk.Extension) {
	e.RegisterEventHandler(sdk.EventHandlerDef{
		Name:     "repomap-stale-check",
		Priority: 50,
		Events:   []string{"turn_end"},
		Handle: func(ctx context.Context, eventType string, data json.RawMessage) *sdk.Action {
			if s.rm == nil {
				return nil
			}

			if turnModifiedCode(data) {
				s.rm.Dirty()
			}

			if !s.rm.Stale() {
				return nil
			}
			go func() {
				buildCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
				defer cancel()
				if err := s.rm.Build(buildCtx); err != nil {
					if !errors.Is(err, ErrNotCodeProject) {
						s.ext.Log("warn", "repomap rebuild failed: "+err.Error())
					}
				}
			}()
			return nil
		},
	})
}

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
			return sdk.TextResult(fmt.Sprintf("repomap v%s\n  State: %s", s.version, state)), nil
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
