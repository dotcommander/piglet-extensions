package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/dotcommander/piglet/core"
	pigletprovider "github.com/dotcommander/piglet/provider"
)

// ---------------------------------------------------------------------------
// HTTP
// ---------------------------------------------------------------------------

func (p *Google) endpoint() string {
	base := strings.TrimSuffix(p.Model.BaseURL, "/")
	return fmt.Sprintf("%s/v1beta/models/%s:streamGenerateContent?alt=sse", base, p.Model.ID)
}

func (p *Google) SendRequest(ctx context.Context, body []byte) (io.ReadCloser, error) {
	return p.DoHTTPRequest(ctx, p.endpoint(), body, func(req *http.Request) {
		if apiKey := p.APIKeyFn(); apiKey != "" {
			req.Header.Set("x-goog-api-key", apiKey)
		}
	})
}

// ---------------------------------------------------------------------------
// Stream parsing
// ---------------------------------------------------------------------------

type gemResponse struct {
	Candidates    []gemCandidate `json:"candidates"`
	UsageMetadata *gemUsage      `json:"usageMetadata,omitempty"`
}

type gemCandidate struct {
	Content      gemContent `json:"content"`
	FinishReason string     `json:"finishReason"`
}

type gemUsage struct {
	PromptTokenCount        int `json:"promptTokenCount"`
	CandidatesTokenCount    int `json:"candidatesTokenCount"`
	CachedContentTokenCount int `json:"cachedContentTokenCount"`
}

func (p *Google) ParseResponse(ctx context.Context, reader io.Reader, ch chan<- core.StreamEvent) core.AssistantMessage {
	var msg core.AssistantMessage
	textBuilders := make(map[int]*strings.Builder)

	pigletprovider.ScanSSE(ctx, reader, ch, func(data []byte) {
		var resp gemResponse
		if err := json.Unmarshal(data, &resp); err != nil {
			return
		}

		if resp.UsageMetadata != nil {
			msg.Usage = core.Usage{
				InputTokens:     resp.UsageMetadata.PromptTokenCount,
				OutputTokens:    resp.UsageMetadata.CandidatesTokenCount,
				CacheReadTokens: resp.UsageMetadata.CachedContentTokenCount,
			}
		}

		if len(resp.Candidates) == 0 {
			return
		}

		candidate := resp.Candidates[0]

		for _, part := range candidate.Content.Parts {
			if part.Text != "" {
				ch <- core.StreamEvent{Type: core.StreamTextDelta, Delta: part.Text}
				pigletprovider.AppendTextBuilder(&msg, part.Text, textBuilders)
			}

			if part.FunctionCall != nil {
				tc := core.ToolCall{
					ID:        fmt.Sprintf("call_%d", len(msg.Content)),
					Name:      part.FunctionCall.Name,
					Arguments: part.FunctionCall.Args,
				}
				if part.ThoughtSignature != "" {
					tc.ProviderMeta = map[string]any{
						"thoughtSignature": part.ThoughtSignature,
					}
				}
				msg.Content = append(msg.Content, tc)
				ch <- core.StreamEvent{
					Type: core.StreamToolCallEnd,
					Tool: &tc,
				}
			}
		}

		if candidate.FinishReason != "" {
			msg.StopReason = p.mapFinishReason(candidate.FinishReason)
		}
	}, pigletprovider.WithLargeBuffer(256*1024, 10*1024*1024))

	pigletprovider.FinalizeTextBuilders(&msg, textBuilders)

	return msg
}

var gemStopReasons = map[string]core.StopReason{
	"STOP":       core.StopReasonStop,
	"MAX_TOKENS": core.StopReasonLength,
	"SAFETY":     core.StopReasonError,
	"RECITATION": core.StopReasonError,
}

func (p *Google) mapFinishReason(reason string) core.StopReason {
	return pigletprovider.MapStopReasonFromTable(reason, gemStopReasons)
}
