package cron

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	sdk "github.com/dotcommander/piglet/sdk"

	"github.com/dotcommander/piglet-extensions/internal/xdg"
)

const msgCronBinNotFound = "Error: piglet-cron binary not found. Run `just cli-piglet-cron` to build it."

func registerCommands(e *sdk.Extension) {
	e.RegisterCommand(sdk.CommandDef{
		Name:        "cron",
		Description: "Manage scheduled tasks (list, test, history, add, remove, install, uninstall, status)",
		Handler: func(ctx context.Context, args string) error {
			sub := strings.TrimSpace(args)
			switch {
			case sub == "" || sub == "list":
				return handleCronList(e)
			case sub == "status":
				return handleCronStatus(e)
			case strings.HasPrefix(sub, "test "):
				return handleCronTest(e, strings.TrimPrefix(sub, "test "))
			case strings.HasPrefix(sub, "history"):
				name := strings.TrimSpace(strings.TrimPrefix(sub, "history"))
				return handleCronHistory(e, name)
			case strings.HasPrefix(sub, "add"):
				return handleCronAdd(e)
			case strings.HasPrefix(sub, "remove "):
				return handleCronRemove(e, strings.TrimPrefix(sub, "remove "))
			case sub == "install":
				return handleCronInstall(e)
			case sub == "uninstall":
				return handleCronUninstall(e)
			default:
				e.ShowMessage("Unknown subcommand: " + sub + "\nUsage: /cron [list|test|history|add|remove|install|uninstall|status]")
			}
			return nil
		},
	})
}

func handleCronList(e *sdk.Extension) error {
	summaries, err := ListTasks()
	if err != nil {
		e.ShowMessage("Error: " + err.Error())
		return nil
	}
	if len(summaries) == 0 {
		if schedDir, err := xdg.ExtensionDir("cron"); err == nil {
			e.ShowMessage("No tasks configured. Edit " + filepath.Join(schedDir, "schedules.yaml") + " to add tasks.")
		} else {
			e.ShowMessage("No tasks configured. Add tasks via the schedules.yaml config file.")
		}
		return nil
	}

	sort.Slice(summaries, func(i, j int) bool {
		return summaries[i].Name < summaries[j].Name
	})

	var b strings.Builder
	b.WriteString("**Scheduled Tasks**\n\n")
	for _, s := range summaries {
		status := "enabled"
		if !s.Enabled {
			status = "disabled"
		}
		if s.Overdue {
			status = "OVERDUE"
		}

		lastRun := "never"
		if !s.LastRun.IsZero() {
			lastRun = s.LastRun.Format("2006-01-02 15:04")
		}
		nextRun := s.NextRun.Format("2006-01-02 15:04")

		fmt.Fprintf(&b, "- **%s** [%s] — %s (%s)\n  Last: %s | Next: %s\n",
			s.Name, s.Action, s.Schedule, status, lastRun, nextRun)
	}
	e.ShowMessage(b.String())
	return nil
}

func handleCronStatus(e *sdk.Extension) error {
	cmd := exec.Command("launchctl", "list", "com.piglet.cron")
	out, err := cmd.CombinedOutput()

	var b strings.Builder
	if err != nil {
		b.WriteString("**Cron Status**: not installed\n")
		b.WriteString("Run `/cron install` to set up the launchd agent.\n")
	} else {
		b.WriteString("**Cron Status**: installed\n")
		b.WriteString("```\n")
		b.WriteString(strings.TrimSpace(string(out)))
		b.WriteString("\n```\n")
	}

	summaries, _ := ListTasks()
	enabled, overdue := countTasks(summaries)
	fmt.Fprintf(&b, "\nTasks: %d total, %d enabled", len(summaries), enabled)
	if overdue > 0 {
		fmt.Fprintf(&b, ", **%d overdue**", overdue)
	}
	b.WriteString("\n")

	e.ShowMessage(b.String())
	return nil
}

