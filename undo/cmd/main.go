// Undo extension. Restores files to their pre-edit state from undo snapshots.
package main

import (
	"github.com/dotcommander/piglet-extensions/undo"
	sdk "github.com/dotcommander/piglet/sdk"
)

func main() {
	e := sdk.New("undo", undo.Version)
	undo.Register(e)
	e.Run()
}
