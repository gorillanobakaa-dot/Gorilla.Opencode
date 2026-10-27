package util

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/opencode-ai/opencode/internal/lsp/protocol"
)

// TestURIApplyWorkspaceEditRealPath: an edit a server sends for a file in a
// folder with a space and non-ASCII letters lands in that file. Before the
// fix the path was the URI minus "file://" ("/C:/..." with %20), which does
// not exist on Windows.
func TestURIApplyWorkspaceEditRealPath(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "my dir é")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	f := filepath.Join(dir, "a.go")
	if err := os.WriteFile(f, []byte("package a\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	edit := protocol.WorkspaceEdit{Changes: map[protocol.DocumentUri][]protocol.TextEdit{
		protocol.URIFromPath(f): {{
			Range:   protocol.Range{Start: protocol.Position{Line: 0, Character: 8}, End: protocol.Position{Line: 0, Character: 9}},
			NewText: "b",
		}},
	}}
	if err := ApplyWorkspaceEdit(edit); err != nil {
		t.Fatalf("ApplyWorkspaceEdit: %v", err)
	}
	got, _ := os.ReadFile(f)
	if string(got) != "package b\n" {
		t.Errorf("file = %q, want %q", got, "package b\n")
	}
	bad := protocol.WorkspaceEdit{Changes: map[protocol.DocumentUri][]protocol.TextEdit{"file:///C:/bad%zz": nil}}
	if err := ApplyWorkspaceEdit(bad); err == nil {
		t.Error("a bad URI was accepted")
	}
}
