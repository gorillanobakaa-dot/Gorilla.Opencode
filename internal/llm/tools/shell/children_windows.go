//go:build windows

// GORILLA OVERRIDE (2026-09-01): enumerate a process's direct children on
// Windows, so an interrupt can kill what the shell started without killing the
// shell.
//
// Unix has pkill -P for this and the shell package used it. Windows has no
// equivalent one-liner: `taskkill /T` always includes the process you name, and
// the alternatives (wmic, which Microsoft has removed from recent Windows; a
// PowerShell Get-CimInstance, which costs a ~300ms process launch on the
// interrupt path) are each worse than asking the OS directly.
//
// CreateToolhelp32Snapshot is in the standard library and answers immediately.
package shell

import (
	"syscall"
	"unsafe"
)

// childPIDs returns the process IDs whose parent is ppid.
//
// Only direct children: taskkill /T then walks each one's own tree, so the
// grandchildren are still reached, and the shell — which is not in this list —
// is not.
func childPIDs(ppid int) []int {
	snapshot, err := syscall.CreateToolhelp32Snapshot(syscall.TH32CS_SNAPPROCESS, 0)
	if err != nil {
		return nil
	}
	defer syscall.CloseHandle(snapshot)

	var entry syscall.ProcessEntry32
	entry.Size = uint32(unsafe.Sizeof(entry))

	if err := syscall.Process32First(snapshot, &entry); err != nil {
		return nil
	}
	var pids []int
	for {
		if int(entry.ParentProcessID) == ppid && int(entry.ProcessID) != ppid {
			pids = append(pids, int(entry.ProcessID))
		}
		if err := syscall.Process32Next(snapshot, &entry); err != nil {
			break // ERROR_NO_MORE_FILES: the walk is finished
		}
	}
	return pids
}
