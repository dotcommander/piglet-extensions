package rtk

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// createMockRtk creates a temporary executable script that simulates rtk rewrite.
// It prepends "optimized:" to the input command.
func createMockRtk(t *testing.T) string {
	t.Helper()

	dir := t.TempDir()
	mockPath := filepath.Join(dir, "rtk")

	// Write a portable mock script
	script := "#!/bin/sh\necho \"optimized: $2\""

	require.NoError(t, os.WriteFile(mockPath, []byte(script), 0o755))
	return mockPath
}

// ---------------------------------------------------------------------------
// rewrite() tests
// ---------------------------------------------------------------------------

func TestRewrite_Success(t *testing.T) {
	t.Parallel()

	mockRtk := createMockRtk(t)
	result, err := rewrite(context.Background(), mockRtk, "git status")
	require.NoError(t, err)
	assert.Equal(t, "optimized: git status", result)
}

func TestRewrite_TrimsOutput(t *testing.T) {
	t.Parallel()

	// Create a mock that outputs with trailing whitespace
	dir := t.TempDir()
	mockPath := filepath.Join(dir, "rtk")
	require.NoError(t, os.WriteFile(mockPath, []byte("#!/bin/sh\nprintf '  output  \\n'"), 0o755))

	result, err := rewrite(context.Background(), mockPath, "test")
	require.NoError(t, err)
	assert.Equal(t, "output", result)
}

func TestRewrite_CommandFailure(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	mockPath := filepath.Join(dir, "rtk")
	require.NoError(t, os.WriteFile(mockPath, []byte("#!/bin/sh\nexit 1"), 0o755))

	_, err := rewrite(context.Background(), mockPath, "test")
	assert.Error(t, err)
}

func TestRewrite_BinaryNotFound(t *testing.T) {
	t.Parallel()

	_, err := rewrite(context.Background(), "/nonexistent/rtk-binary", "test")
	assert.Error(t, err)
}

// ---------------------------------------------------------------------------
// newBeforeFunc() interceptor tests
// ---------------------------------------------------------------------------

func TestInterceptor_NonBashTool_Passthrough(t *testing.T) {
	t.Parallel()

	mockRtk := createMockRtk(t)
	before := newBeforeFunc(mockRtk)

	args := map[string]any{"command": "git status"}
	proceed, result, err := before(context.Background(), "read", args)

	assert.NoError(t, err)
	assert.True(t, proceed)
	assert.Equal(t, args, result) // unchanged
}

func TestInterceptor_EmptyCommand_Passthrough(t *testing.T) {
	t.Parallel()

	mockRtk := createMockRtk(t)
	before := newBeforeFunc(mockRtk)

	args := map[string]any{"command": ""}
	proceed, result, err := before(context.Background(), "bash", args)

	assert.NoError(t, err)
	assert.True(t, proceed)
	assert.Equal(t, args, result) // unchanged
}

func TestInterceptor_NoCommandKey_Passthrough(t *testing.T) {
	t.Parallel()

	mockRtk := createMockRtk(t)
	before := newBeforeFunc(mockRtk)

	args := map[string]any{"other": "value"}
	proceed, result, err := before(context.Background(), "bash", args)

	assert.NoError(t, err)
	assert.True(t, proceed)
	assert.Equal(t, args, result) // unchanged
}

func TestInterceptor_RewriteFails_Passthrough(t *testing.T) {
	t.Parallel()

	// Create a failing mock
	dir := t.TempDir()
	mockPath := filepath.Join(dir, "rtk")
	require.NoError(t, os.WriteFile(mockPath, []byte("#!/bin/sh\nexit 1"), 0o755))

	before := newBeforeFunc(mockPath)
	args := map[string]any{"command": "git status"}

	proceed, result, err := before(context.Background(), "bash", args)

	assert.NoError(t, err)
	assert.True(t, proceed)
	assert.Equal(t, args, result) // fallback to original
}

func TestInterceptor_RewriteSameCommand_Passthrough(t *testing.T) {
	t.Parallel()

	// Create a mock that echoes the input unchanged
	dir := t.TempDir()
	mockPath := filepath.Join(dir, "rtk")
	require.NoError(t, os.WriteFile(mockPath, []byte("#!/bin/sh\necho \"$2\""), 0o755))

	before := newBeforeFunc(mockPath)
	args := map[string]any{"command": "git status"}

	proceed, result, err := before(context.Background(), "bash", args)

	assert.NoError(t, err)
	assert.True(t, proceed)
	assert.Equal(t, args, result) // same command, no modification
}

func TestInterceptor_RewriteDifferentCommand_Modified(t *testing.T) {
	t.Parallel()

	mockRtk := createMockRtk(t)
	before := newBeforeFunc(mockRtk)

	args := map[string]any{"command": "git status"}
	proceed, result, err := before(context.Background(), "bash", args)

	assert.NoError(t, err)
	assert.True(t, proceed)
	assert.Equal(t, "optimized: git status", result["command"])
	assert.NotEqual(t, args, result)               // modified
	assert.Equal(t, "git status", args["command"]) // original unchanged
}

func TestInterceptor_PreservesExtraArgs(t *testing.T) {
	t.Parallel()

	mockRtk := createMockRtk(t)
	before := newBeforeFunc(mockRtk)

	args := map[string]any{
		"command": "git status",
		"timeout": "30",
		"workdir": "/tmp",
	}
	proceed, result, err := before(context.Background(), "bash", args)

	assert.NoError(t, err)
	assert.True(t, proceed)
	assert.Equal(t, "optimized: git status", result["command"])
	assert.Equal(t, "30", result["timeout"])
	assert.Equal(t, "/tmp", result["workdir"])
}
