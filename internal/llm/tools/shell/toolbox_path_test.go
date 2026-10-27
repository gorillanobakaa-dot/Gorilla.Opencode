package shell

import (
	"runtime"
	"strings"
	"testing"

	"github.com/opencode-ai/opencode/internal/toolbox"
)

// Job 04: in a -tags toolbox build with the helpers embedded, the agent's
// shell finds every helper by its plain name. Elsewhere there is nothing to
// put on PATH and these skip.
func toolboxDirsOrSkip(t *testing.T) []string {
	t.Helper()
	dirs, err := toolbox.Dirs()
	if err != nil {
		t.Fatal(err)
	}
	if len(dirs) == 0 {
		t.Skip("no embedded helper in this build (-tags toolbox after scripts/build-toolbox.ps1)")
	}
	return dirs
}

func TestAgentShellEnvPutsTheToolboxFirstOnPATH(t *testing.T) {
	dirs := toolboxDirsOrSkip(t)
	for _, kv := range agentShellEnv(runtime.GOOS == "windows") {
		eq := strings.IndexByte(kv, '=')
		if eq > 0 && strings.EqualFold(kv[:eq], "PATH") {
			if !strings.HasPrefix(kv[eq+1:], dirs[0]) {
				t.Fatalf("PATH does not start with the toolbox: %.200s", kv)
			}
			return
		}
	}
	t.Fatal("the agent shell has no PATH")
}

func TestAgentShellCallsAHelperByPlainName(t *testing.T) {
	toolboxDirsOrSkip(t)
	if _, err := toolbox.Path("sd"); err != nil {
		t.Skipf("sd is not embedded: %v", err)
	}
	s := newTestShell(t)
	stdout, stderr, code := exec1(t, s, "sd --version")
	if code != 0 || !strings.HasPrefix(strings.TrimSpace(stdout), "sd ") {
		t.Fatalf("sd --version: code %d, stdout %q, stderr %q", code, stdout, stderr)
	}
}
