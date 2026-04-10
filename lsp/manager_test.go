package lsp

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestLanguageForFile(t *testing.T) {
	t.Parallel()

	tests := []struct {
		file string
		want string
	}{
		{"main.go", "go"},
		{"foo_test.go", "go"},
		{"index.ts", "typescript"},
		{"component.tsx", "typescript"},
		{"app.js", "javascript"},
		{"App.JSX", "javascript"}, // case-insensitive ext
		{"script.py", "python"},
		{"main.rs", "rust"},
		{"main.c", "c"},
		{"header.h", "c"},
		{"impl.cpp", "cpp"},
		{"impl.cc", "cpp"},
		{"impl.cxx", "cpp"},
		{"header.hpp", "cpp"},
		{"Main.java", "java"},
		{"init.lua", "lua"},
		{"main.zig", "zig"},
		{"readme.md", ""},
		{"Makefile", ""},
		{"config.yaml", ""},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.file, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tc.want, LanguageForFile(tc.file))
		})
	}
}

func TestServerNames(t *testing.T) {
	t.Parallel()

	t.Run("single", func(t *testing.T) {
		t.Parallel()
		configs := []ServerConfig{{Command: "gopls"}}
		assert.Equal(t, "gopls", serverNames(configs))
	})

	t.Run("multiple", func(t *testing.T) {
		t.Parallel()
		configs := []ServerConfig{
			{Command: "pylsp"},
			{Command: "pyright-langserver", Args: []string{"--stdio"}},
		}
		assert.Equal(t, "pylsp, pyright-langserver", serverNames(configs))
	})

	t.Run("empty", func(t *testing.T) {
		t.Parallel()
		assert.Equal(t, "", serverNames(nil))
	})
}
