package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

// ── Interactive REPL ───────────────────────────────────────────────────

func runInteractive(cwd string) {
	// First run: just initialize to see registrations.
	msgs := []jsonRPC{
		{Version: "2.0", ID: intPtr(1), Method: "initialize", Params: map[string]any{"cwd": cwd}},
	}
	r := session(extBinary, msgs)

	regs := extractRegistrations(r.lines)
	extName, extVer := extractInitInfo(r.lines)
	fmt.Printf("\n  %s v%s\n", clr(extName, cBold), extVer)
	printRegistrationSummary(regs)
	fmt.Println()

	reader := bufio.NewReader(os.Stdin)
	nextID := 2

	for {
		fmt.Print(clr("? ", cBold))
		line, err := reader.ReadString('\n')
		if err != nil {
			break
		}
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if line == "quit" || line == "exit" {
			break
		}
		if line == "help" {
			printInteractiveHelp()
			continue
		}
		if line == "registrations" || line == "reg" {
			printRegistrationSummary(regs)
			continue
		}
		if line == "tools" {
			printToolsList(regs)
			continue
		}
		if line == "events" {
			printEventsList(regs)
			continue
		}
		if line == "commands" || line == "cmds" {
			printCommandsList(regs)
			continue
		}

		var rpcMsgs []jsonRPC
		if strings.HasPrefix(line, "tool ") {
			name, toolArgs := parseToolArgs(line[5:])
			fmt.Fprintf(os.Stderr, "→ tool: %s  args: %s\n", name, toolArgs)
			rpcMsgs = []jsonRPC{
				{Version: "2.0", ID: intPtr(nextID), Method: "tool/execute",
					Params: map[string]any{"name": name, "args": toolArgs}},
			}
		} else if strings.HasPrefix(line, "cmd ") {
			name, cmdArgs := parseCmdArgs(line[4:])
			fmt.Fprintf(os.Stderr, "→ command: /%s  args: %q\n", name, cmdArgs)
			rpcMsgs = []jsonRPC{
				{Version: "2.0", ID: intPtr(nextID), Method: "command/execute",
					Params: map[string]any{"name": name, "args": cmdArgs}},
			}
		} else if strings.HasPrefix(line, "event ") {
			evtType, evtData := parseCmdArgs(line[6:])
			var data any
			if err := json.Unmarshal([]byte(evtData), &data); err != nil {
				fmt.Printf("Invalid JSON: %v\n", err)
				continue
			}
			fmt.Fprintf(os.Stderr, "→ event: %s  data: %s\n", evtType, evtData)
			rpcMsgs = []jsonRPC{
				{Version: "2.0", ID: intPtr(nextID), Method: "event/dispatch",
					Params: map[string]any{"type": evtType, "data": data}},
			}
		} else {
			fmt.Println("Usage: tool <name> <json> | cmd <name> [args] | event <type> <json> | tools | commands | events | help | quit")
			continue
		}
		nextID++

		r := session(extBinary, rpcMsgs)
		display(r.lines)
	}
}

func printInteractiveHelp() {
	fmt.Println("  Commands:")
	fmt.Println("    tool <name> <json>  Execute a tool")
	fmt.Println("    cmd <name> [args]   Execute a command")
	fmt.Println("    event <type> <json> Dispatch an event")
	fmt.Println("    tools               List available tools")
	fmt.Println("    commands            List available slash commands")
	fmt.Println("    events              List event handlers")
	fmt.Println("    registrations       Show all registered capabilities")
	fmt.Println("    help                Show this help")
	fmt.Println("    quit                Exit")
	fmt.Println()
	fmt.Println("  Tip: host/chat requests are automatically mocked with -mock text")
}

// ── Argument parsing ───────────────────────────────────────────────────

func parseToolArgs(s string) (string, map[string]any) {
	s = strings.TrimSpace(s)
	name, rest, _ := strings.Cut(s, " ")
	name = strings.TrimSpace(name)
	var toolArgs map[string]any
	if rest != "" {
		json.Unmarshal([]byte(rest), &toolArgs)
	}
	if toolArgs == nil {
		toolArgs = map[string]any{}
	}
	return name, toolArgs
}

func parseCmdArgs(s string) (string, string) {
	s = strings.TrimSpace(s)
	name, cmdArgs, _ := strings.Cut(s, " ")
	return strings.TrimSpace(name), strings.TrimSpace(cmdArgs)
}
