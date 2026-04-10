// Scaffold extension. Creates a new extension skeleton in the extensions directory.
package main

import (
	"github.com/dotcommander/piglet-extensions/scaffold"
	sdk "github.com/dotcommander/piglet/sdk"
)

func main() {
	e := sdk.New("scaffold", scaffold.Version)
	scaffold.Register(e)
	e.Run()
}
