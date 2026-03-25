package plan

import (
	"fmt"
	"strings"
)

func FormatPrompt(p *Plan) string {
	if p == nil {
		return ""
	}

	var b strings.Builder
	fmt.Fprintf(&b, "## Active Plan: %s\n\n", p.Title)

	if p.Mode == ModePropose {
		b.WriteString("**MODE: PROPOSE** — Describe changes as plan steps. Mutating tools (write, edit, bash) are blocked.\n\n")
	}

	// Show resume point if there's an incomplete step
	if resume := p.ResumeStep(); resume != nil {
		fmt.Fprintf(&b, "▶ Resume: step %d — %s\n\n", resume.ID, resume.Text)
	} else if p.IsComplete() {
		b.WriteString("✓ Plan complete\n\n")
	}

	for _, s := range p.Steps {
		switch s.Status {
		case StatusDone:
			fmt.Fprintf(&b, "- [x] %d. %s", s.ID, s.Text)
			if s.CommitSHA != "" {
				fmt.Fprintf(&b, " (%s)", ShortSHA(s.CommitSHA))
			}
			b.WriteByte('\n')
		case StatusInProgress:
			fmt.Fprintf(&b, "- [ ] **%d. %s** ← in progress\n", s.ID, s.Text)
		case StatusSkipped:
			fmt.Fprintf(&b, "- [-] %d. %s\n", s.ID, s.Text)
		case StatusFailed:
			fmt.Fprintf(&b, "- [!] %d. %s", s.ID, s.Text)
			if s.CommitSHA != "" {
				fmt.Fprintf(&b, " (%s)", ShortSHA(s.CommitSHA))
			}
			b.WriteByte('\n')
		default:
			fmt.Fprintf(&b, "- [ ] %d. %s\n", s.ID, s.Text)
		}
		if s.Notes != "" {
			fmt.Fprintf(&b, "  - %s\n", s.Notes)
		}
	}

	done, total := p.Progress()
	fmt.Fprintf(&b, "\nProgress: %d/%d done", done, total)
	if p.GitEnabled {
		b.WriteString(" | checkpoints enabled")
	}
	return b.String()
}
