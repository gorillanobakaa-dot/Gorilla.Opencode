//go:build !windows

package pathutil

import (
	"os"
	"path/filepath"
)

// longName: only Windows has 8.3 short names. An existing path is already
// spelled the one way it has.
func longName(p string) (string, bool) {
	if _, err := os.Lstat(p); err != nil {
		return "", false
	}
	return p, true
}

// realPath is an existing path with every symlink followed.
func realPath(p string) (string, bool) {
	r, err := filepath.EvalSymlinks(p)
	return r, err == nil
}
