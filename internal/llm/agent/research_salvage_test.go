package agent

// GORILLA OVERRIDE (2026-08-18): the safety net has to be exercised, because
// the thing it protects against is exactly the case where nobody is watching.
//
// HOME is redirected to a temp directory for every test here. config.DossierDir
// resolves from the user's real home, so without this these tests would write
// into the owner's actual ~/Documents — the same class of damage that put a
// stray key in his live config earlier today.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func tempHome(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	if got, _ := os.UserHomeDir(); got != home {
		t.Fatalf("HOME redirection failed (got %q) — refusing to run a test that would write to the real home", got)
	}
	return home
}

func TestFindingsAreSavedBeforeAnyModelTouchesThem(t *testing.T) {
	tempHome(t)

	roles := []researchRole{
		{ID: "local", Title: "LOCAL — what already exists here"},
		{ID: "prior_art", Title: "PRIOR ART — has someone already solved this"},
		{ID: "history", Title: "HISTORY — how did it get this way"},
	}
	replies := []string{
		"## ANSWER\nFound the note.\n\n## FINDINGS\n- CLAIM: x | EVIDENCE: file:1 | GRADE: A2",
		"## ANSWER\nSomeone did.\n\n## FINDINGS\n- CLAIM: y | EVIDENCE: url | GRADE: B3",
		"", // the lane that died — this is the real case from 2026-08-17
	}
	audits := []string{"APPROVED", "WEAK — one source", ""}

	path := writeRawFindings("who is Kelexine?", roles, replies, audits, "dossier")
	if path == "" {
		t.Fatal("nothing was saved; an expensive run would be recoverable only from the session store")
	}
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("the reported path does not exist: %v", err)
	}
	s := string(body)

	// Every lane's graded material must be present, verbatim.
	for _, want := range []string{"GRADE: A2", "GRADE: B3", "Someone did", "WEAK — one source"} {
		if !strings.Contains(s, want) {
			t.Errorf("saved findings lost %q", want)
		}
	}
	// A dead lane must be reported as uncovered, never silently omitted — the
	// gap has to be visible in the artifact itself.
	if !strings.Contains(s, "LANE UNCOVERED") {
		t.Errorf("the lane that produced nothing is not marked uncovered")
	}
	if !strings.Contains(s, "2 of 3 lanes produced findings") {
		t.Errorf("the coverage count is missing or wrong")
	}
	// It must not masquerade as the finished product.
	if !strings.Contains(s, "not the finished assessment") {
		t.Errorf("raw findings do not say what they are; someone could mistake them for the dossier")
	}
	if !strings.Contains(s, "/osint --recover") {
		t.Errorf("the file does not tell the reader how to turn it into a dossier")
	}
	// And it lands in the dossier directory, never the working folder.
	if !strings.Contains(filepath.Dir(path), "Gorilla-OSINT-Dossiers") {
		t.Errorf("findings written outside the dossier directory: %s", path)
	}
}

// With no usable Documents directory the findings still land somewhere — in
// the home directory itself. Written after the first version of this test
// assumed a failure and discovered the fallback in config.DossierDir working
// as designed: a blocked Documents is degraded around, not fatal.
func TestFallsBackToHomeWhenDocumentsIsUnusable(t *testing.T) {
	home := tempHome(t)
	if err := os.WriteFile(filepath.Join(home, "Documents"), []byte("not a directory"), 0o644); err != nil {
		t.Fatal(err)
	}
	got := writeRawFindings("q", []researchRole{{ID: "local", Title: "LOCAL"}}, []string{"x"}, nil, "dossier")
	if got == "" {
		t.Fatal("nothing saved; the run's findings would exist only in the session store")
	}
	// Compare the parent directory exactly. A substring check on the full path
	// is not safe here: t.TempDir() names the directory after the test, and
	// this test's name contains the word "Documents".
	if want := filepath.Join(home, "Gorilla-OSINT-Dossiers"); filepath.Dir(got) != want {
		t.Errorf("expected the fallback at %q, got %q", want, filepath.Dir(got))
	}
}

// A genuinely unwritable destination must not turn a successful research run
// into a failed one: it returns no path, logs, and the caller carries on.
func TestSaveFailureIsSurvivable(t *testing.T) {
	home := tempHome(t)
	if err := os.Chmod(home, 0o500); err != nil { // read+execute, no write
		t.Skipf("cannot make the home unwritable here: %v", err)
	}
	t.Cleanup(func() { _ = os.Chmod(home, 0o700) })

	if got := writeRawFindings("q", []researchRole{{ID: "local", Title: "LOCAL"}}, []string{"x"}, nil, "dossier"); got != "" {
		t.Errorf("expected an empty path when the disk refuses, got %q", got)
	}
	// Reaching this line at all is the assertion: no panic, no error escaping
	// into a research run that actually succeeded.
}

func TestSlugifyKeepsFilenamesUsable(t *testing.T) {
	for _, c := range []struct{ in, want string }{
		{"who is Kelexine?", "who-is-kelexine"},
		{"Does /osint work on a 9 KB/s link?", "does-osint-work-on"},
		{"???", "research"},
		{"", "research"},
	} {
		if got := slugify(c.in); got != c.want {
			t.Errorf("slugify(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}
