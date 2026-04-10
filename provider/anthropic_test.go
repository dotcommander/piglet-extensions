package provider

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/dotcommander/piglet/core"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAnthropic_StreamText(t *testing.T) {
	t.Parallel()

	msg := collectFinalMsg(t, streamTestCase{
		ModelID:  "claude-haiku-test",
		Provider: "anthropic",
		API:      core.APIAnthropic,
		APIKey:   "sk-ant-test",
		Request: core.StreamRequest{
			System:   "Be helpful.",
			Messages: []core.Message{&core.UserMessage{Content: "Hi", Timestamp: time.Now()}},
		},
		Handler: func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "sk-ant-test", r.Header.Get("x-api-key"))
			assert.Equal(t, "2023-06-01", r.Header.Get("anthropic-version"))
			assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
			assert.Equal(t, "text/event-stream", r.Header.Get("Accept"))

			w.Header().Set("Content-Type", "text/event-stream")
			fmt.Fprint(w, sseLines(
				`{"type":"message_start","message":{"usage":{"input_tokens":42,"output_tokens":0,"cache_creation_input_tokens":5,"cache_read_input_tokens":0}}}`,
				`{"type":"content_block_start","index":0,"content_block":{"type":"text"}}`,
				`{"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"Hello"}}`,
				`{"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":" world"}}`,
				`{"type":"message_delta","delta":{"stop_reason":"end_turn"},"usage":{"output_tokens":7,"cache_creation_input_tokens":5,"cache_read_input_tokens":0}}`,
			))
		},
	}, anthropicProvider)

	requireFinalText(t, msg, "Hello world")
	assert.Equal(t, core.StopReasonStop, msg.StopReason)
	assert.Equal(t, 42, msg.Usage.InputTokens)
	assert.Equal(t, 7, msg.Usage.OutputTokens)
	assert.Equal(t, 5, msg.Usage.CacheWriteTokens)
}

func TestAnthropic_StreamToolCall(t *testing.T) {
	t.Parallel()

	result := drainStream(t, streamTestCase{
		ModelID:  "claude-haiku-test",
		Provider: "anthropic",
		API:      core.APIAnthropic,
		SSEData: []string{
			`{"type":"message_start","message":{"usage":{"input_tokens":20,"output_tokens":0}}}`,
			`{"type":"content_block_start","index":0,"content_block":{"type":"tool_use","id":"toolu_01","name":"calculator"}}`,
			`{"type":"content_block_delta","index":0,"delta":{"type":"input_json_delta","partial_json":"{\"x\":"}}`,
			`{"type":"content_block_delta","index":0,"delta":{"type":"input_json_delta","partial_json":"42}"}}`,
			`{"type":"message_delta","delta":{"stop_reason":"tool_use"},"usage":{"output_tokens":10}}`,
		},
		Request: core.StreamRequest{
			Messages: []core.Message{&core.UserMessage{Content: "Calculate", Timestamp: time.Now()}},
			Tools:    []core.ToolSchema{{Name: "calculator", Description: "Does math"}},
		},
	}, anthropicProvider)

	require.NotNil(t, result.FinalMessage)
	assert.Equal(t, core.StopReasonTool, result.FinalMessage.StopReason)
	tc := requireToolCall(t, result.FinalMessage, 0)
	assert.Equal(t, "toolu_01", tc.ID)
	assert.Equal(t, "calculator", tc.Name)
	assert.Equal(t, float64(42), tc.Arguments["x"])
	assert.Equal(t, []string{"{\"x\":", "42}"}, result.ToolDeltas)
}

func TestAnthropic_StreamHTTPError(t *testing.T) {
	t.Parallel()

	result := drainStream(t, streamTestCase{
		ModelID:  "claude-haiku-test",
		Provider: "anthropic",
		API:      core.APIAnthropic,
		APIKey:   "bad",
		Handler: func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusUnauthorized)
			fmt.Fprint(w, `{"error":{"type":"authentication_error","message":"Invalid API Key"}}`)
		},
	}, anthropicProvider)

	assert.True(t, result.GotError)
	assert.Contains(t, result.ErrorMessage, "401")
}

func TestAnthropic_StreamRateLimitError(t *testing.T) {
	t.Parallel()

	result := drainStream(t, streamTestCase{
		ModelID:  "claude-test",
		Provider: "anthropic",
		API:      core.APIAnthropic,
		Handler: func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusTooManyRequests)
			fmt.Fprint(w, `{"error":{"type":"rate_limit_error","message":"Rate limit exceeded"}}`)
		},
	}, anthropicProvider)

	assert.True(t, result.GotError)
	assert.Contains(t, result.ErrorMessage, "429")
}

