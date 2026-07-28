package dialog

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// A pasted path arrives quoted, unexpanded and possibly with whitespace, because
// nothing here went through a shell.
func TestResolveExportDirHandlesPastedPaths(t *testing.T) {
	dir := t.TempDir()

	for _, in := range []string{dir, "  " + dir + "  ", `"` + dir + `"`, "'" + dir + "'"} {
		got, err := ResolveExportDir(in)
		if err != nil {
			t.Errorf("ResolveExportDir(%q): %v", in, err)
			continue
		}
		if got != dir {
			t.Errorf("ResolveExportDir(%q) = %q, want %q", in, got, dir)
		}
	}

	if got, err := ResolveExportDir("~"); err != nil {
		t.Errorf("~ was not expanded: %v", err)
	} else if strings.HasPrefix(got, "~") {
		t.Errorf("~ left literal: %q — nothing expanded it, because no shell was involved", got)
	}
}

// Failing early with a clear reason beats writing to a path that cannot work, or
// worse, silently creating one the user did not ask for.
func TestResolveExportDirRejectsUnusableDestinations(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "notadir.txt")
	if err := os.WriteFile(file, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}

	cases := map[string]string{
		"":                         "empty",
		"   ":                      "whitespace only",
		filepath.Join(dir, "nope"): "does not exist",
		file:                       "a file, not a folder",
	}
	for in, why := range cases {
		if _, err := ResolveExportDir(in); err == nil {
			t.Errorf("accepted %q (%s)", in, why)
		}
	}
}

// A name containing a separator means the folder field was misunderstood.
// Flattening it silently would put the file somewhere the user did not choose,
// so it is refused with an explanation instead.
func TestSanitiseExportNameRejectsPathSeparators(t *testing.T) {
	for _, in := range []string{"sub/name.md", `sub\name.md`, "/etc/passwd", "..", "."} {
		if got, err := SanitiseExportName(in); err == nil {
			t.Errorf("accepted %q, produced %q — a filename must not redirect the write", in, got)
		}
	}
	if _, err := SanitiseExportName("   "); err == nil {
		t.Error("accepted an empty name")
	}
}

// The extension is defaulted for convenience but a deliberate one is never
// overridden — someone asking for .txt or .log means it.
func TestSanitiseExportNameDefaultsExtensionWithoutOverriding(t *testing.T) {
	cases := map[string]string{
		"session":       "session.md",
		"session.md":    "session.md",
		"session.txt":   "session.txt",
		"notes.log":     "notes.log",
		`"quoted.md"`:   "quoted.md",
		"  spaced.md  ": "spaced.md",
	}
	for in, want := range cases {
		got, err := SanitiseExportName(in)
		if err != nil {
			t.Errorf("SanitiseExportName(%q): %v", in, err)
			continue
		}
		if got != want {
			t.Errorf("SanitiseExportName(%q) = %q, want %q", in, got, want)
		}
	}
}

// A rejected destination must keep the dialog open with the typing intact and
// the cursor moved to the field at fault. Closing on an error would throw away
// the path the user just entered.
func TestInvalidDestinationKeepsTheDialogOpenAndFocusesTheBadField(t *testing.T) {
	m := NewExportDialogCmp().(*exportDialogCmp)
	m.SetDefaults("/definitely/does/not/exist", "fine.md")
	m.Init()

	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})

	if cmd != nil {
		if _, closing := cmd().(CloseExportDialogMsg); closing {
			t.Fatal("the dialog closed on a bad folder, discarding what was typed")
		}
	}
	if m.err == "" {
		t.Error("no error was shown for a non-existent folder")
	}
	if m.focusIdx != 0 {
		t.Errorf("focus is on field %d, not the folder field that failed", m.focusIdx)
	}
	if got := m.inputs[1].Value(); got != "fine.md" {
		t.Errorf("the filename was lost: %q", got)
	}
}

// A valid destination reports it back and closes.
func TestValidDestinationEmitsTheChoiceAndCloses(t *testing.T) {
	dir := t.TempDir()
	m := NewExportDialogCmp().(*exportDialogCmp)
	m.SetDefaults(dir, "record")
	m.Init()

	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("submitting a valid destination produced no command")
	}

	// tea.Batch returns a BatchMsg holding the individual commands.
	batch, ok := cmd().(tea.BatchMsg)
	if !ok {
		t.Fatalf("expected a batch, got %T", cmd())
	}
	var confirmed *ExportConfirmedMsg
	var closed bool
	for _, c := range batch {
		switch v := c().(type) {
		case ExportConfirmedMsg:
			confirmed = &v
		case CloseExportDialogMsg:
			closed = true
		}
	}
	if !closed {
		t.Error("the dialog did not close after a successful export")
	}
	if confirmed == nil {
		t.Fatal("no destination was reported back")
	}
	if confirmed.Dir != dir {
		t.Errorf("Dir = %q, want %q", confirmed.Dir, dir)
	}
	if confirmed.Name != "record.md" {
		t.Errorf("Name = %q, want record.md (extension defaulted)", confirmed.Name)
	}
}

// Tab moves between the two fields and wraps.
func TestTabCyclesFields(t *testing.T) {
	m := NewExportDialogCmp().(*exportDialogCmp)
	m.SetDefaults(t.TempDir(), "x.md")
	m.Init()

	if m.focusIdx != 0 {
		t.Fatalf("opened focused on field %d, expected the folder", m.focusIdx)
	}
	m.Update(tea.KeyMsg{Type: tea.KeyTab})
	if m.focusIdx != 1 {
		t.Errorf("tab did not reach the filename field (got %d)", m.focusIdx)
	}
	m.Update(tea.KeyMsg{Type: tea.KeyTab})
	if m.focusIdx != 0 {
		t.Errorf("tab did not wrap back to the folder field (got %d)", m.focusIdx)
	}
}

// Every line the same width, including with a long error: lipgloss does not pad
// the short lines of a multi-line render, and unpainted cells are black bars.
func TestExportDialogLinesAreUniformWidth(t *testing.T) {
	m := NewExportDialogCmp().(*exportDialogCmp)
	m.SetDefaults("/a/very/long/path/"+strings.Repeat("segment/", 20), strings.Repeat("name", 40)+".md")
	m.Init()
	m.err = strings.Repeat("a long validation message that would overflow the box ", 4)

	lines := strings.Split(m.View(), "\n")
	if len(lines) < 5 {
		t.Fatalf("rendered only %d lines", len(lines))
	}
	want := lipgloss.Width(lines[0])
	for i, l := range lines {
		if got := lipgloss.Width(l); got != want {
			t.Errorf("line %d is %d columns, first line is %d — the difference renders as a black bar:\n%q",
				i, got, want, l)
		}
	}
}
