package modelsdev

import (
	"context"
	"fmt"
	"time"

	"github.com/dotcommander/piglet/sdk"
)

const refreshTimeout = 10 * time.Second

// Register registers the modelsdev extension's OnInit handler.
func Register(e *sdk.Extension) {
	e.OnInit(func(x *sdk.Extension) {
		start := time.Now()
		x.Log("debug", "[modelsdev] OnInit start")

		if !CacheStale() {
			x.Log("debug", fmt.Sprintf("[modelsdev] OnInit complete — cache fresh (%s)", time.Since(start)))
			return
		}
		// Stale-while-revalidate: models.yaml has last-known-good data.
		// Refresh in background — never block the initialize handshake.
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), refreshTimeout)
			defer cancel()

			updated, err := Refresh(ctx)
			if err != nil {
				x.Log("warn", "modelsdev: "+err.Error())
				return
			}
			if updated > 0 {
				if _, err := x.SyncModels(ctx); err != nil {
					x.Log("warn", "modelsdev sync: "+err.Error())
				}
			}
		}()

		x.Log("debug", fmt.Sprintf("[modelsdev] OnInit complete — refresh running in background (%s)", time.Since(start)))
	})
}
