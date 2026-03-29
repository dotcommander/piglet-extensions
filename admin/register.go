package admin

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/dotcommander/piglet-extensions/internal/xdg"
	"github.com/dotcommander/piglet/sdk"
)

// Register registers the admin extension's commands.
func Register(e *sdk.Extension) {
	e.RegisterCommand(sdk.CommandDef{
		Name:        "config",
		Description: "Show config paths and status. Use --setup for initial config.",
		Handler: func(_ context.Context, args string) error {
			if strings.TrimSpace(args) == "--setup" {
				e.ShowMessage("Run 'piglet init' from the command line to set up config.")
				return nil
			}

			dir, err := xdg.ConfigDir()
			if err != nil {
				e.ShowMessage("Cannot determine config dir: " + err.Error())
				return nil
			}

			var b strings.Builder
			b.WriteString("Config directory: " + dir + "\n")

			files := []struct {
				label, path string
			}{
				{"config.yaml", filepath.Join(dir, "config.yaml")},
				{"behavior.md", filepath.Join(dir, "behavior.md")},
				{"auth.json", filepath.Join(dir, "auth.json")},
				{"sessions/", filepath.Join(dir, "sessions")},
			}

			for _, f := range files {
				b.WriteString("  " + f.label + ":  ")
				if _, err := os.Stat(f.path); err == nil {
					b.WriteString(f.path + "\n")
				} else {
					b.WriteString("(not created)\n")
				}
			}

			e.ShowMessage(b.String())
			return nil
		},
	})

	e.RegisterCommand(sdk.CommandDef{
		Name:        "models-sync",
		Description: "Sync model catalog from models.dev",
		Handler: func(ctx context.Context, _ string) error {
			e.ShowMessage("Syncing models from models.dev...")
			updated, err := e.SyncModels(ctx)
			if err != nil {
				e.ShowMessage("Sync failed: " + err.Error())
				return nil
			}
			if updated == 0 {
				e.ShowMessage("All models up to date.")
			} else {
				e.ShowMessage(fmt.Sprintf("Sync complete: %d model(s) updated", updated))
			}
			return nil
		},
	})
}
