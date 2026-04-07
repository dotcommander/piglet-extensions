package main

import (
	"github.com/dotcommander/piglet-extensions/plan"
	sdk "github.com/dotcommander/piglet/sdk"
)

func main() {
	e := sdk.New("plan", "0.2.0")
	plan.Register(e, "0.2.0")
	e.Run()
}
