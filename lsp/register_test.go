package lsp

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestResolvePosition(t *testing.T) {
	t.Parallel()

	t.Run("with explicit column", func(t *testing.T) {
		t.Parallel()
		mgr := NewManager(t.TempDir())
		args := map[string]any{
			"file":   "test.go",
			"line":   float64(5),
			"column": float64(10),
		}
		file, line, col, err := resolvePosition(mgr, args)
		require.NoError(t, err)
		assert.Equal(t, filepath.Join(mgr.CWD(), "test.go"), file)
		assert.Equal(t, 4, line) // 1-based → 0-based
		assert.Equal(t, 9, col)  // 1-based → 0-based
	})

	t.Run("line only defaults to column 0", func(t *testing.T) {
		t.Parallel()
		mgr := NewManager(t.TempDir())
		args := map[string]any{
			"file": "test.go",
			"line": float64(1),
		}
		_, _, col, err := resolvePosition(mgr, args)
		require.NoError(t, err)
		assert.Equal(t, 0, col)
	})

	t.Run("missing file", func(t *testing.T) {
		t.Parallel()
		mgr := NewManager(t.TempDir())
		args := map[string]any{"line": float64(1)}
		_, _, _, err := resolvePosition(mgr, args)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "file is required")
	})

	t.Run("invalid line", func(t *testing.T) {
		t.Parallel()
		mgr := NewManager(t.TempDir())
		args := map[string]any{"file": "test.go", "line": float64(0)}
		_, _, _, err := resolvePosition(mgr, args)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "line must be >= 1")
	})

	t.Run("with symbol", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		file := filepath.Join(dir, "test.go")
		require.NoError(t, os.WriteFile(file, []byte("package main\n\nfunc foo() {}\n"), 0o600))

		mgr := NewManager(dir)
		args := map[string]any{
			"file":   file,
			"line":   float64(3),
			"symbol": "foo",
		}
		_, line, col, err := resolvePosition(mgr, args)
		require.NoError(t, err)
		assert.Equal(t, 2, line) // 0-based
		assert.Equal(t, 5, col)  // "func " is 5 runes before "foo"
	})
}

func TestResolveFile(t *testing.T) {
	t.Parallel()

	t.Run("relative path resolves to cwd", func(t *testing.T) {
		t.Parallel()
		mgr := NewManager("/home/user/project")
		file := resolveFile(mgr, map[string]any{"file": "src/main.go"})
		assert.Equal(t, "/home/user/project/src/main.go", file)
	})

	t.Run("absolute path unchanged", func(t *testing.T) {
		t.Parallel()
		mgr := NewManager("/home/user/project")
		file := resolveFile(mgr, map[string]any{"file": "/abs/path/main.go"})
		assert.Equal(t, "/abs/path/main.go", file)
	})

	t.Run("empty file returns empty", func(t *testing.T) {
		t.Parallel()
		mgr := NewManager("/home/user/project")
		file := resolveFile(mgr, map[string]any{})
		assert.Equal(t, "", file)
	})

	t.Run("nil manager passes through", func(t *testing.T) {
		t.Parallel()
		file := resolveFile(nil, map[string]any{"file": "relative.go"})
		assert.Equal(t, "relative.go", file)
	})
}

func TestFindSymbolColumn(t *testing.T) {
	t.Parallel()

	t.Run("finds symbol", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		file := filepath.Join(dir, "test.go")
		content := "package main\n\nfunc myFunc() int {\n\treturn 42\n}\n"
		require.NoError(t, os.WriteFile(file, []byte(content), 0o600))

		col, err := FindSymbolColumn(file, 2, "myFunc")
		require.NoError(t, err)
		assert.Equal(t, 5, col) // "func " is 5 runes
	})

	t.Run("symbol not found", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		file := filepath.Join(dir, "test.go")
		require.NoError(t, os.WriteFile(file, []byte("package main\n"), 0o600))

		_, err := FindSymbolColumn(file, 0, "nonexistent")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not found on line")
	})

	t.Run("line out of range", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		file := filepath.Join(dir, "test.go")
		require.NoError(t, os.WriteFile(file, []byte("package main\n"), 0o600))

		_, err := FindSymbolColumn(file, 99, "main")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "out of range")
	})

	t.Run("unicode symbol", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		file := filepath.Join(dir, "test.go")
		require.NoError(t, os.WriteFile(file, []byte("var café = true\n"), 0o600))

		col, err := FindSymbolColumn(file, 0, "café")
		require.NoError(t, err)
		assert.Equal(t, 4, col) // "var " is 4 runes
	})

	t.Run("missing file", func(t *testing.T) {
		t.Parallel()
		_, err := FindSymbolColumn("/nonexistent/file.go", 0, "foo")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "read file")
	})
}
