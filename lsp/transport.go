package lsp

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
)

// ---------------------------------------------------------------------------
// JSON-RPC 2.0 transport
// ---------------------------------------------------------------------------

type jsonrpcRequest struct {
	JSONRPC string `json:"jsonrpc"`
	ID      int64  `json:"id"`
	Method  string `json:"method"`
	Params  any    `json:"params,omitempty"`
}

type jsonrpcResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      *int64          `json:"id"` // pointer: null → nil (notification), 0 → valid
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *jsonrpcError   `json:"error,omitempty"`
}

type jsonrpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func (e *jsonrpcError) Error() string {
	return fmt.Sprintf("LSP error %d: %s", e.Code, e.Message)
}

type rpcResult struct {
	Data json.RawMessage
	Err  error
}

// transport manages JSON-RPC 2.0 communication over stdin/stdout pipes.
type transport struct {
	stdin   io.WriteCloser
	stdout  *bufio.Reader
	nextID  atomic.Int64
	writeMu sync.Mutex // protects stdin writes
	mu      sync.Mutex // protects pending map
	pending map[int64]chan rpcResult
	done    chan struct{} // closed when readLoop exits
}

func newTransport(stdin io.WriteCloser, stdout *bufio.Reader) *transport {
	t := &transport{
		stdin:   stdin,
		stdout:  stdout,
		pending: make(map[int64]chan rpcResult),
		done:    make(chan struct{}),
	}
	go t.readLoop()
	return t
}

func (t *transport) call(ctx context.Context, method string, params any, result any) error {
	raw, err := t.callRaw(ctx, method, params)
	if err != nil {
		return err
	}
	if result != nil && len(raw) > 0 {
		return json.Unmarshal(raw, result)
	}
	return nil
}

func (t *transport) callRaw(ctx context.Context, method string, params any) (json.RawMessage, error) {
	id := t.nextID.Add(1)

	ch := make(chan rpcResult, 1)
	t.mu.Lock()
	t.pending[id] = ch
	t.mu.Unlock()

	defer func() {
		t.mu.Lock()
		delete(t.pending, id)
		t.mu.Unlock()
	}()

	req := jsonrpcRequest{
		JSONRPC: "2.0",
		ID:      id,
		Method:  method,
		Params:  params,
	}

	if err := t.send(req); err != nil {
		return nil, fmt.Errorf("send %s: %w", method, err)
	}

	select {
	case result := <-ch:
		return result.Data, result.Err
	case <-t.done:
		return nil, ErrServerDied
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func (t *transport) notify(method string, params any) error {
	msg := struct {
		JSONRPC string `json:"jsonrpc"`
		Method  string `json:"method"`
		Params  any    `json:"params,omitempty"`
	}{
		JSONRPC: "2.0",
		Method:  method,
		Params:  params,
	}
	return t.send(msg)
}

func (t *transport) send(msg any) error {
	data, err := json.Marshal(msg)
	if err != nil {
		return err
	}

	header := fmt.Sprintf("Content-Length: %d\r\n\r\n", len(data))

	t.writeMu.Lock()
	defer t.writeMu.Unlock()

	if _, err := io.WriteString(t.stdin, header); err != nil {
		return err
	}
	_, err = t.stdin.Write(data)
	return err
}

func (t *transport) readLoop() {
	defer close(t.done)

	for {
		var contentLength int
		for {
			line, err := t.stdout.ReadString('\n')
			if err != nil {
				return
			}
			line = strings.TrimSpace(line)
			if line == "" {
				break
			}
			if strings.HasPrefix(line, "Content-Length:") {
				val := strings.TrimSpace(strings.TrimPrefix(line, "Content-Length:"))
				contentLength, _ = strconv.Atoi(val)
			}
		}

		if contentLength == 0 {
			continue
		}

		body := make([]byte, contentLength)
		if _, err := io.ReadFull(t.stdout, body); err != nil {
			return
		}

		var resp jsonrpcResponse
		if json.Unmarshal(body, &resp) != nil {
			continue
		}

		if resp.ID == nil {
			continue
		}

		t.mu.Lock()
		ch, ok := t.pending[*resp.ID]
		t.mu.Unlock()

		if !ok {
			continue
		}

		if resp.Error != nil {
			ch <- rpcResult{Err: resp.Error}
			continue
		}

		ch <- rpcResult{Data: resp.Result}
	}
}
