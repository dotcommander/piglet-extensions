package main

import (
	"github.com/dotcommander/piglet-extensions/bulk"
	"github.com/dotcommander/piglet-extensions/cache"
	"github.com/dotcommander/piglet-extensions/modelsdev"
	"github.com/dotcommander/piglet-extensions/packs/internal/safety"
	"github.com/dotcommander/piglet-extensions/pipeline"
	"github.com/dotcommander/piglet-extensions/usage"
	"github.com/dotcommander/piglet-extensions/webfetch"
	"github.com/dotcommander/piglet/sdk"
)

func main() {
	e := sdk.New("pack-workflow", "0.1.0")
	safety.RegisterWithVersion(e, "pipeline", "0.2.0", pipeline.Register)
	safety.RegisterWithVersion(e, "bulk", "0.2.0", bulk.Register)
	safety.RegisterWithVersion(e, "webfetch", "0.3.0", webfetch.Register)
	safety.Register(e, "cache", cache.Register)
	safety.RegisterWithVersion(e, "usage", "0.2.0", usage.Register)
	safety.Register(e, "modelsdev", modelsdev.Register)
	e.Run()
}
