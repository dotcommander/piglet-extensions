package main

import (
	"github.com/dotcommander/piglet-extensions/tasklist"
	sdk "github.com/dotcommander/piglet/sdk"
)

func main() {
	e := sdk.New("tasklist", "0.2.0")
	tasklist.Register(e, "0.2.0")
	e.Run()
}
