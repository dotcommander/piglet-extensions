package sift

import (
	"context"
	"slices"

	sdk "github.com/dotcommander/piglet/sdk"
)

// Register wires the sift extension into a shared SDK extension.
func Register(e *sdk.Extension) {
	var cfg Config

	e.OnInit(func(x *sdk.Extension) {
		cfg = LoadConfig()

		if !cfg.Enabled {
			return
		}

		prompt := LoadPrompt(x)
		x.RegisterPromptSection(sdk.PromptSectionDef{
			Title:   "Sift Result Compression",
			Content: prompt,
			Order:   91,
		})
	})

	e.RegisterInterceptor(sdk.InterceptorDef{
		Name:     "sift",
		Priority: 50,
		After: func(_ context.Context, toolName string, details any) (any, error) {
			if !cfg.Enabled {
				return details, nil
			}

			if len(cfg.Tools) > 0 && !slices.Contains(cfg.Tools, toolName) {
				return details, nil
			}

			text, ok := extractText(details)
			if !ok || len(text) < cfg.SizeThreshold {
				return details, nil
			}

			compressed := Compress(text, cfg)
			if compressed == text {
				return details, nil
			}

			return replaceText(details, compressed), nil
		},
	})
}

func extractText(details any) (string, bool) {
	m, ok := details.(map[string]any)
	if !ok {
		s, ok := details.(string)
		return s, ok
	}

	content, ok := m["content"]
	if !ok {
		output, ok := m["output"].(string)
		return output, ok
	}

	if s, ok := content.(string); ok {
		return s, true
	}

	blocks, ok := content.([]any)
	if !ok {
		return "", false
	}

	for _, block := range blocks {
		bm, ok := block.(map[string]any)
		if !ok {
			continue
		}
		if text, ok := bm["text"].(string); ok {
			return text, true
		}
	}

	return "", false
}

func replaceText(details any, compressed string) any {
	m, ok := details.(map[string]any)
	if !ok {
		return compressed
	}

	result := make(map[string]any, len(m))
	for k, v := range m {
		result[k] = v
	}

	if content, ok := result["content"]; ok {
		if blocks, ok := content.([]any); ok && len(blocks) > 0 {
			newBlocks := make([]any, len(blocks))
			copy(newBlocks, blocks)
			if bm, ok := newBlocks[0].(map[string]any); ok {
				newBlock := make(map[string]any, len(bm))
				for k, v := range bm {
					newBlock[k] = v
				}
				newBlock["text"] = compressed
				newBlocks[0] = newBlock
			}
			result["content"] = newBlocks
			return result
		}
		if _, ok := content.(string); ok {
			result["content"] = compressed
			return result
		}
	}

	if _, ok := result["output"]; ok {
		result["output"] = compressed
		return result
	}

	return result
}
