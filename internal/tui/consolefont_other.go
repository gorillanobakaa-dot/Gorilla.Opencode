//go:build !windows && !linux

package tui

// EnsureConsoleFont is a no-op on platforms where the terminal emulator owns
// font selection exclusively.
func EnsureConsoleFont() string { return "" }
