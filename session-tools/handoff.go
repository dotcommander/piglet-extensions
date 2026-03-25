package sessiontools

import (
	"context"
	"fmt"
	"strings"

	sdk "github.com/dotcommander/piglet/sdk"
)

// Handoff builds a structured summary from memory facts and forks the session.
// The summary is injected as a user message so the new session has full context.
func Handoff(ctx context.Context, ext *sdk.Extension, cwd, focus string) error {
	summary, err := BuildSummary(cwd)
	if err != nil {
		return fmt.Errorf("build handoff summary: %w", err)
	}

	var msg strings.Builder
	msg.WriteString(summary)

	if focus != "" {
		msg.WriteString("\n\n## Requested Focus\n\n")
		msg.WriteString(focus)
	}

	msg.WriteString("\n\nThis is a handoff from a previous session. Review the summary above and continue the work.")

	// Inject the summary so it appears as a user message in the new session
	ext.SendMessage(msg.String())

	parentID, count, err := ext.ForkSession(ctx)
	if err != nil {
		return fmt.Errorf("fork session: %w", err)
	}

	ext.ShowMessage(fmt.Sprintf("Handoff complete. Forked from %s (%d messages). Summary injected into new session.", parentID, count))
	return nil
}
