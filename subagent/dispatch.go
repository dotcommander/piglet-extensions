package subagent

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"

	sdk "github.com/dotcommander/piglet/sdk"
)

const (
	defaultTimeout      = 5 * time.Minute
	defaultPollInterval = 500 * time.Millisecond
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

	shellCmd := buildShellCmd(task, args, resultPath, agentID)
	tmuxArgs := tmuxSpawnArgs(args, agentID, shellCmd)

	cmd := exec.CommandContext(ctx, "tmux", tmuxArgs...)
	if out, err := cmd.CombinedOutput(); err != nil {
		return sdk.ErrorResult(fmt.Sprintf("tmux spawn failed: %v: %s", err, string(out))), nil
	}

	return pollResult(ctx, resultPath, agentID, defaultTimeout, defaultPollInterval)
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
		return []string{"split-window", "-v", shellCmd}
	case "window":
		return []string{"new-window", "-n", "agent-" + agentID, shellCmd}
	default:
		return []string{"split-window", "-h", shellCmd}
	}
}

func pollResult(ctx context.Context, resultPath, agentID string, timeout, pollInterval time.Duration) (*sdk.ToolResult, error) {
	deadline := time.Now().Add(timeout)
	timer := time.NewTimer(pollInterval)
	defer timer.Stop()

	for {
		select {
		case <-ctx.Done():
			return sdk.ErrorResult("dispatch cancelled"), nil
		case <-timer.C:
		}

		if time.Now().After(deadline) {
			return sdk.TextResult(fmt.Sprintf("[agent %s timed out after %s — check tmux pane for status]", agentID, timeout)), nil
		}

		if data, err := os.ReadFile(resultPath); err == nil {
			result := strings.TrimSpace(string(data))
			if result == "" {
				return sdk.TextResult(fmt.Sprintf("[agent %s completed with no output]", agentID)), nil
			}
			return sdk.TextResult(fmt.Sprintf("[agent %s]\n\n%s", agentID, result)), nil
		}

		timer.Reset(pollInterval)
	}
}
