package main

import (
	"context"
	"time"

	"github.com/dotcommander/piglet-extensions/modelsdev"
	sdk "github.com/dotcommander/piglet/sdk"
)

const refreshTimeout = 10 * time.Second

func main() {
	e := sdk.New("modelsdev", "0.1.0")

	e.OnInit(func(x *sdk.Extension) {
		if !modelsdev.CacheStale() {
			return
		}

		ctx, cancel := context.WithTimeout(context.Background(), refreshTimeout)
		defer cancel()

		updated, err := modelsdev.Refresh(ctx)
		if err != nil {
			x.Log("warn", "modelsdev: "+err.Error())
			return
		}
		if updated > 0 {
			if _, err := x.SyncModels(ctx); err != nil {
				x.Log("warn", "modelsdev sync: "+err.Error())
			}
		}
	})

	e.Run()
}
