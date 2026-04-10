package scaffold

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestValidateName(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{"valid simple", "my-tool", false},
		{"valid with numbers", "tool123", false},
		{"valid with underscore", "my_tool", false},
		{"valid single char", "a", false},
		{"valid multi hyphen", "my-cool-tool", false},
		{"empty", "", true},
		{"spaces", "my tool", true},
		{"path traversal", "../etc", true},
		{"double dot", "my..tool", true},
		{"starts with number", "1tool", true},
		{"uppercase", "MyTool", true},
		{"special chars", "my@tool", true},
		{"dot", ".", true},
		{"slash", "my/tool", true},
		{"backslash", `my\tool`, true},
		{"null byte", "my\x00tool", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := validateName(tt.input)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestRenderTemplates(t *testing.T) {
	t.Parallel()

	manifest, indexTS, err := renderTemplates("my-tool")
	assert.NoError(t, err)

	assert.Contains(t, manifest, "name: my-tool")
	assert.Contains(t, manifest, "runtime: bun")
	assert.Contains(t, indexTS, "my-tool_hello")
	assert.NotContains(t, indexTS, "{{NAME}}")
}

func TestRenderTemplatesNoCrossContamination(t *testing.T) {
	t.Parallel()

	m1, _, _ := renderTemplates("alpha")
	m2, _, _ := renderTemplates("beta")

	assert.Contains(t, m1, "name: alpha")
	assert.NotContains(t, m1, "beta")
	assert.Contains(t, m2, "name: beta")
	assert.NotContains(t, m2, "alpha")
}

func TestRenderTemplatesContent(t *testing.T) {
	t.Parallel()

	manifest, indexTS, err := renderTemplates("webhook")
	assert.NoError(t, err)

	assert.True(t, strings.HasPrefix(manifest, "name: webhook\n"), "manifest starts with name")
	assert.Contains(t, indexTS, `import { piglet } from "@piglet/sdk"`)
	assert.Contains(t, indexTS, `piglet.setInfo("webhook", "0.1.0")`)
	assert.Contains(t, indexTS, "webhook_hello")
	assert.Contains(t, indexTS, `piglet.notify("webhook:`)
}
