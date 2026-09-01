//go:build !windows

package osutil

// SetWindowTitle is a no-op off Windows.
//
// Linux and macOS terminal emulators name the window themselves — from the
// shell, the profile, or an OSC escape the user's own prompt emits. Writing an
// OSC title sequence from here would fight that, and would also print escape
// bytes into any redirected output. Windows conhost has no such convention,
// which is why the Windows build sets one explicitly.
func SetWindowTitle(string) {}
