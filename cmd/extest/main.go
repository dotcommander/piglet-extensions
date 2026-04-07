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
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"time"
)

// ── Color support ──────────────────────────────────────────────────────

var noColor bool

func init() {
	noColor = os.Getenv("NO_COLOR") != "" || os.Getenv("TERM") == "dumb"
}

const (
	cReset  = "\033[0m"
	cBold   = "\033[1m"
	cDim    = "\033[2m"
	cRed    = "\033[31m"
	cGreen  = "\033[32m"
	cYellow = "\033[33m"
	cCyan   = "\033[36m"
)

func clr(s, color string) string {
	if noColor || color == "" {
		return s
	}
	return color + s + cReset
}

func init() {
	// Clean up child processes on Ctrl+C.
	go func() {
		ch := make(chan os.Signal, 1)
		signal.Notify(ch, os.Interrupt)
		<-ch
		fmt.Fprintf(os.Stderr, "\n→ interrupted\n")
		os.Exit(130)
	}()
}

// ── RPC types ──────────────────────────────────────────────────────────

type jsonRPC struct {
	Version string `json:"jsonrpc"`
	ID      *int   `json:"id,omitempty"`
	Method  string `json:"method,omitempty"`
	Params  any    `json:"params,omitempty"`
}

func intPtr(i int) *int { return &i }

// registration tracks a single register/* notification.
type registration struct {
	kind        string // "tool", "command", "eventHandler", etc.
	name        string
	description string
	events      []string // event handler only
	priority    float64  // event handler only
}

// rpcLine classifies a single JSON-RPC line from the extension.
type rpcLine struct {
	raw       string
	parsed    map[string]any
	isReg     bool // register/* notification
	isShow    bool // showMessage notification
	isInit    bool // initialize response (id=1 with result)
	isError   bool
	isHostRPC bool // extension→host request (host/* with id)
	isAction  bool // response containing action field in result
}

func parseLine(raw string) *rpcLine {
	l := &rpcLine{raw: raw}
	if json.Unmarshal([]byte(raw), &l.parsed) != nil {
		return nil
	}
	if method, ok := l.parsed["method"].(string); ok {
		l.isReg = strings.HasPrefix(method, "register/")
		l.isShow = method == "showMessage"
		l.isHostRPC = strings.HasPrefix(method, "host/") && l.parsed["id"] != nil
	}
	if id, ok := l.parsed["id"].(float64); ok && id == 1 {
		if _, has := l.parsed["result"]; has {
			l.isInit = true
		}
	}
	if _, ok := l.parsed["error"]; ok {
		l.isError = true
	}
	if result, ok := l.parsed["result"].(map[string]any); ok {
		if _, hasAction := result["action"]; hasAction {
			l.isAction = true
		}
	}
	return l
}

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

	cwd, _ := filepathAbs(extCWD)

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
			printInteractiveHelp(regs)
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

