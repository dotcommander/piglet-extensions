// Memory extension binary. Persistent per-project key-value memory.
// Communicates with piglet host via JSON-RPC over stdin/stdout.
package main

import (
	"github.com/dotcommander/piglet-extensions/memory"
	sdk "github.com/dotcommander/piglet/sdk"
)

func main() {
	e := sdk.New("memory", "0.2.0")
	memory.Register(e, "0.2.0")
	e.Run()
}
