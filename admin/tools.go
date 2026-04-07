package admin

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	sdk "github.com/dotcommander/piglet/sdk"

	"github.com/dotcommander/piglet-extensions/internal/xdg"
)

// toolConfigList returns a tool that lists all config files with status.
func toolConfigList() sdk.ToolDef {
	return sdk.ToolDef{
		Name:        "admin_config_list",
		Description: "List all piglet configuration files with their existence status and paths",
		Parameters: map[string]any{
			"type":       "object",
			"properties": map[string]any{},
		},
		PromptHint: "Show piglet config files and their status",
		Execute: func(_ context.Context, _ map[string]any) (*sdk.ToolResult, error) {
			dir, err := xdg.ConfigDir()
			if err != nil {
				return sdk.ErrorResult("cannot determine config dir: " + err.Error()), nil
			}
			entries := scanConfigDir(dir)
			var b strings.Builder
			fmt.Fprintf(&b, "Config directory: %s\n", dir)
			for _, entry := range entries {
				fmt.Fprintf(&b, "  %s: %s\n", entry.label, formatFileStatus(entry))
			}
			return sdk.TextResult(b.String()), nil
		},
	}
}

// toolConfigRead returns a tool that reads a config file's contents.
func toolConfigRead() sdk.ToolDef {
	return sdk.ToolDef{
		Name:        "admin_config_read",
		Description: "Read the contents of a piglet configuration file",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"filename": map[string]any{
					"type":        "string",
					"description": "Config file name (e.g. 'config.yaml', 'behavior.md')",
				},
			},
			"required": []string{"filename"},
		},
		PromptHint: "Read contents of a piglet config file",
		Execute: func(_ context.Context, args map[string]any) (*sdk.ToolResult, error) {
			filename, _ := args["filename"].(string)
			if filename == "" {
				return sdk.ErrorResult("filename is required"), nil
			}
			if strings.Contains(filename, "..") || strings.Contains(filename, "/") {
				return sdk.ErrorResult("filename must be a simple name, not a path"), nil
			}
			dir, err := xdg.ConfigDir()
			if err != nil {
				return sdk.ErrorResult("cannot determine config dir: " + err.Error()), nil
			}
			data, err := os.ReadFile(filepath.Join(dir, filename))
			if err != nil {
				if os.IsNotExist(err) {
					return sdk.TextResult(fmt.Sprintf("File %s does not exist in %s", filename, dir)), nil
				}
				return sdk.ErrorResult("read error: " + err.Error()), nil
			}
			return sdk.TextResult(string(data)), nil
		},
	}
}
