// Route extension binary. Classifies prompts and routes to relevant piglet
// extensions/tools using weighted intent + domain + trigger scoring.
package main

import (
	"github.com/dotcommander/piglet-extensions/route"
	sdk "github.com/dotcommander/piglet/sdk"
)

func main() {
	e := sdk.New("route", "0.2.0")
	route.Register(e, "0.2.0")
	e.Run()
}
