package plan

import (
	_ "embed"
	"fmt"
	"strings"
)

//go:embed defaults/mode-propose.md
var modeProposeMD string

// statusMarker returns the markdown checkbox marker for a step status.
func statusMarker(status string) byte {
	switch status {
	case StatusDone:
		return 'x'
	case StatusInProgress:
		return '>'
	case StatusSkipped:
		return '-'
	case StatusFailed:
		return '!'
	}
	return ' '
}

// formatStep renders a single step as a markdown line.
func formatStep(s Step, showID bool) string {
	var b strings.Builder
	marker := statusMarker(s.Status)
	if showID {
		fmt.Fprintf(&b, "- [%c] %d. %s", marker, s.ID, s.Text)
	} else {
		fmt.Fprintf(&b, "- [%c] %s", marker, s.Text)
	}
	if s.CommitSHA != "" {
		fmt.Fprintf(&b, " (%s)", ShortSHA(s.CommitSHA))
	}
	b.WriteByte('\n')
	if s.Notes != "" {
		fmt.Fprintf(&b, "  - %s\n", s.Notes)
	}
	return b.String()
}

// FormatPrompt renders the plan for injection into the system prompt.
func FormatPrompt(p *Plan) string {
	if p == nil {
		return ""
	}

	var b strings.Builder
	fmt.Fprintf(&b, "## Active Plan: %s\n\n", p.Title)

	if p.Mode == ModePropose {
		b.WriteString(strings.TrimSpace(modeProposeMD) + "\n\n")
	}

	// Show resume point if there's an incomplete step
	if resume := p.ResumeStep(); resume != nil {
		fmt.Fprintf(&b, "▶ Resume: step %d — %s\n\n", resume.ID, resume.Text)
	} else if p.IsComplete() {
		b.WriteString("✓ Plan complete\n\n")
	}

	for _, s := range p.Steps {
		b.WriteString(formatStep(s, true))
	}

	done, total := p.Progress()
	fmt.Fprintf(&b, "\nProgress: %d/%d done", done, total)
	if p.GitEnabled {
		b.WriteString(" | checkpoints enabled")
	}
	b.WriteString("\nPlan file: plan.md (human-editable)")
	return b.String()
}
