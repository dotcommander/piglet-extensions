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

func (rt *memoryRuntime) toolSet() sdk.ToolDef {
	return sdk.ToolDef{
		Name:        "memory_set",
		Description: "Save a key-value fact to project memory, with an optional category.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"key":      map[string]any{"type": "string", "description": "Memory key"},
				"value":    map[string]any{"type": "string", "description": "Memory value"},
				"category": map[string]any{"type": "string", "description": "Optional category for grouping"},
			},
			"required": []string{"key", "value"},
		},
		PromptHint: "Save a fact to project memory",
		Execute: func(_ context.Context, args map[string]any) (*sdk.ToolResult, error) {
			s, errResult := rt.requireStore()
			if errResult != nil {
				return errResult, nil
			}
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
	}
}

func (rt *memoryRuntime) toolGet() sdk.ToolDef {
	return sdk.ToolDef{
		Name:        "memory_get",
		Description: "Retrieve a fact from project memory by key.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"key": map[string]any{"type": "string", "description": "Memory key to retrieve"},
			},
			"required": []string{"key"},
		},
		PromptHint: "Retrieve a fact from project memory",
		Execute: func(_ context.Context, args map[string]any) (*sdk.ToolResult, error) {
			s, errResult := rt.requireStore()
			if errResult != nil {
				return errResult, nil
			}
			key, _ := args["key"].(string)
			fact, ok := s.Get(key)
			if !ok {
				return sdk.TextResult("not found: " + key), nil
			}
			return sdk.TextResult(fact.Value), nil
		},
	}
}

func (rt *memoryRuntime) toolList() sdk.ToolDef {
	return sdk.ToolDef{
		Name:        "memory_list",
		Description: "List all facts in project memory, optionally filtered by category.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"category": map[string]any{"type": "string", "description": "Optional category filter"},
			},
			"required": []string{},
		},
		PromptHint: "List all project memory facts",
		Execute: func(_ context.Context, args map[string]any) (*sdk.ToolResult, error) {
			s, errResult := rt.requireStore()
			if errResult != nil {
				return errResult, nil
			}
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
	}
}

func (rt *memoryRuntime) toolRelate() sdk.ToolDef {
	return sdk.ToolDef{
		Name:        "memory_relate",
		Description: "Create a bidirectional relation between two memory facts. Both keys must exist.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"key_a": map[string]any{"type": "string", "description": "First fact key"},
				"key_b": map[string]any{"type": "string", "description": "Second fact key"},
			},
			"required": []string{"key_a", "key_b"},
		},
		PromptHint: "Link two related facts in memory",
		Execute: func(_ context.Context, args map[string]any) (*sdk.ToolResult, error) {
			s, errResult := rt.requireStore()
			if errResult != nil {
				return errResult, nil
			}
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
	}
}

func (rt *memoryRuntime) toolRelated() sdk.ToolDef {
	return sdk.ToolDef{
		Name:        "memory_related",
		Description: "Find all facts related to a key by traversing memory graph edges. Returns facts within the specified depth.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"key":       map[string]any{"type": "string", "description": "Starting fact key"},
				"max_depth": map[string]any{"type": "integer", "description": "Maximum traversal depth (default: 3)"},
			},
			"required": []string{"key"},
		},
		PromptHint: "Find related facts by traversing memory graph",
		Execute: func(_ context.Context, args map[string]any) (*sdk.ToolResult, error) {
			s, errResult := rt.requireStore()
			if errResult != nil {
				return errResult, nil
			}
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
	}
}
