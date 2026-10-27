package shell

import (
	"syscall"
	"testing"
)

// forceOEMConsole sets this process's console output code page to 437 for
// one test and puts the old one back after. A shell started meanwhile
// inherits 437, as it would in a default Western Windows console. Without
// a console there is nothing to set, and a child then gets the ANSI code
// page, which is not UTF-8 either.
func forceOEMConsole(t *testing.T) {
	t.Helper()
	k := syscall.NewLazyDLL("kernel32.dll")
	get, set := k.NewProc("GetConsoleOutputCP"), k.NewProc("SetConsoleOutputCP")
	old, _, _ := get.Call()
	if old == 0 {
		return
	}
	if r, _, _ := set.Call(437); r == 0 {
		return
	}
	t.Cleanup(func() { set.Call(old) })
}
