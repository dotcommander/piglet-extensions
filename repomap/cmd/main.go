package main

import (
	"github.com/dotcommander/piglet-extensions/repomap"
	sdk "github.com/dotcommander/piglet/sdk"
)

func main() {
	e := sdk.New("repomap", "0.2.0")
	repomap.Register(e, "0.2.0")
	e.Run()
}
