package prompt

// GORILLA (2026-10-27, job 06): the model knows its machine and its tools.

import (
	"fmt"
	"runtime"
	"strings"
	"testing"

	"github.com/opencode-ai/opencode/internal/config"
	"github.com/opencode-ai/opencode/internal/hostprobe"
	"github.com/opencode-ai/opencode/internal/llm/models"
)

func TestTheEnvironmentBlockCarriesTheProbe(t *testing.T) {
	loadCfg(t)
	got := getEnvironmentInfo(models.Model{})
	env := got[strings.Index(got, "<env>"):strings.Index(got, "</env>")]
	for _, l := range hostprobe.Get().Lines() {
		if !strings.Contains(env, l) {
			t.Errorf("<env> lacks the probe line %q:\n%s", l, env)
		}
	}
	if !strings.Contains(env, fmt.Sprintf("CPUs: %d logical", runtime.NumCPU())) {
		t.Errorf("<env> lacks the CPU count:\n%s", env)
	}
	if runtime.GOOS == "windows" {
		for _, want := range []string{"Platform: windows", "PowerShell", "robocopy /MT:n takes 1-128", "Embedded toolbox", "action=howto"} {
			if !strings.Contains(env, want) {
				t.Errorf("Windows <env> lacks %q:\n%s", want, env)
			}
		}
	}
}

func TestTheCoderPromptCarriesTheToolTable(t *testing.T) {
	loadCfg(t)
	SetToolTable("YOUR TOOLS (loaded now; call them by these names)\n  bash           Runs things\n")
	t.Cleanup(func() { SetToolTable("") })
	got := CoderPromptFor(models.Model{})
	if !strings.Contains(got, "YOUR TOOLS") || !strings.Contains(got, "Runs things") {
		t.Fatalf("the tool table is not in the coder prompt")
	}
	if strings.Index(got, "YOUR TOOLS") < strings.Index(got, "</project_summary>") {
		t.Error("the tool table sits inside the environment block")
	}
}

// Gap #20: the ext4 and distro-package hints are Linux facts.
func TestOSGatedLinesOnlyOnTheirOS(t *testing.T) {
	body := "- a [[needs os.linux]]\n- b [[needs os.windows]]\n- c [[needs os.darwin]]\n- d"
	got := applyToolGates(body)
	for goos, line := range map[string]string{"linux": "- a", "windows": "- b", "darwin": "- c"} {
		if (goos == runtime.GOOS) != strings.Contains(got, line) {
			t.Errorf("os.%s line kept=%v on %s:\n%s", goos, strings.Contains(got, line), runtime.GOOS, got)
		}
	}
	if !strings.HasSuffix(got, "- d") || strings.Contains(got, "[[") {
		t.Errorf("ungated line lost or a marker left:\n%s", got)
	}

	if !config.LoadoutEnabled("prompt.localtools") {
		config.ToggleLoadout("prompt.localtools")
		t.Cleanup(func() { config.ToggleLoadout("prompt.localtools") })
	}
	invalidateSections()
	t.Cleanup(invalidateSections)
	base := BaseCoderPrompt("")
	if !strings.Contains(base, "yt-dlp") {
		t.Fatal("prompt.localtools is on but its lines are missing: the check below would be vacuous")
	}
	if runtime.GOOS != "linux" && (strings.Contains(base, "ext4") || strings.Contains(base, "distro package")) {
		t.Errorf("Linux-only hints in the %s prompt", runtime.GOOS)
	}
	if strings.Contains(base, "Windows internals") {
		t.Error(`the prompt still claims "Windows internals"`)
	}
}
