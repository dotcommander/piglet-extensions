package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

// ── Color support ──────────────────────────────────────────────────────

const (
	cReset  = "\033[0m"
	cBold   = "\033[1m"
	cDim    = "\033[2m"
	cRed    = "\033[31m"
	cGreen  = "\033[32m"
	cYellow = "\033[33m"
	cCyan   = "\033[36m"
)

var noColor bool

func clr(s, color string) string {
	if noColor || color == "" {
		return s
	}
	return color + s + cReset
}

func prettyJSON(s string) string {
	var v any
	if json.Unmarshal([]byte(s), &v) == nil {
		b, err := json.MarshalIndent(v, "", "  ")
		if err == nil {
			return string(b)
		}
	}
	return s
}

func logf(format string, a ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", a...)
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

// formatRegistration returns a display string for a single registration.
func formatRegistration(r registration) string {
	if r.kind == "eventHandler" {
		evtStr := strings.Join(r.events, ", ")
		if evtStr == "" {
			evtStr = "(all)"
		}
		return fmt.Sprintf("[%s] %s — events: [%s], priority: %.0f",
			clr(r.kind, cDim), clr(r.name, cCyan), evtStr, r.priority)
	}
	return fmt.Sprintf("[%s] %s — %s",
		clr(r.kind, cDim), clr(r.name, cCyan), r.description)
}
