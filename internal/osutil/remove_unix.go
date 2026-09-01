//go:build !windows

package osutil

import "os"

// Remove delegates to os.Remove directly on Unix.
func Remove(name string) error {
	return os.Remove(name)
}

// RemoveAll delegates to os.RemoveAll directly on Unix.
func RemoveAll(path string) error {
	return os.RemoveAll(path)
}
