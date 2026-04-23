package memory

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/dotcommander/piglet-extensions/internal/xdg"
	sdk "github.com/dotcommander/piglet/sdk"
)

// Version is the memory extension version.
const Version = "0.2.0"

//go:embed defaults/compact-system.md
var defaultCompactSystem string

// Register registers the memory extension's tools, commands, and event handlers.
func Register(e *sdk.Extension) {
	var rt memoryRuntime

	e.OnInitAppend(func(x *sdk.Extension) {
		s, err := NewStore(x.CWD())
		if err != nil {
			return
		}
		rt.store = s
		rt.extractor = NewExtractor(s)

		x.RegisterPromptSection(sdk.PromptSectionDef{
			Title:   "Project Memory",
			Content: BuildMemoryPrompt(s),
			Order:   50,
		})

		x.RegisterCompactor(sdk.CompactorDef{
			Name:      "rolling-memory",
			Threshold: 50000,
			Compact:   makeCompactHandler(x, s),
		})
	})

	e.RegisterEventHandler(sdk.EventHandlerDef{
		Name:     "memory-context-reset",
		Priority: 10,
		Events:   []string{"EventAgentStart"},
		Handle: func(_ context.Context, _ string, _ json.RawMessage) *sdk.Action {
			if rt.store != nil {
				facts := rt.store.List(contextCategory)
				for _, f := range facts {
					if err := rt.store.Delete(f.Key); err != nil {
						e.Log("warn", "memory context reset: "+err.Error())
					}
				}
			}
			return nil
		},
	})

	e.RegisterEventHandler(sdk.EventHandlerDef{
		Name:     "memory-extractor",
		Priority: 50,
		Events:   []string{"EventTurnEnd"},
		Handle: func(_ context.Context, _ string, data json.RawMessage) *sdk.Action {
			if rt.extractor != nil {
				if err := rt.extractor.Extract(data); err != nil {
					e.Log("warn", "memory extract: "+err.Error())
				}
			}
			return nil
		},
	})

	registerClearer(e)
	registerOverflow(e)

	e.RegisterTool(rt.toolSet())
	e.RegisterTool(rt.toolGet())
	e.RegisterTool(rt.toolList())
	e.RegisterTool(rt.toolRelate())
	e.RegisterTool(rt.toolRelated())
	e.RegisterTool(rt.toolDelete())
	e.RegisterTool(rt.toolStatus(Version))

	e.RegisterCommand(rt.command(e))
}

// wireMsg wraps a message with a type discriminator for JSON transport.
type wireMsg struct {
	Type string          `json:"type"`
	Data json.RawMessage `json:"data"`
}

// compactConfig holds configurable parameters for the compaction handler.
type compactConfig struct {
	KeepRecent         int `yaml:"keep_recent"`
	TruncateToolResult int `yaml:"truncate_tool_result"`
	// SkipLLMThreshold skips the LLM summarisation call when estimated tokens
	// across all messages is below this value after lightweight truncation.
	// 0 disables the check (always call LLM). Measured in tokens (~4 bytes each).
	SkipLLMThreshold int `yaml:"skip_llm_threshold"`
}

func defaultCompactConfig() compactConfig {
	return compactConfig{
		KeepRecent:         6,
		TruncateToolResult: 2000,
		SkipLLMThreshold:   8000,
	}
}

// makeCompactHandler returns the SDK compact handler.
func makeCompactHandler(ext *sdk.Extension, s *Store) func(ctx context.Context, raw json.RawMessage) (json.RawMessage, error) {
	cfg := xdg.LoadYAMLExt("memory", "compact.yaml", defaultCompactConfig())

	return func(ctx context.Context, raw json.RawMessage) (json.RawMessage, error) {
		var params struct {
			Messages []wireMsg `json:"messages"`
		}
		if err := json.Unmarshal(raw, &params); err != nil {
			return nil, fmt.Errorf("unmarshal compact params: %w", err)
		}

		if len(params.Messages) <= cfg.KeepRecent+1 {
			return raw, nil
		}

		truncateToolResults(params.Messages[:len(params.Messages)-cfg.KeepRecent], cfg.TruncateToolResult)

		priorRead, priorModified := extractPriorFileLists(params.Messages)

		result := Compact(s)
		summary := result.Summary
		summary = mergeFileLists(summary, priorRead, priorModified)

		skipLLM := cfg.SkipLLMThreshold > 0 && estimateTokens(params.Messages) < cfg.SkipLLMThreshold
		if summary != "" && !skipLLM {
			resp, err := ext.Chat(ctx, sdk.ChatRequest{
				System:   strings.TrimSpace(defaultCompactSystem),
				Messages: []sdk.ChatMessage{{Role: "user", Content: summary}},
				Model:    "small",
			})
			if err == nil && resp.Text != "" {
				summary = resp.Text
			}
		}

		WriteSummary(s, summary)

		var ref strings.Builder
		ref.WriteString("[Context compacted — session memory updated]\n\n")
		ref.WriteString("Use memory_list category=_context to see accumulated context.\n")
		ref.WriteString("Use memory_get to retrieve specific facts.\n")
		if summary != "" {
			ref.WriteString("\nSummary: ")
			ref.WriteString(summary)
		}

		summaryData, err := json.Marshal(map[string]any{
			"role":    "user",
			"content": ref.String(),
		})
		if err != nil {
			return nil, fmt.Errorf("marshal summary message: %w", err)
		}

		kept := params.Messages[len(params.Messages)-cfg.KeepRecent:]
		wire := make([]wireMsg, 0, len(kept)+2)
		wire = append(wire, wireMsg{Type: "user", Data: summaryData})

		items := gatherCriticalContext(s)
		reinjectMsg := buildReinjectMessage(items)
		if reinjectMsg != "" {
			reinjectData, err := json.Marshal(map[string]any{
				"role":    "user",
				"content": reinjectMsg,
			})
			if err != nil {
				ext.Log("warn", "memory reinject marshal: "+err.Error())
			} else {
				wire = append(wire, wireMsg{Type: "user", Data: reinjectData})
			}
		}

		wire = append(wire, kept...)
		return json.Marshal(map[string]any{"messages": wire})
	}
}
