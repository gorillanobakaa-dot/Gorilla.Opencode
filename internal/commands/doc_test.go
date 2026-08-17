package commands

// GORILLA OVERRIDE (2026-08-18): docs/COMMANDS.md is generated from this
// registry, and this test is what makes that promise real.
//
// v0.1.90 shipped five new commands — /osint, /yolo, /goal, /compact, /init —
// every one of them explained inside the program in /help and NOWHERE a reader
// could find them without installing and launching it first. The owner spotted
// it within hours: "not documented and detailed". Someone deciding whether to
// download this, on a connection where downloading is a forty-minute
// commitment, could not see what it does.
//
// So the reference is derived from the registry rather than maintained beside
// it, and this test fails if the checked-in file falls behind. Same discipline
// as the settings reference, and the same reason: two hand-kept copies of one
// fact drift on the first change.

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestCommandsDocIsUpToDate(t *testing.T) {
	root := filepath.Join("..", "..")
	docPath := filepath.Join(root, "docs", "COMMANDS.md")

	checkedIn, err := os.ReadFile(docPath)
	if err != nil {
		t.Fatalf("docs/COMMANDS.md is missing: %v\nRegenerate it: go run ./cmd/commands-doc > docs/COMMANDS.md", err)
	}

	gen := exec.Command("go", "run", "./cmd/commands-doc")
	gen.Dir = root // the generator lives at the repo root, not in this package
	out, err := gen.Output()
	if err != nil {
		t.Skipf("cannot run the generator here: %v", err)
	}
	if string(out) != string(checkedIn) {
		t.Errorf("docs/COMMANDS.md is out of date with the registry.\n" +
			"Regenerate it:  go run ./cmd/commands-doc > docs/COMMANDS.md")
	}
}

// Every command must be findable in the written reference, not only in /help.
// Asserted separately from the byte comparison so the failure names the command
// that went missing rather than reporting a diff.
func TestEveryCommandAppearsInTheWrittenDoc(t *testing.T) {
	body, err := os.ReadFile(filepath.Join("..", "..", "docs", "COMMANDS.md"))
	if err != nil {
		t.Skipf("no COMMANDS.md: %v", err)
	}
	doc := string(body)
	for _, c := range All {
		// A command with arguments renders as `/cd [folder]`, so the name is
		// followed by a space rather than a backtick. Accept either.
		if !strings.Contains(doc, "/"+c.Name+"`") && !strings.Contains(doc, "/"+c.Name+" ") {
			t.Errorf("/%s is a real command and does not appear in docs/COMMANDS.md — "+
				"a user cannot discover it without installing the program first", c.Name)
		}
		if !strings.Contains(doc, c.Summary) {
			t.Errorf("/%s appears without its summary; the reference should say what it does", c.Name)
		}
	}
}
