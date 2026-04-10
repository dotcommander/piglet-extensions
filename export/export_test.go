package export

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRenderUser(t *testing.T) {
	t.Parallel()

	var b strings.Builder
	renderUser(&b, json.RawMessage(`"Hello, world!"`))

	got := b.String()
	assert.Contains(t, got, "## User")
	assert.Contains(t, got, "Hello, world!")
}

func TestRenderUser_InvalidJSON(t *testing.T) {
	t.Parallel()

	var b strings.Builder
	renderUser(&b, json.RawMessage(`{not text}`))

	got := b.String()
	assert.Contains(t, got, "## User")
	// Content section is empty but section header still appears.
	lines := strings.Split(got, "\n")
	assert.Equal(t, "", lines[2]) // blank line after header
}

func TestRenderAssistant_TextBlock(t *testing.T) {
	t.Parallel()

	content := `[{"type":"text","text":"Here is the answer"}]`
	var b strings.Builder
	renderAssistant(&b, json.RawMessage(content))

	got := b.String()
	assert.Contains(t, got, "## Assistant")
	assert.Contains(t, got, "Here is the answer")
}

func TestRenderAssistant_ThinkingBlock(t *testing.T) {
	t.Parallel()

	content := `[{"type":"thinking","thinking":"let me reason..."}]`
	var b strings.Builder
	renderAssistant(&b, json.RawMessage(content))

	got := b.String()
	assert.Contains(t, got, "<details>")
	assert.Contains(t, got, "let me reason...")
	assert.Contains(t, got, "</details>")
}

func TestRenderAssistant_MixedBlocks(t *testing.T) {
	t.Parallel()

	content := `[
		{"type":"thinking","thinking":"hmm"},
		{"type":"text","text":"result"},
		{"type":"image","text":"ignored"}
	]`
	var b strings.Builder
	renderAssistant(&b, json.RawMessage(content))

	got := b.String()
	assert.Contains(t, got, "hmm")
	assert.Contains(t, got, "result")
	assert.NotContains(t, got, "ignored")
}

func TestRenderAssistant_InvalidContent(t *testing.T) {
	t.Parallel()

	var b strings.Builder
	renderAssistant(&b, json.RawMessage(`"not an array"`))

	got := b.String()
	assert.Contains(t, got, "## Assistant")
}

func TestRenderToolResult(t *testing.T) {
	t.Parallel()

	content := `[{"type":"text","text":"file contents here"}]`
	var b strings.Builder
	renderToolResult(&b, "ReadFile", json.RawMessage(content))

	got := b.String()
	assert.Contains(t, got, "### Tool: ReadFile")
	assert.Contains(t, got, "file contents here")
}

func TestRenderToolResult_NonTextBlock(t *testing.T) {
	t.Parallel()

	content := `[{"type":"image","text":"ignored"}]`
	var b strings.Builder
	renderToolResult(&b, "Fetch", json.RawMessage(content))

	got := b.String()
	assert.Contains(t, got, "### Tool: Fetch")
	assert.NotContains(t, got, "ignored")
}

func TestExportMarkdown_FullConversation(t *testing.T) {
	t.Parallel()

	msgs := []json.RawMessage{
		json.RawMessage(`{"role":"user","content":"What is Go?"}`),
		json.RawMessage(`{"role":"assistant","content":[{"type":"text","text":"Go is a statically typed language"}]}`),
		json.RawMessage(`{"role":"tool_result","toolName":"Search","content":[{"type":"text","text":"results here"}]}`),
	}

	dir := t.TempDir()
	path := filepath.Join(dir, "test-export.md")
	err := exportMarkdown(msgs, path)
	require.NoError(t, err)

	got, err := os.ReadFile(path)
	require.NoError(t, err)

	content := string(got)
	assert.Contains(t, content, "# Piglet Conversation")
	assert.Contains(t, content, "What is Go?")
	assert.Contains(t, content, "Go is a statically typed language")
	assert.Contains(t, content, "### Tool: Search")
	assert.Contains(t, content, "results here")
}

func TestExportMarkdown_SkipsInvalidMessages(t *testing.T) {
	t.Parallel()

	msgs := []json.RawMessage{
		json.RawMessage(`{bad json}`),
		json.RawMessage(`{"role":"user","content":"valid"}`),
	}

	dir := t.TempDir()
	path := filepath.Join(dir, "skip-test.md")
	err := exportMarkdown(msgs, path)
	require.NoError(t, err)

	got, err := os.ReadFile(path)
	require.NoError(t, err)

	assert.Contains(t, string(got), "valid")
}

func TestExportMarkdown_EmptyMessages(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "empty.md")
	err := exportMarkdown([]json.RawMessage{}, path)
	require.NoError(t, err)

	got, err := os.ReadFile(path)
	require.NoError(t, err)

	assert.Equal(t, "# Piglet Conversation\n\n", string(got))
}
