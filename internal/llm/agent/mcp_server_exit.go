package agent

import (
	"context"
	"errors"
	"time"

	"github.com/mark3labs/mcp-go/client"
)

// GORILLA FIX (2026-10-27, job 07): a stdio MCP server that dies during a call
// left the call waiting forever. mcp-go v0.17.0 (pinned) stops reading at the
// server's end of output and never answers the requests still pending, so
// CallTool waits on a context nobody cancels. Measured in the acceptance run:
// the Fieldkit server crashed on a discover call (a TypeError in its own code),
// and the chat showed "Waiting for tool response... (320s)" with no process
// left to answer. The call now ends when the server's process is gone, and
// says so.

// errMCPServerExited is the cause a call's context carries when the server's
// process is no longer running.
var errMCPServerExited = errors.New("the MCP server's process exited before it answered (it may have crashed)")

// mcpServerWatchEvery is how often a running call checks the server's process.
// A variable so the tests do not wait for it.
var mcpServerWatchEvery = 250 * time.Millisecond

// watchMCPServer returns a context that is cancelled with errMCPServerExited
// once a stdio server's process is gone. Any other client is returned as it is.
// The stop function ends the watch; call it when the call is over.
func watchMCPServer(ctx context.Context, c MCPClient) (context.Context, func()) {
	sc, ok := c.(*client.StdioMCPClient)
	if !ok {
		return ctx, func() {}
	}
	cmd := stdioCmd(sc)
	if cmd == nil || cmd.Process == nil {
		return ctx, func() {}
	}
	return watchPid(ctx, cmd.Process.Pid, processRunning)
}

// watchPid is watchMCPServer's loop, with the liveness check passed in so it is
// tested without a real process.
func watchPid(ctx context.Context, pid int, alive func(int) bool) (context.Context, func()) {
	wctx, cancel := context.WithCancelCause(ctx)
	done := make(chan struct{})
	go func() {
		t := time.NewTicker(mcpServerWatchEvery)
		defer t.Stop()
		for {
			select {
			case <-done:
				return
			case <-wctx.Done():
				return
			case <-t.C:
				if !alive(pid) {
					cancel(errMCPServerExited)
					return
				}
			}
		}
	}()
	return wctx, func() { close(done); cancel(nil) }
}

// mcpCallError is the text a failed call returns: the server's exit when that
// is why the call ended, the transport's own error otherwise.
func mcpCallError(ctx context.Context, err error) string {
	if errors.Is(context.Cause(ctx), errMCPServerExited) {
		return errMCPServerExited.Error()
	}
	return err.Error()
}
