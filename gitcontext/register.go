// Package gitcontext injects uncommitted changes, recent commits, and
// small diffs into the system prompt so the model knows the repo state.
package gitcontext

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"sync"
	"time"

	sdk "github.com/dotcommander/piglet/sdk"
)

const (
	promptOrder = 40 // before memory (50)

	defaultMaxDiffStatFiles  = 30
	defaultMaxLogLines       = 5
	defaultMaxDiffHunkLines  = 50
	defaultGitCommandTimeout = 5 * time.Second
)

// Register schedules OnInit work via OnInitAppend to build and inject
// git context into the system prompt.
func Register(e *sdk.Extension) {
	e.OnInitAppend(func(ext *sdk.Extension) {
		cwd := ext.CWD()
		content := buildGitContext(cwd)

		if content == "" {
			return
		}
		ext.RegisterPromptSection(sdk.PromptSectionDef{
			Title:   "Recent Changes",
			Content: content,
			Order:   promptOrder,
		})
	})
}

func buildGitContext(cwd string) string {
	timeout := defaultGitCommandTimeout

	var diffStat, log, hunks string
	var wg sync.WaitGroup
	wg.Add(3)

	go func() {
		defer wg.Done()
		diffStat = gitRun(cwd, timeout, "diff", "--stat")
	}()
	go func() {
		defer wg.Done()
		log = gitRun(cwd, timeout, "log", "--oneline", fmt.Sprintf("-%d", defaultMaxLogLines))
	}()
	go func() {
		defer wg.Done()
		hunks = gitRun(cwd, timeout, "diff", "--no-color")
	}()
	wg.Wait()

	var b strings.Builder

	if diffStat != "" {
		b.WriteString("Uncommitted changes:\n```\n")
		b.WriteString(capDiffStat(diffStat, defaultMaxDiffStatFiles))
		b.WriteString("```\n\n")
	}

	if log != "" {
		b.WriteString("Recent commits:\n```\n")
		b.WriteString(log)
		b.WriteString("```\n\n")
	}

	if hunks != "" {
		lines := strings.Split(hunks, "\n")
		if len(lines) <= defaultMaxDiffHunkLines {
			b.WriteString("Diff:\n```diff\n")
			b.WriteString(hunks)
			b.WriteString("\n```\n")
		}
	}

	return strings.TrimSpace(b.String())
}

func capDiffStat(stat string, maxFiles int) string {
	lines := strings.Split(stat, "\n")
	if len(lines) <= maxFiles+1 {
		return stat
	}
	kept := lines[:maxFiles]
	remaining := len(lines) - maxFiles - 1
	summary := lines[len(lines)-1]
	kept = append(kept, fmt.Sprintf(" ... and %d more files", remaining))
	kept = append(kept, summary)
	return strings.Join(kept, "\n")
}

func gitRun(cwd string, timeout time.Duration, args ...string) string {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = cwd
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}
