package main

import (
	"github.com/dotcommander/piglet-extensions/inbox"
	sdk "github.com/dotcommander/piglet/sdk"
)

func main() {
	e := sdk.New("inbox", inbox.Version)
	inbox.Register(e)
	e.Run()
}
