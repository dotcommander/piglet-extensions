package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

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

		// Print registrations using shared formatter
		for _, r := range regs {
			fmt.Println(formatRegistration(r))
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
		return formatResultContent(l)
	}
}

// formatResultContent extracts text from tool/command result responses.
func formatResultContent(l *rpcLine) string {
	result, ok := l.parsed["result"].(map[string]any)
	if !ok {
		return prettyJSON(l.raw)
	}
	isErr, _ := result["IsError"].(bool)
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
			joined := strings.Join(texts, "\n")
			if isErr {
				return clr("ERROR: "+joined, cRed)
			}
			return joined
		}
	}
	return ""
}

func extractShowText(l *rpcLine) string {
	if params, ok := l.parsed["params"].(map[string]any); ok {
		if text, ok := params["text"].(string); ok {
			return text
		}
	}
	return l.raw
}

// ── Registration display helpers ───────────────────────────────────────

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
			fmt.Println("    " + formatRegistration(o))
		}
	}
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
