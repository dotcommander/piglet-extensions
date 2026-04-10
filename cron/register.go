package cron

import sdk "github.com/dotcommander/piglet/sdk"

// Version is the cron extension version.
const Version = "0.2.0"

// Register registers the cron extension's commands, tools, and event handlers.
func Register(e *sdk.Extension) {
	registerCommands(e)
	registerTools(e)
	registerEventHandler(e)
}
