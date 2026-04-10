package sift

import (
	"fmt"
	"slices"
	"strings"
)

func CompressStructured(toolName, text string, cfg Config, cwd string) (string, bool) {
	if len(cfg.Structured) == 0 {
		return "", false
	}

	rule := matchRule(toolName, text, cfg.Structured)
	if rule == nil {
		return "", false
	}

	rows := parseLinterOutput(text, rule.Detect, cwd)
	if len(rows) == 0 {
		return "", false
	}

	if rule.SortBy != "" {
		sortRows(rows, rule.Columns, rule.SortBy)
	}

	if rule.MaxRows > 0 && len(rows) > rule.MaxRows {
		rows = rows[:rule.MaxRows]
	}

	table := buildTable(rule.Columns, rows)
	header := fmt.Sprintf("[SIFT: structured — %d findings extracted from %d bytes]\n", len(rows), len(text))
	return header + table, true
}

func matchRule(toolName, text string, rules []StructuredRule) *StructuredRule {
	for i := range rules {
		r := &rules[i]
		if r.Tool == toolName && strings.Contains(text, r.Detect) {
			return r
		}
	}
	return nil
}

func sortRows(rows []row, columns []string, sortBy string) {
	if slices.Index(columns, sortBy) < 0 {
		return
	}
	slices.SortStableFunc(rows, func(a, b row) int {
		return severityRank(a[sortBy]) - severityRank(b[sortBy])
	})
}

func severityRank(s string) int {
	lower := strings.ToLower(s)
	switch {
	case strings.HasPrefix(lower, "error"):
		return 0
	case strings.HasPrefix(lower, "warn"):
		return 1
	case strings.HasPrefix(lower, "info"):
		return 2
	default:
		return 3
	}
}

func buildTable(columns []string, rows []row) string {
	var b strings.Builder

	b.WriteByte('|')
	for _, col := range columns {
		b.WriteByte(' ')
		b.WriteString(col)
		b.WriteByte(' ')
		b.WriteByte('|')
	}
	b.WriteByte('\n')

	b.WriteByte('|')
	for range columns {
		b.WriteString("------|")
	}
	b.WriteByte('\n')

	for _, r := range rows {
		b.WriteByte('|')
		for _, col := range columns {
			b.WriteByte(' ')
			b.WriteString(r[col])
			b.WriteByte(' ')
			b.WriteByte('|')
		}
		b.WriteByte('\n')
	}

	return b.String()
}
