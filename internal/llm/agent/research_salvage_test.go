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
	"runtime"
	"strings"
	"testing"
)

// tempHome redirects os.UserHomeDir at a temporary directory for one test.
//
// GORILLA OVERRIDE (2026-09-01): set the variable this PLATFORM reads.
//
// It set only HOME. os.UserHomeDir reads USERPROFILE on Windows and ignores
// HOME entirely, so the redirection silently did nothing there — and the guard
// below, which exists precisely to stop a test writing into somebody's real
// home directory, correctly refused to run. The guard was right; the helper was
// Unix-shaped. Every research-recovery test in this package was blocked by it.
func tempHome(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	if runtime.GOOS == "windows" {
		t.Setenv("USERPROFILE", home)
	} else {
		t.Setenv("HOME", home)
	}
	// Kept as a hard failure rather than a skip: a test that writes findings
	// files into a real home directory is worse than a test that does not run.
	if got, _ := os.UserHomeDir(); got != home {
		t.Fatalf("home redirection failed (got %q) — refusing to run a test that would write to the real home", got)
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

	// GORILLA OVERRIDE (2026-09-01): verify the premise before asserting on it.
	//
	// os.Chmod SUCCEEDS on Windows and changes nothing that matters: the only
	// bit it can set is read-only, and a read-only directory is still writable
	// there. So the chmod above returned nil, the directory stayed writable, the
	// save succeeded, and the test failed while reporting that the code was
	// wrong. It was not — the test could not build the situation it was testing.
	//
	// Probing is better than a blanket GOOS skip: if a future Windows Go release
	// makes Chmod meaningful, or the test runs somewhere it does bite, the test
	// resumes doing its job on its own.
	probe := filepath.Join(home, ".writable-probe")
	if err := os.WriteFile(probe, []byte("x"), 0o600); err == nil {
		os.Remove(probe)
		t.Skip("this platform's Chmod cannot make a directory unwritable, so the " +
			"disk-refuses path cannot be exercised here")
	}

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
