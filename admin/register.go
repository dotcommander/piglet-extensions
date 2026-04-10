package admin

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	sdk "github.com/dotcommander/piglet/sdk"

	"github.com/dotcommander/piglet-extensions/internal/xdg"
)

// Version is the admin extension version.
const Version = "0.3.0"

// configFile is a config file entry for display.
type configFile struct {
	label, path string
}

// knownConfigFiles maps well-known config files to human-readable labels.
// Entries shown first even if missing, for discoverability.
var knownConfigFiles = []struct {
	name, label string
	isDir       bool
}{
	{"config.yaml", "config.yaml", false},
	{"behavior.md", "behavior.md", false},
	{"auth.json", "auth.json", false},
	{"models.yaml", "models.yaml", false},
	{"sessions", "sessions/", true},
}

// scanConfigDir reads the piglet config directory and returns all entries.
// Well-known files appear first (even if missing), followed by any additional entries.
func scanConfigDir(dir string) []configFile {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return defaultConfigEntries(dir)
	}

	var files []configFile
	shown := make(map[string]bool)

	// Known files first
	for _, k := range knownConfigFiles {
		shown[k.name] = true
		files = append(files, configFile{k.label, filepath.Join(dir, k.name)})
	}

	// Additional entries sorted alphabetically
	var extra []configFile
	for _, entry := range entries {
		name := entry.Name()
		if shown[name] || strings.HasPrefix(name, ".") {
			continue
		}
		label := name
		if entry.IsDir() {
			label += "/"
		}
		extra = append(extra, configFile{label, filepath.Join(dir, name)})
	}
	sort.Slice(extra, func(i, j int) bool { return extra[i].label < extra[j].label })
	files = append(files, extra...)

	return files
}

// defaultConfigEntries returns the well-known entries when the config dir doesn't exist yet.
func defaultConfigEntries(dir string) []configFile {
	files := make([]configFile, len(knownConfigFiles))
	for i, k := range knownConfigFiles {
		files[i] = configFile{k.label, filepath.Join(dir, k.name)}
	}
	return files
}

// formatFileStatus returns a human-readable status line for a config file.
func formatFileStatus(cf configFile) string {
	_, err := os.Stat(cf.path)
	if err == nil {
		return cf.path
	}
	if os.IsNotExist(err) {
		return "(not created)"
	}
	return "(error: " + err.Error() + ")"
}

type messenger interface {
	ShowMessage(msg string)
}

// runSetup bootstraps the config directory with default files, then shows the status listing.
func runSetup(e messenger, dir string) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		e.ShowMessage("Cannot create config dir: " + err.Error())
		return
	}
	var created []string
	for _, name := range []string{"config.yaml", "behavior.md"} {
		p := filepath.Join(dir, name)
		if _, err := os.Stat(p); os.IsNotExist(err) {
			if f, err := os.Create(p); err == nil {
				f.Close()
				created = append(created, name)
			}
		}
	}
	sessionsDir := filepath.Join(dir, "sessions")
	if _, err := os.Stat(sessionsDir); os.IsNotExist(err) {
		if err := os.MkdirAll(sessionsDir, 0o755); err == nil {
			created = append(created, "sessions/")
		}
	}
	if len(created) == 0 {
		e.ShowMessage("Config already set up at " + dir)
	} else {
		e.ShowMessage("Created: " + strings.Join(created, ", ") + " in " + dir)
	}

	// Always show listing after setup
	showConfigListing(e, dir)
}

// showConfigListing displays all config files with their status.
func showConfigListing(e messenger, dir string) {
	var b strings.Builder
	b.WriteString("Config directory: " + dir + "\n")
	for _, cf := range scanConfigDir(dir) {
		b.WriteString("  " + cf.label + ":  " + formatFileStatus(cf) + "\n")
	}
	e.ShowMessage(b.String())
}

// configDir returns the piglet config directory or shows an error.
func configDir(e messenger) (string, bool) {
	dir, err := xdg.ConfigDir()
	if err != nil {
		e.ShowMessage("Cannot determine config dir: " + err.Error())
		return "", false
	}
	return dir, true
}

// Register registers all admin extension capabilities.
func Register(e *sdk.Extension) {
	// Tools for LLM access
	e.RegisterTool(toolConfigList())
	e.RegisterTool(toolConfigRead())

	// Primary command with subcommands
	e.RegisterCommand(sdk.CommandDef{
		Name:        "config",
		Description: "Inspect and manage piglet configuration",
		Handler:     configCommand(e),
	})

	// Status tool
	e.RegisterTool(sdk.ToolDef{
		Name:        "admin_status",
		Description: "Show admin extension status: version and config directory path.",
		Parameters:  map[string]any{"type": "object", "properties": map[string]any{}},
		Execute: func(_ context.Context, _ map[string]any) (*sdk.ToolResult, error) {
			dir, err := xdg.ConfigDir()
			if err != nil {
				return sdk.ErrorResult("cannot determine config dir: " + err.Error()), nil
			}
			return sdk.TextResult(fmt.Sprintf("admin %s\n  Config dir: %s\n", Version, dir)), nil
		},
	})

	// Aliases
	e.RegisterCommand(sdk.CommandDef{
		Name:        "status",
		Description: "Show piglet config file status (alias for /config)",
		Handler: func(_ context.Context, _ string) error {
			if dir, ok := configDir(e); ok {
				showConfigListing(e, dir)
			}
			return nil
		},
	})
	e.RegisterCommand(sdk.CommandDef{
		Name:        "settings",
		Description: "Show piglet config file status (alias for /config)",
		Handler: func(_ context.Context, _ string) error {
			if dir, ok := configDir(e); ok {
				showConfigListing(e, dir)
			}
			return nil
		},
	})
}
