// Admin extension. Configuration viewer and model catalog sync.
package main

import (
	"github.com/dotcommander/piglet-extensions/admin"
	sdk "github.com/dotcommander/piglet/sdk"
)

func main() {
	e := sdk.New("admin", "0.3.0")
	admin.Register(e, "0.3.0")
	e.Run()
}
