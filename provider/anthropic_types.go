package provider

import "encoding/json"

// ---------------------------------------------------------------------------
// Anthropic request types
// ---------------------------------------------------------------------------

type antRequest struct {
	Model     string       `json:"model"`
	MaxTokens int          `json:"max_tokens"`
	System    any          `json:"system,omitempty"` // string or []antSystemBlock
	Messages  []antMessage `json:"messages"`
	Stream    bool         `json:"stream"`
	Tools     []antTool    `json:"tools,omitempty"`
}

type antCacheControl struct {
	Type string `json:"type"`
}

type antSystemBlock struct {
	Type         string           `json:"type"`
	Text         string           `json:"text"`
	CacheControl *antCacheControl `json:"cache_control,omitempty"`
}

type antMessage struct {
	Role    string `json:"role"`
	Content any    `json:"content"` // string or []antBlock
}

type antBlock struct {
	Type         string           `json:"type"`
	CacheControl *antCacheControl `json:"cache_control,omitempty"`
	// Text block
	Text string `json:"text,omitempty"`
	// Image block
	Source *antImageSource `json:"source,omitempty"`
	// Tool use block
	ID    string `json:"id,omitempty"`
	Name  string `json:"name,omitempty"`
	Input any    `json:"input,omitempty"`
	// Tool result block
	ToolUseID string `json:"tool_use_id,omitempty"`
	Content   any    `json:"content,omitempty"` // string or []antBlock
	IsError   bool   `json:"is_error,omitempty"`
}

type antImageSource struct {
	Type      string `json:"type"`
	MediaType string `json:"media_type"`
	Data      string `json:"data"`
}

type antTool struct {
	Name         string           `json:"name"`
	Description  string           `json:"description"`
	InputSchema  any              `json:"input_schema"`
	CacheControl *antCacheControl `json:"cache_control,omitempty"`
}

// ---------------------------------------------------------------------------
// Anthropic SSE response types
// ---------------------------------------------------------------------------

type antStreamEvent struct {
	Type  string          `json:"type"`
	Index int             `json:"index"`
	Delta json.RawMessage `json:"delta,omitempty"`

	// content_block_start
	ContentBlock *antContentBlock `json:"content_block,omitempty"`

	// message_start
	Message *antStreamMessage `json:"message,omitempty"`

	// message_delta
	Usage *antStreamUsage `json:"usage,omitempty"`
}

type antContentBlock struct {
	Type string `json:"type"`
	ID   string `json:"id,omitempty"`
	Name string `json:"name,omitempty"`
	Text string `json:"text,omitempty"`
}

type antStreamMessage struct {
	Usage *antStreamUsage `json:"usage,omitempty"`
}

type antStreamUsage struct {
	InputTokens              int `json:"input_tokens"`
	OutputTokens             int `json:"output_tokens"`
	CacheCreationInputTokens int `json:"cache_creation_input_tokens"`
	CacheReadInputTokens     int `json:"cache_read_input_tokens"`
}

type antDelta struct {
	Type        string `json:"type"`
	Text        string `json:"text,omitempty"`
	PartialJSON string `json:"partial_json,omitempty"`
	StopReason  string `json:"stop_reason,omitempty"`
}
