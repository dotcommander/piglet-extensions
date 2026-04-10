package provider

import (
	"context"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/dotcommander/piglet/core"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGoogle_StreamText(t *testing.T) {
	t.Parallel()

	msg := collectFinalMsg(t, streamTestCase{
		ModelID:  "gemini-test",
		Provider: "google",
		API:      core.APIGoogle,
		APIKey:   "goog-test",
		Request: core.StreamRequest{
			System:   "Be helpful.",
			Messages: []core.Message{&core.UserMessage{Content: "Hi", Timestamp: time.Now()}},
		},
		Handler: func(w http.ResponseWriter, r *http.Request) {
			assert.Contains(t, r.URL.RawQuery, "alt=sse")
			assert.Equal(t, "goog-test", r.Header.Get("x-goog-api-key"))

			w.Header().Set("Content-Type", "text/event-stream")
			fmt.Fprint(w, sseLines(
				`{"candidates":[{"content":{"role":"model","parts":[{"text":"Hello"}]}}],"usageMetadata":{"promptTokenCount":10,"candidatesTokenCount":1}}`,
				`{"candidates":[{"content":{"role":"model","parts":[{"text":" world"}]},"finishReason":"STOP"}],"usageMetadata":{"promptTokenCount":10,"candidatesTokenCount":2}}`,
			))
		},
	}, googleProvider)

	requireFinalText(t, msg, "Hello world")
	assert.Equal(t, core.StopReasonStop, msg.StopReason)
	assert.Equal(t, 10, msg.Usage.InputTokens)
	assert.Equal(t, 2, msg.Usage.OutputTokens)
}

func TestGoogle_StreamToolCall(t *testing.T) {
	t.Parallel()

	result := drainStream(t, streamTestCase{
		ModelID:  "gemini-test",
		Provider: "google",
		API:      core.APIGoogle,
		SSEData: []string{
			`{"candidates":[{"content":{"role":"model","parts":[{"functionCall":{"name":"search","args":{"query":"cats"}}}]},"finishReason":"STOP"}]}`,
		},
		Request: core.StreamRequest{
			Messages: []core.Message{&core.UserMessage{Content: "search cats", Timestamp: time.Now()}},
			Tools:    []core.ToolSchema{{Name: "search", Description: "web search"}},
		},
	}, googleProvider)

	require.NotNil(t, result.FinalMessage)
	require.NotNil(t, result.ToolCallEnd)
	assert.Equal(t, "search", result.ToolCallEnd.Name)
	assert.Equal(t, "cats", result.ToolCallEnd.Arguments["query"])
	tc := requireToolCall(t, result.FinalMessage, 0)
	assert.Equal(t, "search", tc.Name)
	assert.Equal(t, "cats", tc.Arguments["query"])
}

func TestGoogle_StreamHTTPError(t *testing.T) {
	t.Parallel()

	result := drainStream(t, streamTestCase{
		ModelID:  "gemini-test",
		Provider: "google",
		API:      core.APIGoogle,
		APIKey:   "bad-key",
		Handler: func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusForbidden)
			fmt.Fprint(w, `{"error":{"code":403,"message":"API key not valid"}}`)
		},
	}, googleProvider)

	assert.True(t, result.GotError)
	assert.Contains(t, result.ErrorMessage, "403")
}

func TestGoogle_StreamMaxTokensStopReason(t *testing.T) {
	t.Parallel()

	msg := collectFinalMsg(t, streamTestCase{
		ModelID:   "gemini-test",
		Provider:  "google",
		API:       core.APIGoogle,
		MaxTokens: 5,
		SSEData: []string{
			`{"candidates":[{"content":{"role":"model","parts":[{"text":"Truncated"}]},"finishReason":"MAX_TOKENS"}]}`,
		},
		Request: core.StreamRequest{
			Messages: []core.Message{&core.UserMessage{Content: "long story", Timestamp: time.Now()}},
		},
	}, googleProvider)

	assert.Equal(t, core.StopReasonLength, msg.StopReason)
}

func TestGoogle_StreamSafetyStopReason(t *testing.T) {
	t.Parallel()

	msg := collectFinalMsg(t, streamTestCase{
		ModelID:  "gemini-test",
		Provider: "google",
		API:      core.APIGoogle,
		SSEData: []string{
			`{"candidates":[{"content":{"role":"model","parts":[]},"finishReason":"SAFETY"}]}`,
		},
	}, googleProvider)

	assert.Equal(t, core.StopReasonError, msg.StopReason)
}

