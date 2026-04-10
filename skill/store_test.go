package skill

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- parseSkillFile ---

func TestParseSkillFile(t *testing.T) {
	t.Parallel()

	t.Run("valid frontmatter", func(t *testing.T) {
		t.Parallel()
		content := "---\nname: test-skill\ndescription: A test\ntriggers:\n  - testing\n  - demo\n---\n\nSkill body content here.\n"
		path := writeFile(t, "test.md", content)

		sk, body, err := parseSkillFile(path)
		require.NoError(t, err)
		assert.Equal(t, "test-skill", sk.Name)
		assert.Equal(t, "A test", sk.Description)
		assert.Equal(t, []string{"testing", "demo"}, sk.Triggers)
		assert.Equal(t, "Skill body content here.", body)
	})

	t.Run("no frontmatter uses filename", func(t *testing.T) {
		t.Parallel()
		content := "Just some content without frontmatter."
		path := writeFile(t, "plain.md", content)

		sk, body, err := parseSkillFile(path)
		require.NoError(t, err)
		assert.Equal(t, "plain", sk.Name)
		assert.Empty(t, sk.Description)
		assert.Equal(t, content, body)
	})

	t.Run("opening delimiter only uses filename", func(t *testing.T) {
		t.Parallel()
		content := "---\nname: incomplete\n"
		path := writeFile(t, "incomplete.md", content)

		sk, _, err := parseSkillFile(path)
		require.NoError(t, err)
		assert.Equal(t, "incomplete", sk.Name)
	})

	t.Run("empty frontmatter name uses filename", func(t *testing.T) {
		t.Parallel()
		content := "---\ndescription: No name field\n---\nBody."
		path := writeFile(t, "no-name.md", content)

		sk, body, err := parseSkillFile(path)
		require.NoError(t, err)
		assert.Equal(t, "no-name", sk.Name)
		assert.Equal(t, "No name field", sk.Description)
		assert.Equal(t, "Body.", body)
	})

	t.Run("malformed yaml returns error", func(t *testing.T) {
		t.Parallel()
		content := "---\nname: [\n---\nBody."
		path := writeFile(t, "bad-yaml.md", content)

		_, _, err := parseSkillFile(path)
		assert.Error(t, err)
	})

	t.Run("nonexistent file returns error", func(t *testing.T) {
		t.Parallel()
		_, _, err := parseSkillFile("/nonexistent/path/skill.md")
		assert.Error(t, err)
	})
}

// --- Match ---

func TestMatch(t *testing.T) {
	t.Parallel()

	skills := []Skill{
		{Name: "testing", Triggers: []string{"test", "verify"}, body: "testing body"},
		{Name: "demo", Triggers: []string{"demo", "show"}, body: "demo body"},
		{Name: "general", Triggers: []string{"general approach"}, body: "general body"},
	}

	s := &Store{skills: skills}

	t.Run("exact trigger match", func(t *testing.T) {
		t.Parallel()
		matches := s.Match("I need to test this")
		require.Len(t, matches, 1)
		assert.Equal(t, "testing", matches[0].Name)
	})

	t.Run("case insensitive", func(t *testing.T) {
		t.Parallel()
		matches := s.Match("DEMO TIME")
		require.Len(t, matches, 1)
		assert.Equal(t, "demo", matches[0].Name)
	})

	t.Run("longest trigger first", func(t *testing.T) {
		t.Parallel()
		matches := s.Match("I want a general approach to test this")
		require.Len(t, matches, 2)
		// "general approach" (16 chars) > "test" (4 chars)
		assert.Equal(t, "general", matches[0].Name)
		assert.Equal(t, "testing", matches[1].Name)
	})

	t.Run("no match", func(t *testing.T) {
		t.Parallel()
		matches := s.Match("something completely unrelated")
		assert.Empty(t, matches)
	})

	t.Run("empty text no match", func(t *testing.T) {
		t.Parallel()
		matches := s.Match("")
		assert.Empty(t, matches)
	})

	t.Run("no triggers no match", func(t *testing.T) {
		t.Parallel()
		s := &Store{skills: []Skill{{Name: "bare", Triggers: nil}}}
		matches := s.Match("anything")
		assert.Empty(t, matches)
	})
}

