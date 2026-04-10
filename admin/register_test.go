package admin

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestScanConfigDir_ExistingDir(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	// Create known files
	require.NoError(t, os.WriteFile(filepath.Join(dir, "config.yaml"), []byte("key: val"), 0o644))
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "sessions"), 0o755))
	// Create an extra file
	require.NoError(t, os.WriteFile(filepath.Join(dir, "custom.yaml"), []byte(""), 0o644))

	files := scanConfigDir(dir)

	// Known files come first
	require.Len(t, files, 6) // 5 known + 1 extra
	assert.Equal(t, "config.yaml", files[0].label)
	assert.Equal(t, "custom.yaml", files[5].label)
}

func TestScanConfigDir_NonexistentDir(t *testing.T) {
	t.Parallel()

	files := scanConfigDir("/nonexistent/path")
	assert.Len(t, files, 5) // defaults
	for _, f := range files {
		assert.Equal(t, "(not created)", formatFileStatus(f))
	}
}

func TestScanConfigDir_HiddenFilesExcluded(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, ".hidden"), []byte(""), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "visible.yaml"), []byte(""), 0o644))

	files := scanConfigDir(dir)
	for _, f := range files {
		assert.NotContains(t, f.label, ".hidden")
	}
}

func TestScanConfigDir_DirectoriesGetSlashSuffix(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "mydata"), 0o755))

	files := scanConfigDir(dir)
	var labels []string
	for _, f := range files {
		labels = append(labels, f.label)
	}
	assert.Contains(t, labels, "mydata/")
}

func TestScanConfigDir_SortedExtras(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	for _, name := range []string{"zebra.yaml", "alpha.yaml", "mid.yaml"} {
		require.NoError(t, os.WriteFile(filepath.Join(dir, name), []byte(""), 0o644))
	}

	files := scanConfigDir(dir)
	// Extras start after the 5 known entries
	extras := files[5:]
	require.Len(t, extras, 3)
	assert.True(t, extras[0].label < extras[1].label)
	assert.True(t, extras[1].label < extras[2].label)
}

func TestFormatFileStatus_Existing(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	p := filepath.Join(dir, "config.yaml")
	require.NoError(t, os.WriteFile(p, []byte("key: val"), 0o644))

	got := formatFileStatus(configFile{"config.yaml", p})
	assert.Equal(t, p, got)
}

func TestFormatFileStatus_NotCreated(t *testing.T) {
	t.Parallel()

	got := formatFileStatus(configFile{"config.yaml", "/nonexistent/path/config.yaml"})
	assert.Equal(t, "(not created)", got)
}

func TestFormatFileStatus_SymlinkLoop(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	p := filepath.Join(dir, "loop.yaml")
	require.NoError(t, os.Symlink(p, p)) // self-referencing symlink

	got := formatFileStatus(configFile{"loop.yaml", p})
	assert.Contains(t, got, "(error:")
	assert.NotContains(t, got, "(not created)")
}

func TestRunSetup_CreatesFiles(t *testing.T) {
	t.Parallel()

	dir := filepath.Join(t.TempDir(), "piglet-config")
	var messages []string
	mock := &stubExt{messages: &messages}

	runSetup(mock, dir)

	// Two messages: creation report + listing
	assert.Len(t, messages, 2)
	assert.Contains(t, messages[0], "Created:")
	assert.Contains(t, messages[1], "Config directory:")

	for _, name := range []string{"config.yaml", "behavior.md"} {
		assert.FileExists(t, filepath.Join(dir, name))
	}
	assert.DirExists(t, filepath.Join(dir, "sessions"))
}

func TestRunSetup_AlreadyExists(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	for _, name := range []string{"config.yaml", "behavior.md"} {
		require.NoError(t, os.WriteFile(filepath.Join(dir, name), []byte(""), 0o644))
	}
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "sessions"), 0o755))

	var messages []string
	mock := &stubExt{messages: &messages}

	runSetup(mock, dir)

	assert.Len(t, messages, 2)
	assert.Contains(t, messages[0], "already set up")
	assert.Contains(t, messages[1], "Config directory:")
}

func TestRunSetup_Idempotent(t *testing.T) {
	t.Parallel()

	dir := filepath.Join(t.TempDir(), "piglet-config2")

	var msgs1, msgs2 []string
	runSetup(&stubExt{messages: &msgs1}, dir)
	runSetup(&stubExt{messages: &msgs2}, dir)

	assert.Contains(t, msgs1[0], "Created:")
	assert.Contains(t, msgs2[0], "already set up")
}

func TestConfigCommand_DefaultShowsListing(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "config.yaml"), []byte("key: val"), 0o644))

	var messages []string
	mock := &stubExt{messages: &messages}

	handler := configCommand(mock)
	require.NoError(t, handler(context.Background(), ""))
	require.Len(t, messages, 1)
	assert.Contains(t, messages[0], "Config directory:")
	assert.Contains(t, messages[0], "config.yaml")
}

func TestConfigCommand_SetupSubcommand(t *testing.T) {
	t.Parallel()

	var messages []string
	mock := &stubExt{messages: &messages}

	handler := configCommand(mock)
	require.NoError(t, handler(context.Background(), "--version"))
	require.Len(t, messages, 1)
	assert.Equal(t, "admin v0.3.0", messages[0])
}

func TestConfigCommand_VersionFlag(t *testing.T) {
	t.Parallel()

	var messages []string
	mock := &stubExt{messages: &messages}

	handler := configCommand(mock)
	require.NoError(t, handler(context.Background(), "--version"))
	require.Len(t, messages, 1)
	assert.Equal(t, "admin v0.3.0", messages[0])
}

func TestConfigCommand_UnknownArg(t *testing.T) {
	t.Parallel()

	var messages []string
	mock := &stubExt{messages: &messages}

	handler := configCommand(mock)
	require.NoError(t, handler(context.Background(), "garbage"))
	require.Len(t, messages, 1)
	assert.Contains(t, messages[0], "Usage:")
	assert.Contains(t, messages[0], "Unknown argument: garbage")
}

func TestHandleConfigRead(t *testing.T) {
	t.Parallel()

	var messages []string
	mock := &stubExt{messages: &messages}

	// Reads from real config dir — just verify no panic
	handleConfigRead(mock, "config.yaml")
}

func TestHandleConfigRead_PathTraversal(t *testing.T) {
	t.Parallel()

	var messages []string
	mock := &stubExt{messages: &messages}

	handleConfigRead(mock, "../../etc/passwd")
	require.Len(t, messages, 1)
	assert.Contains(t, messages[0], "simple name")
}

func TestHandleConfigRead_EmptyFilename(t *testing.T) {
	t.Parallel()

	var messages []string
	mock := &stubExt{messages: &messages}

	handleConfigRead(mock, "")
	require.Len(t, messages, 1)
	assert.Contains(t, messages[0], "Usage:")
}

func TestToolConfigRead_PathTraversal(t *testing.T) {
	t.Parallel()

	tool := toolConfigRead()
	result, err := tool.Execute(context.Background(), map[string]any{"filename": "../secret"})
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Content[0].Text, "simple name")
}

func TestToolConfigRead_MissingFilename(t *testing.T) {
	t.Parallel()

	tool := toolConfigRead()
	result, err := tool.Execute(context.Background(), map[string]any{})
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Content[0].Text, "required")
}

// stubExt captures ShowMessage calls for testing.
type stubExt struct {
	messages *[]string
}

func (s *stubExt) ShowMessage(msg string) {
	*s.messages = append(*s.messages, msg)
}
