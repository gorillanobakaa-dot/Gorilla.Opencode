package codereview

import (
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// installerScoopCheck drives install_tools.py with a fake PATH lookup and a
// fake process runner: nothing is installed and nothing is downloaded.
const installerScoopCheck = `
import sys
sys.path.insert(0, sys.argv[1])
import install_tools as it
from tools_registry import TOOLS

scoop_only = lambda n: "C:/Users/u/scoop/shims/scoop.cmd" if n == "scoop" else None
got = it.detect_pkg_manager(which=scoop_only, windows=True)
assert got == "scoop", "Windows with scoop detected %r" % got
apt_only = lambda n: "/usr/bin/apt-get" if n == "apt-get" else None
got = it.detect_pkg_manager(which=apt_only, windows=True)
assert got is None, "Windows offered a Linux package manager: %r" % got
got = it.detect_pkg_manager(which=apt_only, windows=False)
assert got == "apt-get", "Linux lost apt-get: %r" % got

calls = []
def fake_run(cmd, **kw):
    calls.append(cmd)
    return 0, "installed", ""
it.run = fake_run
it.PKG_MANAGER = "scoop"
class Args:
    yes = True
for tool_id, pkg in (("cppcheck", "cppcheck"), ("shellcheck", "shellcheck"), ("clang-format-check", "llvm")):
    tool = next(t for t in TOOLS if t.id == tool_id)
    del calls[:]
    how, _, attempts = it.try_install(tool, Args())
    assert how == "scoop", "%s installed by %r: %r" % (tool_id, how, attempts)
    assert calls and calls[0][1:] == ["install", pkg], "%s ran %r" % (tool_id, calls)
print("SCOOP-OK")
`

// TestToolkitInstallerKnowsScoop (gap #13): on Windows the code-review
// toolkit's installer detects Scoop and installs through it, and never
// offers apt there. Before the fix it knew no Windows package manager, so
// every tool fell through to "install it by hand".
func TestToolkitInstallerKnowsScoop(t *testing.T) {
	py := ""
	for _, name := range []string{"python3", "python"} {
		if runtime.GOOS == "windows" && name == "python3" {
			continue // the Store stub answers to python3
		}
		if p, err := exec.LookPath(name); err == nil {
			py = p
			break
		}
	}
	if py == "" {
		t.Skip("no Python here; the toolkit needs one to run at all")
	}
	dir, err := filepath.Abs("toolkit")
	if err != nil {
		t.Fatal(err)
	}
	out, err := exec.Command(py, "-I", "-c", installerScoopCheck, dir).CombinedOutput()
	if err != nil || !strings.Contains(string(out), "SCOOP-OK") {
		t.Fatalf("installer check failed: %v\n%s", err, out)
	}
}