// --- Load ---

func TestLoad(t *testing.T) {
	t.Parallel()

	s := &Store{skills: []Skill{
		{Name: "Go Dev", body: "go content"},
		{Name: "Rust Dev", body: "rust content"},
	}}

	t.Run("found case insensitive", func(t *testing.T) {
		t.Parallel()
		content, err := s.Load("go dev")
		assert.NoError(t, err)
		assert.Equal(t, "go content", content)
	})

	t.Run("found exact case", func(t *testing.T) {
		t.Parallel()
		content, err := s.Load("Go Dev")
		assert.NoError(t, err)
		assert.Equal(t, "go content", content)
	})

	t.Run("not found", func(t *testing.T) {
		t.Parallel()
		_, err := s.Load("nonexistent")
		assert.ErrorIs(t, err, os.ErrNotExist)
	})
}

// --- FormatList ---

func TestFormatList(t *testing.T) {
	t.Parallel()

	skills := []Skill{
		{Name: "go-dev", Description: "Go patterns", Triggers: []string{"go", "golang"}},
		{Name: "rust-dev", Description: "Rust patterns"},
		{Name: "bare"},
	}

	t.Run("with triggers", func(t *testing.T) {
		t.Parallel()
		got := FormatList(skills, FormatOpts{Indent: "- ", Separator: ": ", Triggers: true})
		assert.Contains(t, got, "- go-dev: Go patterns (triggers: go, golang)\n")
		assert.Contains(t, got, "- rust-dev: Rust patterns\n")
		assert.Contains(t, got, "- bare\n")
	})

	t.Run("without triggers", func(t *testing.T) {
		t.Parallel()
		got := FormatList(skills, FormatOpts{Indent: "  ", Separator: " — "})
		assert.Contains(t, got, "  go-dev — Go patterns\n")
		assert.NotContains(t, got, "triggers")
	})

	t.Run("with prefix", func(t *testing.T) {
		t.Parallel()
		got := FormatList(skills, FormatOpts{Prefix: "Available:\n", Indent: "- ", Separator: ": "})
		assert.Contains(t, got, "Available:\n- go-dev")
	})

	t.Run("empty skills", func(t *testing.T) {
		t.Parallel()
		got := FormatList(nil, FormatOpts{Indent: "- "})
		assert.Empty(t, got)
	})
}

// --- NewStore integration ---

func TestNewStore(t *testing.T) {
	t.Parallel()

	t.Run("nonexistent directory returns empty store", func(t *testing.T) {
		t.Parallel()
		s := NewStore("/nonexistent/dir")
		assert.Empty(t, s.List())
	})

	t.Run("scans and loads skill files", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		content := "---\nname: my-skill\ndescription: Test skill\n---\n\nBody content.\n"
		require.NoError(t, os.WriteFile(filepath.Join(dir, "my-skill.md"), []byte(content), 0o644))

		s := NewStore(dir)
		skills := s.List()
		require.Len(t, skills, 1)
		assert.Equal(t, "my-skill", skills[0].Name)
		assert.Equal(t, "Test skill", skills[0].Description)
		assert.Equal(t, dir, s.Dir())

		body, err := s.Load("my-skill")
		require.NoError(t, err)
		assert.Equal(t, "Body content.", body)
	})

	t.Run("skips non-md files", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		require.NoError(t, os.WriteFile(filepath.Join(dir, "notes.txt"), []byte("not a skill"), 0o644))
		require.NoError(t, os.WriteFile(filepath.Join(dir, "real.md"), []byte("---\nname: real\n---\nBody."), 0o644))

		s := NewStore(dir)
		require.Len(t, s.List(), 1)
		assert.Equal(t, "real", s.List()[0].Name)
	})

	t.Run("skips subdirectories", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		require.NoError(t, os.MkdirAll(filepath.Join(dir, "subdir"), 0o755))

		s := NewStore(dir)
		assert.Empty(t, s.List())
	})
}

// --- helper ---

func writeFile(t *testing.T, name, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, name)
	require.NoError(t, os.WriteFile(path, []byte(content), 0o644))
	return path
}
