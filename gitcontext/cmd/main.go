// Git context extension. Injects uncommitted changes, recent commits, and
// small diffs into the system prompt so the model knows the repo state.
package main

import (
	"github.com/dotcommander/piglet-extensions/gitcontext"
	sdk "github.com/dotcommander/piglet/sdk"
)

func main() {
	e := sdk.New("gitcontext", gitcontext.Version)
	gitcontext.Register(e)
	e.Run()
}
