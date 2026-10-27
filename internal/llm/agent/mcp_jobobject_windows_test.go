//go:build windows

package agent

import (
	"context"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/opencode-ai/opencode/internal/config"
	"github.com/opencode-ai/opencode/internal/procjob"
)

// gap #9: a stdio MCP server behind a launcher (npx, uvx) is a tree. When
// the server is abandoned, the whole tree goes, not only the launcher.
func TestJobObjectMCPAbandonKillsGrandchild(t *testing.T) {
	c, err := newStdioMCPClient("cmd.exe", nil, "/c", "cmd /c ping -n 60 127.0.0.1 >nul 2>nul")
	if err != nil {
		t.Fatalf("start: %v", err)
	}
	v, ok := mcpJobs.Load(c)
	if !ok {
		closeMCPClient(c)
		t.Fatal("the MCP server was started without a Job Object")
	}
	job := v.(*procjob.Job)
	var gc int
	for deadline := time.Now().Add(10 * time.Second); gc == 0 && time.Now().Before(deadline); {
		pids, _ := job.Pids()
		for _, p := range pids {
			if procjob.ImageName(p) == "ping.exe" {
				gc = p
			}
		}
		time.Sleep(50 * time.Millisecond)
	}
	if gc == 0 {
		abandonMCPClient(c)
		t.Fatal("no ping in the server's job")
	}
	abandonMCPClient(c)
	for deadline := time.Now().Add(5 * time.Second); procjob.Alive(gc) && time.Now().Before(deadline); {
		time.Sleep(50 * time.Millisecond)
	}
	if procjob.Alive(gc) {
		t.Fatalf("the server's grandchild %d outlived abandonMCPClient", gc)
	}
	if _, still := mcpJobs.Load(c); still {
		t.Error("the abandoned server's job is still registered")
	}
}

// gap #9 for hooks: a hook that starts something in the background and
// exits does not leave it running; the hook's job is closed when it ends.
func TestJobObjectHookBackgroundGrandchildDies(t *testing.T) {
	// Another test may have left the configured folder deleted; a hook
	// refuses to start there (fail closed, TestABeforeToolHookThatCannotRunRefuses).
	if cfg := config.Get(); cfg != nil {
		old := cfg.WorkingDir
		cfg.WorkingDir = t.TempDir()
		t.Cleanup(func() { cfg.WorkingDir = old })
	}
	o := runHook(context.Background(), config.Hook{
		Command:        `(Start-Process -FilePath ping.exe -ArgumentList '-n','60','127.0.0.1' -WindowStyle Hidden -PassThru).Id`,
		TimeoutSeconds: 30,
	}, nil, "")
	if o.exitCode != 0 {
		t.Fatalf("hook failed: exit %d, %q, %v", o.exitCode, o.output, o.startErr)
	}
	pid, err := strconv.Atoi(strings.TrimSpace(o.output))
	if err != nil {
		t.Fatalf("hook printed no pid: %q", o.output)
	}
	for deadline := time.Now().Add(5 * time.Second); procjob.Alive(pid) && time.Now().Before(deadline); {
		time.Sleep(50 * time.Millisecond)
	}
	if procjob.Alive(pid) {
		t.Fatalf("the hook's background ping %d outlived the hook", pid)
	}
}