func TestAnthropic_StreamMaxTokensStopReason(t *testing.T) {
	t.Parallel()

	msg := collectFinalMsg(t, streamTestCase{
		ModelID:   "claude-test",
		Provider:  "anthropic",
		API:       core.APIAnthropic,
		MaxTokens: 5,
		SSEData: []string{
			`{"type":"message_start","message":{"usage":{"input_tokens":10}}}`,
			`{"type":"content_block_start","index":0,"content_block":{"type":"text"}}`,
			`{"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"Truncated"}}`,
			`{"type":"message_delta","delta":{"stop_reason":"max_tokens"},"usage":{"output_tokens":3}}`,
		},
		Request: core.StreamRequest{
			Messages: []core.Message{&core.UserMessage{Content: "tell me a long story", Timestamp: time.Now()}},
		},
	}, anthropicProvider)

	assert.Equal(t, core.StopReasonLength, msg.StopReason)
}

func TestAnthropic_StreamCancellation(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		flusher, _ := w.(http.Flusher)
		fmt.Fprint(w, "data: {\"type\":\"content_block_delta\",\"index\":0,\"delta\":{\"type\":\"text_delta\",\"text\":\"Hi\"}}\n\n")
		if flusher != nil {
			flusher.Flush()
		}
		<-r.Context().Done()
	}))
	defer server.CloseClientConnections()
	defer server.Close()

	model := core.Model{
		ID: "claude-test", Provider: "anthropic", API: core.APIAnthropic,
		BaseURL: server.URL, MaxTokens: 1024,
	}
	prov := NewAnthropic(model, func() string { return "key" })

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	ch := prov.Stream(ctx, core.StreamRequest{
		Messages: []core.Message{&core.UserMessage{Content: "test", Timestamp: time.Now()}},
	})
	for range ch {
	}
}

func TestAnthropic_StreamWithCustomMaxTokens(t *testing.T) {
	t.Parallel()

	maxTok := 512
	result := drainStream(t, streamTestCase{
		ModelID:     "claude-test",
		Provider:    "anthropic",
		API:         core.APIAnthropic,
		MaxTokens:   2048,
		CollectBody: true,
		Request: core.StreamRequest{
			Messages: []core.Message{&core.UserMessage{Content: "hi", Timestamp: time.Now()}},
			Options:  core.StreamOptions{MaxTokens: &maxTok},
		},
		SSEData: []string{
			`{"type":"message_start","message":{"usage":{"input_tokens":5}}}`,
			`{"type":"message_delta","delta":{"stop_reason":"end_turn"},"usage":{"output_tokens":0}}`,
		},
	}, anthropicProvider)

	assert.Contains(t, string(result.CapturedBody), `"max_tokens":512`)
}

func TestAnthropic_StreamMultipleContentBlocks(t *testing.T) {
	t.Parallel()

	msg := collectFinalMsg(t, streamTestCase{
		ModelID:  "claude-test",
		Provider: "anthropic",
		API:      core.APIAnthropic,
		SSEData: []string{
			`{"type":"message_start","message":{"usage":{"input_tokens":10}}}`,
			`{"type":"content_block_start","index":0,"content_block":{"type":"text"}}`,
			`{"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"First"}}`,
			`{"type":"content_block_start","index":1,"content_block":{"type":"tool_use","id":"toolu_02","name":"search"}}`,
			`{"type":"content_block_delta","index":1,"delta":{"type":"input_json_delta","partial_json":"{\"q\":\"hello\"}"}}`,
			`{"type":"message_delta","delta":{"stop_reason":"tool_use"},"usage":{"output_tokens":15}}`,
		},
		Request: core.StreamRequest{
			Messages: []core.Message{&core.UserMessage{Content: "search", Timestamp: time.Now()}},
			Tools:    []core.ToolSchema{{Name: "search", Description: "web search"}},
		},
	}, anthropicProvider)

	require.Len(t, msg.Content, 2)
	_, isText := msg.Content[0].(core.TextContent)
	assert.True(t, isText, "first block should be TextContent")
	tc := requireToolCall(t, msg, 1)
	assert.Equal(t, "toolu_02", tc.ID)
	assert.Equal(t, "search", tc.Name)
}

func TestAnthropic_StreamEndpointURL(t *testing.T) {
	t.Parallel()

	var gotPath string
	result := drainStream(t, streamTestCase{
		ModelID:   "claude-test",
		Provider:  "anthropic",
		API:       core.APIAnthropic,
		MaxTokens: 100,
		Handler: func(w http.ResponseWriter, r *http.Request) {
			gotPath = r.URL.Path
			w.Header().Set("Content-Type", "text/event-stream")
			fmt.Fprint(w, sseLines(
				`{"type":"message_delta","delta":{"stop_reason":"end_turn"},"usage":{"output_tokens":0}}`,
			))
		},
	}, anthropicProvider)
	_ = result

	assert.Equal(t, "/v1/messages", gotPath)
}

func TestAnthropic_StreamSystemPromptCached(t *testing.T) {
	t.Parallel()

	result := drainStream(t, streamTestCase{
		ModelID:     "claude-test",
		Provider:    "anthropic",
		API:         core.APIAnthropic,
		CollectBody: true,
		Request: core.StreamRequest{
			System:   "You are an expert assistant.",
			Messages: []core.Message{&core.UserMessage{Content: "hi", Timestamp: time.Now()}},
		},
		SSEData: []string{
			`{"type":"message_delta","delta":{"stop_reason":"end_turn"},"usage":{"output_tokens":0}}`,
		},
	}, anthropicProvider)

	body := string(result.CapturedBody)
	assert.Contains(t, body, `"system"`)
	assert.Contains(t, body, `"cache_control"`)
	assert.Contains(t, body, `"ephemeral"`)
}

