// Behavior extension. Loads behavioral guidelines from
// ~/.config/piglet/extensions/behavior/behavior.md and injects them
// as the earliest system prompt section.
package main

import (
	"github.com/dotcommander/piglet-extensions/behavior"
	sdk "github.com/dotcommander/piglet/sdk"
)

func main() {
	e := sdk.New("behavior", behavior.Version)
	behavior.Register(e)
	e.Run()
}
