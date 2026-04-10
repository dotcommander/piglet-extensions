package main

import (
	"github.com/dotcommander/piglet-extensions/suggest"
	sdk "github.com/dotcommander/piglet/sdk"
)

func main() {
	e := sdk.New("suggest", suggest.Version)
	suggest.Register(e)
	e.Run()
}
