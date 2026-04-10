// Background extension. Registers /bg and /bg-cancel commands.
package main

import (
	"github.com/dotcommander/piglet-extensions/background"
	sdk "github.com/dotcommander/piglet/sdk"
)

func main() {
	e := sdk.New("background", background.Version)
	background.Register(e)
	e.Run()
}
