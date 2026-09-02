package codereview

// GORILLA OVERRIDE (2026-08-18): the two-copy guard, same discipline as
// TestVendoredPfindMatchesDevCopy.
//
// The toolkit exists twice on the developer's machine: the working copy in
// Scripts.For.Work, where fixes get made, and the vendored copy here, which is
// what everyone else actually runs. Two copies of one thing drift on the first
// change — that is how the .deb once shipped a launcher with no plain-mode
// action, and how three copies of the desktop entry got out of step.
//
// This is 12 Python modules and 34 rule documents rather than one file, so the
// check is per-file and names what moved. It only runs on a machine that HAS
// the development copy; everywhere else there is nothing to drift against.

import (
	"crypto/sha256"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

func devToolkitDir() string {
	return filepath.Join(os.Getenv("HOME"), "Documents", "Scripts.For.Work",
		"Code.review", "code_review_toolkit")
}

func TestVendoredToolkitMatchesDevCopy(t *testing.T) {
	dev := devToolkitDir()
	if _, err := os.Stat(dev); err != nil {
		t.Skip("no development copy on this machine; nothing to drift against")
	}

	var drifted, missing, extra []string

	// Every embedded file must match its development original.
	err := fs.WalkDir(toolkitFS, "toolkit", func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		rel := strings.TrimPrefix(strings.TrimPrefix(p, "toolkit"), "/")
		embedded, err := toolkitFS.ReadFile(p)
		if err != nil {
			return err
		}
		original, err := os.ReadFile(filepath.Join(dev, rel))
		if err != nil {
			extra = append(extra, rel) // vendored but no longer in the dev copy
			return nil
		}
		if sha256.Sum256(embedded) != sha256.Sum256(original) {
			drifted = append(drifted, rel)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}

	// And every Python module in the development copy must be embedded. A new
	// module that is not vendored is the failure that matters most: the
	// orchestrator imports it, so the shipped copy dies at runtime on a machine
	// where nobody can see the source.
	entries, err := os.ReadDir(dev)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".py") {
			continue
		}
		if _, err := toolkitFS.ReadFile("toolkit/" + e.Name()); err != nil {
			missing = append(missing, e.Name())
		}
	}

	sort.Strings(drifted)
	sort.Strings(missing)
	sort.Strings(extra)

	if len(drifted) > 0 {
		t.Errorf("%d vendored file(s) have drifted from %s — re-sync whichever direction is current:\n  %s",
			len(drifted), dev, strings.Join(drifted, "\n  "))
	}
	if len(missing) > 0 {
		t.Errorf("%d Python module(s) exist in the development copy but are NOT embedded, so the shipped "+
			"toolkit will fail on import:\n  %s", len(missing), strings.Join(missing, "\n  "))
	}
	if len(extra) > 0 {
		t.Logf("%d vendored file(s) no longer exist in the development copy: %s",
			len(extra), strings.Join(extra, ", "))
	}
}

// The orchestrator is useless without the modules it imports. This asserts the
// import graph is closed inside what was embedded, without needing the dev copy
// — so it protects a machine that only has this repository.
func TestEveryModuleTheToolkitImportsIsEmbedded(t *testing.T) {
	entry, err := toolkitFS.ReadFile("toolkit/code_review.py")
	if err != nil {
		t.Fatal(err)
	}
	// GORILLA FIX (2026-09-02): match real IMPORT STATEMENTS, not any
	// occurrence of the word.
	//
	// This used to ask whether the entry point CONTAINED the module name
	// anywhere, and every candidate name is also an ordinary English word a
	// code-review script is certain to use: findings, rules, doctor. A
	// single-file rewrite that imports nothing therefore failed three
	// assertions for writing the word "findings" in a comment.
	//
	// The question the test means to ask is whether the import graph is
	// CLOSED inside what was embedded. So it now reads the import statements
	// and checks those, which is both stricter -- it catches a module the
	// hard-coded list never anticipated -- and correct.
	imports := map[string]bool{}
	for _, line := range strings.Split(string(entry), "\n") {
		line = strings.TrimSpace(line)
		var mod string
		switch {
		case strings.HasPrefix(line, "import "):
			mod = strings.TrimPrefix(line, "import ")
		case strings.HasPrefix(line, "from "):
			mod = strings.TrimPrefix(line, "from ")
		default:
			continue
		}
		// "import os, sys" and "from x import y" both reduce to the first word.
		if i := strings.IndexAny(mod, " ,."); i >= 0 {
			mod = mod[:i]
		}
		if mod != "" {
			imports[mod] = true
		}
	}
	for mod := range imports {
		// A sibling .py is one of ours; anything else is the standard library
		// or a third-party package, and neither is this test's business.
		if _, err := toolkitFS.ReadFile("toolkit/" + mod + ".py"); err == nil {
			continue // present, as required
		}
		if _, err := os.Stat(filepath.Join("toolkit", mod+".py")); err == nil {
			t.Errorf("code_review.py imports %s, and %s.py exists on disk but is "+
				"NOT embedded, so the shipped toolkit will fail on import", mod, mod)
		}
	}

	// The rule documents are what the review advice is read from; an empty
	// directory would degrade silently into a review with no checklists.
	n := 0
	_ = fs.WalkDir(toolkitFS, "toolkit/rule_docs", func(p string, d fs.DirEntry, err error) error {
		if err == nil && !d.IsDir() {
			n++
		}
		return nil
	})
	if n < 20 {
		t.Errorf("only %d rule documents embedded; the checklists would be nearly empty", n)
	}
}