func printInteractiveHelp(regs []registration) {
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

func printToolsList(regs []registration) {
	var tools []registration
	for _, r := range regs {
		if r.kind == "tool" {
			tools = append(tools, r)
		}
	}
	if len(tools) == 0 {
		fmt.Println("  No tools registered")
		return
	}
	for _, t := range tools {
		fmt.Printf("  %s — %s\n", clr(t.name, cCyan), t.description)
	}
}

func printEventsList(regs []registration) {
	var handlers []registration
	for _, r := range regs {
		if r.kind == "eventHandler" {
			handlers = append(handlers, r)
		}
	}
	if len(handlers) == 0 {
		fmt.Println("  No event handlers registered")
		return
	}
	for _, h := range handlers {
		evtStr := strings.Join(h.events, ", ")
		if evtStr == "" {
			evtStr = "(all)"
		}
		fmt.Printf("  %s — events: [%s], priority: %.0f\n", clr(h.name, cCyan), evtStr, h.priority)
	}
}

func printCommandsList(regs []registration) {
	var cmds []registration
	for _, r := range regs {
		if r.kind == "command" {
			cmds = append(cmds, r)
		}
	}
	if len(cmds) == 0 {
		fmt.Println("  No commands registered")
		return
	}
	for _, c := range cmds {
		fmt.Printf("  /%s — %s\n", clr(c.name, cCyan), c.description)
	}
}

// ── Host RPC handling ──────────────────────────────────────────────────

// respondHostRPC writes a mock response for extension→host requests.
func respondHostRPC(w *os.File, l *rpcLine) {
	method, _ := l.parsed["method"].(string)
	id := l.parsed["id"]

	var response any

	switch method {
	case "host/chat":
		if mockChat == "" {
			logf("← host/chat received (mocking disabled, extension will timeout)")
			return
		}
		response = map[string]any{
			"jsonrpc": "2.0",
			"id":      id,
			"result":  map[string]any{"text": mockChat},
		}
		params, _ := l.parsed["params"].(map[string]any)
		model, _ := params["model"].(string)
		maxTokens, _ := params["maxTokens"].(float64)
		logf("← mock host/chat (model: %s, maxTokens: %.0f) → %q", model, maxTokens, mockChat)
	case "host/runBackground":
		response = map[string]any{
			"jsonrpc": "2.0",
			"id":      id,
			"result":  map[string]any{},
		}
		params, _ := l.parsed["params"].(map[string]any)
		prompt, _ := params["prompt"].(string)
		logf("← mock host/runBackground (prompt: %q)", truncate(prompt, 50))
	case "host/isBackgroundRunning":
		response = map[string]any{
			"jsonrpc": "2.0",
			"id":      id,
			"result":  map[string]any{"running": false},
		}
		logf("← mock host/isBackgroundRunning → false")
	case "host/cancelBackground":
		response = map[string]any{
			"jsonrpc": "2.0",
			"id":      id,
			"result":  map[string]any{},
		}
		logf("← mock host/cancelBackground")
	default:
		response = map[string]any{
			"jsonrpc": "2.0",
			"id":      id,
			"error":   map[string]any{"code": float64(-32601), "message": "extest: " + method + " not available"},
		}
		logf("← error for %s (not available in extest)", method)
	}

	b, _ := json.Marshal(response)
	w.Write(b)
	w.Write([]byte("\n"))
}

// ── Registration extraction ────────────────────────────────────────────

func extractRegistrations(lines []*rpcLine) []registration {
	var regs []registration
	for _, l := range lines {
		if !l.isReg {
			continue
		}
		method, _ := l.parsed["method"].(string)
		params, _ := l.parsed["params"].(map[string]any)
		name, _ := params["name"].(string)
		desc, _ := params["description"].(string)
		reg := registration{
			kind:        strings.TrimPrefix(method, "register/"),
			name:        name,
			description: desc,
		}
		if evts, ok := params["events"].([]any); ok {
			for _, e := range evts {
				if s, ok := e.(string); ok {
					reg.events = append(reg.events, s)
				}
			}
		}
		if p, ok := params["priority"].(float64); ok {
			reg.priority = p
		}
		regs = append(regs, reg)
	}
	return regs
}

func extractInitInfo(lines []*rpcLine) (string, string) {
	for _, l := range lines {
		if !l.isInit {
			continue
		}
		result, _ := l.parsed["result"].(map[string]any)
		name, _ := result["name"].(string)
		ver, _ := result["version"].(string)
		return name, ver
	}
	return "?", "?"
}

func printRegistrationSummary(regs []registration) {
	var tools, cmds, others []registration
	for _, r := range regs {
		switch r.kind {
		case "tool":
			tools = append(tools, r)
		case "command":
			cmds = append(cmds, r)
		default:
			others = append(others, r)
		}
	}
	if len(tools) > 0 {
		fmt.Println("  Tools:")
		for _, t := range tools {
			fmt.Printf("    %s — %s\n", clr(t.name, cGreen), t.description)
		}
	}
	if len(cmds) > 0 {
		fmt.Println("  Commands:")
		for _, c := range cmds {
			fmt.Printf("    /%s — %s\n", clr(c.name, cGreen), c.description)
		}
	}
	if len(others) > 0 {
		fmt.Println("  Event handlers:")
		for _, o := range others {
			if o.kind == "eventHandler" {
				evtStr := strings.Join(o.events, ", ")
				if evtStr == "" {
					evtStr = "(all)"
				}
				fmt.Printf("    %s — events: [%s], priority: %.0f\n", o.name, evtStr, o.priority)
			} else {
				fmt.Printf("    %s: %s — %s\n", o.kind, o.name, o.description)
			}
		}
	}
}

// ── Output formatting ──────────────────────────────────────────────────

func display(lines []*rpcLine) {
	switch mode {
	case "trace":
		for _, l := range lines {
			fmt.Println(l.raw)
		}
	case "json":
		for _, l := range lines {
			fmt.Println(formatLine(l))
		}
	case "show":
		for _, l := range lines {
			if l.isReg || l.isInit {
				continue
			}
			if s := formatLine(l); s != "" {
				fmt.Println(s)
			}
		}
	default: // auto
		extName, extVer := extractInitInfo(lines)
		regs := extractRegistrations(lines)

		var toolCount, cmdCount, otherCount int
		for _, r := range regs {
			switch r.kind {
			case "tool":
				toolCount++
			case "command":
				cmdCount++
			default:
				otherCount++
			}
		}
		parts := []string{}
		if toolCount > 0 {
			parts = append(parts, fmt.Sprintf("%d tools", toolCount))
		}
		if cmdCount > 0 {
			parts = append(parts, fmt.Sprintf("%d commands", cmdCount))
		}
		if otherCount > 0 {
			parts = append(parts, fmt.Sprintf("%d handlers", otherCount))
		}
		summary := strings.Join(parts, ", ")
		fmt.Fprintf(os.Stderr, "→ %s v%s — %s registered\n", extName, extVer, summary)

		// Print registrations
		for _, r := range regs {
			if r.kind == "eventHandler" {
				evtStr := strings.Join(r.events, ", ")
				if evtStr == "" {
					evtStr = "(all)"
				}
				fmt.Printf("[%s] %s — events: [%s], priority: %.0f\n",
					clr(r.kind, cDim), clr(r.name, cCyan), evtStr, r.priority)
			} else {
				fmt.Printf("[%s] %s — %s\n",
					clr(r.kind, cDim), clr(r.name, cCyan), r.description)
			}
		}

		// Print non-registration output
		for _, l := range lines {
			if l.isReg || l.isInit {
				continue
			}
			if s := formatLine(l); s != "" {
				fmt.Println(s)
			}
		}
	}
}

func formatLine(l *rpcLine) string {
	switch {
	case l.isHostRPC:
		method, _ := l.parsed["method"].(string)
		params, _ := l.parsed["params"].(map[string]any)
		detail := ""
		switch method {
		case "host/chat":
			if model, ok := params["model"].(string); ok {
				detail = fmt.Sprintf(" (model: %s)", model)
			}
		case "host/agent.run":
			detail = " (agent loop)"
		}
		label := "mocked"
		if mockChat == "" {
			label = "no mock — will timeout"
		}
		return clr(fmt.Sprintf("→ host request: %s%s [%s]", method, detail, label), cYellow)

	case l.isAction:
		result, _ := l.parsed["result"].(map[string]any)
		action, _ := result["action"].(map[string]any)
		if action == nil {
			return clr("← action: (null — no action returned)", cDim)
		}
		actionType, _ := action["type"].(string)
		payload := action["payload"]
		switch p := payload.(type) {
		case string:
			return clr(fmt.Sprintf("← action: %s(%q)", actionType, p), cCyan)
		case nil:
			return clr(fmt.Sprintf("← action: %s", actionType), cCyan)
		default:
			b, _ := json.Marshal(p)
			return clr(fmt.Sprintf("← action: %s(%s)", actionType, string(b)), cCyan)
		}

	case l.isShow:
		return extractShowText(l)

	case l.isError:
		return clr("ERROR: "+prettyJSON(l.raw), cRed)

	default:
		// Tool/command result — extract content text if present.
		if result, ok := l.parsed["result"].(map[string]any); ok {
			if content, ok := result["content"].([]any); ok && len(content) > 0 {
				var texts []string
				for _, c := range content {
					if block, ok := c.(map[string]any); ok {
						if text, ok := block["Text"].(string); ok && text != "" {
							texts = append(texts, text)
						}
					}
				}
				if len(texts) > 0 {
					return strings.Join(texts, "\n")
				}
			}
			// Check for isError flag
			if isErr, _ := result["IsError"].(bool); isErr {
				if content, ok := result["content"].([]any); ok && len(content) > 0 {
					if block, ok := content[0].(map[string]any); ok {
						if text, ok := block["Text"].(string); ok {
							return clr("ERROR: "+text, cRed)
						}
					}
				}
			}
			// Empty result (no content, no error) — skip.
			return ""
		}
		return prettyJSON(l.raw)
	}
}

func extractShowText(l *rpcLine) string {
	if params, ok := l.parsed["params"].(map[string]any); ok {
		if text, ok := params["text"].(string); ok {
			return text
		}
	}
	return l.raw
}

func prettyJSON(s string) string {
	var v any
	if json.Unmarshal([]byte(s), &v) == nil {
		b, err := json.MarshalIndent(v, "", "  ")
		if err == nil {
			return string(b)
		}
	}
	return s
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

// ── Session management ─────────────────────────────────────────────────

// sessionResult holds the output from running an extension session.
type sessionResult struct {
	lines    []*rpcLine
	timedOut bool
}

// hasErrors returns true if any response lines indicate an error.
func (r *sessionResult) hasErrors() bool {
	for _, l := range r.lines {
		if l.isError {
			return true
		}
	}
	return false
}

// session runs an extension binary, sends messages, and collects output.
//
// It spawns the binary, streams stdout through a channel, writes all messages,
// waits for responses with matching IDs, then closes stdin to let the process
// exit cleanly. Host RPC requests from the extension are automatically mocked.
func session(binary string, msgs []jsonRPC) *sessionResult {
	stdinR, stdinW, _ := os.Pipe()
	stdoutR, stdoutW, _ := os.Pipe()

	cmd := exec.Command(binary)
	cmd.Stdin = stdinR
	cmd.Stdout = stdoutW
	cmd.Stderr = os.Stderr

	if err := cmd.Start(); err != nil {
		fmt.Fprintf(os.Stderr, "error: failed to start %s: %v\n", binary, err)
		os.Exit(1)
	}
	stdinR.Close()
	stdoutW.Close()

	// Stream stdout lines as they arrive.
	lineCh := make(chan string, 64)
	go func() {
		defer close(lineCh)
		scanner := bufio.NewScanner(stdoutR)
		scanner.Buffer(make([]byte, 0, 256*1024), 4*1024*1024)
		for scanner.Scan() {
			if line := strings.TrimSpace(scanner.Text()); line != "" {
				lineCh <- line
			}
		}
	}()

	// Track which message IDs we expect responses for.
	want := map[float64]bool{}
	for _, m := range msgs {
		if m.ID != nil {
			want[float64(*m.ID)] = true
		}
	}

	start := time.Now()
	logf("→ started %s", binary)

	// Pre-declare result so we can collect during init wait.
	result := &sessionResult{}
	deadline := time.After(time.Duration(timeout) * time.Second)
	collectLine := func(l *rpcLine) {
		result.lines = append(result.lines, l)
		if l.isHostRPC {
			respondHostRPC(stdinW, l)
		}
		// Only count responses (no method) against want map.
		if id, ok := l.parsed["id"].(float64); ok {
			if _, hasMethod := l.parsed["method"]; !hasMethod {
				delete(want, id)
			}
		}
	}

	// Phase 1: Send initialize and wait for its response.
	if len(msgs) > 0 && msgs[0].Method == "initialize" {
		line, _ := json.Marshal(msgs[0])
		stdinW.Write(line)
		stdinW.Write([]byte("\n"))
		logf("â sent %s", msgs[0].Method)

		logf("â waiting for init response...")
		initDeadline := time.After(time.Duration(timeout) * time.Second)
		for {
			select {
			case raw, ok := <-lineCh:
				if !ok {
					logf("â stdout closed during init")
					result.timedOut = true
					goto finish
				}
				l := parseLine(raw)
				if l == nil {
					continue
				}
				collectLine(l)
				if l.isInit {
					logf("â init received (%dms)", time.Since(start).Milliseconds())
					goto initDone
				}
			case <-initDeadline:
				logf("â init timeout (%ds)", timeout)
				result.timedOut = true
				goto finish
			}
		}
	}

initDone:
	// Phase 2: Send remaining messages (tool/execute, command/execute, etc.)
	for _, m := range msgs[1:] {
		line, _ := json.Marshal(m)
		stdinW.Write(line)
		stdinW.Write([]byte("\n"))
		logf("â sent %s", m.Method)
	}

	// Remove init ID from want since we already collected it.
	delete(want, 1)

	for len(want) > 0 {
		select {
		case raw, ok := <-lineCh:
			if !ok {
				logf("→ stdout closed before all responses received")
				goto finish
			}
			l := parseLine(raw)
			if l == nil {
				continue
			}
			collectLine(l)
		case <-deadline:
			logf("→ timeout (%ds)", timeout)
			result.timedOut = true
			goto finish
		}
	}

	logf("→ responses received (%dms)", time.Since(start).Milliseconds())

finish:
	// Close stdin so process exits cleanly.
	stdinW.Close()

	// Brief drain for late-arriving notifications.
	drainDeadline := time.After(300 * time.Millisecond)
	for {
		select {
		case raw, ok := <-lineCh:
			if !ok {
				goto waitExit
			}
			if l := parseLine(raw); l != nil {
				result.lines = append(result.lines, l)
			}
		case <-drainDeadline:
			goto waitExit
		}
	}

waitExit:
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	select {
	case <-done:
		logf("→ done (%dms, %d lines)", time.Since(start).Milliseconds(), len(result.lines))
	case <-time.After(3 * time.Second):
		cmd.Process.Kill()
		logf("→ killed (hung after 3s)")
	}

	return result
}

// ── Helpers ────────────────────────────────────────────────────────────

func filepathAbs(p string) (string, error) {
	if strings.HasPrefix(p, "/") {
		return p, nil
	}
	wd, err := os.Getwd()
	if err != nil {
		return p, err
	}
	return wd + "/" + p, nil
}

func logf(format string, a ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", a...)
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
