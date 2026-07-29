package prompt

import (
	"os"
	"path/filepath"
	"testing"
)

// GORILLA OVERRIDE: this file did not exist upstream.
//
// system-prompts/current/ is published so anyone can read the instructions this
// program actually sends. That promise is only worth something if the published
// copy is the shipped copy. Before this guard the two were maintained by hand
// and had already drifted: system-prompts/ described coder-anthropic.md as "used
// by the Anthropic family" months after the constant behind it was deleted from
// coder.go, and the study copy of the live prompt was a prose draft that was
// never the thing being sent.
//
// A stale study copy is worse than none: it is documentation that reads as
// authoritative and is false. So the copies are byte-compared here, and CI fails
// rather than letting the repository publish a prompt it does not use.
func TestStudyCopiesMatchTheShippedPrompts(t *testing.T) {
	// This test file lives at internal/llm/prompt/, so the repo root is three up.
	root := filepath.Join("..", "..", "..")

	for _, tc := range []struct {
		embedded string // the //go:embed source, relative to this package
		study    string // the published copy, relative to the repo root
	}{
		{"coder-modern.txt", "system-prompts/current/coder-modern.md"},
		{"task.txt", "system-prompts/current/task.md"},
		{"summarizer.txt", "system-prompts/current/summarizer.md"},
		{"title.txt", "system-prompts/current/title.md"},
	} {
		t.Run(tc.study, func(t *testing.T) {
			shipped, err := os.ReadFile(tc.embedded)
			if err != nil {
				t.Fatalf("cannot read the embedded prompt %s: %v", tc.embedded, err)
			}
			published, err := os.ReadFile(filepath.Join(root, tc.study))
			if err != nil {
				t.Fatalf("cannot read the study copy %s: %v — every shipped prompt must have one", tc.study, err)
			}
			if string(shipped) != string(published) {
				t.Errorf("%s does not match %s.\n"+
					"The published copy is what users read to audit this program; it must be the\n"+
					"bytes actually sent. Re-copy it:\n\n    cp %s %s\n",
					tc.study, tc.embedded, filepath.Join("internal/llm/prompt", tc.embedded), tc.study)
			}
		})
	}
}
