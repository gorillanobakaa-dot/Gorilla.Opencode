package agent

// GORILLA (2026-10-27, job 06): the rendered system prompt (what
// `gorilla-opencode system-prompt` prints) carries the machine and the tools.

import (
	"fmt"
	"runtime"
	"strings"
	"testing"

	"github.com/opencode-ai/opencode/internal/config"
	"github.com/opencode-ai/opencode/internal/llm/prompt"
	"github.com/opencode-ai/opencode/internal/llm/tools"
)

func TestTheRenderedSystemPromptKnowsItsToolsAndMachine(t *testing.T) {
	loadRoleConfig(t)
	mcpLoadMu.Lock()
	wasLoaded := mcpLoaded
	mcpLoadMu.Unlock()
	t.Cleanup(func() { mcpLoadMu.Lock(); mcpLoaded = wasLoaded; mcpLoadMu.Unlock() })
	got := CoderSystemPrompt(true)
	t.Cleanup(func() { prompt.SetToolTable("") })

	// The table is generated from the tool set itself: every tool the coder
	// gets is in it (or, when deferred, in the deferred catalogue), one line each.
	set := CoderAgentTools(nil, nil, nil, nil, nil)
	deferral := config.LoadoutEnabled(config.ToolSearchComponentID)
	if !strings.Contains(got, "YOUR TOOLS") {
		t.Fatalf("no tool table in the prompt:\n%s", got)
	}
	for _, tl := range set {
		name := tl.Info().Name
		if deferral && tools.IsDeferrable(name) {
			if !strings.Contains(got, "  "+name) {
				t.Errorf("deferred tool %s is in neither the table nor the catalogue", name)
			}
			continue
		}
		line := fmt.Sprintf("  %-14s %s\n", name, tools.ToolSummary(tl))
		if strings.Count(got, line) != 1 {
			t.Errorf("tool table line for %s missing or repeated: %q", name, line)
		}
	}

	if !strings.Contains(got, fmt.Sprintf("CPUs: %d logical", runtime.NumCPU())) {
		t.Error("no CPU count in the rendered prompt")
	}
	if runtime.GOOS == "windows" {
		for _, want := range []string{
			"Platform: windows", "Shell (the bash tool): ", "PowerShell", "robocopy /MT:n takes 1-128",
			"Embedded toolbox", "How-to cards", "robocopy, powershell-files",
		} {
			if !strings.Contains(got, want) {
				t.Errorf("rendered Windows prompt lacks %q", want)
			}
		}
		bash := ""
		for _, l := range strings.Split(got, "\n") {
			if strings.HasPrefix(l, "  bash ") {
				bash = l
			}
		}
		if !strings.Contains(bash, "PowerShell") || !strings.Contains(bash, "not bash") {
			t.Errorf("on Windows the bash tool's line must say PowerShell and its edition: %q", bash)
		}
		for _, linuxOnly := range []string{"ext4", "distro package"} {
			if strings.Contains(got, linuxOnly) {
				t.Errorf("Linux-only hint %q in the Windows prompt", linuxOnly)
			}
		}
	}
}
