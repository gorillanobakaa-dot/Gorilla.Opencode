//go:build windows

// GORILLA OVERRIDE (2026-09-01): give the console window a name.
//
// Windows takes a console window's title from whatever launched it. Started
// through conhost — which is what the Desktop shortcut needs to do if the app is
// to own its own window and therefore its own taskbar icon — the title bar reads
//
//	C:\Windows\System32\conhost.exe
//
// which is the path of the host, not the name of the program. It is the first
// thing a person sees, it is in every screenshot they take, and it says nothing
// about what they are running.
//
// Nothing in the codebase ever set a title, because on Linux the terminal
// emulator supplies one from the shell and under Windows Terminal the tab is
// named from the process. conhost does neither.
package osutil

import (
	"syscall"
	"unsafe"
)

var (
	kernel32           = syscall.NewLazyDLL("kernel32.dll")
	procSetConsoleName = kernel32.NewProc("SetConsoleTitleW")
)

// SetWindowTitle names the console window this process is attached to.
//
// Best effort by design: there may be no console at all (a service, a redirected
// pipe, a GUI host), and failing to set a cosmetic title must never be worth
// reporting to the user, let alone failing a launch over.
func SetWindowTitle(title string) {
	p, err := syscall.UTF16PtrFromString(title)
	if err != nil {
		return
	}
	_, _, _ = procSetConsoleName.Call(uintptr(unsafe.Pointer(p)))
}
