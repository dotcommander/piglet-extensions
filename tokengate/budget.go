package tokengate

import (
	"encoding/json"
	"fmt"
	"strings"
	"sync"
)

// TurnUsageEvent matches the usage extension's event structure.
type TurnUsageEvent struct {
	Turn      int        `json:"turn"`
	Usage     TokenUsage `json:"usage"`
	Breakdown Breakdown  `json:"breakdown"`
}

type TokenUsage struct {
	Input      int `json:"input"`
	Output     int `json:"output"`
	CacheRead  int `json:"cache_read"`
	CacheWrite int `json:"cache_write"`
}

type Breakdown struct {
	SystemPrompt int         `json:"system_prompt"`
	Extensions   []ExtTokens `json:"extensions"`
	RepoMap      int         `json:"repo_map"`
	Tools        int         `json:"tools"`
	History      int         `json:"history"`
}

type ExtTokens struct {
	Name   string `json:"name"`
	Tokens int    `json:"tokens"`
}

// BudgetState tracks token usage against configured limits.
type BudgetState struct {
	mu          sync.RWMutex
	cfg         Config
	current     Breakdown
	totalInput  int
	totalOutput int
	turns       int
	warned      bool
}

func NewBudgetState(cfg Config) *BudgetState {
	return &BudgetState{cfg: cfg}
}

// Record updates budget state from a turn usage event.
// Returns true if the warning threshold was just crossed (first time only).
func (bs *BudgetState) Record(event TurnUsageEvent) bool {
	bs.mu.Lock()
	defer bs.mu.Unlock()

	bs.current = event.Breakdown
	bs.totalInput += event.Usage.Input
	bs.totalOutput += event.Usage.Output
	bs.turns++

	if !bs.warned && bs.cfg.ContextWindow > 0 && bs.cfg.WarnPercent > 0 {
		used := bs.promptTokens()
		threshold := bs.cfg.ContextWindow * bs.cfg.WarnPercent / 100
		if used >= threshold {
			bs.warned = true
			return true
		}
	}
	return false
}

// promptTokens returns current prompt fill (everything that counts toward context window).
func (bs *BudgetState) promptTokens() int {
	total := bs.current.SystemPrompt + bs.current.RepoMap + bs.current.Tools + bs.current.History
	for _, ext := range bs.current.Extensions {
		total += ext.Tokens
	}
	return total
}

// Summary returns a formatted budget summary string.
func (bs *BudgetState) Summary() string {
	bs.mu.RLock()
	defer bs.mu.RUnlock()

	used := bs.promptTokens()
	pct := 0
	if bs.cfg.ContextWindow > 0 {
		pct = used * 100 / bs.cfg.ContextWindow
	}

	var b strings.Builder
	b.WriteString("Context Budget\n")
	b.WriteString("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n\n")

	fmt.Fprintf(&b, "Context window:    %s tokens\n", fmtNum(bs.cfg.ContextWindow))
	fmt.Fprintf(&b, "Current fill:      %s tokens (%d%%)\n", fmtNum(used), pct)
	fmt.Fprintf(&b, "Remaining:         %s tokens\n\n", fmtNum(bs.cfg.ContextWindow-used))

	b.WriteString("BREAKDOWN\n")
	fmt.Fprintf(&b, "  System prompt:     %s\n", fmtNum(bs.current.SystemPrompt))
	fmt.Fprintf(&b, "  Repo map:          %s\n", fmtNum(bs.current.RepoMap))
	fmt.Fprintf(&b, "  Tool definitions:  %s\n", fmtNum(bs.current.Tools))
	fmt.Fprintf(&b, "  Conversation:      %s\n", fmtNum(bs.current.History))

	if len(bs.current.Extensions) > 0 {
		b.WriteString("\n  Extensions:\n")
		for _, ext := range bs.current.Extensions {
			fmt.Fprintf(&b, "    %-18s %s\n", ext.Name, fmtNum(ext.Tokens))
		}
	}

	b.WriteString("\nCUMULATIVE\n")
	fmt.Fprintf(&b, "  Turns:             %d\n", bs.turns)
	fmt.Fprintf(&b, "  Total input:       %s\n", fmtNum(bs.totalInput))
	fmt.Fprintf(&b, "  Total output:      %s\n", fmtNum(bs.totalOutput))

	return b.String()
}

// ParseTurnUsage decodes a TurnUsageEvent from JSON.
func ParseTurnUsage(data json.RawMessage) (TurnUsageEvent, error) {
	var event TurnUsageEvent
	err := json.Unmarshal(data, &event)
	return event, err
}

func fmtNum(n int) string {
	if n < 0 {
		return "0"
	}
	s := fmt.Sprintf("%d", n)
	if len(s) <= 3 {
		return s
	}
	// Insert commas
	result := make([]byte, 0, len(s)+len(s)/3)
	for i, c := range s {
		if i > 0 && (len(s)-i)%3 == 0 {
			result = append(result, ',')
		}
		result = append(result, byte(c))
	}
	return string(result)
}