func TestAnthropic_BuildRequest_ToolsCached(t *testing.T) {
	t.Parallel()

	result := drainStream(t, streamTestCase{
		ModelID:     "claude-test",
		Provider:    "anthropic",
		API:         core.APIAnthropic,
		CollectBody: true,
		Request: core.StreamRequest{
			Messages: []core.Message{&core.UserMessage{Content: "hi", Timestamp: time.Now()}},
			Tools: []core.ToolSchema{
				{Name: "tool_a", Description: "first tool"},
				{Name: "tool_b", Description: "last tool"},
			},
		},
		SSEData: []string{
			`{"type":"message_delta","delta":{"stop_reason":"end_turn"},"usage":{"output_tokens":0}}`,
		},
	}, anthropicProvider)

	body := string(result.CapturedBody)
	assert.Contains(t, body, `"cache_control"`)
	assert.Contains(t, body, `"tool_a"`)
	assert.Contains(t, body, `"tool_b"`)
}

func TestAnthropic_StreamCacheReadTokens(t *testing.T) {
	t.Parallel()

	msg := collectFinalMsg(t, streamTestCase{
		ModelID:  "claude-test",
		Provider: "anthropic",
		API:      core.APIAnthropic,
		SSEData: []string{
			`{"type":"message_start","message":{"usage":{"input_tokens":5,"cache_read_input_tokens":100}}}`,
			`{"type":"message_delta","delta":{"stop_reason":"end_turn"},"usage":{"output_tokens":3,"cache_read_input_tokens":100}}`,
		},
	}, anthropicProvider)

	assert.Equal(t, 100, msg.Usage.CacheReadTokens)
}

func TestAnthropic_StreamMessageMetadata(t *testing.T) {
	t.Parallel()

	msg := collectFinalMsg(t, streamTestCase{
		ModelID:  "claude-test",
		Provider: "anthropic",
		API:      core.APIAnthropic,
		SSEData: []string{
			`{"type":"message_start","message":{"usage":{"input_tokens":10}}}`,
			`{"type":"message_delta","delta":{"stop_reason":"end_turn"},"usage":{"output_tokens":5}}`,
		},
	}, anthropicProvider)

	assert.Equal(t, "claude-test", msg.Model)
	assert.Equal(t, "anthropic", msg.Provider)
	assert.False(t, msg.Timestamp.IsZero())
}

func TestAnthropic_ConversationHistory(t *testing.T) {
	t.Parallel()

	now := time.Now()
	result := drainStream(t, streamTestCase{
		ModelID:     "claude-test",
		Provider:    "anthropic",
		API:         core.APIAnthropic,
		CollectBody: true,
		Request: core.StreamRequest{
			Messages: []core.Message{
				&core.UserMessage{Content: "first", Timestamp: now},
				&core.AssistantMessage{
					Content:    []core.AssistantContent{core.TextContent{Text: "response"}},
					StopReason: core.StopReasonStop,
					Timestamp:  now,
				},
				&core.UserMessage{Content: "second", Timestamp: now},
			},
		},
		SSEData: []string{
			`{"type":"message_delta","delta":{"stop_reason":"end_turn"},"usage":{"output_tokens":0}}`,
		},
	}, anthropicProvider)

	body := string(result.CapturedBody)
	assert.Contains(t, body, `"first"`)
	assert.Contains(t, body, `"response"`)
	assert.Contains(t, body, `"second"`)
	assert.Contains(t, body, `"cache_control"`)
}

func TestAnthropic_ToolResultMessage(t *testing.T) {
	t.Parallel()

	now := time.Now()
	result := drainStream(t, streamTestCase{
		ModelID:     "claude-test",
		Provider:    "anthropic",
		API:         core.APIAnthropic,
		CollectBody: true,
		Request: core.StreamRequest{
			Messages: []core.Message{
				&core.UserMessage{Content: "run the tool", Timestamp: now},
				&core.AssistantMessage{
					Content: []core.AssistantContent{core.ToolCall{
						ID: "toolu_01", Name: "calc", Arguments: map[string]any{"x": 1},
					}},
					StopReason: core.StopReasonTool,
					Timestamp:  now,
				},
				&core.ToolResultMessage{
					ToolCallID: "toolu_01",
					ToolName:   "calc",
					Content:    []core.ContentBlock{core.TextContent{Text: "42"}},
					Timestamp:  now,
				},
			},
		},
		SSEData: []string{
			`{"type":"message_delta","delta":{"stop_reason":"end_turn"},"usage":{"output_tokens":0}}`,
		},
	}, anthropicProvider)

	body := string(result.CapturedBody)
	assert.Contains(t, body, `"tool_result"`)
	assert.Contains(t, body, `"toolu_01"`)
	assert.Contains(t, body, `"42"`)
}
