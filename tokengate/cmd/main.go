package main

import (
	"github.com/dotcommander/piglet-extensions/tokengate"
	sdk "github.com/dotcommander/piglet/sdk"
)

func main() {
	e := sdk.New("tokengate", "0.2.0")
	tokengate.Register(e, "0.2.0")
	e.Run()
}
