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
	// The toolkit's own modules are imported by bare name; stdlib is not our
	// problem. Anything that has a matching .py beside it must be present.
	for _, name := range []string{
		"tools_registry", "findings", "heuristics", "rules",
		"baseline", "doctor", "local_tier", "llm_client",
	} {
		if !strings.Contains(string(entry), name) {
			continue // not imported by the entry point; nothing to assert
		}
		if _, err := toolkitFS.ReadFile("toolkit/" + name + ".py"); err != nil {
			t.Errorf("code_review.py refers to %s but %s.py is not embedded", name, name)
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
