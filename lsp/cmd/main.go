package main

import (
	"github.com/dotcommander/piglet-extensions/lsp"
	sdk "github.com/dotcommander/piglet/sdk"
)

func main() {
	e := sdk.New("lsp", lsp.Version)
	lsp.Register(e)
	e.Run()
}
