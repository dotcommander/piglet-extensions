package main

import (
	"github.com/dotcommander/piglet-extensions/behavior"
	"github.com/dotcommander/piglet-extensions/distill"
	"github.com/dotcommander/piglet-extensions/gitcontext"
	"github.com/dotcommander/piglet-extensions/inbox"
	"github.com/dotcommander/piglet-extensions/memory"
	"github.com/dotcommander/piglet-extensions/packs/internal/safety"
	"github.com/dotcommander/piglet-extensions/prompts"
	"github.com/dotcommander/piglet-extensions/recall"
	"github.com/dotcommander/piglet-extensions/route"
	sessiontools "github.com/dotcommander/piglet-extensions/session-tools"
	"github.com/dotcommander/piglet-extensions/skill"
	"github.com/dotcommander/piglet/sdk"
)

func main() {
	e := sdk.New("pack-context", "0.1.0")
	safety.RegisterWithVersion(e, "memory", "0.2.0", memory.Register)
	safety.RegisterWithVersion(e, "skill", "0.2.0", skill.Register)
	safety.Register(e, "gitcontext", gitcontext.Register)
	safety.RegisterWithVersion(e, "behavior", "0.2.0", behavior.Register)
	safety.Register(e, "prompts", prompts.Register)
	safety.RegisterWithVersion(e, "session-tools", "0.3.0", sessiontools.Register)
	safety.Register(e, "inbox", inbox.Register)
	safety.Register(e, "distill", distill.Register)
	safety.Register(e, "recall", recall.Register)
	safety.RegisterWithVersion(e, "route", "0.2.0", route.Register)
	e.Run()
}
