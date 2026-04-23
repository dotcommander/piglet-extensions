package selfupdate

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"

	sdk "github.com/dotcommander/piglet/sdk"
)

// Register registers /update and /upgrade commands with the extension.
func Register(e *sdk.Extension) {
	e.RegisterCommand(sdk.CommandDef{
		Name:        "update",
		Description: "Check for a newer piglet binary and upgrade if available",
		Handler: func(ctx context.Context, _ string) error {
			version := currentBinaryVersion()

			ctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
			defer cancel()

			var b strings.Builder
			upgraded, err := CheckAndUpgrade(ctx, &b, version)
			if err != nil {
				fmt.Fprintf(&b, "Upgrade failed: %v\n", err)
			}
			if upgraded {
				b.WriteString("\nBinary replaced — restart piglet to use the new version.")
			}
			e.ShowMessage(b.String())
			return nil
		},
	})

	e.RegisterCommand(sdk.CommandDef{
		Name:        "upgrade",
		Description: "Alias for /update",
		Handler: func(ctx context.Context, _ string) error {
			// Reuse the update handler by calling it directly.
			version := currentBinaryVersion()

			ctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
			defer cancel()

			var b strings.Builder
			upgraded, err := CheckAndUpgrade(ctx, &b, version)
			if err != nil {
				fmt.Fprintf(&b, "Upgrade failed: %v\n", err)
			}
			if upgraded {
				b.WriteString("\nBinary replaced — restart piglet to use the new version.")
			}
			e.ShowMessage(b.String())
			return nil
		},
	})
}

// currentBinaryVersion resolves the running piglet version by executing
// `piglet --version`. Returns "dev" if the binary is not found or errors.
func currentBinaryVersion() string {
	out, err := exec.Command("piglet", "--version").Output()
	if err != nil {
		return "dev"
	}
	// Output is "piglet v1.2.3" or "piglet dev-abc123".
	parts := strings.Fields(strings.TrimSpace(string(out)))
	if len(parts) >= 2 {
		return parts[1]
	}
	return "dev"
}
