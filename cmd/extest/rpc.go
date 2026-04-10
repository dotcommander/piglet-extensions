package main

import (
	"encoding/json"
	"strings"
)

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
