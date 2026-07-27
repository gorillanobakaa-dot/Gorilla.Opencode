package tools

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/opencode-ai/opencode/internal/config"
)

// runSparse drives the tool the way the agent does, with a session id in the
// context and no permission service (nil = auto-grant, so the test does not
// block on a UI prompt that will never come).
func runSparse(t *testing.T, params SparseParams) ToolResponse {
	t.Helper()
	in, err := json.Marshal(params)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.WithValue(context.Background(), SessionIDContextKey, "test-session")
	resp, err := NewSparseTool(nil).Run(ctx, ToolCall{Input: string(in)})
	if err != nil {
		t.Fatalf("Run returned a hard error: %v", err)
	}
	return resp
}

func TestSparseToolInfoIsWellFormed(t *testing.T) {
	info := NewSparseTool(nil).Info()
	if info.Name != SparseToolName {
		t.Errorf("Name = %q, want %q", info.Name, SparseToolName)
	}
	if !strings.Contains(info.Description, "__user") {
		t.Error("description does not mention __user — the model needs to know what sparse uniquely catches, or it will never reach for this tool")
	}
	if len(info.Required) != 1 || info.Required[0] != "file_path" {
		t.Errorf("Required = %v, want [file_path]", info.Required)
	}
	if _, ok := info.Parameters["file_path"]; !ok {
		t.Error("file_path missing from the schema")
	}
}

func TestSparseRejectsBadInput(t *testing.T) {
	if _, err := config.Load(t.TempDir(), false); err != nil {
		t.Fatalf("config.Load: %v", err)
	}

	t.Run("empty file_path", func(t *testing.T) {
		resp := runSparse(t, SparseParams{})
		if !resp.IsError {
			t.Error("empty file_path should be an error response")
		}
	})

	t.Run("nonexistent file", func(t *testing.T) {
		resp := runSparse(t, SparseParams{FilePath: filepath.Join(t.TempDir(), "nope.c")})
		if !resp.IsError {
			t.Error("nonexistent file should be an error response")
		}
		if !strings.Contains(resp.Content, "cannot read") {
			t.Errorf("error should name the read failure, got: %s", resp.Content)
		}
	})

	t.Run("malformed json", func(t *testing.T) {
		ctx := context.WithValue(context.Background(), SessionIDContextKey, "s")
		resp, err := NewSparseTool(nil).Run(ctx, ToolCall{Input: "{not json"})
		if err != nil {
			t.Fatalf("malformed input should be a soft error, got hard error: %v", err)
		}
		if !resp.IsError {
			t.Error("malformed json should be an error response")
		}
	})
}

// A missing session id must fail loudly rather than silently skipping the
// permission gate.
func TestSparseRequiresSessionID(t *testing.T) {
	if _, err := config.Load(t.TempDir(), false); err != nil {
		t.Fatalf("config.Load: %v", err)
	}
	dir := t.TempDir()
	f := filepath.Join(dir, "x.c")
	if err := writeFile(f, "int main(void){return 0;}\n"); err != nil {
		t.Fatal(err)
	}
	in, _ := json.Marshal(SparseParams{FilePath: f})
	// No SessionIDContextKey in the context.
	_, err := NewSparseTool(nil).Run(context.Background(), ToolCall{Input: string(in)})
	if err == nil {
		t.Error("missing session id should be a hard error — the permission gate would otherwise be skipped")
	}
}

// The real thing: sparse must actually flag a kernel-convention violation that
// a C compiler accepts. This is the whole justification for the tool.
func TestSparseFlagsAddressSpaceViolation(t *testing.T) {
	if _, err := exec.LookPath("sparse"); err != nil {
		t.Skip("sparse not installed; skipping live check")
	}
	if _, err := config.Load(t.TempDir(), false); err != nil {
		t.Fatalf("config.Load: %v", err)
	}

	dir := t.TempDir()
	// Dereferencing a __user pointer directly. gcc/clang compile this happily.
	// sparse must complain about the address space.
	src := `
#define __user __attribute__((noderef, address_space(__user)))
int read_it(int __user *p) { return *p; }
`
	f := filepath.Join(dir, "addrspace.c")
	if err := writeFile(f, src); err != nil {
		t.Fatal(err)
	}

	resp := runSparse(t, SparseParams{FilePath: f})
	if resp.IsError {
		t.Fatalf("sparse run failed: %s", resp.Content)
	}

	low := strings.ToLower(resp.Content)
	if !strings.Contains(low, "address space") && !strings.Contains(low, "dereference") {
		t.Errorf("sparse did not flag the __user dereference — the tool is not doing the one job it exists for.\nOutput:\n%s", resp.Content)
	}

	var meta SparseResponseMetadata
	if resp.Metadata != "" {
		if err := json.Unmarshal([]byte(resp.Metadata), &meta); err != nil {
			t.Fatalf("metadata is not valid json: %v", err)
		}
	}
	if meta.Clean {
		t.Error("metadata says clean, but sparse reported findings")
	}
	if meta.Warnings == 0 && meta.Errors == 0 {
		t.Errorf("metadata counted 0 findings; content was:\n%s", resp.Content)
	}
}

// A clean file must report clean, with Clean=true in metadata — otherwise the
// agent cannot tell "checked and fine" from "did not run".
func TestSparseReportsCleanFile(t *testing.T) {
	if _, err := exec.LookPath("sparse"); err != nil {
		t.Skip("sparse not installed")
	}
	if _, err := config.Load(t.TempDir(), false); err != nil {
		t.Fatalf("config.Load: %v", err)
	}
	dir := t.TempDir()
	f := filepath.Join(dir, "clean.c")
	if err := writeFile(f, "static int add(int a, int b) { return a + b; }\n"); err != nil {
		t.Fatal(err)
	}
	resp := runSparse(t, SparseParams{FilePath: f})
	if resp.IsError {
		t.Fatalf("clean file produced an error: %s", resp.Content)
	}
	var meta SparseResponseMetadata
	_ = json.Unmarshal([]byte(resp.Metadata), &meta)
	if !meta.Clean {
		t.Errorf("clean file not reported clean. content:\n%s\nmetadata: %s", resp.Content, resp.Metadata)
	}
}

func writeFile(path, content string) error {
	return os.WriteFile(path, []byte(content), 0o644)
}
