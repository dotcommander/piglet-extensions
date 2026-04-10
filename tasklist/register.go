package tasklist

import (
	sdk "github.com/dotcommander/piglet/sdk"
)

const Version = "0.2.0"

// Register sets up all tasklist capabilities on the extension.
func Register(e *sdk.Extension) {
	// Use a shared pointer so closures created before OnInit see the store after init.
	store := new(*Store)

	e.OnInit(func(x *sdk.Extension) {
		var err error
		*store, err = NewStore(x.CWD())
		if err != nil {
			x.ShowMessage("tasklist: " + err.Error())
			return
		}

		// Prompt section with current active tasks.
		x.RegisterPromptSection(sdk.PromptSectionDef{
			Title:   "Task List",
			Content: buildPrompt(*store),
			Order:   55,
		})
	})

	// Tools — each dereferences store at execution time, not registration time.
	e.RegisterTool(toolAdd(store))
	e.RegisterTool(toolList(store))
	e.RegisterTool(toolGet(store))
	e.RegisterTool(toolUpdate(store))
	e.RegisterTool(toolDone(store))
	e.RegisterTool(toolUndone(store))
	e.RegisterTool(toolDelete(store))
	e.RegisterTool(toolMove(store))
	e.RegisterTool(toolPlan(store))
	e.RegisterTool(toolLink(store))
	e.RegisterTool(toolSearch(store))
	e.RegisterTool(toolStatus(store))

	// Command.
	e.RegisterCommand(sdk.CommandDef{
		Name:        "todo",
		Description: "Manage tasks: /todo [add|done|delete|backlog|plan|list] [args]",
		Handler:     todoCommand(store, e),
	})
}
