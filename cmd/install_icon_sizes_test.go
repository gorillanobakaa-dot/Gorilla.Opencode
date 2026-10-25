package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/opencode-ai/opencode/internal/assets"
)

// The installer said "(256/128/64/48/32/24/16)", typed by hand, after the
// v0.1.152 icon brought 21 sizes. It now reads them from the file it wrote.
func TestTheInstallerReportsTheSizesTheShippedIconHas(t *testing.T) {
	data, err := assets.Icons.ReadFile("icons/" + appBinName + ".ico")
	if err != nil {
		t.Fatal(err)
	}
	p := filepath.Join(t.TempDir(), "x.ico")
	if err := os.WriteFile(p, data, 0o644); err != nil {
		t.Fatal(err)
	}
	s := icoSizes(p)
	if len(s) < 13 || s[0] != 16 || s[len(s)-1] != 256 {
		t.Fatalf("shipped .ico sizes %v: want 16..256 with every size Windows asks for", s)
	}
	if got := icoSizesLabel(p); !strings.Contains(got, "16-256") || strings.Contains(got, "/") {
		t.Errorf("label %q", got)
	}
	if got := icoSizesLabel(filepath.Join(t.TempDir(), "missing.ico")); got != "(sizes unreadable)" {
		t.Errorf("missing file label %q", got)
	}
}
