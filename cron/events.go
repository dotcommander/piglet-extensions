package cron

import (
	"context"
	"encoding/json"
	"fmt"

	sdk "github.com/dotcommander/piglet/sdk"
)

func registerEventHandler(e *sdk.Extension) {
	e.RegisterEventHandler(sdk.EventHandlerDef{
		Name:     "cron-status",
		Priority: 100,
		Events:   []string{"EventAgentStart"},
		Handle: func(_ context.Context, _ string, _ json.RawMessage) *sdk.Action {
			summaries, err := ListTasks()
			if err != nil || len(summaries) == 0 {
				return nil
			}

			enabled, overdue := countTasks(summaries)

			status := fmt.Sprintf("%d tasks", enabled)
			if overdue > 0 {
				status = fmt.Sprintf("%d tasks, %d overdue", enabled, overdue)
			}
			return sdk.ActionSetStatus("cron", status)
		},
	})
}
