//go:build !windows

// GORILLA OVERRIDE (2026-09-01): tagged !windows along with the .desktop entry
// itself. A freedesktop.org application entry is a Linux artifact; Windows uses
// .lnk shortcuts, which install_windows.go creates and its own tests cover.
package cmd

import (
	"os"
	"strings"
	"testing"
)

// There are TWO desktop entries: packaging/gorilla-opencode.desktop, which the
// .deb installs, and the desktopEntry string in install.go, which
// `gorilla-opencode install` writes for a single user. They are edited by hand and
// will drift — this keeps the parts that matter in step.
//
// Why it matters more than it looks: the entry is how most people start this
// program. `Exec=gorilla-opencode launch` passes no arguments, so anything
// reachable only by typing a flag is unreachable for them. That is the same
// mistake that once left the agent's working directory at $HOME on an icon launch,
// with a million files in scope before a word was typed.
func TestBothDesktopEntriesOfferPlainMode(t *testing.T) {
	packaged, err := os.ReadFile("../packaging/gorilla-opencode.desktop")
	if err != nil {
		t.Fatalf("reading the packaged entry: %v", err)
	}

	for name, body := range map[string]string{
		"packaging/gorilla-opencode.desktop": string(packaged),
		"install.go desktopEntry":            desktopEntry,
	} {
		if !strings.Contains(body, "Actions=plain;") {
			t.Errorf("%s declares no plain-mode action, so the copyable interface cannot be reached from the icon", name)
		}
		if !strings.Contains(body, "[Desktop Action plain]") {
			t.Errorf("%s declares the action but does not define it, which desktop-file-validate rejects", name)
		}
		if !strings.Contains(body, "launch --plain") {
			t.Errorf("%s does not pass --plain to launch", name)
		}
		// The ordinary click must stay the full interface: plain mode carries fewer
		// commands, so it must be chosen, never inherited.
		if !strings.Contains(body, "Exec="+appBinName+" launch\n") {
			t.Errorf("%s changed the default click away from the full interface", name)
		}
	}
}

// The action's own name has to say what it is FOR. "Plain mode" alone means
// nothing to someone who has never hit the copy problem.
func TestPlainActionNameExplainsWhatItIsFor(t *testing.T) {
	packaged, err := os.ReadFile("../packaging/gorilla-opencode.desktop")
	if err != nil {
		t.Fatal(err)
	}
	for name, body := range map[string]string{
		"packaged": string(packaged),
		"embedded": desktopEntry,
	} {
		i := strings.Index(body, "[Desktop Action plain]")
		if i < 0 {
			t.Fatalf("%s: no plain action", name)
		}
		section := body[i:]
		if !strings.Contains(section, "copyable") && !strings.Contains(section, "copy") {
			t.Errorf("%s: the action name does not mention copying, which is the entire reason it exists:\n%s", name, section)
		}
	}
}
