package main

import (
	"github.com/dotcommander/piglet-extensions/changelog"
	sdk "github.com/dotcommander/piglet/sdk"
)

func main() {
	e := sdk.New("changelog", changelog.Version)
	changelog.Register(e)
	e.Run()
}
