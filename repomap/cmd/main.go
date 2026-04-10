package main

import (
	"github.com/dotcommander/piglet-extensions/repomap"
	sdk "github.com/dotcommander/piglet/sdk"
)

func main() {
	e := sdk.New("repomap", repomap.Version)
	repomap.Register(e)
	e.Run()
}
