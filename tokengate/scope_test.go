package tokengate

import (
	"regexp"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRewriteBash(t *testing.T) {
	t.Parallel()

	t.Run("appends head to matching command", func(t *testing.T) {
		t.Parallel()
		args := map[string]any{"command": "grep -r 'TODO' ."}
		re := regexp.MustCompile(`grep\s+-r`)
		rule := compiledRule{tool: "bash", pattern: re, action: "append_head", value: "100"}

		ok, modified, err := rewriteBash(args, rule)
		assert.NoError(t, err)
		assert.True(t, ok)
		assert.Equal(t, "grep -r 'TODO' . | head -100", modified["command"])
	})

	t.Run("skips already scoped command", func(t *testing.T) {
		t.Parallel()
		args := map[string]any{"command": "grep -r 'TODO' . | head -50"}
		re := regexp.MustCompile(`grep\s+-r`)
		rule := compiledRule{tool: "bash", pattern: re, action: "append_head", value: "100"}

		ok, modified, err := rewriteBash(args, rule)
		assert.NoError(t, err)
		assert.True(t, ok)
		assert.Equal(t, "grep -r 'TODO' . | head -50", modified["command"])
	})

	t.Run("skips non-matching pattern", func(t *testing.T) {
		t.Parallel()
		args := map[string]any{"command": "ls -la"}
		re := regexp.MustCompile(`grep\s+-r`)
		rule := compiledRule{tool: "bash", pattern: re, action: "append_head", value: "100"}

		ok, modified, err := rewriteBash(args, rule)
		assert.NoError(t, err)
		assert.True(t, ok)
		assert.Equal(t, args, modified, "should return original args unchanged")
	})

	t.Run("skips empty command", func(t *testing.T) {
		t.Parallel()
		args := map[string]any{"command": ""}
		rule := compiledRule{tool: "bash", action: "append_head", value: "100"}

		ok, modified, err := rewriteBash(args, rule)
		assert.NoError(t, err)
		assert.True(t, ok)
		assert.Equal(t, args, modified)
	})

	t.Run("skips non-append_head action", func(t *testing.T) {
		t.Parallel()
		args := map[string]any{"command": "grep -r 'TODO' ."}
		re := regexp.MustCompile(`grep\s+-r`)
		rule := compiledRule{tool: "bash", pattern: re, action: "other", value: "100"}

		ok, modified, err := rewriteBash(args, rule)
		assert.NoError(t, err)
		assert.True(t, ok)
		assert.Equal(t, args, modified)
	})

	t.Run("no pattern matches all commands", func(t *testing.T) {
		t.Parallel()
		args := map[string]any{"command": "any command at all"}
		rule := compiledRule{tool: "bash", action: "append_head", value: "50"}

		ok, modified, err := rewriteBash(args, rule)
		assert.NoError(t, err)
		assert.True(t, ok)
		assert.Equal(t, "any command at all | head -50", modified["command"])
	})
}

func TestRewriteRead(t *testing.T) {
	t.Parallel()

	t.Run("adds limit to read without limit", func(t *testing.T) {
		t.Parallel()
		args := map[string]any{"file_path": "/some/file.go"}
		rule := compiledRule{tool: "Read", action: "limit_lines", value: "200"}

		ok, modified, err := rewriteRead(args, rule)
		assert.NoError(t, err)
		assert.True(t, ok)
		assert.Equal(t, float64(200), modified["limit"])
	})

	t.Run("skips read with existing limit", func(t *testing.T) {
		t.Parallel()
		args := map[string]any{"file_path": "/some/file.go", "limit": float64(50)}
		rule := compiledRule{tool: "Read", action: "limit_lines", value: "200"}

		ok, modified, err := rewriteRead(args, rule)
		assert.NoError(t, err)
		assert.True(t, ok)
		assert.Equal(t, args, modified)
	})

	t.Run("skips read with existing offset", func(t *testing.T) {
		t.Parallel()
		args := map[string]any{"file_path": "/some/file.go", "offset": float64(100)}
		rule := compiledRule{tool: "Read", action: "limit_lines", value: "200"}

		ok, modified, err := rewriteRead(args, rule)
		assert.NoError(t, err)
		assert.True(t, ok)
		assert.Equal(t, args, modified)
	})

	t.Run("skips non-limit_lines action", func(t *testing.T) {
		t.Parallel()
		args := map[string]any{"file_path": "/some/file.go"}
		rule := compiledRule{tool: "Read", action: "other", value: "200"}

		ok, modified, err := rewriteRead(args, rule)
		assert.NoError(t, err)
		assert.True(t, ok)
		assert.Equal(t, args, modified)
	})

	t.Run("skips invalid value", func(t *testing.T) {
		t.Parallel()
		args := map[string]any{"file_path": "/some/file.go"}
		rule := compiledRule{tool: "Read", action: "limit_lines", value: "notanumber"}

		ok, modified, err := rewriteRead(args, rule)
		assert.NoError(t, err)
		assert.True(t, ok)
		assert.Equal(t, args, modified)
	})
}

func TestRewriteGrep(t *testing.T) {
	t.Parallel()

	t.Run("adds head_limit to grep without limit", func(t *testing.T) {
		t.Parallel()
		args := map[string]any{"pattern": "TODO"}
		rule := compiledRule{tool: "Grep", action: "limit_lines", value: "50"}

		ok, modified, err := rewriteGrep(args, rule)
		assert.NoError(t, err)
		assert.True(t, ok)
		assert.Equal(t, float64(50), modified["head_limit"])
	})

	t.Run("skips grep with existing head_limit", func(t *testing.T) {
		t.Parallel()
		args := map[string]any{"pattern": "TODO", "head_limit": float64(25)}
		rule := compiledRule{tool: "Grep", action: "limit_lines", value: "50"}

		ok, modified, err := rewriteGrep(args, rule)
		assert.NoError(t, err)
		assert.True(t, ok)
		assert.Equal(t, args, modified)
	})

	t.Run("skips invalid value", func(t *testing.T) {
		t.Parallel()
		args := map[string]any{"pattern": "TODO"}
		rule := compiledRule{tool: "Grep", action: "limit_lines", value: "abc"}

		ok, modified, err := rewriteGrep(args, rule)
		assert.NoError(t, err)
		assert.True(t, ok)
		assert.Equal(t, args, modified)
	})
}

func TestAlreadyScoped(t *testing.T) {
	t.Parallel()

	tests := []struct {
		cmd  string
		want bool
	}{
		{"cat file.txt | head -20", true},
		{"cat file.txt | tail -20", true},
		{"cat file.txt | head", true},
		{"grep pattern file.txt", false},
		{"cat file.txt", false},
		{"echo hello | sort | uniq", false},
	}

	for _, tt := range tests {
		assert.Equal(t, tt.want, alreadyScoped(tt.cmd), "alreadyScoped(%q)", tt.cmd)
	}
}
