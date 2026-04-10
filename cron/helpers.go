package cron

import (
	_ "embed"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"text/template"

	sdk "github.com/dotcommander/piglet/sdk"

	"github.com/dotcommander/piglet-extensions/internal/xdg"
)

// argStr extracts a string argument from the args map.
func argStr(args map[string]any, key string) string {
	v, _ := args[key].(string)
	return v
}

// filterHistory filters run entries by task name. Empty name returns all.
func filterHistory(entries []RunEntry, name string) []RunEntry {
	if name == "" {
		return entries
	}
	var filtered []RunEntry
	for _, entry := range entries {
		if entry.Task == name {
			filtered = append(filtered, entry)
		}
	}
	return filtered
}

// removeTask deletes a task from the config and persists it.
func removeTask(cfg Config, name string) (*sdk.ToolResult, error) {
	delete(cfg.Tasks, name)
	if err := SaveConfig(cfg); err != nil {
		return sdk.ErrorResult("Error saving: " + err.Error()), nil
	}
	return sdk.TextResult(fmt.Sprintf("Task %q removed.", name)), nil
}

// countTasks returns enabled and overdue counts from summaries.
// If nil, loads from config.
func countTasks(summaries []TaskSummary) (enabled, overdue int) {
	if summaries == nil {
		var err error
		summaries, err = ListTasks()
		if err != nil {
			return 0, 0
		}
	}
	for _, s := range summaries {
		if s.Enabled {
			enabled++
		}
		if s.Overdue {
			overdue++
		}
	}
	return enabled, overdue
}

func pigletCronBin() string {
	home, _ := os.UserHomeDir()
	bin := filepath.Join(home, "go", "bin", "piglet-cron")
	if _, err := os.Stat(bin); err == nil {
		return bin
	}
	if p, err := exec.LookPath("piglet-cron"); err == nil {
		return p
	}
	return ""
}

func plistPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, "Library", "LaunchAgents", "com.piglet.cron.plist")
}

//go:embed defaults/launchd.plist
var plistTmpl string

func generatePlist(binPath string) string {
	configDir, err := xdg.ConfigDir()
	if err != nil {
		home, _ := os.UserHomeDir()
		configDir = filepath.Join(home, ".config", "piglet")
	}
	logDir := filepath.Join(configDir, "logs")
	os.MkdirAll(logDir, 0o755) //nolint:errcheck // best-effort log dir creation

	tmpl := template.Must(template.New("plist").Parse(plistTmpl))
	var b strings.Builder
	_ = tmpl.Execute(&b, map[string]string{
		"BinPath": binPath,
		"LogDir":  logDir,
	})
	return b.String()
}
