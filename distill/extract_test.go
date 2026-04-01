package distill

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestScoreComplexity(t *testing.T) {
	t.Parallel()

	t.Run("empty messages", func(t *testing.T) {
		t.Parallel()
		assert.Equal(t, 0, scoreComplexity(nil))
	})

	t.Run("simple exchange", func(t *testing.T) {
		t.Parallel()
		messages := []json.RawMessage{
			json.RawMessage(`{"role":"user","content":"hello"}`),
			json.RawMessage(`{"role":"assistant","content":"hi there"}`),
		}
		score := scoreComplexity(messages)
		// 2 messages = 2 points, no tools, no error recovery
		assert.Equal(t, 2, score)
	})

	t.Run("complex session with tools and errors", func(t *testing.T) {
		t.Parallel()
		messages := []json.RawMessage{
			// user message
			json.RawMessage(`{"role":"user","content":"fix the bug"}`),
			// assistant with error mention
			json.RawMessage(`{"role":"assistant","content":[{"type":"text","text":"got an error"},{"type":"tool_use","name":"read_file","input":{}}]}`),
			// tool result
			json.RawMessage(`{"role":"tool","content":"error: file not found"}`),
			// assistant recovers with tool use
			json.RawMessage(`{"role":"assistant","content":[{"type":"tool_use","name":"list_files","input":{}},{"type":"tool_use","name":"read_file","input":{}}]}`),
			// more messages
			json.RawMessage(`{"role":"user","content":"try again"}`),
			json.RawMessage(`{"role":"assistant","content":"done"}`),
		}
		score := scoreComplexity(messages)
		// 6 messages = 6
		// msg[1]: 1 tool_use = +2
		// msg[3]: 2 tool_use = +4
		// msg[1] has "error" + msg[3] has tool_use = +3 error recovery
		// total = 6 + 2 + 4 + 3 = 15
		assert.Greater(t, score, 10, "complex session should score above 10")
	})
}

func TestFormatMessages(t *testing.T) {
	t.Parallel()

	t.Run("basic formatting", func(t *testing.T) {
		t.Parallel()
		messages := []json.RawMessage{
			json.RawMessage(`{"role":"user","content":"hello world"}`),
			json.RawMessage(`{"role":"assistant","content":"hi there"}`),
		}
		result := formatMessages(messages, maxFormatChars)
		assert.Contains(t, result, "User: hello world")
		assert.Contains(t, result, "Assistant: hi there")
	})

	t.Run("truncation", func(t *testing.T) {
		t.Parallel()
		// Build a long message
		long := strings.Repeat("x", 500)
		messages := []json.RawMessage{
			json.RawMessage(fmt.Sprintf(`{"role":"user","content":%q}`, long)),
			json.RawMessage(fmt.Sprintf(`{"role":"assistant","content":%q}`, long)),
		}
		result := formatMessages(messages, 100)
		assert.LessOrEqual(t, len(result), 110, "should be truncated near maxChars")
	})

	t.Run("tool use blocks show tool name", func(t *testing.T) {
		t.Parallel()
		messages := []json.RawMessage{
			json.RawMessage(`{"role":"assistant","content":[{"type":"tool_use","name":"read_file","input":{}}]}`),
		}
		result := formatMessages(messages, maxFormatChars)
		assert.Contains(t, result, "[tool: read_file]")
	})

	t.Run("empty messages returns empty", func(t *testing.T) {
		t.Parallel()
		result := formatMessages(nil, maxFormatChars)
		assert.Empty(t, result)
	})
}

func TestWriteSkill(t *testing.T) {
	t.Parallel()

	t.Run("writes file with valid frontmatter", func(t *testing.T) {
		t.Parallel()
		tmp := t.TempDir()

		content := "---\nname: my-test-skill\ndescription: A test skill\ntriggers:\n  - test\nsource: distill\n---\n\nThis is the body.\n"
		path, err := writeSkillTo(tmp, content)
		require.NoError(t, err)
		assert.Equal(t, filepath.Join(tmp, "my-test-skill.md"), path)

		data, err := os.ReadFile(path)
		require.NoError(t, err)
		assert.Equal(t, content, string(data))
	})

	t.Run("sanitizes name with spaces", func(t *testing.T) {
		t.Parallel()
		tmp := t.TempDir()

		content := "---\nname: my skill with spaces\ndescription: Test\ntriggers:\n  - test\nsource: distill\n---\n\nBody.\n"
		path, err := writeSkillTo(tmp, content)
		require.NoError(t, err)
		assert.Equal(t, filepath.Join(tmp, "my-skill-with-spaces.md"), path)
	})
}

func TestWriteSkillNoFrontmatter(t *testing.T) {
	t.Parallel()

	tmp := t.TempDir()

	content := "This is just bare content without frontmatter."
	path, err := writeSkillTo(tmp, content)
	require.NoError(t, err)

	data, err := os.ReadFile(path)
	require.NoError(t, err)

	body := string(data)
	assert.True(t, strings.HasPrefix(body, "---"), "should have frontmatter prefix")
	assert.Contains(t, body, "source: distill")
	assert.Contains(t, body, content)
}

func TestReadSessionMessages(t *testing.T) {
	t.Parallel()

	t.Run("parses non-meta entries", func(t *testing.T) {
		t.Parallel()
		tmp := t.TempDir()
		path := filepath.Join(tmp, "session.jsonl")

		lines := []string{
			`{"type":"meta","data":{"title":"test session"}}`,
			`{"type":"message","data":{"role":"user","content":"hello"}}`,
			`{"type":"message","data":{"role":"assistant","content":"hi"}}`,
		}
		content := strings.Join(lines, "\n") + "\n"
		require.NoError(t, os.WriteFile(path, []byte(content), 0600))

		messages, err := readSessionMessages(path)
		require.NoError(t, err)
		assert.Len(t, messages, 2)

		var msg struct {
			Role    string `json:"role"`
			Content string `json:"content"`
		}
		require.NoError(t, json.Unmarshal(messages[0], &msg))
		assert.Equal(t, "user", msg.Role)
		assert.Equal(t, "hello", msg.Content)
	})

	t.Run("skips blank lines and invalid JSON", func(t *testing.T) {
		t.Parallel()
		tmp := t.TempDir()
		path := filepath.Join(tmp, "session.jsonl")

		lines := []string{
			``,
			`not valid json`,
			`{"type":"message","data":{"role":"user","content":"valid"}}`,
		}
		content := strings.Join(lines, "\n") + "\n"
		require.NoError(t, os.WriteFile(path, []byte(content), 0600))

		messages, err := readSessionMessages(path)
		require.NoError(t, err)
		assert.Len(t, messages, 1)
	})

	t.Run("missing file returns error", func(t *testing.T) {
		t.Parallel()
		_, err := readSessionMessages("/nonexistent/path/session.jsonl")
		assert.Error(t, err)
	})
}
