package subagent

import (
	"bytes"
	"context"
	"fmt"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"

	sdk "github.com/dotcommander/piglet/sdk"
)

const (
	defaultAbsoluteTimeout   = 30 * time.Minute
	defaultInactivityTimeout = 5 * time.Minute
	pollInterval             = 500 * time.Millisecond
)

func dispatch(ctx context.Context, args map[string]any) (*sdk.ToolResult, error) {
	task, _ := args["task"].(string)
	if task == "" {
		return sdk.ErrorResult("task is required"), nil
	}

	if os.Getenv("TMUX") == "" {
		return sdk.ErrorResult("dispatch requires tmux — run piglet inside a tmux session"), nil
	}
	if _, err := exec.LookPath("tmux"); err != nil {
		return sdk.ErrorResult("tmux not found in PATH"), nil
	}

	agentID := uuid.New().String()[:8]
	tmpDir := filepath.Join(os.TempDir(), "piglet-agent-"+agentID)
	if err := os.MkdirAll(tmpDir, 0700); err != nil {
		return sdk.ErrorResult(fmt.Sprintf("create agent dir: %v", err)), nil
	}
	defer os.RemoveAll(tmpDir)

	resultPath := filepath.Join(tmpDir, "result.md")

	absTimeout := durationFromMs(args, "absolute_timeout_ms", defaultAbsoluteTimeout)
	inactTimeout := durationFromMs(args, "inactivity_timeout_ms", defaultInactivityTimeout)

	shellCmd := buildShellCmd(task, args, resultPath, agentID)
	tmuxArgs := tmuxSpawnArgs(args, agentID, shellCmd)

	cmd := exec.CommandContext(ctx, "tmux", tmuxArgs...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return sdk.ErrorResult(fmt.Sprintf("tmux spawn failed: %v: %s", err, stderr.String())), nil
	}

	paneID := strings.TrimSpace(stdout.String())

	return pollResult(ctx, resultPath, agentID, paneID, absTimeout, inactTimeout)
}

func buildShellCmd(task string, args map[string]any, resultPath, agentID string) string {
	var cmdParts []string
	cmdParts = append(cmdParts, "piglet")
	if model, _ := args["model"].(string); model != "" {
		cmdParts = append(cmdParts, "--result", resultPath, "--model", model)
	} else {
		cmdParts = append(cmdParts, "--result", resultPath)
	}
	cmdParts = append(cmdParts, fmt.Sprintf("%q", task))

	pigletCmd := strings.Join(cmdParts, " ")
	return fmt.Sprintf("%s; echo ''; echo '[agent %s complete — press enter to close]'; read", pigletCmd, agentID)
}

func tmuxSpawnArgs(args map[string]any, agentID, shellCmd string) []string {
	split, _ := args["split"].(string)
	switch split {
	case "vertical":
		return []string{"split-window", "-P", "-F", "#{pane_id}", "-v", shellCmd}
	case "window":
		return []string{"new-window", "-P", "-F", "#{pane_id}", "-n", "agent-" + agentID, shellCmd}
	default:
		return []string{"split-window", "-P", "-F", "#{pane_id}", "-h", shellCmd}
	}
}

func pollResult(ctx context.Context, resultPath, agentID, paneID string, absTimeout, inactTimeout time.Duration) (*sdk.ToolResult, error) {
	absDeadline := time.Now().Add(absTimeout)
	lastActivity := time.Time{}
	timer := time.NewTimer(pollInterval)
	defer timer.Stop()

	for {
		select {
		case <-ctx.Done():
			return sdk.ErrorResult("dispatch cancelled"), nil
		case <-timer.C:
		}

		now := time.Now()

		// (1) Result file ready → normal completion
		if data, err := os.ReadFile(resultPath); err == nil {
			result := strings.TrimSpace(string(data))
			if result == "" {
				return sdk.TextResult(fmt.Sprintf("[agent %s completed with no output]", agentID)), nil
			}
			return sdk.TextResult(fmt.Sprintf("[agent %s]\n\n%s", agentID, result)), nil
		}

		// (2) Absolute deadline exceeded
		if now.After(absDeadline) {
			killPane(ctx, paneID)
			return sdk.TextResult(fmt.Sprintf("[agent %s timed out after %s — check tmux pane for status]", agentID, absTimeout)), nil
		}

		// (3) Inactivity deadline exceeded
		if inactTimeout > 0 && paneStalled(now, lastActivity, inactTimeout) {
			killPane(ctx, paneID)
			return sdk.TextResult(fmt.Sprintf("[agent %s killed — no output for %s (inactivity timeout)]", agentID, inactTimeout)), nil
		}

		// (4) Update activity, reset timer
		if paneID != "" && inactTimeout > 0 {
			if t, err := queryPaneActivity(ctx, paneID); err == nil && !t.IsZero() {
				if t.After(lastActivity) {
					lastActivity = t
				}
			}
		}

		timer.Reset(pollInterval)
	}
}

// paneStalled returns true if the pane has produced no output for longer than limit.
// Zero lastActivity means fresh pane — not stalled.
func paneStalled(now, lastActivity time.Time, limit time.Duration) bool {
	if lastActivity.IsZero() {
		return false
	}
	return now.Sub(lastActivity) > limit
}

// queryPaneActivity runs tmux display-message to get the pane's last activity unix timestamp.
func queryPaneActivity(ctx context.Context, paneID string) (time.Time, error) {
	cmd := exec.CommandContext(ctx, "tmux", "display-message", "-p", "-t", paneID, "#{pane_last_activity}")
	out, err := cmd.Output()
	if err != nil {
		return time.Time{}, err
	}
	secs, err := strconv.ParseInt(strings.TrimSpace(string(out)), 10, 64)
	if err != nil {
		return time.Time{}, err
	}
	return time.Unix(secs, 0), nil
}

// killPane best-effort kills a tmux pane, ignoring all errors.
func killPane(ctx context.Context, paneID string) {
	_ = exec.CommandContext(ctx, "tmux", "kill-pane", "-t", paneID).Run()
}

// durationFromMs reads a millisecond value from args, converting to time.Duration.
// Missing/wrong type → fallback. <= 0 → 0 (disabled).
func durationFromMs(args map[string]any, key string, fallback time.Duration) time.Duration {
	v, ok := args[key]
	if !ok {
		return fallback
	}
	f, ok := v.(float64)
	if !ok || math.IsNaN(f) {
		return fallback
	}
	if f <= 0 {
		return 0
	}
	return time.Duration(int64(f)) * time.Millisecond
}
