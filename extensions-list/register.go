package extlist

import (
	"context"
	"fmt"
	"strings"

	"github.com/dotcommander/piglet/sdk"
)

// Register registers the extensions-list extension's commands.
func Register(e *sdk.Extension) {
	e.RegisterCommand(sdk.CommandDef{
		Name:        "extensions",
		Description: "List loaded extensions, or install/update official extensions",
		Handler: func(ctx context.Context, args string) error {
			sub := strings.TrimSpace(args)
			if sub == "install" || sub == "update" {
				e.ShowMessage("please install piglet extensions using: cd ~/go/src/piglet-extensions && just extensions")
				return nil
			}

			infos, err := e.ExtInfos(ctx)
			if err != nil {
				e.ShowMessage("Failed to list extensions: " + err.Error())
				return nil
			}
			if len(infos) == 0 {
				e.ShowMessage("No extensions loaded.")
				return nil
			}

			var b strings.Builder
			fmt.Fprintf(&b, "Loaded extensions (%d)\n\n", len(infos))
			for _, info := range infos {
				label := info.Name
				if info.Version != "" {
					label = fmt.Sprintf("%s v%s", info.Name, info.Version)
				}
				fmt.Fprintf(&b, "  %s  [%s]\n", label, info.Kind)

				var caps []string
				caps = appendCap(caps, "tools", info.Tools)
				caps = appendCap(caps, "commands", info.Commands)
				caps = appendCap(caps, "interceptors", info.Interceptors)
				caps = appendCap(caps, "events", info.EventHandlers)
				caps = appendCap(caps, "shortcuts", info.Shortcuts)
				caps = appendCap(caps, "hooks", info.MessageHooks)
				if info.Compactor != "" {
					caps = append(caps, fmt.Sprintf("compactor: %s", info.Compactor))
				}
				for _, c := range caps {
					fmt.Fprintf(&b, "    %s\n", c)
				}
			}
			b.WriteString("\nUse /extensions install to install official extensions.")
			e.ShowMessage(b.String())
			return nil
		},
	})
}

// appendCap adds a formatted capability line if items is non-empty.
func appendCap(caps []string, label string, items []string) []string {
	if len(items) > 0 {
		return append(caps, fmt.Sprintf("%s: %s", label, strings.Join(items, ", ")))
	}
	return caps
}
