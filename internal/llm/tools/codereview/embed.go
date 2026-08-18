// Package codereview vendors the local code-review toolkit and unpacks it on
// demand.
//
// GORILLA OVERRIDE (2026-08-18): the toolkit is a real, finished piece of
// software that lived outside this repository, in `Scripts.For.Work/`, which is
// deliberately untracked. That meant it could not ship, and a review capability
// that only exists on the developer's own machine is not a capability.
//
// WHY EMBED RATHER THAN DEPEND. The audience is on connections measured in
// single-digit KB/s. "Install this other thing first" is a wall, not a step —
// the same reasoning that put `lynx` in Depends rather than Recommends, and the
// same reasoning that embedded pfind. 444 KB against a 19 MB package is about
// fifty seconds on an 8 KB/s line, and it buys a capability that works the
// moment the program does.
//
// WHAT IT STILL NEEDS, and this is the honest part: the toolkit does not review
// code by itself. It drives around thirty real analysers — cppcheck,
// clang-tidy, bandit, semgrep, gitleaks, clippy, golangci-lint, gosec,
// shellcheck — and those are not embedded and never will be. What ships here is
// the ORCHESTRATOR: the thing that knows which analyser to run on what, how to
// normalise thirty different output formats into one shape, how to verify that
// a reported line actually says what the tool claims, and — most importantly —
// how to report what did NOT run.
//
// That last part is why this is worth shipping at all. A report full of
// "MISSING" looks exactly like a report that found no problems, and a small
// model cannot tell those apart. The toolkit's `trust` block can.
package codereview

import (
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

//go:embed all:toolkit
var toolkitFS embed.FS

// Version is a content hash of everything embedded. It names the unpack
// directory, so upgrading the binary unpacks a fresh copy beside the old one
// rather than mixing versions — a half-updated Python package is a class of bug
// nobody should have to debug from a bug report.
func Version() (string, error) {
	var paths []string
	err := fs.WalkDir(toolkitFS, "toolkit", func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() {
			paths = append(paths, p)
		}
		return nil
	})
	if err != nil {
		return "", err
	}
	sort.Strings(paths) // WalkDir is ordered, but do not depend on it

	h := sha256.New()
	for _, p := range paths {
		b, err := toolkitFS.ReadFile(p)
		if err != nil {
			return "", err
		}
		fmt.Fprintf(h, "%s\x00%d\x00", p, len(b))
		h.Write(b)
	}
	return hex.EncodeToString(h.Sum(nil))[:12], nil
}

// Unpack writes the toolkit under dir and returns the path to code_review.py.
//
// It is idempotent and cheap on the common path: if the version directory
// already holds a complete copy, nothing is written. A partial unpack — killed
// mid-write, out of disk — is detected by a marker file that is written LAST,
// so an interrupted extraction is redone rather than silently used.
func Unpack(baseDir string) (string, error) {
	version, err := Version()
	if err != nil {
		return "", err
	}
	root := filepath.Join(baseDir, "code-review-toolkit", version)
	done := filepath.Join(root, ".complete")

	if _, err := os.Stat(done); err == nil {
		return filepath.Join(root, "code_review.py"), nil
	}

	if err := os.MkdirAll(root, 0o755); err != nil {
		return "", fmt.Errorf("cannot create %s: %w", root, err)
	}

	err = fs.WalkDir(toolkitFS, "toolkit", func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel := strings.TrimPrefix(p, "toolkit")
		rel = strings.TrimPrefix(rel, "/")
		if rel == "" {
			return nil
		}
		dst := filepath.Join(root, rel)
		if d.IsDir() {
			return os.MkdirAll(dst, 0o755)
		}
		b, err := toolkitFS.ReadFile(p)
		if err != nil {
			return err
		}
		return os.WriteFile(dst, b, 0o644)
	})
	if err != nil {
		return "", fmt.Errorf("unpacking the review toolkit: %w", err)
	}

	// Written last, on purpose: its presence is the only claim that the
	// directory is complete.
	if err := os.WriteFile(done, []byte(version+"\n"), 0o644); err != nil {
		return "", err
	}
	return filepath.Join(root, "code_review.py"), nil
}
