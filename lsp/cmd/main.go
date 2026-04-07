package main

import (
	"github.com/dotcommander/piglet-extensions/lsp"
	sdk "github.com/dotcommander/piglet/sdk"
)

func main() {
	e := sdk.New("lsp", "0.2.0")
	lsp.Register(e, "0.2.0")
	e.Run()
}
