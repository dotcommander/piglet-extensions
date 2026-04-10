package main

import (
	"github.com/dotcommander/piglet-extensions/tasklist"
	sdk "github.com/dotcommander/piglet/sdk"
)

func main() {
	e := sdk.New("tasklist", tasklist.Version)
	tasklist.Register(e)
	e.Run()
}
