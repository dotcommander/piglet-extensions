// Coordinator extension binary. Discovers capabilities, decomposes tasks, and dispatches
// parallel sub-agents. Communicates with piglet host via JSON-RPC over stdin/stdout.
package main

import (
	"github.com/dotcommander/piglet-extensions/coordinator"
	sdk "github.com/dotcommander/piglet/sdk"
)

func main() {
	e := sdk.New("coordinator", coordinator.Version)
	coordinator.Register(e)
	e.Run()
}
