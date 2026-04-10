package distill

import (
	"encoding/json"
	"fmt"
	"strings"
)

const maxFormatChars = 6000

// contentBlock represents a single block in a content array.
type contentBlock struct {
	Type string `json:"type"`
	Text string `json:"text"`
	Name string `json:"name"`
}

// messageContent holds the parsed content of a message field (string or []block).
type messageContent struct {
	RawString string
	Blocks    []contentBlock
}

// message is the common envelope for JSON-RPC message parsing.
type message struct {
	Role    string          `json:"role"`
	Content json.RawMessage `json:"content"`
}

// parseContent unmarshals a content field which may be a plain string or []block.
func parseContent(raw json.RawMessage) messageContent {
	if len(raw) == 0 {
		return messageContent{}
	}
	if raw[0] == '"' {
		var s string
		if json.Unmarshal(raw, &s) == nil {
			return messageContent{RawString: s}
		}
		return messageContent{}
	}
	var blocks []contentBlock
	if json.Unmarshal(raw, &blocks) != nil {
		return messageContent{}
	}
	return messageContent{Blocks: blocks}
}

// scoreComplexity analyzes messages for session complexity.
func scoreComplexity(messages []json.RawMessage) int {
	score := len(messages)

	var prevHadError bool
	for _, raw := range messages {
		tools := countToolUseBlocks(raw)
		score += tools * 2
		if prevHadError && tools > 0 {
			score += 3
		}
		prevHadError = assistantTextContainsError(raw)
	}

	return score
}

// countToolUseBlocks counts tool_use content blocks in a message.
func countToolUseBlocks(raw json.RawMessage) int {
	var msg struct {
		Content json.RawMessage `json:"content"`
	}
	if json.Unmarshal(raw, &msg) != nil || len(msg.Content) == 0 {
		return 0
	}

	pc := parseContent(msg.Content)
	count := 0
	for _, b := range pc.Blocks {
		if b.Type == "tool_use" {
			count++
		}
	}
	return count
}

// assistantTextContainsError returns true if msg is an assistant message
// with "error" in any text block.
func assistantTextContainsError(raw json.RawMessage) bool {
	var msg message
	if json.Unmarshal(raw, &msg) != nil || msg.Role != "assistant" {
		return false
	}
	pc := parseContent(msg.Content)
	if pc.RawString != "" {
		return strings.Contains(strings.ToLower(pc.RawString), "error")
	}
	for _, b := range pc.Blocks {
		if b.Type == "text" && strings.Contains(strings.ToLower(b.Text), "error") {
			return true
		}
	}
	return false
}

// formatMessages converts raw JSON messages into a readable transcript for the LLM.
func formatMessages(messages []json.RawMessage, maxBytes int) string {
	var b strings.Builder
	for _, raw := range messages {
		line := formatMessage(raw)
		if line == "" {
			continue
		}
		if b.Len()+len(line) > maxBytes {
			remaining := maxBytes - b.Len()
			if remaining > 0 && remaining < len(line) {
				b.WriteString(line[:remaining])
			}
			break
		}
		b.WriteString(line)
	}
	return b.String()
}

// formatMessage renders a single message as a readable line.
func formatMessage(raw json.RawMessage) string {
	var msg message
	if json.Unmarshal(raw, &msg) != nil || msg.Role == "" {
		return ""
	}

	role := strings.ToUpper(msg.Role[:1]) + msg.Role[1:]
	content := extractTextContent(msg.Content)
	if content == "" {
		return ""
	}
	return role + ": " + content + "\n"
}

// extractTextContent pulls readable text out of a content field (string or []block).
func extractTextContent(raw json.RawMessage) string {
	pc := parseContent(raw)
	if pc.RawString != "" {
		return pc.RawString
	}

	var parts []string
	for _, b := range pc.Blocks {
		switch b.Type {
		case "text":
			if b.Text != "" {
				parts = append(parts, b.Text)
			}
		case "tool_use":
			parts = append(parts, fmt.Sprintf("[tool: %s]", b.Name))
		case "tool_result":
			parts = append(parts, "[tool result]")
		}
	}
	return strings.Join(parts, " ")
}
