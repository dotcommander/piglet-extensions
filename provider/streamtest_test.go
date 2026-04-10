package provider

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/dotcommander/piglet/core"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// sseLines builds a mock SSE stream from data payloads.
func sseLines(lines ...string) string {
	var b strings.Builder
	for _, line := range lines {
		b.WriteString("data: ")
		b.WriteString(line)
		b.WriteString("\n\n")
	}
	return b.String()
}

// streamTestResult holds the collected output from a stream test.
type streamTestResult struct {
	TextDeltas   []string
	ToolDeltas   []string
	ToolCallEnd  *core.ToolCall
	FinalMessage *core.AssistantMessage
	CapturedBody []byte
	GotError     bool
	ErrorMessage string
}

// streamTestCase configures a provider streaming test.
type streamTestCase struct {
	// Handler is the test server handler. If nil, one is created from sseData.
	Handler http.HandlerFunc
	// SSE data lines to serve (used when Handler is nil).
	SSEData []string
	// Model fields.
	ModelID   string
	Provider  string
	API       core.API
	MaxTokens int
	// API key returned by the provider.
	APIKey string
	// Request to send.
	Request core.StreamRequest
	// CollectBody captures the request body if true.
	CollectBody bool
}

// streamTestDefault returns a streamTestCase with sensible defaults.
// Caller sets ModelID, API, and Provider for the target provider.
func streamTestDefault() streamTestCase {
	return streamTestCase{
		MaxTokens: 1024,
		APIKey:    "test-key",
		Request: core.StreamRequest{
			Messages: []core.Message{&core.UserMessage{Content: "test", Timestamp: time.Now()}},
		},
	}
}

// runStreamTest executes a provider stream test against an httptest server.
// It collects text deltas, tool deltas, the final message, and optionally the request body.
func runStreamTest(t *testing.T, tc streamTestCase, newProvider func(core.Model, func() string) interface {
	Stream(context.Context, core.StreamRequest) <-chan core.StreamEvent
}) streamTestResult {
	t.Helper()

	var result streamTestResult

	handler := tc.Handler
	if handler == nil {
		handler = func(w http.ResponseWriter, r *http.Request) {
			if tc.CollectBody {
				result.CapturedBody, _ = io.ReadAll(r.Body)
			}
			w.Header().Set("Content-Type", "text/event-stream")
			fmt.Fprint(w, sseLines(tc.SSEData...))
		}
	}

	server := httptest.NewServer(handler)
	defer server.Close()

	model := core.Model{
		ID:        tc.ModelID,
		Provider:  tc.Provider,
		API:       tc.API,
		BaseURL:   server.URL,
		MaxTokens: tc.MaxTokens,
	}

	prov := newProvider(model, func() string { return tc.APIKey })
	ch := prov.Stream(context.Background(), tc.Request)

	for evt := range ch {
		switch evt.Type {
		case core.StreamTextDelta:
			result.TextDeltas = append(result.TextDeltas, evt.Delta)
		case core.StreamToolCallDelta:
			result.ToolDeltas = append(result.ToolDeltas, evt.Delta)
		case core.StreamToolCallEnd:
			result.ToolCallEnd = evt.Tool
		case core.StreamDone:
			result.FinalMessage = evt.Message
		case core.StreamError:
			result.GotError = true
			result.ErrorMessage = evt.Error.Error()
		}
	}

	return result
}

// collectFinalMsg runs a stream and returns only the final message.
// Fatal-fails the test if no final message is received.
func collectFinalMsg(t *testing.T, tc streamTestCase, newProvider func(core.Model, func() string) interface {
	Stream(context.Context, core.StreamRequest) <-chan core.StreamEvent
}) *core.AssistantMessage {
	t.Helper()
	result := runStreamTest(t, tc, newProvider)
	if result.GotError {
		t.Fatalf("unexpected stream error: %s", result.ErrorMessage)
	}
	require.NotNil(t, result.FinalMessage)
	return result.FinalMessage
}

// drainStream runs a stream to completion, discarding all events.
func drainStream(t *testing.T, tc streamTestCase, newProvider func(core.Model, func() string) interface {
	Stream(context.Context, core.StreamRequest) <-chan core.StreamEvent
}) streamTestResult {
	t.Helper()
	return runStreamTest(t, tc, newProvider)
}

// requireFinalText asserts that the final message contains a single TextContent with the given text.
func requireFinalText(t *testing.T, msg *core.AssistantMessage, text string) {
	t.Helper()
	require.NotNil(t, msg)
	require.Len(t, msg.Content, 1)
	tc, ok := msg.Content[0].(core.TextContent)
	require.True(t, ok)
	assert.Equal(t, text, tc.Text)
}

// requireToolCall asserts that the final message contains a ToolCall at the given index.
func requireToolCall(t *testing.T, msg *core.AssistantMessage, idx int) core.ToolCall {
	t.Helper()
	require.NotNil(t, msg)
	require.True(t, idx < len(msg.Content), "content index %d out of range (len=%d)", idx, len(msg.Content))
	tc, ok := msg.Content[idx].(core.ToolCall)
	require.True(t, ok, "content[%d] is not a ToolCall", idx)
	return tc
}

// newCancellationServer creates an httptest server that sends one SSE event
// then blocks until the client cancels. Used for cancellation tests.
func newCancellationServer(sseData string) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		flusher, _ := w.(http.Flusher)
		fmt.Fprintf(w, "data: %s\n\n", sseData)
		if flusher != nil {
			flusher.Flush()
		}
		<-r.Context().Done()
	}))
}

// anthropicProvider is a typed wrapper for use in runStreamTest.
func anthropicProvider(model core.Model, keyFn func() string) interface {
	Stream(context.Context, core.StreamRequest) <-chan core.StreamEvent
} {
	return NewAnthropic(model, keyFn)
}

// googleProvider is a typed wrapper for use in runStreamTest.
func googleProvider(model core.Model, keyFn func() string) interface {
	Stream(context.Context, core.StreamRequest) <-chan core.StreamEvent
} {
	return NewGoogle(model, keyFn)
}
