package memory

import "strings"

// textBlock is a text content element used in tool result wire messages.
type textBlock struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

// contextKind classifies a context fact by its key prefix.
type contextKind int

const (
	contextFile    contextKind = iota // ctx:file:* or ctx:search:*
	contextEdit                       // ctx:edit:*
	contextError                      // ctx:error:*
	contextCmd                        // ctx:cmd:*
	contextSummary                    // ctx:summary
	contextPlan                       // ctx:plan:*
	contextTool                       // ctx:tool:*
	contextOther                      // anything else
)

// classifyFact returns the contextKind for a fact key.
func classifyFact(key string) contextKind {
	switch {
	case strings.HasPrefix(key, "ctx:file:"), strings.HasPrefix(key, "ctx:search:"):
		return contextFile
	case strings.HasPrefix(key, "ctx:edit:"):
		return contextEdit
	case strings.HasPrefix(key, "ctx:error:"):
		return contextError
	case strings.HasPrefix(key, "ctx:cmd:"):
		return contextCmd
	case strings.HasPrefix(key, "ctx:summary"):
		return contextSummary
	case strings.HasPrefix(key, "ctx:plan:"):
		return contextPlan
	case strings.HasPrefix(key, "ctx:tool:"):
		return contextTool
	default:
		return contextOther
	}
}
