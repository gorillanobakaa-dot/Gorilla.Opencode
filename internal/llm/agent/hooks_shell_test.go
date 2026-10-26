package agent

import (
	"runtime"
	"slices"
	"strings"
	"testing"
)

// TestExecutionPolicyHookShell_executionpolicy: a hook's PowerShell bypasses
// the execution policy for its own process (gap #3) and is PowerShell
// (pwsh when the bash tool drives pwsh, gap #18).
func TestExecutionPolicyHookShell_executionpolicy(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Windows hook shell")
	}
	exe, args := hookShell("Write-Output hi")
	base := strings.ToLower(exe)
	if !strings.Contains(base, "powershell") && !strings.Contains(base, "pwsh") {
		t.Errorf("hook shell = %q, want PowerShell", exe)
	}
	i := slices.Index(args, "-ExecutionPolicy")
	if i < 0 || args[i+1] != "Bypass" {
		t.Errorf("hook args = %q, want -ExecutionPolicy Bypass", args)
	}
	if args[len(args)-1] != "Write-Output hi" || !slices.Contains(args, "-NonInteractive") {
		t.Errorf("hook args = %q", args)
	}
}
