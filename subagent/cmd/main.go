// Subagent extension binary. Delegates tasks to independent sub-agents.
// Communicates with piglet host via JSON-RPC over stdin/stdout.
package main

import (
	"github.com/dotcommander/piglet-extensions/subagent"
	sdk "github.com/dotcommander/piglet/sdk"
)

func main() {
	e := sdk.New("subagent", subagent.Version)
	subagent.Register(e)
	e.Run()
}
