package tasklist

import (
	sdk "github.com/dotcommander/piglet/sdk"
)

// requireStore dereferences the store pointer, returning an error result if nil.
func requireStore(sp **Store) (*Store, *sdk.ToolResult) {
	s := *sp
	if s == nil {
		return nil, sdk.ErrorResult("tasklist: not initialized")
	}
	return s, nil
}

// resolveID extracts "id" from args, resolves it to a task, and returns the task.
// Returns an error result if id is missing or the task is not found.
func resolveID(s *Store, args map[string]any) (*Task, *sdk.ToolResult) {
	id, _ := args["id"].(string)
	if id == "" {
		return nil, sdk.ErrorResult("id is required")
	}
	t, err := s.Resolve(id)
	if err != nil {
		return nil, sdk.ErrorResult(err.Error())
	}
	return t, nil
}

// requireStoreAndResolve combines requireStore and resolveID.
func requireStoreAndResolve(sp **Store, args map[string]any) (*Store, *Task, *sdk.ToolResult) {
	s, errRes := requireStore(sp)
	if errRes != nil {
		return nil, nil, errRes
	}
	t, errRes := resolveID(s, args)
	if errRes != nil {
		return nil, nil, errRes
	}
	return s, t, nil
}
