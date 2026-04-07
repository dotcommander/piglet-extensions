// Skill extension binary. On-demand methodology loading from markdown files.
// Communicates with piglet host via JSON-RPC over stdin/stdout.
package main

import (
	"github.com/dotcommander/piglet-extensions/skill"
	sdk "github.com/dotcommander/piglet/sdk"
)

func main() {
	e := sdk.New("skill", "0.2.0")
	skill.Register(e, "0.2.0")
	e.Run()
}
