package howto

import (
	"bufio"
	"os"
	"path"
	"strings"
	"testing"

	"github.com/opencode-ai/opencode/internal/hostprobe"
)

// Every helper the toolbox may embed has its card (one card each, what it is
// for and examples), read from the same SHA256SUMS the binary trusts.
func TestEveryPinnedToolboxHelperHasACard(t *testing.T) {
	f, err := os.Open("../toolbox/SHA256SUMS")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	n := 0
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		fields := strings.Fields(line)
		name := strings.TrimSuffix(path.Base(fields[len(fields)-1]), ".exe")
		n++
		if !Exists(name) {
			t.Errorf("toolbox helper %q has no card cards/%s.md", name, name)
			continue
		}
		card, ok := getFor(hostprobe.Facts{OS: "windows", Toolbox: []string{name}}, name)
		if !ok || strings.Count(card, "\n    ") < 3 {
			t.Errorf("card %s: want at least 3 examples, got:\n%s", name, card)
		}
	}
	if n == 0 {
		t.Fatal("no helper read from SHA256SUMS")
	}
}

func TestRobocopyCardCarriesTheSwitchesAndExitCodes(t *testing.T) {
	card, ok := getFor(hostprobe.Facts{OS: "windows"}, "robocopy")
	if !ok {
		t.Fatal("no robocopy card on Windows")
	}
	for _, want := range []string{"/MIR", "/PURGE", "/MOVE", "/MOV ", "/MT:n", "1-128", "/Z", "/B", "/L", "/XD", "/XF", "/XO",
		"/COPY:", "/DCOPY:", "8 some files", "16 fatal", "0-7 is success"} {
		if !strings.Contains(card, want) {
			t.Errorf("robocopy card lacks %q", want)
		}
	}
}

func TestCardsAreOfferedOnlyWhereTheyAreTrue(t *testing.T) {
	win := hostprobe.Facts{OS: "windows"}
	got := strings.Join(availableFor(win), ",")
	if got != "robocopy,powershell-files,attributes-acl-registry,services-events-tasks" {
		t.Errorf("plain Windows: %s", got)
	}
	if _, ok := getFor(win, "powertoys"); ok {
		t.Error("PowerToys card served although PowerToys is not installed")
	}
	if _, ok := getFor(win, "sysinternals"); ok {
		t.Error("Sysinternals card served although none is on PATH")
	}
	if _, ok := getFor(win, "sd"); ok {
		t.Error("toolbox card served for a helper this build does not embed")
	}

	win.PowerToys, win.Sysinternals, win.Toolbox = `C:\PT`, []string{"handle"}, []string{"sd", "pfind"}
	got = strings.Join(availableFor(win), ",")
	if !strings.HasSuffix(got, ",powertoys,sysinternals,pfind,sd") {
		t.Errorf("everything found: %s", got)
	}

	linux := hostprobe.Facts{OS: "linux", Toolbox: []string{"sd"}}
	if got := strings.Join(availableFor(linux), ","); got != "sd" {
		t.Errorf("Linux offers %q; the Windows cards must not appear there", got)
	}
	if promptLineFor(hostprobe.Facts{OS: "linux"}) != "" {
		t.Error("a prompt line with no cards")
	}
	if l := promptLineFor(hostprobe.Facts{OS: "windows"}); !strings.Contains(l, "action=howto") || !strings.Contains(l, "robocopy") {
		t.Errorf("prompt line: %q", l)
	}
}

func TestCardNamesAreForgiving(t *testing.T) {
	if _, ok := getFor(hostprobe.Facts{OS: "windows"}, " Robocopy.md "); !ok {
		t.Error("case, spaces and .md should not matter")
	}
}
