package tools

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/opencode-ai/opencode/internal/config"
	"github.com/opencode-ai/opencode/internal/permission"
)

func forensicsFixture(t *testing.T, name string) string {
	t.Helper()
	p, err := filepath.Abs(filepath.Join("..", "..", "forensics", "testdata", name))
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func forensicsCall(t *testing.T, tool BaseTool, params map[string]any) (ToolResponse, error) {
	t.Helper()
	in, _ := json.Marshal(params)
	ctx := context.WithValue(context.Background(), SessionIDContextKey, "s1")
	ctx = context.WithValue(ctx, MessageIDContextKey, "m1")
	return tool.Run(ctx, ToolCall{Name: tool.Info().Name, Input: string(in)})
}

func TestBinaryInspectToolSummaryAndStrings(t *testing.T) {
	tool := NewBinaryInspectTool()
	r, err := forensicsCall(t, tool, map[string]any{"path": forensicsFixture(t, "fx-fat.macho")})
	if err != nil || r.IsError {
		t.Fatalf("summary: %v %s", err, r.Content)
	}
	if !strings.Contains(r.Content, "macOS 10.13.6 (MacBook Air 2011, Intel) check: OK by header") {
		t.Errorf("no 10.13 verdict:\n%s", r.Content)
	}
	// Strings are the author's words, so they come back fenced.
	r, _ = forensicsCall(t, tool, map[string]any{"path": forensicsFixture(t, "fx.pe"), "part": "strings", "filter": "gorilla"})
	if !strings.Contains(r.Content, "UNTRUSTED CONTENT BEGINS") || !strings.Contains(r.Content, "GORILLA_WIDE_16") {
		t.Errorf("strings not fenced or missing:\n%s", r.Content)
	}
	r, _ = forensicsCall(t, tool, map[string]any{})
	if !r.IsError {
		t.Error("missing path accepted")
	}
}

func TestFullReachesBothTools(t *testing.T) {
	r, _ := forensicsCall(t, NewBinaryInspectTool(), map[string]any{"path": forensicsFixture(t, "fx.pe"), "part": "imports", "full": true})
	if r.IsError || strings.Contains(r.Content, "more not shown") || !strings.Contains(r.Content, "KERNEL32.dll!VirtualQuery") {
		t.Errorf("binary_inspect full:\n%s", r.Content)
	}
	r, _ = forensicsCall(t, NewArchiveTool(&recordingPermissions{}), map[string]any{"path": forensicsFixture(t, "omni.ja"), "full": true})
	if r.IsError || !strings.Contains(r.Content, "entries: 4") {
		t.Errorf("archive full:\n%s", r.Content)
	}
	// A list longer than the cap, so a dropped full shows up as a capped result.
	var data []byte
	for i := 0; i < 200; i++ {
		data = append(data, []byte("stringnumber")...)
		data = append(data, 0)
	}
	many := filepath.Join(t.TempDir(), "many.bin")
	os.WriteFile(many, data, 0o644)
	r, _ = forensicsCall(t, NewBinaryInspectTool(), map[string]any{"path": many, "part": "strings", "full": true})
	if r.IsError || strings.Contains(r.Content, "more not shown") {
		t.Errorf("binary_inspect dropped full: %s", r.Content[:200])
	}
	r, _ = forensicsCall(t, NewBinaryInspectTool(), map[string]any{"path": many, "part": "strings"})
	if !strings.Contains(r.Content, "more not shown") {
		t.Error("binary_inspect default is not capped")
	}
	for _, tool := range []BaseTool{NewBinaryInspectTool(), NewArchiveTool(nil)} {
		if _, ok := tool.Info().Parameters["full"]; !ok {
			t.Errorf("%s has no full parameter", tool.Info().Name)
		}
	}
}

func TestBinaryInspectToolKeepsTheCredentialGuard(t *testing.T) {
	home, _ := os.UserHomeDir()
	r, _ := forensicsCall(t, NewBinaryInspectTool(), map[string]any{"path": filepath.Join(home, ".ssh", "id_rsa"), "part": "strings"})
	if !r.IsError || !strings.Contains(r.Content, "credential") {
		t.Errorf("~/.ssh/id_rsa not refused: %s", r.Content)
	}
	r, _ = forensicsCall(t, NewArchiveTool(&recordingPermissions{}), map[string]any{"path": filepath.Join(home, ".ssh", "id_rsa")})
	if !r.IsError || !strings.Contains(r.Content, "credential") {
		t.Errorf("archive tool: ~/.ssh/id_rsa not refused: %s", r.Content)
	}
}

func TestArchiveToolListAndFind(t *testing.T) {
	tool := NewArchiveTool(&recordingPermissions{})
	r, _ := forensicsCall(t, tool, map[string]any{"path": forensicsFixture(t, "omni.ja")})
	if r.IsError || !strings.Contains(r.Content, "Mozilla optimized jar") {
		t.Errorf("list:\n%s", r.Content)
	}
	r, _ = forensicsCall(t, tool, map[string]any{"path": forensicsFixture(t, "omni.ja"), "action": "find"})
	if !r.IsError {
		t.Error("find without a pattern accepted")
	}
	r, _ = forensicsCall(t, tool, map[string]any{"path": forensicsFixture(t, "omni.ja"), "action": "find", "pattern": "*.js"})
	if !strings.Contains(r.Content, "defaults/pref/gorilla.js") || strings.Contains(r.Content, "chrome.manifest") {
		t.Errorf("find *.js:\n%s", r.Content)
	}
}

func TestArchiveToolExtractAsksAndStaysInTheProject(t *testing.T) {
	wd := config.WorkingDirectory()
	out := filepath.Join(wd, "extracted")
	t.Cleanup(func() { os.RemoveAll(out) })
	omni := forensicsFixture(t, "omni.ja")

	// Denied: an error, and nothing created, not even the folder.
	deny := &recordingPermissions{grant: false}
	_, err := forensicsCall(t, NewArchiveTool(deny), map[string]any{"path": omni, "action": "extract", "entry": "chrome.manifest"})
	if !errors.Is(err, permission.ErrorPermissionDenied) {
		t.Errorf("denied extraction: err = %v", err)
	}
	if _, statErr := os.Stat(out); statErr == nil {
		t.Error("denied extraction left a folder behind")
	}
	if len(deny.calls) != 1 || !strings.Contains(deny.calls[0].Description, "Extract chrome.manifest from omni.ja") {
		t.Errorf("permission request: %+v", deny.calls)
	}

	// Granted: the file lands under extracted/<archive>/.
	allow := &recordingPermissions{grant: true}
	r, err := forensicsCall(t, NewArchiveTool(allow), map[string]any{"path": omni, "action": "extract", "entry": "defaults/pref/gorilla.js"})
	if err != nil || r.IsError {
		t.Fatalf("granted extraction: %v %s", err, r.Content)
	}
	got, _ := os.ReadFile(filepath.Join(out, "omni.ja", "defaults", "pref", "gorilla.js"))
	if string(got) != "pref(\"gorilla.fixture\", true);\n" {
		t.Errorf("extracted content = %q", got)
	}
	// Again: refused before asking, nothing overwritten.
	before := len(allow.calls)
	r, _ = forensicsCall(t, NewArchiveTool(allow), map[string]any{"path": omni, "action": "extract", "entry": "defaults/pref/gorilla.js"})
	if !r.IsError || !strings.Contains(r.Content, "already exists") || len(allow.calls) != before {
		t.Errorf("second extraction: %s (asked %d times)", r.Content, len(allow.calls)-before)
	}

	// Outside the project, or a name that climbs out: refused before asking.
	for _, params := range []map[string]any{
		{"path": omni, "action": "extract", "entry": "chrome.manifest", "dest": t.TempDir()},
		{"path": omni, "action": "extract", "entry": "chrome.manifest", "dest": filepath.Join("..", "escape")},
		{"path": omni, "action": "extract", "entry": "../chrome.manifest"},
		{"path": omni, "action": "extract"},
	} {
		perms := &recordingPermissions{grant: true}
		r, _ := forensicsCall(t, NewArchiveTool(perms), params)
		if !r.IsError || len(perms.calls) != 0 {
			t.Errorf("%v: not refused before asking: %s", params, r.Content)
		}
	}
}