func handleCronTest(e *sdk.Extension, name string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		e.ShowMessage("Usage: /cron test <task-name>")
		return nil
	}

	bin := pigletCronBin()
	if bin == "" {
		e.ShowMessage(msgCronBinNotFound)
		return nil
	}

	e.ShowMessage(fmt.Sprintf("Running task **%s**...", name))

	cmd := exec.Command(bin, "run", "--verbose", "--task", name)
	out, err := cmd.CombinedOutput()
	output := strings.TrimSpace(string(out))

	if err != nil {
		e.ShowMessage(fmt.Sprintf("Task **%s** failed:\n```\n%s\n```", name, output))
	} else {
		e.ShowMessage(fmt.Sprintf("Task **%s** completed.\n```\n%s\n```", name, output))
	}
	return nil
}

func handleCronHistory(e *sdk.Extension, name string) error {
	entries, err := ReadHistory()
	if err != nil {
		e.ShowMessage("Error reading history: " + err.Error())
		return nil
	}

	entries = filterHistory(entries, name)
	if len(entries) == 0 {
		e.ShowMessage("No history found.")
		return nil
	}

	start := 0
	if len(entries) > 20 {
		start = len(entries) - 20
	}

	var b strings.Builder
	b.WriteString("**Recent History**\n\n")
	for _, entry := range entries[start:] {
		status := "ok"
		if !entry.Success {
			status = "FAIL"
		}
		fmt.Fprintf(&b, "- %s [%s] %s (%dms)",
			entry.RanAt, status, entry.Task, entry.DurationMs)
		if entry.Error != "" {
			fmt.Fprintf(&b, " — %s", entry.Error)
		}
		b.WriteString("\n")
	}
	e.ShowMessage(b.String())
	return nil
}

func handleCronAdd(e *sdk.Extension) error {
	if schedDir, err := xdg.ExtensionDir("cron"); err == nil {
		e.ShowMessage("Edit `" + filepath.Join(schedDir, "schedules.yaml") + "` to add tasks.\nSee the file for examples and documentation.")
	} else {
		e.ShowMessage("Add tasks via the schedules.yaml config file. See the file for examples and documentation.")
	}
	return nil
}

func handleCronRemove(e *sdk.Extension, name string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		e.ShowMessage("Usage: /cron remove <task-name>")
		return nil
	}

	cfg := LoadConfig()
	if _, ok := cfg.Tasks[name]; !ok {
		e.ShowMessage(fmt.Sprintf("Task **%s** not found.", name))
		return nil
	}

	delete(cfg.Tasks, name)
	if err := SaveConfig(cfg); err != nil {
		e.ShowMessage("Error saving config: " + err.Error())
		return nil
	}
	e.ShowMessage(fmt.Sprintf("Task **%s** removed.", name))
	return nil
}

func handleCronInstall(e *sdk.Extension) error {
	bin := pigletCronBin()
	if bin == "" {
		e.ShowMessage(msgCronBinNotFound)
		return nil
	}

	plist := generatePlist(bin)
	path := plistPath()

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		e.ShowMessage("Error creating LaunchAgents dir: " + err.Error())
		return nil
	}

	if err := os.WriteFile(path, []byte(plist), 0o644); err != nil {
		e.ShowMessage("Error writing plist: " + err.Error())
		return nil
	}

	target := fmt.Sprintf("gui/%d", os.Getuid())
	exec.Command("launchctl", "bootout", target, path).Run() //nolint:errcheck // may not be loaded yet

	cmd := exec.Command("launchctl", "bootstrap", target, path)
	if out, err := cmd.CombinedOutput(); err != nil {
		e.ShowMessage(fmt.Sprintf("Error loading agent: %s\n%s", err, string(out)))
		return nil
	}

	e.ShowMessage(fmt.Sprintf("Cron agent installed and loaded.\nPlist: %s\nBinary: %s\nInterval: 60s", path, bin))
	return nil
}

func handleCronUninstall(e *sdk.Extension) error {
	path := plistPath()

	target := fmt.Sprintf("gui/%d", os.Getuid())
	exec.Command("launchctl", "bootout", target, path).Run() //nolint:errcheck // may not be loaded

	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		e.ShowMessage("Error removing plist: " + err.Error())
		return nil
	}

	e.ShowMessage("Cron agent uninstalled.")
	return nil
}
