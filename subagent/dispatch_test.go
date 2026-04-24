package subagent

import (
	"context"
	"math"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestBuildShellCmd(t *testing.T) {
	t.Parallel()

	resultPath := "/tmp/piglet-agent-abc12345/result.md"

	t.Run("with model", func(t *testing.T) {
		t.Parallel()
		args := map[string]any{"task": "fix bugs", "model": "anthropic/claude-haiku-4-5"}
		got := buildShellCmd("fix bugs", args, resultPath, "abc12345")

		assert.Contains(t, got, "piglet")
		assert.Contains(t, got, "--result "+resultPath)
		assert.Contains(t, got, "--model anthropic/claude-haiku-4-5")
		assert.Contains(t, got, `"fix bugs"`)
		assert.Contains(t, got, "[agent abc12345 complete")
	})

	t.Run("without model", func(t *testing.T) {
		t.Parallel()
		args := map[string]any{"task": "fix bugs"}
		got := buildShellCmd("fix bugs", args, resultPath, "abc12345")

		assert.Contains(t, got, "piglet")
		assert.Contains(t, got, "--result "+resultPath)
		assert.NotContains(t, got, "--model")
	})

	t.Run("shell quoting", func(t *testing.T) {
		t.Parallel()
		args := map[string]any{"task": "it's a \"test\""}
		got := buildShellCmd("it's a \"test\"", args, resultPath, "abc12345")

		assert.Contains(t, got, `it's a \"test\"`)
	})
}

func TestTmuxSpawnArgs(t *testing.T) {
	t.Parallel()

	agentID := "abc12345"
	shellCmd := "piglet --result /tmp/result.md 'do thing'"

	t.Run("horizontal default", func(t *testing.T) {
		t.Parallel()
		args := map[string]any{"split": ""}
		got := tmuxSpawnArgs(args, agentID, shellCmd)

		assert.Equal(t, []string{"split-window", "-P", "-F", "#{pane_id}", "-h", shellCmd}, got)
	})

	t.Run("vertical", func(t *testing.T) {
		t.Parallel()
		args := map[string]any{"split": "vertical"}
		got := tmuxSpawnArgs(args, agentID, shellCmd)

		assert.Equal(t, []string{"split-window", "-P", "-F", "#{pane_id}", "-v", shellCmd}, got)
	})

	t.Run("window", func(t *testing.T) {
		t.Parallel()
		args := map[string]any{"split": "window"}
		got := tmuxSpawnArgs(args, agentID, shellCmd)

		assert.Equal(t, []string{"new-window", "-P", "-F", "#{pane_id}", "-n", "agent-" + agentID, shellCmd}, got)
	})

	t.Run("unknown defaults to horizontal", func(t *testing.T) {
		t.Parallel()
		args := map[string]any{"split": "garbage"}
		got := tmuxSpawnArgs(args, agentID, shellCmd)

		assert.Equal(t, []string{"split-window", "-P", "-F", "#{pane_id}", "-h", shellCmd}, got)
	})

	t.Run("nil args defaults to horizontal", func(t *testing.T) {
		t.Parallel()
		got := tmuxSpawnArgs(nil, agentID, shellCmd)

		assert.Equal(t, []string{"split-window", "-P", "-F", "#{pane_id}", "-h", shellCmd}, got)
	})
}

func TestPollResult_Success(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	resultPath := filepath.Join(dir, "result.md")

	content := "Found 3 issues in main.go"
	if err := os.WriteFile(resultPath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	result, err := pollResult(context.Background(), resultPath, "testid", "", time.Second, 0)
	assert.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Content[0].Text, "[agent testid]")
	assert.Contains(t, result.Content[0].Text, content)
}

func TestPollResult_EmptyResult(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	resultPath := filepath.Join(dir, "result.md")

	if err := os.WriteFile(resultPath, []byte("  "), 0644); err != nil {
		t.Fatal(err)
	}

	result, err := pollResult(context.Background(), resultPath, "testid", "", time.Second, 0)
	assert.NoError(t, err)
	assert.Contains(t, result.Content[0].Text, "completed with no output")
}

func TestPollResult_Cancellation(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	resultPath := filepath.Join(dir, "result.md")

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	result, err := pollResult(ctx, resultPath, "testid", "", time.Second, 0)
	assert.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Content[0].Text, "cancelled")
}

func TestPollResult_Timeout(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	resultPath := filepath.Join(dir, "result.md")
	// Don't create file — will timeout

	result, err := pollResult(context.Background(), resultPath, "testid", "", 50*time.Millisecond, 0)
	assert.NoError(t, err)
	assert.Contains(t, result.Content[0].Text, "timed out")
}

func TestPaneStalled(t *testing.T) {
	t.Parallel()

	now := time.Now()

	tests := []struct {
		name         string
		lastActivity time.Time
		limit        time.Duration
		want         bool
	}{
		{"zero last activity is not stalled", time.Time{}, time.Minute, false},
		{"active recently is not stalled", now.Add(-30 * time.Second), time.Minute, false},
		{"exactly at limit is not stalled", now.Add(-time.Minute), time.Minute, false},
		{"over limit is stalled", now.Add(-2 * time.Minute), time.Minute, true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tc.want, paneStalled(now, tc.lastActivity, tc.limit))
		})
	}
}

func TestDurationFromMs(t *testing.T) {
	t.Parallel()

	fallback := 5 * time.Minute

	tests := []struct {
		name string
		args map[string]any
		key  string
		want time.Duration
	}{
		{"missing key returns fallback", map[string]any{"other": 1000.0}, "timeout_ms", fallback},
		{"wrong type returns fallback", map[string]any{"timeout_ms": "1000"}, "timeout_ms", fallback},
		{"negative returns disabled", map[string]any{"timeout_ms": -500.0}, "timeout_ms", 0},
		{"zero returns disabled", map[string]any{"timeout_ms": 0.0}, "timeout_ms", 0},
		{"positive returns duration", map[string]any{"timeout_ms": 60000.0}, "timeout_ms", 60 * time.Second},
		{"very large value", map[string]any{"timeout_ms": 1.8e6}, "timeout_ms", 30 * time.Minute},
		{"NaN returns fallback", map[string]any{"timeout_ms": math.NaN()}, "timeout_ms", fallback},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tc.want, durationFromMs(tc.args, tc.key, fallback))
		})
	}
}
