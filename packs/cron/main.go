package main

import (
	"github.com/dotcommander/piglet-extensions/cron"
	"github.com/dotcommander/piglet-extensions/packs/internal/safety"
	sdk "github.com/dotcommander/piglet/sdk"
)

func main() {
	e := sdk.New("pack-cron", "0.1.0")
	safety.RegisterWithVersion(e, "cron", "0.2.0", cron.Register)
	e.Run()
}
