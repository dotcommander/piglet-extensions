package depgraph

import (
	"fmt"
	"sort"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newTestGraph builds a small dependency graph for unit tests:
//
//	A → B → C
//	A → D
//	B → D
//	C → D
//	E (isolated)
func newTestGraph() *Graph {
	return &Graph{
		Root:   "/test",
		Module: "example.com/test",
		Packages: map[string]*Package{
			"example.com/test/a": {ImportPath: "example.com/test/a", Dir: "/test/a", Name: "a"},
			"example.com/test/b": {ImportPath: "example.com/test/b", Dir: "/test/b", Name: "b"},
			"example.com/test/c": {ImportPath: "example.com/test/c", Dir: "/test/c", Name: "c"},
			"example.com/test/d": {ImportPath: "example.com/test/d", Dir: "/test/d", Name: "d"},
			"example.com/test/e": {ImportPath: "example.com/test/e", Dir: "/test/e", Name: "e"},
		},
		Forward: map[string][]string{
			"example.com/test/a": {"example.com/test/b", "example.com/test/d"},
			"example.com/test/b": {"example.com/test/c", "example.com/test/d"},
			"example.com/test/c": {"example.com/test/d"},
		},
		Reverse: map[string][]string{
			"example.com/test/b": {"example.com/test/a"},
			"example.com/test/c": {"example.com/test/b"},
			"example.com/test/d": {"example.com/test/a", "example.com/test/b", "example.com/test/c"},
		},
		dirIndex: map[string]string{
			"/test/a": "example.com/test/a",
			"/test/b": "example.com/test/b",
			"/test/c": "example.com/test/c",
			"/test/d": "example.com/test/d",
			"/test/e": "example.com/test/e",
		},
	}
}

// --- BFS queries ---

func TestDeps(t *testing.T) {
	t.Parallel()
	g := newTestGraph()

	entries := g.Deps("example.com/test/a", 0)
	require.Len(t, entries, 3)
	// Sorted by depth, then name within depth: B(1), D(1), C(2)
	assert.Equal(t, DepEntry{"example.com/test/b", 1}, entries[0])
	assert.Equal(t, DepEntry{"example.com/test/d", 1}, entries[1])
	assert.Equal(t, DepEntry{"example.com/test/c", 2}, entries[2])
}

func TestDeps_DepthLimited(t *testing.T) {
	t.Parallel()
	g := newTestGraph()

	entries := g.Deps("example.com/test/a", 1)
	require.Len(t, entries, 2)
	for _, e := range entries {
		assert.Equal(t, 1, e.Depth)
	}
}

func TestDeps_Isolated(t *testing.T) {
	t.Parallel()
	g := newTestGraph()

	entries := g.Deps("example.com/test/e", 0)
	assert.Empty(t, entries)
}

func TestReverseDeps(t *testing.T) {
	t.Parallel()
	g := newTestGraph()

	entries := g.ReverseDeps("example.com/test/d", 0)
	require.Len(t, entries, 3)
	for _, e := range entries {
		assert.Equal(t, 1, e.Depth)
	}
}

func TestReverseDeps_DepthLimited(t *testing.T) {
	t.Parallel()
	g := newTestGraph()

	// A's reverse deps at depth 1: nobody directly imports A
	entries := g.ReverseDeps("example.com/test/a", 1)
	assert.Empty(t, entries)
}

func TestReverseDeps_Transitive(t *testing.T) {
	t.Parallel()
	g := newTestGraph()

	// B's reverse deps: A(1). C is not a reverse dep of B's importers.
	entries := g.ReverseDeps("example.com/test/b", 0)
	require.Len(t, entries, 1)
	assert.Equal(t, "example.com/test/a", entries[0].ImportPath)
}

// --- Impact ---

func TestImpact(t *testing.T) {
	t.Parallel()
	g := newTestGraph()

	affected := g.Impact("example.com/test/d")
	sort.Strings(affected)
	assert.Equal(t, []string{
		"example.com/test/a",
		"example.com/test/b",
		"example.com/test/c",
		"example.com/test/d",
	}, affected)
}

func TestImpact_ByFilePath(t *testing.T) {
	t.Parallel()
	g := newTestGraph()

	affected := g.Impact("/test/d")
	assert.NotEmpty(t, affected)
	assert.Contains(t, affected, "example.com/test/d")
}

func TestImpact_NotFound(t *testing.T) {
	t.Parallel()
	g := newTestGraph()

	affected := g.Impact("nonexistent")
	assert.Nil(t, affected)
}

// --- Cycle detection ---

func TestDetectCycles_NoCycles(t *testing.T) {
	t.Parallel()
	g := newTestGraph()

	cycles := g.DetectCycles()
	assert.Empty(t, cycles)
}

func TestDetectCycles_WithCycle(t *testing.T) {
	t.Parallel()
	g := newTestGraph()

	// Create cycle: C → A
	g.Forward["example.com/test/c"] = append(g.Forward["example.com/test/c"], "example.com/test/a")

	cycles := g.DetectCycles()
	assert.NotEmpty(t, cycles)
	// Verify cycle contains both A and C
	var found bool
	for _, c := range cycles {
		hasA, hasC := false, false
		for _, p := range c.Path {
			if p == "example.com/test/a" {
				hasA = true
			}
			if p == "example.com/test/c" {
				hasC = true
			}
		}
		if hasA && hasC {
			found = true
			break
		}
	}
	assert.True(t, found, "cycle involving A and C not found")
}

// --- Shortest path ---

func TestShortestPath(t *testing.T) {
	t.Parallel()
	g := newTestGraph()

	// A → D is direct (depth 1), shorter than A → B → C → D
	path := g.ShortestPath("example.com/test/a", "example.com/test/d")
	require.NotNil(t, path)
	assert.Equal(t, []string{"example.com/test/a", "example.com/test/d"}, path)
}

func TestShortestPath_MultiHop(t *testing.T) {
	t.Parallel()
	g := newTestGraph()

	// A → B is direct
	path := g.ShortestPath("example.com/test/a", "example.com/test/b")
	require.NotNil(t, path)
	assert.Equal(t, []string{"example.com/test/a", "example.com/test/b"}, path)
}

func TestShortestPath_NoPath(t *testing.T) {
	t.Parallel()
	g := newTestGraph()

	// E is isolated
	path := g.ShortestPath("example.com/test/e", "example.com/test/a")
	assert.Nil(t, path)
}

func TestShortestPath_SameNode(t *testing.T) {
	t.Parallel()
	g := newTestGraph()

	path := g.ShortestPath("example.com/test/a", "example.com/test/a")
	assert.Equal(t, []string{"example.com/test/a"}, path)
}

// --- Package resolution ---

func TestResolvePackage_ByImportPath(t *testing.T) {
	t.Parallel()
	g := newTestGraph()

	ip, ok := g.ResolvePackage("example.com/test/a")
	assert.True(t, ok)
	assert.Equal(t, "example.com/test/a", ip)
}

func TestResolvePackage_ByRelativePath(t *testing.T) {
	t.Parallel()
	g := newTestGraph()

	ip, ok := g.ResolvePackage("./a")
	assert.True(t, ok)
	assert.Equal(t, "example.com/test/a", ip)
}

func TestResolvePackage_ByDirPath(t *testing.T) {
	t.Parallel()
	g := newTestGraph()

	ip, ok := g.ResolvePackage("/test/a")
	assert.True(t, ok)
	assert.Equal(t, "example.com/test/a", ip)
}

func TestResolvePackage_ByFilePath(t *testing.T) {
	t.Parallel()
	g := newTestGraph()

	ip, ok := g.ResolvePackage("/test/a/foo.go")
	assert.True(t, ok)
	assert.Equal(t, "example.com/test/a", ip)
}

func TestResolvePackage_NotFound(t *testing.T) {
	t.Parallel()
	g := newTestGraph()

	_, ok := g.ResolvePackage("nonexistent")
	assert.False(t, ok)
}

func TestResolvePackage_LazyDirIndex(t *testing.T) {
	t.Parallel()

	// Graph without dirIndex (simulates deserialized cache).
	g := &Graph{
		Root:   "/test",
		Module: "example.com/test",
		Packages: map[string]*Package{
			"example.com/test/a": {ImportPath: "example.com/test/a", Dir: "/test/a", Name: "a"},
		},
		Forward: map[string][]string{},
		Reverse: map[string][]string{},
	}

	ip, ok := g.ResolvePackage("/test/a")
	assert.True(t, ok)
	assert.Equal(t, "example.com/test/a", ip)
}

// --- Formatting ---

func TestFormatDeps(t *testing.T) {
	t.Parallel()

	entries := []DepEntry{
		{ImportPath: "example.com/test/b", Depth: 1},
		{ImportPath: "example.com/test/c", Depth: 2},
	}

	result := FormatDeps("example.com/test/a", "example.com/test", entries, 0)
	assert.Contains(t, result, "example.com/test/a")
	assert.Contains(t, result, "  b")
	assert.Contains(t, result, "    c")
}

func TestFormatDeps_TokenBudget(t *testing.T) {
	t.Parallel()

	entries := make([]DepEntry, 100)
	for i := range entries {
		entries[i] = DepEntry{ImportPath: fmt.Sprintf("example.com/test/pkg%03d", i), Depth: 1}
	}

	result := FormatDeps("example.com/test/a", "example.com/test", entries, 1)
	assert.Contains(t, result, "more")
}

func TestFormatImpact(t *testing.T) {
	t.Parallel()

	result := FormatImpact(
		[]string{"example.com/test/a", "example.com/test/b"},
		"example.com/test",
		0,
	)
	assert.Contains(t, result, "2 packages")
	assert.Contains(t, result, "  a")
	assert.Contains(t, result, "  b")
}

func TestFormatImpact_SinglePackage(t *testing.T) {
	t.Parallel()

	result := FormatImpact(
		[]string{"example.com/test/a"},
		"example.com/test",
		0,
	)
	assert.Contains(t, result, "1 package")
	assert.NotContains(t, result, "packages")
}

func TestFormatImpact_ConsistentPrefix(t *testing.T) {
	t.Parallel()

	result := FormatImpact(
		[]string{"example.com/test/a", "example.com/test/b"},
		"example.com/test",
		0,
	)
	// Both entries should use the same "\n  " prefix — no double-newline.
	assert.NotContains(t, result, "\n\n  ")
}

func TestFormatCycles_None(t *testing.T) {
	t.Parallel()

	result := FormatCycles(nil, "example.com/test")
	assert.Equal(t, "No cycles detected", result)
}

func TestFormatCycles_WithCycles(t *testing.T) {
	t.Parallel()

	cycles := []Cycle{
		{Path: []string{"example.com/test/a", "example.com/test/b", "example.com/test/c"}},
	}

	result := FormatCycles(cycles, "example.com/test")
	assert.Contains(t, result, "1 cycle")
	assert.Contains(t, result, "a → b → c → a")
}

func TestFormatCycles_MultipleCycles(t *testing.T) {
	t.Parallel()

	cycles := []Cycle{
		{Path: []string{"example.com/test/a", "example.com/test/b"}},
		{Path: []string{"example.com/test/c", "example.com/test/d"}},
	}

	result := FormatCycles(cycles, "example.com/test")
	assert.Contains(t, result, "2 cycles")
}

func TestFormatPath(t *testing.T) {
	t.Parallel()

	result := FormatPath([]string{"example.com/test/a", "example.com/test/b"}, "example.com/test")
	assert.Equal(t, "a → b", result)
}

func TestFormatPath_Empty(t *testing.T) {
	t.Parallel()

	result := FormatPath(nil, "example.com/test")
	assert.Equal(t, "No path found", result)
}

// --- Cycle normalization ---

func TestNormalizeCycle(t *testing.T) {
	t.Parallel()

	// Rotate so smallest element is first.
	normalized := normalizeCycle([]string{"c", "a", "b"})
	assert.Equal(t, []string{"a", "b", "c"}, normalized)
}

func TestNormalizeCycle_Empty(t *testing.T) {
	t.Parallel()

	normalized := normalizeCycle(nil)
	assert.Nil(t, normalized)
}

func TestCycleKey(t *testing.T) {
	t.Parallel()

	key1 := cycleKey([]string{"a", "b"})
	key2 := cycleKey([]string{"a", "b"})
	assert.Equal(t, key1, key2)
}
