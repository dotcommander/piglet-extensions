// Autotitle extension binary. Generates session titles from first exchange.
// Communicates with piglet host via JSON-RPC over stdin/stdout.
package main

import (
	"github.com/dotcommander/piglet-extensions/autotitle"
	sdk "github.com/dotcommander/piglet/sdk"
)

func main() {
	e := sdk.New("autotitle", "0.2.0")
	autotitle.Register(e, "0.2.0")
	e.Run()
}
