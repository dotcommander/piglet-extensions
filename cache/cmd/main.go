// Cache extension binary. Provides persistent file-backed caching for other extensions.
// Currently a no-op anchor — future versions may register /cache commands.
package main

import (
	sdk "github.com/dotcommander/piglet/sdk"
)

func main() {
	e := sdk.New("cache", "0.1.0")
	e.Run()
}
