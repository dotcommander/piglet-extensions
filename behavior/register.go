// Package behavior loads behavioral guidelines from
// ~/.config/piglet/extensions/behavior/behavior.md and injects them
// as the earliest system prompt section.
package behavior

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/dotcommander/piglet-extensions/internal/xdg"
	sdk "github.com/dotcommander/piglet/sdk"
)

const behaviorOrder = 10

var (
	mu       sync.Mutex
	version  string
	loaded   string // content loaded from behavior.md
	filePath string // resolved file path
)

// Register adds the behavior prompt section loader and status tool.
func Register(e *sdk.Extension, ver string) {
	mu.Lock()
	version = ver
	mu.Unlock()

	e.OnInitAppend(func(ext *sdk.Extension) {
		content := strings.TrimSpace(xdg.LoadOrCreateExt("behavior", "behavior.md", ""))

		mu.Lock()
		loaded = content
		if dir, err := xdg.ExtensionDir("behavior"); err == nil {
			filePath = dir + "/behavior.md"
		}
		mu.Unlock()

		if content == "" {
			return
		}
		ext.RegisterPromptSection(sdk.PromptSectionDef{
			Title:   "Guidelines",
			Content: content,
			Order:   behaviorOrder,
		})
	})

	e.RegisterTool(behaviorStatusTool())
}

func behaviorStatusTool() sdk.ToolDef {
	return sdk.ToolDef{
		Name:        "behavior_status",
		Description: "Show behavior extension status: loaded guidelines summary, file path, version",
		Execute: func(ctx context.Context, args map[string]any) (*sdk.ToolResult, error) {
			mu.Lock()
			ver := version
			content := loaded
			path := filePath
			mu.Unlock()

			state := "empty (no guidelines loaded)"
			lineCount := 0
			if strings.TrimSpace(content) != "" {
				state = "active"
				lineCount = strings.Count(content, "\n") + 1
			}

			var b strings.Builder
			fmt.Fprintf(&b, "behavior %s\n", ver)
			fmt.Fprintf(&b, "  State: %s\n", state)
			fmt.Fprintf(&b, "  File:  %s\n", path)
			if strings.TrimSpace(content) != "" {
				fmt.Fprintf(&b, "  Lines: %d\n", lineCount)
				// Show first few lines as preview
				lines := strings.Split(strings.TrimSpace(content), "\n")
				preview := 5
				if len(lines) < preview {
					preview = len(lines)
				}
				fmt.Fprintf(&b, "  Preview:\n")
				for _, l := range lines[:preview] {
					fmt.Fprintf(&b, "    %s\n", l)
				}
				if len(lines) > 5 {
					fmt.Fprintf(&b, "    ... (%d more lines)\n", len(lines)-5)
				}
			}

			return sdk.TextResult(b.String()), nil
		},
	}
}
