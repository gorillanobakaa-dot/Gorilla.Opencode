// Package acp makes Gorilla OpenCode an Agent Client Protocol agent, so an
// editor (Zed, JetBrains, anything that speaks ACP) can drive a session over
// stdio instead of the terminal.
//
// The protocol is JSON-RPC 2.0, one message per line, client on stdin, agent on
// stdout. Nothing else may be written to stdout while the server runs: a log
// line there corrupts the channel. Logs go to the usual log file.
//
// Spec: https://agentclientprotocol.com (schema v1.25.0 read 2026-10-09).
package acp

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"sync"
	"sync/atomic"
)

// JSON-RPC 2.0 error codes used here.
const (
	codeParse          = -32700
	codeInvalidRequest = -32600
	codeMethodNotFound = -32601
	codeInvalidParams  = -32602
	codeInternal       = -32603
)

type rpcMessage struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method,omitempty"`
	Params  json.RawMessage `json:"params,omitempty"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *rpcError       `json:"error,omitempty"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"`
}

func (e *rpcError) Error() string { return fmt.Sprintf("%d: %s", e.Code, e.Message) }

// conn is one JSON-RPC connection: a reader for the client's lines and a
// serialised writer for ours. Requests the agent makes to the client
// (session/request_permission) are matched to their answers by id.
type conn struct {
	in  *bufio.Scanner
	out io.Writer

	writeMu sync.Mutex
	nextID  atomic.Int64

	pendingMu sync.Mutex
	pending   map[int64]chan rpcMessage
}

func newConn(r io.Reader, w io.Writer) *conn {
	sc := bufio.NewScanner(r)
	// An embedded resource (a whole file in the prompt) can be large.
	sc.Buffer(make([]byte, 0, 64*1024), 64*1024*1024)
	return &conn{in: sc, out: w, pending: map[int64]chan rpcMessage{}}
}

func (c *conn) write(v any) error {
	b, err := json.Marshal(v)
	if err != nil {
		return err
	}
	c.writeMu.Lock()
	defer c.writeMu.Unlock()
	_, err = c.out.Write(append(b, '\n'))
	return err
}

func (c *conn) notify(method string, params any) error {
	p, err := json.Marshal(params)
	if err != nil {
		return err
	}
	return c.write(rpcMessage{JSONRPC: "2.0", Method: method, Params: p})
}

func (c *conn) respond(id json.RawMessage, result any) error {
	r, err := json.Marshal(result)
	if err != nil {
		return err
	}
	return c.write(rpcMessage{JSONRPC: "2.0", ID: id, Result: r})
}

func (c *conn) respondError(id json.RawMessage, code int, msg string) error {
	if len(id) == 0 {
		id = json.RawMessage("null")
	}
	return c.write(rpcMessage{JSONRPC: "2.0", ID: id, Error: &rpcError{Code: code, Message: msg}})
}

// call sends a request to the client and waits for its answer, or for ctx.
func (c *conn) call(ctx context.Context, method string, params any) (json.RawMessage, error) {
	id := c.nextID.Add(1)
	ch := make(chan rpcMessage, 1)
	c.pendingMu.Lock()
	c.pending[id] = ch
	c.pendingMu.Unlock()
	defer func() {
		c.pendingMu.Lock()
		delete(c.pending, id)
		c.pendingMu.Unlock()
	}()
	p, err := json.Marshal(params)
	if err != nil {
		return nil, err
	}
	rawID, _ := json.Marshal(id)
	if err := c.write(rpcMessage{JSONRPC: "2.0", ID: rawID, Method: method, Params: p}); err != nil {
		return nil, err
	}
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case m := <-ch:
		if m.Error != nil {
			return nil, m.Error
		}
		return m.Result, nil
	}
}

// deliverResponse hands a client's answer to the waiting call. Returns false
// when nothing was waiting for that id.
func (c *conn) deliverResponse(m rpcMessage) bool {
	var id int64
	if json.Unmarshal(m.ID, &id) != nil {
		return false
	}
	c.pendingMu.Lock()
	ch, ok := c.pending[id]
	c.pendingMu.Unlock()
	if !ok {
		return false
	}
	ch <- m
	return true
}