func TestGoogle_StreamCancellation(t *testing.T) {
	t.Parallel()

	server := newCancellationServer(`{"candidates":[{"content":{"parts":[{"text":"Hi"}]}}]}`)
	defer server.CloseClientConnections()
	defer server.Close()

	model := core.Model{
		ID: "gemini-test", Provider: "google", API: core.APIGoogle,
		BaseURL: server.URL, MaxTokens: 1024,
	}
	prov := NewGoogle(model, func() string { return "key" })

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	ch := prov.Stream(ctx, core.StreamRequest{
		Messages: []core.Message{&core.UserMessage{Content: "test", Timestamp: time.Now()}},
	})
	for range ch {
	}
}

func TestGoogle_StreamEndpointURL(t *testing.T) {
	t.Parallel()

	var gotPath, gotQuery, gotHeader string
	result := drainStream(t, streamTestCase{
		ModelID:   "gemini-2.0-flash",
		Provider:  "google",
		API:       core.APIGoogle,
		APIKey:    "mykey",
		MaxTokens: 100,
		Handler: func(w http.ResponseWriter, r *http.Request) {
			gotPath = r.URL.Path
			gotQuery = r.URL.RawQuery
			gotHeader = r.Header.Get("x-goog-api-key")
			w.Header().Set("Content-Type", "text/event-stream")
			fmt.Fprint(w, sseLines(
				`{"candidates":[{"content":{"role":"model","parts":[]},"finishReason":"STOP"}]}`,
			))
		},
	}, googleProvider)
	_ = result

	assert.Equal(t, "/v1beta/models/gemini-2.0-flash:streamGenerateContent", gotPath)
	assert.Contains(t, gotQuery, "alt=sse")
	assert.Equal(t, "mykey", gotHeader)
}

func TestGoogle_BuildRequest_SystemInstruction(t *testing.T) {
	t.Parallel()

	result := drainStream(t, streamTestCase{
		ModelID:     "gemini-test",
		Provider:    "google",
		API:         core.APIGoogle,
		CollectBody: true,
		Request: core.StreamRequest{
			System:   "You are a helpful assistant.",
			Messages: []core.Message{&core.UserMessage{Content: "hi", Timestamp: time.Now()}},
		},
		SSEData: []string{
			`{"candidates":[{"content":{"role":"model","parts":[]},"finishReason":"STOP"}]}`,
		},
	}, googleProvider)

	body := string(result.CapturedBody)
	assert.Contains(t, body, `"systemInstruction"`)
	assert.Contains(t, body, "You are a helpful assistant.")
}

func TestGoogle_BuildRequest_Tools(t *testing.T) {
	t.Parallel()

	result := drainStream(t, streamTestCase{
		ModelID:     "gemini-test",
		Provider:    "google",
		API:         core.APIGoogle,
		CollectBody: true,
		Request: core.StreamRequest{
			Messages: []core.Message{&core.UserMessage{Content: "hi", Timestamp: time.Now()}},
			Tools: []core.ToolSchema{
				{Name: "my_tool", Description: "does something", Parameters: map[string]any{"type": "object"}},
			},
		},
		SSEData: []string{
			`{"candidates":[{"content":{"role":"model","parts":[]},"finishReason":"STOP"}]}`,
		},
	}, googleProvider)

	body := string(result.CapturedBody)
	assert.Contains(t, body, `"functionDeclarations"`)
	assert.Contains(t, body, `"my_tool"`)
}

func TestGoogle_BuildRequest_MaxTokens(t *testing.T) {
	t.Parallel()

	customMax := 256
	result := drainStream(t, streamTestCase{
		ModelID:     "gemini-test",
		Provider:    "google",
		API:         core.APIGoogle,
		CollectBody: true,
		Request: core.StreamRequest{
			Messages: []core.Message{&core.UserMessage{Content: "hi", Timestamp: time.Now()}},
			Options:  core.StreamOptions{MaxTokens: &customMax},
		},
		SSEData: []string{
			`{"candidates":[{"content":{"role":"model","parts":[]},"finishReason":"STOP"}]}`,
		},
	}, googleProvider)

	assert.Contains(t, string(result.CapturedBody), `"maxOutputTokens":256`)
}

func TestGoogle_StreamImageBlock(t *testing.T) {
	t.Parallel()

	result := drainStream(t, streamTestCase{
		ModelID:     "gemini-test",
		Provider:    "google",
		API:         core.APIGoogle,
		CollectBody: true,
		Request: core.StreamRequest{
			Messages: []core.Message{
				&core.UserMessage{
					Content: "describe this",
					Blocks: []core.ContentBlock{
						core.ImageContent{MimeType: "image/png", Data: "abc123"},
					},
					Timestamp: time.Now(),
				},
			},
		},
		SSEData: []string{
			`{"candidates":[{"content":{"role":"model","parts":[{"text":"Nice image"}]},"finishReason":"STOP"}]}`,
		},
	}, googleProvider)

	body := string(result.CapturedBody)
	assert.Contains(t, body, `"inlineData"`)
	assert.Contains(t, body, `"image/png"`)
	assert.Contains(t, body, `"abc123"`)
}

