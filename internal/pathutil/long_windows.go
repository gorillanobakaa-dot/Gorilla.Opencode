//go:build windows

package pathutil

import (
	"strings"

	"golang.org/x/sys/windows"
)

// longName asks Windows for the long spelling of an existing path. It fails
// for a path that does not exist, which expandExisting relies on.
func longName(p string) (string, bool) {
	u, err := windows.UTF16PtrFromString(p)
	if err != nil {
		return "", false
	}
	buf := make([]uint16, 260)
	for {
		n, err := windows.GetLongPathName(u, &buf[0], uint32(len(buf)))
		if err != nil || n == 0 {
			return "", false
		}
		if int(n) < len(buf) {
			return windows.UTF16ToString(buf[:n]), true
		}
		buf = make([]uint16, n) // too small: n is the size it needs
	}
}

// realPath is where an existing path actually is, as Windows itself resolves
// it when the file is opened: symlinks AND junctions followed, short names
// expanded. filepath.EvalSymlinks is not enough here: since Go 1.23 it no
// longer follows junctions (mount points), and a junction needs no privilege
// to make, so any user can make one.
func realPath(p string) (string, bool) {
	u, err := windows.UTF16PtrFromString(p)
	if err != nil {
		return "", false
	}
	// FILE_FLAG_BACKUP_SEMANTICS opens directories too; no access rights are
	// asked for, so this works on folders the caller cannot read.
	h, err := windows.CreateFile(u, 0,
		windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE,
		nil, windows.OPEN_EXISTING, windows.FILE_FLAG_BACKUP_SEMANTICS, 0)
	if err != nil {
		return "", false
	}
	defer windows.CloseHandle(h)
	buf := make([]uint16, 260)
	for {
		n, err := windows.GetFinalPathNameByHandle(h, &buf[0], uint32(len(buf)), 0 /* VOLUME_NAME_DOS */)
		if err != nil || n == 0 {
			return "", false
		}
		if int(n) < len(buf) {
			s := windows.UTF16ToString(buf[:n])
			switch {
			case strings.HasPrefix(s, `\\?\UNC\`):
				s = `\\` + s[len(`\\?\UNC\`):]
			case strings.HasPrefix(s, `\\?\`):
				s = s[len(`\\?\`):]
			}
			return s, true
		}
		buf = make([]uint16, n)
	}
}
