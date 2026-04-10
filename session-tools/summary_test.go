package sessiontools

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFormatSummary(t *testing.T) {
	t.Parallel()

	t.Run("empty facts returns header only", func(t *testing.T) {
		t.Parallel()
		got := formatSummary(nil)
		assert.Equal(t, "# Session Handoff Summary", got)
	})

	t.Run("single goal", func(t *testing.T) {
		t.Parallel()
		facts := []memoryFact{
			{Key: "ctx:goal:main", Value: "Ship v0.7.0"},
		}
		got := formatSummary(facts)
		assert.Contains(t, got, "## Goal")
		assert.Contains(t, got, "- Ship v0.7.0")
	})

	t.Run("multiple categories", func(t *testing.T) {
		t.Parallel()
		facts := []memoryFact{
			{Key: "ctx:goal:main", Value: "Ship v0.7.0"},
			{Key: "ctx:edit:register", Value: "Refactored register.go"},
			{Key: "ctx:error:build", Value: "go build failed"},
			{Key: "ctx:decision:naming", Value: "Keep session-handoff namespace"},
		}
		got := formatSummary(facts)
		assert.Contains(t, got, "## Goal")
		assert.Contains(t, got, "## Progress")
		assert.Contains(t, got, "## Key Decisions")
		assert.Contains(t, got, "## Errors Encountered")
		assert.Contains(t, got, "- Ship v0.7.0")
		assert.Contains(t, got, "- Refactored register.go")
		assert.Contains(t, got, "- go build failed")
		assert.Contains(t, got, "- Keep session-handoff namespace")
	})

	t.Run("files and commands grouped under context", func(t *testing.T) {
		t.Parallel()
		facts := []memoryFact{
			{Key: "ctx:file:main", Value: "register.go"},
			{Key: "ctx:cmd:build", Value: "go build ./..."},
		}
		got := formatSummary(facts)
		assert.Contains(t, got, "## Context")
		assert.Contains(t, got, "### Files")
		assert.Contains(t, got, "### Commands")
		assert.Contains(t, got, "- `go build ./...`") // code-formatted commands
	})

	t.Run("unknown prefixes go to other facts", func(t *testing.T) {
		t.Parallel()
		facts := []memoryFact{
			{Key: "misc:note", Value: "some note"},
			{Key: "random:key", Value: "some value"},
		}
		got := formatSummary(facts)
		assert.Contains(t, got, "## Other Facts")
		assert.Contains(t, got, "- **misc:note**: some note")
		assert.Contains(t, got, "- **random:key**: some value")
		assert.NotContains(t, got, "## Goal")
	})

	t.Run("mixed known and unknown", func(t *testing.T) {
		t.Parallel()
		facts := []memoryFact{
			{Key: "ctx:goal:x", Value: "goal"},
			{Key: "other:y", Value: "misc"},
		}
		got := formatSummary(facts)
		assert.Contains(t, got, "## Goal")
		assert.Contains(t, got, "## Other Facts")
	})
}

func TestReadFacts(t *testing.T) {
	t.Parallel()

	t.Run("nonexistent file returns nil", func(t *testing.T) {
		t.Parallel()
		facts, err := readFacts("/nonexistent/path/file.jsonl")
		assert.NoError(t, err)
		assert.Nil(t, facts)
	})

	t.Run("empty file returns nil", func(t *testing.T) {
		t.Parallel()
		path := filepath.Join(t.TempDir(), "empty.jsonl")
		require.NoError(t, os.WriteFile(path, []byte(""), 0o644))

		facts, err := readFacts(path)
		assert.NoError(t, err)
		assert.Nil(t, facts)
	})

	t.Run("valid jsonl parsed and sorted", func(t *testing.T) {
		t.Parallel()
		content := `{"key":"ctx:goal:b","value":"second","created_at":"2026-01-01T00:00:00Z","updated_at":"2026-01-01T00:00:00Z"}
{"key":"ctx:goal:a","value":"first","created_at":"2026-01-01T00:00:00Z","updated_at":"2026-01-01T00:00:00Z"}
`
		path := filepath.Join(t.TempDir(), "facts.jsonl")
		require.NoError(t, os.WriteFile(path, []byte(content), 0o644))

		facts, err := readFacts(path)
		require.NoError(t, err)
		require.Len(t, facts, 2)
		assert.Equal(t, "ctx:goal:a", facts[0].Key)
		assert.Equal(t, "ctx:goal:b", facts[1].Key)
	})

	t.Run("malformed lines skipped", func(t *testing.T) {
		t.Parallel()
		content := "not json\n{\"key\":\"ctx:goal:x\",\"value\":\"valid\"}\n{bad\n"
		path := filepath.Join(t.TempDir(), "mixed.jsonl")
		require.NoError(t, os.WriteFile(path, []byte(content), 0o644))

		facts, err := readFacts(path)
		require.NoError(t, err)
		require.Len(t, facts, 1)
		assert.Equal(t, "ctx:goal:x", facts[0].Key)
	})

	t.Run("blank lines skipped", func(t *testing.T) {
		t.Parallel()
		content := "\n\n{\"key\":\"ctx:goal:x\",\"value\":\"valid\"}\n\n"
		path := filepath.Join(t.TempDir(), "blanks.jsonl")
		require.NoError(t, os.WriteFile(path, []byte(content), 0o644))

		facts, err := readFacts(path)
		require.NoError(t, err)
		require.Len(t, facts, 1)
	})
}

func TestBuildSummary(t *testing.T) {
	t.Parallel()

	t.Run("no memory file returns no facts message", func(t *testing.T) {
		t.Parallel()
		// Use a CWD that definitely has no memory file
		summary, facts, err := BuildSummary("/nonexistent/path/that/does/not/exist")
		require.NoError(t, err)
		assert.Equal(t, "No memory facts available for this project.", summary)
		assert.Nil(t, facts)
	})

	t.Run("with facts returns formatted summary", func(t *testing.T) {
		t.Parallel()
		// Create a temp directory and write a memory file
		tmpDir := t.TempDir()
		memDir := filepath.Join(tmpDir, "memory")
		require.NoError(t, os.MkdirAll(memDir, 0o755))

		// Compute the path the same way MemoryStorePath does
		path, err := MemoryStorePath(tmpDir)
		require.NoError(t, err)

		content := `{"key":"ctx:goal:main","value":"Test goal","created_at":"2026-01-01T00:00:00Z","updated_at":"2026-01-01T00:00:00Z"}
`
		require.NoError(t, os.WriteFile(path, []byte(content), 0o644))

		summary, facts, err := BuildSummary(tmpDir)
		require.NoError(t, err)
		assert.Contains(t, summary, "## Goal")
		assert.Contains(t, summary, "- Test goal")
		assert.Len(t, facts, 1)
	})
}
