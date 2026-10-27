package agent

import (
	"context"
	"errors"
	"runtime"
	"sync/atomic"
	"testing"
	"time"
)

// The watch cancels the call with errMCPServerExited once the process is gone,
// and not before.
func TestMCPServerWatchCancelsOnlyWhenTheProcessIsGone(t *testing.T) {
	prev := mcpServerWatchEvery
	mcpServerWatchEvery = 10 * time.Millisecond
	t.Cleanup(func() { mcpServerWatchEvery = prev })

	var alive atomic.Bool
	alive.Store(true)
	ctx, stop := watchPid(context.Background(), 4242, func(pid int) bool { return pid == 4242 && alive.Load() })
	defer stop()
	select {
	case <-ctx.Done():
		t.Fatal("the call was cancelled while the server was still running")
	case <-time.After(100 * time.Millisecond):
	}
	alive.Store(false)
	select {
	case <-ctx.Done():
	case <-time.After(5 * time.Second):
		t.Fatal("the call is still waiting on a server that exited")
	}
	if !errors.Is(context.Cause(ctx), errMCPServerExited) {
		t.Errorf("cause = %v, want errMCPServerExited", context.Cause(ctx))
	}
	if got := mcpCallError(ctx, context.Canceled); got != errMCPServerExited.Error() {
		t.Errorf("the call said %q, not that the server exited", got)
	}
}

// Stopping the watch is not an exit: the error stays the transport's own.
func TestMCPServerWatchStopIsNotReportedAsAnExit(t *testing.T) {
	ctx, stop := watchPid(context.Background(), 1, func(int) bool { return true })
	stop()
	if errors.Is(context.Cause(ctx), errMCPServerExited) {
		t.Fatal("a stopped watch was reported as the server exiting")
	}
	if got := mcpCallError(ctx, errors.New("transport broke")); got != "transport broke" {
		t.Errorf("got %q", got)
	}
}

// A real stdio server that reads the first request and exits without
// answering (what the Fieldkit server did when discover raised) used to leave
// the call waiting forever: mcp-go never answers a pending request after the
// server's output ends.
func TestAnMCPServerThatExitsMidCallEndsTheCall(t *testing.T) {
	if runtime.GOOS == "darwin" {
		t.Skip("macOS keeps an exited child as a zombie that answers signal 0; the watch cannot see it there")
	}
	prev := mcpServerWatchEvery
	mcpServerWatchEvery = 20 * time.Millisecond
	t.Cleanup(func() { mcpServerWatchEvery = prev })

	cmd, args := "sh", []string{"-c", "read x; exit 0"}
	if runtime.GOOS == "windows" {
		cmd, args = "cmd.exe", []string{"/d", "/c", "set /p x= & exit 0"}
	}
	c, err := newStdioMCPClient(cmd, nil, args...)
	if err != nil {
		t.Fatalf("start: %v", err)
	}
	done := make(chan mcpAnswer, 1)
	go func() { done <- callMCP(context.Background(), c, "discover", `{"goal":"x"}`) }()
	select {
	case a := <-done:
		if !a.failed {
			t.Fatalf("a server that exited produced an answer: %+v", a)
		}
		if a.text != errMCPServerExited.Error() {
			t.Errorf("the call said %q, not that the server exited", a.text)
		}
	case <-time.After(15 * time.Second):
		abandonMCPClient(c)
		t.Fatal("the call is still waiting on a server that exited")
	}
}
