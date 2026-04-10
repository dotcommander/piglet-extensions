package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
)

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
		logf("→ sent %s", msgs[0].Method)

		logf("→ waiting for init response...")
		initDeadline := time.After(time.Duration(timeout) * time.Second)
		for {
			select {
			case raw, ok := <-lineCh:
				if !ok {
					logf("→ stdout closed during init")
					result.timedOut = true
					goto finish
				}
				l := parseLine(raw)
				if l == nil {
					continue
				}
				collectLine(l)
				if l.isInit {
					logf("→ init received (%dms)", time.Since(start).Milliseconds())
					goto initDone
				}
			case <-initDeadline:
				logf("→ init timeout (%ds)", timeout)
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
		logf("→ sent %s", m.Method)
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
