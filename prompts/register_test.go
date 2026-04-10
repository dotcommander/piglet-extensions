package prompts

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParsePromptFile(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    string
		wantDesc string
		wantBody string
	}{
		{
			name:     "empty input",
			input:    "",
			wantDesc: "",
			wantBody: "",
		},
		{
			name:     "no frontmatter",
			input:    "Hello {{.Name}}!",
			wantDesc: "",
			wantBody: "Hello {{.Name}}!",
		},
		{
			name:     "frontmatter with description",
			input:    "---\ndescription: My prompt\n---\nBody content here",
			wantDesc: "My prompt",
			wantBody: "Body content here",
		},
		{
			name:     "frontmatter without description",
			input:    "---\n---\nJust the body",
			wantDesc: "",
			wantBody: "Just the body",
		},
		{
			name:     "frontmatter empty fields",
			input:    "---\ndescription: \"\"\n---\nBody",
			wantDesc: "",
			wantBody: "Body",
		},
		{
			name:     "missing closing fence",
			input:    "---\ndescription: Oops\nNo closing fence",
			wantDesc: "",
			wantBody: "---\ndescription: Oops\nNo closing fence",
		},
		{
			name:     "CRLF line endings",
			input:    "---\r\ndescription: CRLF prompt\r\n---\r\nBody with CRLF",
			wantDesc: "CRLF prompt",
			wantBody: "Body with CRLF",
		},
		{
			name:     "CRLF no frontmatter",
			input:    "Just a body\r\nwith CRLF",
			wantDesc: "",
			wantBody: "Just a body\nwith CRLF",
		},
		{
			name:     "body with trailing whitespace",
			input:    "---\n---\n  padded body  ",
			wantDesc: "",
			wantBody: "padded body",
		},
		{
			name:     "multibody content",
			input:    "---\ndescription: Complex\n---\nLine 1\n\nLine 3\n- bullet",
			wantDesc: "Complex",
			wantBody: "Line 1\n\nLine 3\n- bullet",
		},
		{
			name:     "invalid YAML in frontmatter",
			input:    "---\n: invalid : yaml : here\n---\nBody",
			wantDesc: "",
			wantBody: "---\n: invalid : yaml : here\n---\nBody",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			desc, body := parsePromptFile([]byte(tt.input))
			assert.Equal(t, tt.wantDesc, desc, "description")
			assert.Equal(t, tt.wantBody, body, "body")
		})
	}
}

func TestExpandTemplate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		body string
		args []string
		want string
	}{
		{
			name: "no placeholders",
			body: "static text",
			args: []string{"ignored"},
			want: "static text",
		},
		{
			name: "$@ all args",
			body: "prefix $@ suffix",
			args: []string{"a", "b", "c"},
			want: "prefix a b c suffix",
		},
		{
			name: "$@ no args",
			body: "prefix $@ suffix",
			args: nil,
			want: "prefix  suffix",
		},
		{
			name: "$1 positional",
			body: "hello $1",
			args: []string{"world"},
			want: "hello world",
		},
		{
			name: "$1 through $5",
			body: "$1 $2 $3 $4 $5",
			args: []string{"a", "b", "c", "d", "e"},
			want: "a b c d e",
		},
		{
			name: "out of range $5",
			body: "$1 and $5",
			args: []string{"only", "two"},
			want: "only and ",
		},
		{
			name: "no args positional",
			body: "$1 $2",
			args: nil,
			want: " ",
		},
		{
			name: "${@:2} slice from index",
			body: "${@:2}",
			args: []string{"skip", "this", "and", "this"},
			want: "this and this",
		},
		{
			name: "${@:1} slice from start",
			body: "${@:1}",
			args: []string{"a", "b", "c"},
			want: "a b c",
		},
		{
			name: "${@:2:1} slice with length",
			body: "${@:2:1}",
			args: []string{"skip", "only", "this", "not"},
			want: "only",
		},
		{
			name: "${@:1:2} first two",
			body: "${@:1:2}",
			args: []string{"a", "b", "c", "d"},
			want: "a b",
		},
		{
			name: "${@:10} out of range",
			body: "${@:10}",
			args: []string{"a", "b"},
			want: "",
		},
		{
			name: "${@:0} invalid index",
			body: "${@:0}",
			args: []string{"a"},
			want: "",
		},
		{
			name: "mixed placeholders",
			body: "dear $1,\n${@:2}\nregards",
			args: []string{"Alice", "please", "review", "this"},
			want: "dear Alice,\nplease review this\nregards",
		},
		{
			name: "reverse order safety $1 vs $10",
			body: "$1 and $9",
			args: []string{"first", "2", "3", "4", "5", "6", "7", "8", "ninth"},
			want: "first and ninth",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := expandTemplate(tt.body, tt.args)
			assert.Equal(t, tt.want, got)
		})
	}
}
