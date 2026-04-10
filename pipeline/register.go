package pipeline

import (
	"context"
	_ "embed"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/dotcommander/piglet-extensions/internal/xdg"
	"github.com/dotcommander/piglet/sdk"
)

//go:embed defaults/prompt.md
var defaultPrompt string

const (
	Version      = "0.2.0"
	pipelinesDir = "pipelines"
)

// configDir is set during OnInit and used by tool/command handlers.
var configDir string

// Register registers the pipeline extension's tools and commands.
func Register(e *sdk.Extension) {
	e.OnInit(func(ext *sdk.Extension) {
		home, err := xdg.ConfigDir()
		if err != nil {
			return
		}
		configDir = home

		content := xdg.LoadOrCreateExt("pipeline", "prompt.md", strings.TrimSpace(defaultPrompt))
		if content != "" {
			ext.RegisterPromptSection(sdk.PromptSectionDef{
				Title:   "Pipelines",
				Content: content,
				Order:   75,
			})
		}
	})

	e.RegisterTool(sdk.ToolDef{
		Name:              "pipeline",
		Description:       "Run a multi-step workflow. Load a saved pipeline by name from ~/.config/piglet/pipelines/ or provide an inline pipeline definition. Steps execute sequentially with output passing, retries, loops, and error handling.",
		InterruptBehavior: "block",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"name": map[string]any{
					"type":        "string",
					"description": "Name of a saved pipeline (loads from ~/.config/piglet/pipelines/<name>.yaml)",
				},
				"inline": map[string]any{
					"type":        "object",
					"description": "Ad-hoc pipeline definition. Same schema as YAML: name, description, steps[], params{}.",
				},
				"params": map[string]any{
					"type":        "object",
					"description": "Parameter overrides as key-value pairs.",
					"additionalProperties": map[string]any{
						"type": "string",
					},
				},
				"dry_run": map[string]any{
					"type":        "boolean",
					"description": "Preview all steps without executing (default: false).",
				},
			},
		},
		Execute: func(ctx context.Context, args map[string]any) (*sdk.ToolResult, error) {
			return executePipeline(ctx, args)
		},
	})

	e.RegisterTool(sdk.ToolDef{
		Name:        "pipeline_list",
		Description: "List all saved pipelines in ~/.config/piglet/pipelines/.",
		Parameters: map[string]any{
			"type":       "object",
			"properties": map[string]any{},
		},
		Execute: func(ctx context.Context, args map[string]any) (*sdk.ToolResult, error) {
			return listPipelines()
		},
	})

	e.RegisterTool(sdk.ToolDef{
		Name:        "pipeline_status",
		Description: "Show pipeline extension status: version, pipeline directory, and pipeline count.",
		Parameters: map[string]any{
			"type":       "object",
			"properties": map[string]any{},
		},
		Execute: func(_ context.Context, _ map[string]any) (*sdk.ToolResult, error) {
			if configDir == "" {
				return sdk.TextResult(fmt.Sprintf("pipeline v%s\nState: not initialized", Version)), nil
			}
			dir := filepath.Join(configDir, pipelinesDir)
			pipes, err := LoadDir(dir)
			if err != nil {
				return sdk.TextResult(fmt.Sprintf("pipeline v%s\nDir: %s\nError: %s", Version, dir, err)), nil
			}
			return sdk.TextResult(fmt.Sprintf("pipeline v%s\nDir: %s\nPipelines: %d", Version, dir, len(pipes))), nil
		},
	})

	e.RegisterCommand(sdk.CommandDef{
		Name:        "pipe",
		Description: "Run a saved pipeline: /pipe <name> [--param key=value ...] [--dry-run]",
		Handler: func(ctx context.Context, args string) error {
			return handlePipeCommand(ctx, e, args)
		},
	})

	e.RegisterCommand(sdk.CommandDef{
		Name:        "pipe-new",
		Description: "Create a new pipeline template: /pipe-new <name>",
		Handler: func(ctx context.Context, args string) error {
			return handlePipeNewCommand(e, args)
		},
	})
}
