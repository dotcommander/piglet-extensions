// Skill extension binary. On-demand methodology loading from markdown files.
// Communicates with piglet host via JSON-RPC over stdin/stdout.
package main

import (
	"github.com/dotcommander/piglet-extensions/skill"
	sdk "github.com/dotcommander/piglet/sdk"
)

func main() {
	e := sdk.New("skill", skill.Version)
	skill.Register(e)
	e.Run()
}
