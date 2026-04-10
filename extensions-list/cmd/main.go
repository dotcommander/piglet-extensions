// Extensions-list extension. Registers /extensions command.
package main

import (
	extlist "github.com/dotcommander/piglet-extensions/extensions-list"
	sdk "github.com/dotcommander/piglet/sdk"
)

func main() {
	e := sdk.New("extensions-list", extlist.Version)
	extlist.Register(e)
	e.Run()
}