func TestGoogle_StreamToolResult(t *testing.T) {
	t.Parallel()

	now := time.Now()
	result := drainStream(t, streamTestCase{
		ModelID:     "gemini-test",
		Provider:    "google",
		API:         core.APIGoogle,
		CollectBody: true,
		Request: core.StreamRequest{
			Messages: []core.Message{
				&core.UserMessage{Content: "use the tool", Timestamp: now},
				&core.AssistantMessage{
					Content: []core.AssistantContent{core.ToolCall{
						ID: "call_1", Name: "my_func", Arguments: map[string]any{"arg": "val"},
					}},
					StopReason: core.StopReasonTool,
					Timestamp:  now,
				},
				&core.ToolResultMessage{
					ToolCallID: "call_1",
					ToolName:   "my_func",
					Content:    []core.ContentBlock{core.TextContent{Text: "result_value"}},
					Timestamp:  now,
				},
			},
		},
		SSEData: []string{
			`{"candidates":[{"content":{"role":"model","parts":[{"text":"Done"}]},"finishReason":"STOP"}]}`,
		},
	}, googleProvider)

	body := string(result.CapturedBody)
	assert.Contains(t, body, `"functionResponse"`)
	assert.Contains(t, body, `"my_func"`)
	assert.Contains(t, body, `"result_value"`)
}

func TestGoogle_StreamToolErrorResult(t *testing.T) {
	t.Parallel()

	now := time.Now()
	result := drainStream(t, streamTestCase{
		ModelID:     "gemini-test",
		Provider:    "google",
		API:         core.APIGoogle,
		CollectBody: true,
		Request: core.StreamRequest{
			Messages: []core.Message{
				&core.UserMessage{Content: "use the tool", Timestamp: now},
				&core.AssistantMessage{
					Content: []core.AssistantContent{core.ToolCall{
						ID: "call_1", Name: "my_func", Arguments: map[string]any{},
					}},
					StopReason: core.StopReasonTool,
					Timestamp:  now,
				},
				&core.ToolResultMessage{
					ToolCallID: "call_1",
					ToolName:   "my_func",
					Content:    []core.ContentBlock{core.TextContent{Text: "it failed"}},
					IsError:    true,
					Timestamp:  now,
				},
			},
		},
		SSEData: []string{
			`{"candidates":[{"content":{"role":"model","parts":[{"text":"Sorry"}]},"finishReason":"STOP"}]}`,
		},
	}, googleProvider)

	body := string(result.CapturedBody)
	assert.Contains(t, body, `"error"`)
	assert.Contains(t, body, `"it failed"`)
}

func TestGoogle_StreamAssistantMessage(t *testing.T) {
	t.Parallel()

	now := time.Now()
	result := drainStream(t, streamTestCase{
		ModelID:     "gemini-test",
		Provider:    "google",
		API:         core.APIGoogle,
		CollectBody: true,
		Request: core.StreamRequest{
			Messages: []core.Message{
				&core.UserMessage{Content: "hello", Timestamp: now},
				&core.AssistantMessage{
					Content:    []core.AssistantContent{core.TextContent{Text: "hi there"}},
					StopReason: core.StopReasonStop,
					Timestamp:  now,
				},
				&core.UserMessage{Content: "how are you", Timestamp: now},
			},
		},
		SSEData: []string{
			`{"candidates":[{"content":{"role":"model","parts":[{"text":"Yes"}]},"finishReason":"STOP"}]}`,
		},
	}, googleProvider)

	body := string(result.CapturedBody)
	assert.Contains(t, body, `"model"`)
	assert.Contains(t, body, `"hi there"`)
}

func TestGoogle_StreamMetadata(t *testing.T) {
	t.Parallel()

	msg := collectFinalMsg(t, streamTestCase{
		ModelID:  "gemini-test",
		Provider: "google",
		API:      core.APIGoogle,
		SSEData: []string{
			`{"candidates":[{"content":{"role":"model","parts":[{"text":"Done"}]},"finishReason":"STOP"}],"usageMetadata":{"promptTokenCount":20,"candidatesTokenCount":5,"cachedContentTokenCount":10}}`,
		},
	}, googleProvider)

	assert.Equal(t, 20, msg.Usage.InputTokens)
	assert.Equal(t, 5, msg.Usage.OutputTokens)
	assert.Equal(t, 10, msg.Usage.CacheReadTokens)
	assert.Equal(t, "gemini-test", msg.Model)
	assert.Equal(t, "google", msg.Provider)
	assert.False(t, msg.Timestamp.IsZero())
}
