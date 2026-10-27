//go:build toolbox

package toolbox

import (
	"os/exec"
	"runtime"
	"strings"
	"testing"
)

// The real embedded set of THIS build (-tags toolbox). With only bin/README.md
// present (CI, a fresh clone) there is nothing to check and the test says so.
func TestEmbeddedHelpersMatchTheirHashAndExtract(t *testing.T) {
	if !tagged {
		t.Fatal("embed_on.go must be the file built with -tags toolbox")
	}
	b, err := Default()
	if err != nil {
		t.Fatal(err)
	}
	names := b.Names()
	if len(names) == 0 {
		t.Skip("no helper in bin/" + runtime.GOOS + "-" + runtime.GOARCH + " (run scripts/build-toolbox.ps1 first)")
	}
	for _, n := range names {
		p, err := b.Path(n)
		if err != nil {
			t.Errorf("%s: %v", n, err)
			continue
		}
		if !IsToolboxFile(p) {
			t.Errorf("%s extracted outside the toolbox folder: %s", n, p)
		}
		t.Logf("%s -> %s", n, p)
	}
	// One real exec through the checked path.
	if p, err := b.Path("pfind"); err == nil {
		out, err := exec.Command(p, "--version").CombinedOutput()
		if err != nil || !strings.Contains(string(out), "pfind") {
			t.Errorf("pfind --version: %v: %s", err, out)
		}
		t.Logf("pfind --version: %s", strings.TrimSpace(string(out)))
	}
}
