package memory

import (
	"context"
	"fmt"
	"strings"

	sdk "github.com/dotcommander/piglet/sdk"
)

// memoryRuntime holds initialized state for the memory extension.
// Initialized in Register's OnInitAppend callback; replaces mutable package globals.
type memoryRuntime struct {
	store     *Store
	extractor *Extractor
}

// requireStore returns the store or an sdk.ErrorResult if not initialized.
func (rt *memoryRuntime) requireStore() (*Store, *sdk.ToolResult) {
	if rt == nil || rt.store == nil {
		return nil, sdk.ErrorResult("memory store not available")
	}
	return rt.store, nil
}

// toolDef creates a ToolDef with an automatic requireStore guard.
func (rt *memoryRuntime) toolDef(name, desc string, params map[string]any, promptHint string,
	exec func(ctx context.Context, s *Store, args map[string]any) (*sdk.ToolResult, error),
) sdk.ToolDef {
	return sdk.ToolDef{
		Name:        name,
		Description: desc,
		Parameters:  params,
		PromptHint:  promptHint,
		Execute: func(ctx context.Context, args map[string]any) (*sdk.ToolResult, error) {
			s, errResult := rt.requireStore()
			if errResult != nil {
				return errResult, nil
			}
			return exec(ctx, s, args)
		},
	}
}

func (rt *memoryRuntime) toolSet() sdk.ToolDef {
	return rt.toolDef(
		"memory_set",
		"Save a key-value fact to project memory, with an optional category.",
		map[string]any{
			"type": "object",
			"properties": map[string]any{
				"key":      map[string]any{"type": "string", "description": "Memory key"},
				"value":    map[string]any{"type": "string", "description": "Memory value"},
				"category": map[string]any{"type": "string", "description": "Optional category for grouping"},
			},
			"required": []string{"key", "value"},
		},
		"Save a fact to project memory",
		func(_ context.Context, s *Store, args map[string]any) (*sdk.ToolResult, error) {
			key, _ := args["key"].(string)
			value, _ := args["value"].(string)
			category, _ := args["category"].(string)
			if key == "" || value == "" {
				return sdk.ErrorResult("key and value are required"), nil
			}
			if err := s.Set(key, value, category); err != nil {
				return sdk.ErrorResult(fmt.Sprintf("error: %v", err)), nil
			}
			return sdk.TextResult("Saved: " + key), nil
		},
	)
}

func (rt *memoryRuntime) toolGet() sdk.ToolDef {
	return rt.toolDef(
		"memory_get",
		"Retrieve a fact from project memory by key.",
		map[string]any{
			"type": "object",
			"properties": map[string]any{
				"key": map[string]any{"type": "string", "description": "Memory key to retrieve"},
			},
			"required": []string{"key"},
		},
		"Retrieve a fact from project memory",
		func(_ context.Context, s *Store, args map[string]any) (*sdk.ToolResult, error) {
			key, _ := args["key"].(string)
			fact, ok := s.Get(key)
			if !ok {
				return sdk.TextResult("not found: " + key), nil
			}
			return sdk.TextResult(fact.Value), nil
		},
	)
}

func (rt *memoryRuntime) toolList() sdk.ToolDef {
	return rt.toolDef(
		"memory_list",
		"List all facts in project memory, optionally filtered by category.",
		map[string]any{
			"type": "object",
			"properties": map[string]any{
				"category": map[string]any{"type": "string", "description": "Optional category filter"},
			},
			"required": []string{},
		},
		"List all project memory facts",
		func(_ context.Context, s *Store, args map[string]any) (*sdk.ToolResult, error) {
			category, _ := args["category"].(string)
			facts := s.List(category)
			if len(facts) == 0 {
				return sdk.TextResult("No memories stored"), nil
			}
			var b strings.Builder
			for _, f := range facts {
				b.WriteString(f.Key)
				b.WriteString(": ")
				b.WriteString(f.Value)
				b.WriteByte('\n')
			}
			return sdk.TextResult(strings.TrimRight(b.String(), "\n")), nil
		},
	)
}

