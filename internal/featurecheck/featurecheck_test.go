package featurecheck

import (
	"os"
	"path/filepath"
	"testing"
)

func write(t *testing.T, root, name, body string) {
	t.Helper()
	p := filepath.Join(root, name)
	os.MkdirAll(filepath.Dir(p), 0o755)
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestStaleModelFails(t *testing.T) {
	root := t.TempDir()
	write(t, root, "docs/A.md", "use claude-opus-5-5 and gpt-oss-120b here")
	got, err := Check(Input{Root: root, Catalogue: map[string]bool{"claude-opus-5-5-high": true}})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Kind != "stale-model" || got[0].Detail != "gpt-oss-120b is not in the catalogue" {
		t.Fatalf("got %v", got)
	}
}

func TestCleanTreePasses(t *testing.T) {
	root := t.TempDir()
	write(t, root, "docs/A.md", "claude-opus-5-5 is fine; claude-code is prose")
	got, _ := Check(Input{Root: root, Catalogue: map[string]bool{"claude-opus-5-5-high": true}})
	if len(got) != 0 {
		t.Fatalf("got %v", got)
	}
}

func TestNewToolMustBeEverywhere(t *testing.T) {
	root := t.TempDir()
	write(t, root, "docs/COMMANDS.md", "checklist tool")
	write(t, root, "internal/helppages/h.go", "nothing")
	got, _ := Check(Input{Root: root, NewTools: []string{"checklist"}})
	var missing int
	for _, f := range got {
		if f.Kind == "missing-tool" {
			missing++
		}
	}
	// help page lacks it; arsenal + release notes surfaces not found
	if missing != 3 {
		t.Fatalf("want 3 missing, got %d: %v", missing, got)
	}
}

func TestSavedConfigPointingAtRemovedModel(t *testing.T) {
	got, _ := Check(Input{Root: t.TempDir(), Catalogue: map[string]bool{"a-1": true},
		Saved: map[string]string{"coder": "gpt-oss-120b", "task": "a-1"}})
	if len(got) != 1 || got[0].File != "coder" {
		t.Fatalf("got %v", got)
	}
}
