//go:build !windows

package tools

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"runtime"
	"strconv"
	"strings"
	"syscall"
)

// No robocopy here: the Go engine plans and runs every tree action.
func previewSync(op syncOp) (syncPlan, error) { return goPlan(op) }
func runSync(op syncOp) (syncPlan, error)     { return goRun(op) }

// archiverPath is bsdtar for zip with a password, 7z and tar.xz; for tar.xz a
// GNU tar does when no bsdtar is installed (gnu=true).
func archiverPath(format string) (path string, gnu bool, err error) {
	if p, err := exec.LookPath("bsdtar"); err == nil {
		return p, false, nil
	}
	if p, err := exec.LookPath("tar"); err == nil && format == "tar.xz" {
		return p, true, nil
	}
	return "", false, fmt.Errorf("%s needs bsdtar (Debian/Ubuntu: libarchive-tools); only zip without a password works without it", format)
}

func hideConsole(*exec.Cmd) {}

// Outside Windows only ReadOnly maps onto the file (its write bits); Hidden is
// read from a leading dot and cannot be set.
const settableAttrs = attrReadOnly

func readAttrs(path string) (uint32, error) {
	fi, err := os.Lstat(path)
	if err != nil {
		return 0, err
	}
	var a uint32
	if fi.Mode().Perm()&0o200 == 0 {
		a |= attrReadOnly
	}
	if name := fi.Name(); strings.HasPrefix(name, ".") && name != "." && name != ".." {
		a |= attrHidden
	}
	if fi.IsDir() {
		a |= attrDirectory
	}
	if fi.Mode()&os.ModeSymlink != 0 {
		a |= attrReparsePoint
	}
	return a, nil
}

func writeAttrs(path string, attrs uint32) error {
	fi, err := os.Lstat(path)
	if err != nil {
		return err
	}
	mode := fi.Mode().Perm()
	if attrs&attrReadOnly != 0 {
		mode &^= 0o222
	} else {
		mode |= 0o200
	}
	return os.Chmod(path, mode)
}

func platformInfo(path string) []string {
	fi, err := os.Lstat(path)
	if err != nil {
		return nil
	}
	st, ok := fi.Sys().(*syscall.Stat_t)
	if !ok {
		return nil
	}
	lines := []string{fmt.Sprintf("hard links: %d", uint64(st.Nlink)), "mode: " + fi.Mode().String()}
	owner := strconv.FormatUint(uint64(st.Uid), 10)
	if u, err := user.LookupId(owner); err == nil {
		owner = u.Username
	}
	group := strconv.FormatUint(uint64(st.Gid), 10)
	if g, err := user.LookupGroupId(group); err == nil {
		group = g.Name
	}
	return append(lines, "owner: "+owner, "group: "+group)
}

func reparseTarget(path string) string {
	t, err := os.Readlink(path)
	if err != nil {
		return ""
	}
	return t
}

// recycleFunc sends paths to the desktop's trash. A variable so tests never
// fill the person's real trash.
var recycleFunc = desktopTrash

// desktopTrash uses what the desktop provides; with none it refuses rather
// than deleting for good.
func desktopTrash(paths []string) error {
	var cmd *exec.Cmd
	switch {
	case runtime.GOOS == "darwin":
		var list []string
		for _, p := range paths {
			list = append(list, "POSIX file "+strconv.Quote(p))
		}
		cmd = exec.Command("osascript", "-e", "tell application \"Finder\" to delete {"+strings.Join(list, ", ")+"}")
	default:
		if gio, err := exec.LookPath("gio"); err == nil {
			cmd = exec.Command(gio, append([]string{"trash", "--"}, paths...)...)
		} else if tp, err := exec.LookPath("trash-put"); err == nil {
			cmd = exec.Command(tp, append([]string{"--"}, paths...)...)
		}
	}
	if cmd == nil {
		return errors.New("no trash command (gio or trash-put) is installed; nothing was deleted. permanent=true deletes for good")
	}
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("moving to the trash failed: %v %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}
