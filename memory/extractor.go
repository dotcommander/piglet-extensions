package memory

import (
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"
)

const (
	contextCategory = "_context"
	maxContextFacts = 50
	pruneCount      = 10
)

// TurnData represents the relevant data from an EventTurnEnd, parsed from JSON.
type TurnData struct {
	Assistant   json.RawMessage   `json:"Assistant"`
	ToolResults []json.RawMessage `json:"ToolResults"`
}

// toolCall represents a tool call from an assistant message.
type toolCall struct {
	Name string         `json:"name"`
	Args map[string]any `json:"args"`
}

// toolResult represents a tool result message.
type toolResult struct {
	ToolName   string      `json:"tool_name"`
	ToolCallID string      `json:"tool_call_id"`
	IsError    bool        `json:"is_error"`
	Content    []textBlock `json:"content"`
}

// assistantMsg represents an assistant message (partial parse).
type assistantMsg struct {
	Content []json.RawMessage `json:"content"`
}

// Extractor writes structured facts from each turn to the memory store.
type Extractor struct {
	store   *Store
	turnNum int
}

// NewExtractor creates an extractor backed by the given store.
func NewExtractor(store *Store) *Extractor {
	return &Extractor{store: store}
}

// Extract parses turn data and writes facts to the store.
// Returns any store write errors joined together.
func (e *Extractor) Extract(eventData json.RawMessage) error {
	var turn TurnData
	if err := json.Unmarshal(eventData, &turn); err != nil {
		return nil // silently skip unparseable events
	}
	e.turnNum++

	var errs []error
	for _, raw := range turn.ToolResults {
		var tr toolResult
		if json.Unmarshal(raw, &tr) != nil {
			continue
		}
		if err := e.extractToolResult(tr); err != nil {
			errs = append(errs, err)
		}
	}

	e.pruneIfNeeded()
	return errors.Join(errs...)
}

func (e *Extractor) extractToolResult(tr toolResult) error {
	text := e.resultText(tr)

	switch tr.ToolName {
	case "Read":
		if path := extractFilePath(text); path != "" {
			if err := e.store.Set("ctx:file:"+path, truncRunes(text, 300), contextCategory); err != nil {
				return fmt.Errorf("set ctx:file:%s: %w", path, err)
			}
		}
	case "Grep", "Glob":
		if path := extractFilePath(text); path != "" {
			if err := e.store.Set("ctx:search:"+path, truncRunes(text, 300), contextCategory); err != nil {
				return fmt.Errorf("set ctx:search:%s: %w", path, err)
			}
		}
	case "Edit", "Write":
		if path := extractFilePath(text); path != "" {
			if err := e.store.Set("ctx:edit:"+path, truncRunes(text, 200), contextCategory); err != nil {
				return fmt.Errorf("set ctx:edit:%s: %w", path, err)
			}
		}
	case "Bash":
		key := fmt.Sprintf("ctx:cmd:%d", e.turnNum)
		if tr.IsError {
			key = fmt.Sprintf("ctx:error:%d", e.turnNum)
		}
		if err := e.store.Set(key, truncRunes(text, 300), contextCategory); err != nil {
			return fmt.Errorf("set %s: %w", key, err)
		}
	default:
		if text != "" {
			key := fmt.Sprintf("ctx:tool:%s:%d", tr.ToolName, e.turnNum)
			if err := e.store.Set(key, truncRunes(text, 200), contextCategory); err != nil {
				return fmt.Errorf("set %s: %w", key, err)
			}
		}
	}
	return nil
}

func (e *Extractor) resultText(tr toolResult) string {
	for _, c := range tr.Content {
		if c.Type == "text" && c.Text != "" {
			return c.Text
		}
	}
	return ""
}

func (e *Extractor) pruneIfNeeded() {
	facts := e.store.List(contextCategory)
	if len(facts) <= maxContextFacts {
		return
	}

	// Sort by UpdatedAt ascending, prune oldest
	slices.SortFunc(facts, func(a, b Fact) int {
		return a.UpdatedAt.Compare(b.UpdatedAt)
	})

	for i := range min(pruneCount, len(facts)) {
		_ = e.store.Delete(facts[i].Key)
	}
}

// extractFilePath tries to find a file path in tool result text.
func extractFilePath(text string) string {
	line, _, _ := strings.Cut(text, "\n")
	line = strings.TrimSpace(line)

	if strings.HasPrefix(line, "/") {
		if idx := strings.IndexByte(line, ' '); idx > 0 {
			return line[:idx]
		}
		return line
	}

	if strings.Contains(text, "/") {
		for _, word := range strings.Fields(line) {
			if strings.Count(word, "/") >= 2 && !strings.HasPrefix(word, "http") {
				return strings.Trim(word, "\"'`,;:")
			}
		}
	}

	return ""
}
