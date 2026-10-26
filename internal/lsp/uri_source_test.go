package lsp

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestURINoHandBuiltFileURIs: outside the protocol package nothing in
// internal/lsp builds or strips "file://" by hand; that produced
// "file://C:\x" on Windows. Conversions go through protocol.URIFromPath
// and DocumentUri.Path.
func TestURINoHandBuiltFileURIs(t *testing.T) {
	err := filepath.WalkDir(".", func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() && d.Name() == "protocol" {
			return filepath.SkipDir
		}
		if d.IsDir() || !strings.HasSuffix(p, ".go") || strings.HasSuffix(p, "_test.go") {
			return nil
		}
		b, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		for i, line := range strings.Split(string(b), "\n") {
			if strings.Contains(line, `"file://`) && !strings.Contains(line, "HasPrefix") {
				t.Errorf("%s:%d builds or strips a file URI by hand: %s", p, i+1, strings.TrimSpace(line))
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
