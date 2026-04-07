package main

import (
	"github.com/dotcommander/piglet-extensions/admin"
	"github.com/dotcommander/piglet-extensions/background"
	"github.com/dotcommander/piglet-extensions/export"
	extlist "github.com/dotcommander/piglet-extensions/extensions-list"
	"github.com/dotcommander/piglet-extensions/packs/internal/safety"
	"github.com/dotcommander/piglet-extensions/scaffold"
	"github.com/dotcommander/piglet-extensions/undo"
	"github.com/dotcommander/piglet/sdk"
)

func main() {
	e := sdk.New("pack-core", "0.1.0")
	safety.RegisterWithVersion(e, "admin", "0.3.0", admin.Register)
	safety.Register(e, "export", export.Register)
	safety.Register(e, "extensions-list", extlist.Register)
	safety.Register(e, "undo", undo.Register)
	safety.Register(e, "scaffold", scaffold.Register)
	safety.RegisterWithVersion(e, "background", "0.2.0", background.Register)
	e.Run()
}
