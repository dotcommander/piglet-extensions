package lsp

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFormatLocations(t *testing.T) {
	t.Parallel()

	t.Run("empty", func(t *testing.T) {
		t.Parallel()
		assert.Equal(t, "No results found.", FormatLocations(nil, "", 0))
	})

	t.Run("single location", func(t *testing.T) {
		t.Parallel()
		locs := []Location{
			{URI: "file:///tmp/test.go", Range: Range{Start: Position{Line: 9, Character: 0}}},
		}
		out := FormatLocations(locs, "/tmp", 0)
		assert.Contains(t, out, "1 location(s) found")
		assert.Contains(t, out, "test.go:10")
	})

	t.Run("multiple locations", func(t *testing.T) {
		t.Parallel()
		locs := []Location{
			{URI: "file:///tmp/a.go", Range: Range{Start: Position{Line: 0, Character: 0}}},
			{URI: "file:///tmp/b.go", Range: Range{Start: Position{Line: 4, Character: 2}}},
		}
		out := FormatLocations(locs, "/tmp", 0)
		assert.Contains(t, out, "2 location(s) found")
		assert.Contains(t, out, "a.go:1")
		assert.Contains(t, out, "b.go:5")
	})
}

func TestFormatHover(t *testing.T) {
	t.Parallel()

	t.Run("nil", func(t *testing.T) {
		t.Parallel()
		assert.Equal(t, "No hover information available.", FormatHover(nil))
	})

	t.Run("empty value", func(t *testing.T) {
		t.Parallel()
		hover := &HoverResult{Contents: MarkupContent{Kind: "plaintext", Value: ""}}
		assert.Equal(t, "No hover information available.", FormatHover(hover))
	})

	t.Run("with content", func(t *testing.T) {
		t.Parallel()
		hover := &HoverResult{Contents: MarkupContent{Kind: "markdown", Value: "```go\nfunc foo() int\n```"}}
		assert.Equal(t, "```go\nfunc foo() int\n```", FormatHover(hover))
	})
}

func TestFormatWorkspaceEdit(t *testing.T) {
	t.Parallel()

	t.Run("nil", func(t *testing.T) {
		t.Parallel()
		assert.Equal(t, "No changes.", FormatWorkspaceEdit(nil, ""))
	})

	t.Run("empty", func(t *testing.T) {
		t.Parallel()
		edit := &WorkspaceEdit{Changes: map[string][]TextEdit{}}
		assert.Equal(t, "No changes.", FormatWorkspaceEdit(edit, ""))
	})

	t.Run("single file single edit", func(t *testing.T) {
		t.Parallel()
		edit := &WorkspaceEdit{
			Changes: map[string][]TextEdit{
				"file:///tmp/main.go": {{Range: Range{Start: Position{Line: 4, Character: 0}}, NewText: "renamedFunc"}},
			},
		}
		out := FormatWorkspaceEdit(edit, "/tmp")
		assert.Contains(t, out, "main.go: 1 edit(s)")
		assert.Contains(t, out, "line 5: \"renamedFunc\"")
		assert.Contains(t, out, "1 file(s), 1 edit(s) total")
	})

	t.Run("multi file multi edit", func(t *testing.T) {
		t.Parallel()
		edit := &WorkspaceEdit{
			Changes: map[string][]TextEdit{
				"file:///tmp/a.go": {
					{Range: Range{Start: Position{Line: 0}}, NewText: "new1"},
					{Range: Range{Start: Position{Line: 1}}, NewText: "new2"},
				},
				"file:///tmp/b.go": {
					{Range: Range{Start: Position{Line: 9}}, NewText: "new3"},
				},
			},
		}
		out := FormatWorkspaceEdit(edit, "/tmp")
		assert.Contains(t, out, "2 file(s), 3 edit(s) total")
	})
}

func TestFormatSymbols(t *testing.T) {
	t.Parallel()

	t.Run("empty", func(t *testing.T) {
		t.Parallel()
		assert.Equal(t, "No symbols found.", FormatSymbols(nil, ""))
	})

	t.Run("flat symbols", func(t *testing.T) {
		t.Parallel()
		symbols := []DocumentSymbol{
			{Name: "main", Kind: SymbolKindFunction, Range: Range{Start: Position{Line: 9}}},
			{Name: "Config", Kind: SymbolKindStruct, Range: Range{Start: Position{Line: 3}}},
		}
		out := FormatSymbols(symbols, "")
		assert.Contains(t, out, "function main (line 10)")
		assert.Contains(t, out, "struct Config (line 4)")
	})

	t.Run("nested symbols", func(t *testing.T) {
		t.Parallel()
		symbols := []DocumentSymbol{
			{
				Name: "Server", Kind: SymbolKindStruct,
				Range:          Range{Start: Position{Line: 4}},
				SelectionRange: Range{Start: Position{Line: 4}},
				Children: []DocumentSymbol{
					{Name: "Start", Kind: SymbolKindMethod, Range: Range{Start: Position{Line: 10}}},
					{Name: "Stop", Kind: SymbolKindMethod, Range: Range{Start: Position{Line: 20}}},
				},
			},
		}
		out := FormatSymbols(symbols, "")
		assert.Contains(t, out, "struct Server (line 5)")
		assert.Contains(t, out, "  method Start (line 11)")
		assert.Contains(t, out, "  method Stop (line 21)")
	})
}

func TestFormatContext(t *testing.T) {
	t.Parallel()

	lines := []string{"line0", "line1", "target", "line3", "line4"}

	t.Run("with context", func(t *testing.T) {
		t.Parallel()
		out := formatContext(lines, 2, 1)
		assert.Contains(t, out, ">    3 | target")
		assert.Contains(t, out, "     2 | line1")
		assert.Contains(t, out, "     4 | line3")
	})

	t.Run("empty lines", func(t *testing.T) {
		t.Parallel()
		assert.Equal(t, "", formatContext(nil, 0, 1))
	})
}

func TestRelativePath(t *testing.T) {
	t.Parallel()

	t.Run("empty cwd", func(t *testing.T) {
		t.Parallel()
		assert.Equal(t, "/abs/path", relativePath("/abs/path", ""))
	})

	t.Run("relative within cwd", func(t *testing.T) {
		t.Parallel()
		rel := relativePath("/home/user/project/main.go", "/home/user/project")
		assert.Equal(t, "main.go", rel)
	})

	t.Run("different roots", func(t *testing.T) {
		t.Parallel()
		rel := relativePath("/other/path/file.go", "/home/user/project")
		// filepath.Rel produces ../../other/path/file.go
		assert.Contains(t, rel, "file.go")
	})
}

func TestCachedLines(t *testing.T) {
	t.Parallel()

	t.Run("missing file", func(t *testing.T) {
		t.Parallel()
		cache := make(map[string][]string)
		assert.Nil(t, cachedLines(cache, "/nonexistent/file.go"))
	})

	t.Run("cache hit", func(t *testing.T) {
		t.Parallel()
		cache := map[string][]string{"/tmp/test.go": {"line1", "line2"}}
		result := cachedLines(cache, "/tmp/test.go")
		require.Len(t, result, 2)
		assert.Equal(t, "line1", result[0])
	})
}
