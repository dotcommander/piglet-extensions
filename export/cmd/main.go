// Export extension. Exports the current conversation to a markdown file.
package main

import (
	"github.com/dotcommander/piglet-extensions/export"
	sdk "github.com/dotcommander/piglet/sdk"
)

func main() {
	e := sdk.New("export", export.Version)
	export.Register(e)
	e.Run()
}
