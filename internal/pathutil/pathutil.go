// Package pathutil answers the two questions every guard in this program asks
// about a path: "is this the same place as that" and "is this inside that".
//
// GORILLA FIX (2026-10-27): they used to be answered by comparing strings, one
// way per package (filepath.Rel, HasPrefix, ==), and a path has more than one
// spelling. On the windows-latest CI runner TEMP is C:\Users\RUNNER~1\... (an
// 8.3 short name) while the profile is C:\Users\runneradmin, so a file in the
// temp folder was "outside the user profile", the embedded helpers refused to
// run from their own cache, a project's own environment file was refused as
// "not part of a project", and a workspace was not the folder it had been
// started in. A user whose TEMP or project path is spelled short hits the same
// refusals. One helper, used by every one of those checks, spells both sides
// the same way first.
package pathutil

import (
	"path/filepath"
	"runtime"
	"strings"
)

// Long returns p absolute and cleaned, with every 8.3 short component
// (RUNNER~1) expanded to its long name on Windows. Links and junctions are NOT
// followed, so it changes the spelling of a path, never where it points. The
// part of p that does not exist yet is kept as written. Elsewhere it is
// filepath.Abs.
func Long(p string) string {
	abs, err := filepath.Abs(p)
	if err != nil {
		abs = filepath.Clean(p)
	}
	return expandExisting(abs, longName)
}

// Canonical returns where p actually is: absolute, every symlink and junction
// in its longest existing prefix followed, short names expanded. The part
// that does not exist yet is re-attached as written, because that is where a
// file about to be created will land.
func Canonical(p string) string {
	return expandExisting(Long(p), realPath)
}

// Same reports whether a and b name the same place once both are made
// Canonical. Case-insensitive on Windows, where the file system is.
func Same(a, b string) bool {
	return equal(Canonical(a), Canonical(b))
}

// Within reports whether target is dir itself or lies beneath it, after both
// are made Canonical: a link inside dir that points elsewhere is outside, and
// a short or linked spelling of a folder inside dir is inside. For the checks
// that guard a location (inside the user profile, inside the private cache).
func Within(dir, target string) bool {
	if dir == "" || target == "" {
		return false
	}
	return within(Canonical(dir), Canonical(target))
}

// WithinLexical is Within without following links: both sides are only
// respelled with Long. For callers whose paths are already resolved, or whose
// question is about the path as written (a workspace root, a permission
// scope).
func WithinLexical(dir, target string) bool {
	if dir == "" || target == "" {
		return false
	}
	return within(Long(dir), Long(target))
}

func within(dir, target string) bool {
	if equal(dir, target) {
		return true
	}
	rel, err := filepath.Rel(dir, target)
	if err != nil || filepath.IsAbs(rel) {
		return false
	}
	return rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

// Equal compares two spellings as the file system does: cleaned, and
// case-insensitive on Windows. It does not respell or resolve either side.
func Equal(a, b string) bool {
	return equal(filepath.Clean(a), filepath.Clean(b))
}

func equal(a, b string) bool {
	if runtime.GOOS == "windows" {
		return strings.EqualFold(a, b)
	}
	return a == b
}

// expandExisting applies f to the longest prefix of abs that f accepts (in
// practice, that exists) and re-attaches the rest.
func expandExisting(abs string, f func(string) (string, bool)) string {
	head, tail := abs, ""
	for {
		if r, ok := f(head); ok {
			if tail == "" {
				return r
			}
			return filepath.Join(r, tail)
		}
		parent := filepath.Dir(head)
		if parent == head {
			return abs
		}
		if tail == "" {
			tail = filepath.Base(head)
		} else {
			tail = filepath.Join(filepath.Base(head), tail)
		}
		head = parent
	}
}
