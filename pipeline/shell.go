package pipeline

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"
	"unicode/utf8"
)

// shellRun executes a shell command and returns stdout+stderr combined.
func shellRun(ctx context.Context, shell, command, workDir string, env map[string]string, timeout time.Duration) (string, error) {
	if shell == "" {
		shell = "sh"
	}

	cmdCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	cmd := exec.CommandContext(cmdCtx, shell, "-c", command)
	if workDir != "" {
		cmd.Dir = workDir
	}

	if len(env) > 0 {
		cmd.Env = cmd.Environ()
		for k, v := range env {
			cmd.Env = append(cmd.Env, k+"="+v)
		}
	}

	var buf bytes.Buffer
	cmd.Stdout = &buf
	cmd.Stderr = &buf

	if err := cmd.Run(); err != nil {
		return buf.String(), fmt.Errorf("%w: %s", err, strings.TrimSpace(buf.String()))
	}
	return buf.String(), nil
}

// shellPredicate runs a shell command and returns true if exit code is 0.
func shellPredicate(ctx context.Context, command, workDir string) bool {
	cmdCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	cmd := exec.CommandContext(cmdCtx, "sh", "-c", command)
	if workDir != "" {
		cmd.Dir = workDir
	}
	return cmd.Run() == nil
}

// TruncateUTF8 safely truncates a string to at most maxBytes without splitting
// a multi-byte character. The suffix is included within the budget.
func TruncateUTF8(s string, maxBytes int) string {
	if len(s) <= maxBytes {
		return s
	}
	const suffix = "... (truncated)"
	cut := maxBytes - len(suffix)
	if cut <= 0 {
		return s[:maxBytes]
	}
	// Walk back from cut to find a valid UTF-8 boundary
	for cut > 0 && !utf8.RuneStart(s[cut]) {
		cut--
	}
	return s[:cut] + suffix
}
