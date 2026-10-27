package lsp

import (
	"bufio"
	"strings"
	"testing"
	"time"
)

// CI run 40535301006 (windows-latest): the Job Object test's fake server
// exited, ReadMessage failed, and handleMessages read cnf.DebugLSP from a nil
// config (no config is loaded in package tests or before start-up loads it).
// The panic took the whole test binary down. The read loop must just return.
func TestHandleMessagesWithoutConfigReturnsOnEOF(t *testing.T) {
	c := &Client{
		stdout:                bufio.NewReader(strings.NewReader("")),
		handlers:              make(map[int32]chan *Message),
		notificationHandlers:  make(map[string]NotificationHandler),
		serverRequestHandlers: make(map[string]ServerRequestHandler),
	}
	done := make(chan struct{})
	go func() {
		defer close(done)
		c.handleMessages()
	}()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("handleMessages did not return at end of input")
	}
}
