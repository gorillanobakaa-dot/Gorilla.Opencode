package tools

// GORILLA FIX (2026-08-19): every permission request must scope its grant.
//
// The 2026-08-18 audit added GrantKey so that "Allow for session" covers the
// thing the user was SHOWN rather than the whole tool. That fix was applied to
// bash, edit, patch and write — and MISSED fetch, websearch, review and sparse.
// A follow-up audit found the gap the next morning. fetch was the serious one:
// approving one URL authorised every later URL in the session, which is an
// exfiltration path, because a poisoned page can steer the model to a URL of its
// choosing and the user is never asked again.
//
// Fixing the four is not enough — the same omission will happen with the next
// tool someone adds. This test walks the SOURCE and fails if any permission
// request in the package forgets its key.

import (
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

var (
	reRequest  = regexp.MustCompile(`CreatePermissionRequest\{`)
	reGrantKey = regexp.MustCompile(`GrantKey:`)
)

func TestEveryPermissionRequestSetsAGrantKey(t *testing.T) {
	// Walk the WHOLE repository, not this package.
	//
	// The first version of this test globbed "*.go" here and passed — while
	// internal/llm/agent/mcp-tools.go:93 still had an unkeyed request. The guard
	// had the same blind spot as the bug it was written to catch: it only looked
	// where someone had already looked. A structural test that is scoped to the
	// place you happened to be standing is not structural.
	var files []string
	err := filepath.WalkDir("../../..", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() && (d.Name() == ".git" || d.Name() == "vendor" || d.Name() == "node_modules") {
			return fs.SkipDir
		}
		if !d.IsDir() && strings.HasSuffix(path, ".go") {
			files = append(files, path)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}

	checked := 0
	for _, f := range files {
		if strings.HasSuffix(f, "_test.go") {
			continue
		}
		src, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		requests := len(reRequest.FindAll(src, -1))
		if requests == 0 {
			continue
		}
		checked++
		keys := len(reGrantKey.FindAll(src, -1))
		if keys < requests {
			t.Errorf("%s: %d permission request(s) but only %d GrantKey — an unkeyed "+
				"request means \"allow for session\" grants the WHOLE TOOL, not the thing "+
				"the user was shown", f, requests, keys)
		}
	}

	if checked == 0 {
		t.Fatal("no permission requests found at all — this test is vacuous, " +
			"check the regex still matches the code")
	}
	t.Logf("checked %d files containing permission requests", checked)
}