func (rt *memoryRuntime) toolRelate() sdk.ToolDef {
	return rt.toolDef(
		"memory_relate",
		"Create a bidirectional relation between two memory facts. Both keys must exist.",
		map[string]any{
			"type": "object",
			"properties": map[string]any{
				"key_a": map[string]any{"type": "string", "description": "First fact key"},
				"key_b": map[string]any{"type": "string", "description": "Second fact key"},
			},
			"required": []string{"key_a", "key_b"},
		},
		"Link two related facts in memory",
		func(_ context.Context, s *Store, args map[string]any) (*sdk.ToolResult, error) {
			keyA, _ := args["key_a"].(string)
			keyB, _ := args["key_b"].(string)
			if keyA == "" || keyB == "" {
				return sdk.ErrorResult("key_a and key_b are required"), nil
			}
			if err := s.Relate(keyA, keyB); err != nil {
				return sdk.ErrorResult(err.Error()), nil
			}
			return sdk.TextResult(fmt.Sprintf("Linked: %s ↔ %s", keyA, keyB)), nil
		},
	)
}

func (rt *memoryRuntime) toolRelated() sdk.ToolDef {
	return rt.toolDef(
		"memory_related",
		"Find all facts related to a key by traversing memory graph edges. Returns facts within the specified depth.",
		map[string]any{
			"type": "object",
			"properties": map[string]any{
				"key":       map[string]any{"type": "string", "description": "Starting fact key"},
				"max_depth": map[string]any{"type": "integer", "description": "Maximum traversal depth (default: 3)"},
			},
			"required": []string{"key"},
		},
		"Find related facts by traversing memory graph",
		func(_ context.Context, s *Store, args map[string]any) (*sdk.ToolResult, error) {
			key, _ := args["key"].(string)
			if key == "" {
				return sdk.ErrorResult("key is required"), nil
			}
			maxDepth := 3
			if md, ok := args["max_depth"].(float64); ok && int(md) > 0 {
				maxDepth = int(md)
			}
			facts := Related(s, key, maxDepth)
			if len(facts) == 0 {
				return sdk.TextResult("No related facts found for: " + key), nil
			}
			var b strings.Builder
			for _, f := range facts {
				b.WriteString(f.Key)
				b.WriteString(": ")
				b.WriteString(f.Value)
				if len(f.Relations) > 0 {
					fmt.Fprintf(&b, " [→ %s]", strings.Join(f.Relations, ", "))
				}
				b.WriteByte('\n')
			}
			return sdk.TextResult(strings.TrimRight(b.String(), "\n")), nil
		},
	)
}

func (rt *memoryRuntime) toolDelete() sdk.ToolDef {
	return rt.toolDef(
		"memory_delete",
		"Delete a fact from project memory by key.",
		map[string]any{
			"type": "object",
			"properties": map[string]any{
				"key": map[string]any{"type": "string", "description": "Memory key to delete"},
			},
			"required": []string{"key"},
		},
		"Delete a fact from project memory",
		func(_ context.Context, s *Store, args map[string]any) (*sdk.ToolResult, error) {
			key, _ := args["key"].(string)
			if key == "" {
				return sdk.ErrorResult("key is required"), nil
			}
			if err := s.Delete(key); err != nil {
				return sdk.ErrorResult(err.Error()), nil
			}
			return sdk.TextResult("Deleted: " + key), nil
		},
	)
}

func (rt *memoryRuntime) toolStatus(version string) sdk.ToolDef {
	return sdk.ToolDef{
		Name:        "memory_status",
		Description: "Show memory extension status: version, store path, and fact counts.",
		Parameters:  map[string]any{"type": "object", "properties": map[string]any{}},
		Execute: func(_ context.Context, _ map[string]any) (*sdk.ToolResult, error) {
			if rt == nil || rt.store == nil {
				return sdk.TextResult(fmt.Sprintf("memory v%s\nStore not available.", version)), nil
			}
			facts := rt.store.List("")
			return sdk.TextResult(fmt.Sprintf("memory v%s\nStore: %s\nFacts: %d stored", version, rt.store.Path(), len(facts))), nil
		},
	}
}
