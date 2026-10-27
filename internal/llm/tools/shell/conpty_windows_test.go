//go:build windows

package shell

import (
	"context"
	"strings"
	"testing"
	"time"
)

// gap #11: Read-Host in the pipe-driven session has no console and hangs to
// the timeout. In console mode it reads the line typed into the console.
func TestReadHostInConsole_conpty(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	start := time.Now()
	res, err := RunInConsole(ctx, "powershell.exe",
		`$n = Read-Host 'Your name'; Write-Output ('HELLO-' + $n.ToUpper())`, t.TempDir(), "gorilla", 45*time.Second)
	if err != nil {
		t.Fatalf("RunInConsole: %v", err)
	}
	t.Logf("took %v, exit %d, output:\n%s", time.Since(start), res.ExitCode, res.Output)
	if res.Interrupted {
		t.Fatal("Read-Host did not return: the command was stopped at the timeout")
	}
	if !strings.Contains(res.Output, "HELLO-GORILLA") {
		t.Fatalf("the typed answer did not reach Read-Host: %q", res.Output)
	}
	if res.ExitCode != 0 {
		t.Errorf("exit code %d, want 0", res.ExitCode)
	}
}

// A program that asks whether it has a console gets yes.
func TestConsoleIsARealConsole_conpty(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	res, err := RunInConsole(ctx, "powershell.exe",
		`Write-Output ('REDIRECTED=' + [Console]::IsOutputRedirected + ' INPUT=' + [Console]::IsInputRedirected)`, "", "", 45*time.Second)
	if err != nil {
		t.Fatalf("RunInConsole: %v", err)
	}
	if !strings.Contains(res.Output, "REDIRECTED=False INPUT=False") {
		t.Fatalf("the command does not see a console: %q", res.Output)
	}
}

// The timeout kills the console command's whole tree and says so.
func TestConsoleTimeoutStopsTheCommand_conpty(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	start := time.Now()
	res, err := RunInConsole(ctx, "powershell.exe", `Start-Sleep -Seconds 60`, "", "", 2*time.Second)
	if err != nil {
		t.Fatalf("RunInConsole: %v", err)
	}
	if !res.Interrupted || res.ExitCode != 143 {
		t.Fatalf("not reported as stopped: %+v", res)
	}
	// 2 s timeout + the kill; a stop that waits for the process to give up
	// on its own would take its 5 s grace on top.
	if took := time.Since(start); took > 4500*time.Millisecond {
		t.Fatalf("the stop took %v", took)
	}
}

func TestConsoleOutputDropsEscapes_conpty(t *testing.T) {
	got := cleanConsoleOutput("\x1b]0;title\x07\x1b[?25l\x1b[2JHELLO\x1b[0m  \r\nworld\r\n")
	if got != "HELLO\nworld" {
		t.Fatalf("got %q", got)
	}
	if consoleKeys("a\nb") != "a\rb\r" {
		t.Fatalf("keys %q", consoleKeys("a\nb"))
	}
}
