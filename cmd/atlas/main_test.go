package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestTheRegistryIsValidData(t *testing.T) {
	r, _, err := load(repoRoot())
	if err != nil {
		t.Fatal(err)
	}
	problems, notes := checkRegistry(r)
	for _, p := range problems {
		t.Error(p)
	}
	t.Logf("%d sources, %d note(s) (no URL)", len(r.Sources), len(notes))
}

func TestTheCommittedAtlasIsWhatTheRegistryAndCurationRender(t *testing.T) {
	r, c, err := load(repoRoot())
	if err != nil {
		t.Fatal(err)
	}
	got, err := render(r, c)
	if err != nil {
		t.Fatal(err)
	}
	committed, err := os.ReadFile(filepath.Join(repoRoot(), "internal", "llm", "agent", "source-atlas.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(bytes.ReplaceAll(committed, []byte("\r\n"), []byte("\n")), []byte(got)) {
		t.Fatal("source-atlas.txt is not what cmd/atlas renders: go run ./cmd/atlas > internal/llm/agent/source-atlas.txt")
	}
}

// The grade on the page is the registry's, whatever the curation's author
// remembered it as.
func TestTheGradeAndReachComeFromTheRegistry(t *testing.T) {
	free := true
	r := registry{Sources: []source{{Name: "X", Access: "api_key", Category: "c", Free: &free, Reliability: "C", URL: "https://x.org/api/"}}}
	c := curation{Header: "H", Sections: []section{{Title: "S",
		Entries: []entry{{Name: "Shown as Y", Source: "x", Advice: "one\ntwo"}}}}}
	got, err := render(r, c)
	if err != nil {
		t.Fatal(err)
	}
	if want := "H\n\nS:\n  Shown as Y [C API-key] x.org/api — one\n    two\n"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
	c.Sections[0].Entries[0].Source = "missing"
	if _, err := render(r, c); err == nil || !strings.Contains(err.Error(), "no source") {
		t.Errorf("an entry the registry does not hold must fail, err=%v", err)
	}
}
