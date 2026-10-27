//go:build windows

package lsp

import (
	"context"
	"testing"
	"time"

	"github.com/opencode-ai/opencode/internal/procjob"
)

// gap #9: a language server that does not exit when its stdin closes was
// ended with a bare Process.Kill, which left its children running. The
// "server" here is cmd -> cmd -> ping; Close must take the grandchild too.
func TestJobObjectLSPCloseKillsGrandchild(t *testing.T) {
	c, err := NewClient(context.Background(), "cmd.exe", "/c", "cmd /c ping -n 60 127.0.0.1 >nul 2>nul")
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	if c.job == nil {
		t.Fatal("the language server was started without a Job Object")
	}
	var gc int
	for deadline := time.Now().Add(10 * time.Second); gc == 0 && time.Now().Before(deadline); {
		pids, _ := c.job.Pids()
		for _, p := range pids {
			if procjob.ImageName(p) == "ping.exe" {
				gc = p
			}
		}
		time.Sleep(50 * time.Millisecond)
	}
	if gc == 0 {
		_ = c.job.Terminate()
		t.Fatal("no ping in the server's job")
	}
	_ = c.Close() // stdin closed, 2 s, then the job is killed
	for deadline := time.Now().Add(5 * time.Second); procjob.Alive(gc) && time.Now().Before(deadline); {
		time.Sleep(50 * time.Millisecond)
	}
	if procjob.Alive(gc) {
		t.Fatalf("the server's grandchild %d outlived Close", gc)
	}
}

// The context the server was started with ends the whole tree when it is
// cancelled (exec's Cancel hook), not only the server process.
func TestJobObjectLSPContextCancelKillsGrandchild(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	c, err := NewClient(ctx, "cmd.exe", "/c", "cmd /c ping -n 60 127.0.0.1 >nul 2>nul")
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	defer c.job.Close()
	var gc int
	for deadline := time.Now().Add(10 * time.Second); gc == 0 && time.Now().Before(deadline); {
		pids, _ := c.job.Pids()
		for _, p := range pids {
			if procjob.ImageName(p) == "ping.exe" {
				gc = p
			}
		}
		time.Sleep(50 * time.Millisecond)
	}
	if gc == 0 {
		t.Fatal("no ping in the server's job")
	}
	cancel()
	for deadline := time.Now().Add(5 * time.Second); procjob.Alive(gc) && time.Now().Before(deadline); {
		time.Sleep(50 * time.Millisecond)
	}
	if procjob.Alive(gc) {
		t.Fatalf("the server's grandchild %d outlived the cancelled context", gc)
	}
}
