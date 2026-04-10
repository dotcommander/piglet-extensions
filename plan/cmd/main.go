package main

import (
	"github.com/dotcommander/piglet-extensions/plan"
	sdk "github.com/dotcommander/piglet/sdk"
)

func main() {
	e := sdk.New("plan", plan.Version)
	plan.Register(e)
	e.Run()
}
