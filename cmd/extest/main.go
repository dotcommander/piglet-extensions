// extest exercises a piglet extension binary via JSON-RPC over stdin/stdout.
//
// Usage:
//
//	extest [flags] <binary>
//	extest -e tasklist_add -a '{"title":"Fix bug"}' ./tasklist
//	extest -c todo -a "add Fix bug" ./tasklist
//	extest -E EventAgentEnd -a '{"Messages":[...]}' ./autotitle
//	extest -i ./tasklist   # interactive mode
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
)

// ── Flags ──────────────────────────────────────────────────────────────

var (
	extBinary     string
	extCWD        string
	toolName      string
	args          string
	cmdName       string
	eventName     string
	isInteractive bool
	mode          string
	timeout       int
	mockChat      string
)

func main() {
	// Resolve color support once at startup.
	noColor = os.Getenv("NO_COLOR") != "" || os.Getenv("TERM") == "dumb"

	// Clean up child processes on Ctrl+C.
	go func() {
		ch := make(chan os.Signal, 1)
		signal.Notify(ch, os.Interrupt)
		<-ch
		fmt.Fprintf(os.Stderr, "\n→ interrupted\n")
		os.Exit(130)
	}()

	flag.StringVar(&extCWD, "cwd", ".", "Working directory sent to extension")
	flag.StringVar(&toolName, "e", "", "Execute a tool by name")
	flag.StringVar(&args, "a", "", "Arguments: JSON for tools/events, text for commands")
	flag.StringVar(&cmdName, "c", "", "Execute a slash command by name")
	flag.StringVar(&eventName, "E", "", "Dispatch an event by type (e.g. EventAgentEnd)")
	flag.BoolVar(&isInteractive, "i", false, "Interactive REPL mode")
	flag.StringVar(&mode, "m", "auto", "Output mode: auto, show, json, trace")
	flag.IntVar(&timeout, "t", 5, "Timeout in seconds for response waiting")
	flag.StringVar(&mockChat, "mock", "Test session title", "Mock response for host/chat RPC (empty to disable)")
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "extest — exercise piglet extensions via JSON-RPC\n\n")
		fmt.Fprintf(os.Stderr, "Usage: extest [flags] <extension-binary>\n\n")
		fmt.Fprintf(os.Stderr, "Flags:\n")
		flag.PrintDefaults()
		fmt.Fprintf(os.Stderr, `
Output modes:
  auto   Registrations + messages + tool results (default)
  show   User-visible text only (tool results, command output, errors)
  json   Pretty-printed JSON (full protocol trace)
  trace  Raw JSON lines (one per line, for piping)

Host RPC mocking:
  Extensions that call e.Chat() send a host/chat request to the host.
  extest automatically responds with the -mock text, making event handler
  testing work end-to-end. Pass -mock "" to disable and see raw timeouts.

Examples:
  extest ~/.config/piglet/extensions/tasklist/tasklist
  extest -m show -e tasklist_add -a '{"title":"Fix bug"}' ./tasklist
  extest -m show -c config -a "setup" ~/.config/piglet/extensions/admin/admin
  extest -E EventAgentEnd -a '{"Messages":[{"role":"user","content":"Fix login"}]}' ./autotitle
  extest -m trace -e tasklist_list -a '{}' ~/.config/piglet/extensions/tasklist/tasklist
  extest -i ~/.config/piglet/extensions/tasklist/tasklist
`)
	}
	flag.Parse()

	flagArgs := flag.Args()
	if len(flagArgs) == 0 {
		flag.Usage()
		os.Exit(1)
	}
	extBinary = flagArgs[0]

	if _, err := os.Stat(extBinary); os.IsNotExist(err) {
		fmt.Fprintf(os.Stderr, "error: binary not found: %s\n", extBinary)
		os.Exit(1)
	}

	cwd, err := filepath.Abs(extCWD)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: resolving cwd: %v\n", err)
		os.Exit(1)
	}

	switch {
	case isInteractive:
		runInteractive(cwd)
	case toolName != "":
		toolJSON := args
		if toolJSON == "" {
			toolJSON = "{}"
		}
		runTool(cwd, toolName, toolJSON)
	case cmdName != "":
		runCommand(cwd, cmdName, args)
	case eventName != "":
		eventJSON := args
		if eventJSON == "" {
			eventJSON = "{}"
		}
		runEvent(cwd, eventName, eventJSON)
	default:
		runInit(cwd)
	}
}

// ── Run modes ──────────────────────────────────────────────────────────

func runInit(cwd string) {
	msgs := []jsonRPC{
		{Version: "2.0", ID: intPtr(1), Method: "initialize", Params: map[string]any{"cwd": cwd}},
	}
	r := session(extBinary, msgs)
	display(r.lines)
	exitFromResult(r)
}

func runTool(cwd, tool, argsJSON string) {
	var params map[string]any
	if err := json.Unmarshal([]byte(argsJSON), &params); err != nil {
		fmt.Fprintf(os.Stderr, "error: invalid JSON args: %v\n", err)
		os.Exit(1)
	}

	logf("→ tool: %s  args: %s", tool, argsJSON)
	msgs := []jsonRPC{
		{Version: "2.0", ID: intPtr(1), Method: "initialize", Params: map[string]any{"cwd": cwd}},
		{Version: "2.0", ID: intPtr(2), Method: "tool/execute", Params: map[string]any{"name": tool, "args": params}},
	}
	r := session(extBinary, msgs)
	display(r.lines)
	exitFromResult(r)
}

func runCommand(cwd, name, cmdArgs string) {
	logf("→ command: /%s  args: %q", name, cmdArgs)
	msgs := []jsonRPC{
		{Version: "2.0", ID: intPtr(1), Method: "initialize", Params: map[string]any{"cwd": cwd}},
		{Version: "2.0", ID: intPtr(2), Method: "command/execute", Params: map[string]any{"name": name, "args": cmdArgs}},
	}
	r := session(extBinary, msgs)
	display(r.lines)
	exitFromResult(r)
}

func runEvent(cwd, eventType, dataJSON string) {
	var data any
	if err := json.Unmarshal([]byte(dataJSON), &data); err != nil {
		fmt.Fprintf(os.Stderr, "error: invalid JSON for event data: %v\n", err)
		os.Exit(1)
	}
	logf("→ event: %s  data: %s", eventType, dataJSON)
	msgs := []jsonRPC{
		{Version: "2.0", ID: intPtr(1), Method: "initialize", Params: map[string]any{"cwd": cwd}},
		{Version: "2.0", ID: intPtr(2), Method: "event/dispatch", Params: map[string]any{"type": eventType, "data": data}},
	}
	r := session(extBinary, msgs)
	display(r.lines)
	exitFromResult(r)
}

// exitFromResult exits with appropriate code: 2=extension error, 3=timeout.
func exitFromResult(r *sessionResult) {
	if r.timedOut {
		os.Exit(3)
	}
	if r.hasErrors() {
		os.Exit(2)
	}
}
