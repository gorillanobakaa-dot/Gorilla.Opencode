//go:build windows

package tools

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

// robocopyPath is robocopy.exe in System32, never a robocopy found on PATH.
func robocopyPath() string {
	dir, err := windows.GetSystemDirectory()
	if err != nil {
		dir = `C:\Windows\System32`
	}
	return filepath.Join(dir, "robocopy.exe")
}

// archiverPath is Windows' own tar.exe (bsdtar) in System32, never a tar
// found on PATH (files_archive.go, rule 3).
func archiverPath(string) (path string, gnu bool, err error) {
	dir, err := windows.GetSystemDirectory()
	if err != nil {
		dir = `C:\Windows\System32`
	}
	p := filepath.Join(dir, "tar.exe")
	if _, err := os.Stat(p); err != nil {
		return "", false, fmt.Errorf("%s is missing (Windows 10 1803 and later ship it); only zip without a password works here", p)
	}
	return p, false, nil
}

func hideConsole(cmd *exec.Cmd) { cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true} }

// runRobocopy runs one robocopy pass with its log in a private temp file and
// returns the parsed log and the exit code.
func runRobocopy(op syncOp, preview bool) (syncPlan, error) {
	logf, err := os.CreateTemp("", "gorilla-robocopy-*.log")
	if err != nil {
		return syncPlan{}, err
	}
	logPath := logf.Name()
	logf.Close()
	ctx := op.Ctx
	if ctx == nil {
		ctx = context.Background()
	}
	cmd := exec.CommandContext(ctx, robocopyPath(), robocopyArgs(op, preview, logPath)...)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	out, runErr := cmd.CombinedOutput()
	code := 0
	if runErr != nil {
		var ee *exec.ExitError
		if !errors.As(runErr, &ee) {
			os.Remove(logPath)
			return syncPlan{}, runErr
		}
		code = ee.ExitCode()
	}
	raw, err := os.ReadFile(logPath)
	if err != nil {
		return syncPlan{}, err
	}
	plan := parseRobocopyLog(decodeUnilog(raw), op.Action)
	plan.ExitCode = code
	if code >= 16 && len(plan.Errors) == 0 {
		plan.Errors = append(plan.Errors, strings.TrimSpace(string(out)))
	}
	if preview {
		os.Remove(logPath)
	} else {
		plan.Log = logPath // the log of a real run is kept for the person
	}
	return plan, nil
}

func previewSync(op syncOp) (syncPlan, error) {
	plan, err := runRobocopy(op, true)
	if err == nil && plan.ExitCode >= 8 {
		meaning, _ := robocopyExit(plan.ExitCode)
		return plan, fmt.Errorf("%s %s", meaning, strings.Join(plan.Errors, "; "))
	}
	return plan, err
}

func runSync(op syncOp) (syncPlan, error) { return runRobocopy(op, false) }

// ── attributes ─────────────────────────────────────────────────────────────

func readAttrs(path string) (uint32, error) {
	p, err := windows.UTF16PtrFromString(longPathForAPI(path))
	if err != nil {
		return 0, err
	}
	return windows.GetFileAttributes(p)
}

func writeAttrs(path string, attrs uint32) error {
	p, err := windows.UTF16PtrFromString(longPathForAPI(path))
	if err != nil {
		return err
	}
	if attrs&^windows.FILE_ATTRIBUTE_DIRECTORY == 0 {
		attrs |= windows.FILE_ATTRIBUTE_NORMAL
	}
	return windows.SetFileAttributes(p, attrs&^windows.FILE_ATTRIBUTE_DIRECTORY)
}

// settableAttrs are the bits SetFileAttributes changes. Compressed and
// Encrypted need compact.exe / cipher.exe.
const settableAttrs = attrReadOnly | attrHidden | attrSystem | attrArchive | attrTemporary | attrOffline | attrNotIndexed

// longPathForAPI adds the \\?\ prefix to an absolute path past MAX_PATH, for
// the calls Go's own long-path handling does not wrap.
func longPathForAPI(p string) string {
	if len(p) < 248 || strings.HasPrefix(p, `\\?\`) || !filepath.IsAbs(p) {
		return p
	}
	if strings.HasPrefix(p, `\\`) {
		return `\\?\UNC\` + p[2:]
	}
	return `\\?\` + p
}

// ── info ───────────────────────────────────────────────────────────────────

func platformInfo(path string) []string {
	var lines []string
	p, _ := windows.UTF16PtrFromString(longPathForAPI(path))
	h, err := windows.CreateFile(p, 0, windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE, nil,
		windows.OPEN_EXISTING, windows.FILE_FLAG_BACKUP_SEMANTICS|windows.FILE_FLAG_OPEN_REPARSE_POINT, 0)
	if err == nil {
		var bi windows.ByHandleFileInformation
		if windows.GetFileInformationByHandle(h, &bi) == nil {
			lines = append(lines,
				"created: "+filesTime(bi.CreationTime),
				"accessed: "+filesTime(bi.LastAccessTime),
				fmt.Sprintf("hard links: %d", bi.NumberOfLinks))
		}
		windows.CloseHandle(h)
	}
	if sd, err := windows.GetNamedSecurityInfo(longPathForAPI(path), windows.SE_FILE_OBJECT,
		windows.OWNER_SECURITY_INFORMATION|windows.DACL_SECURITY_INFORMATION); err == nil {
		if owner, _, err := sd.Owner(); err == nil && owner != nil {
			lines = append(lines, "owner: "+sidName(owner))
		}
		if dacl, _, err := sd.DACL(); err == nil && dacl != nil {
			lines = append(lines, aclSummary(dacl)...)
		} else if err == nil {
			lines = append(lines, "acl: none (everyone has full access)")
		}
	} else {
		lines = append(lines, "owner/acl: "+err.Error())
	}
	streams := alternateStreams(path)
	for _, s := range streams {
		lines = append(lines, "stream: "+s)
	}
	for _, s := range streams {
		if strings.HasPrefix(strings.ToLower(s), ":zone.identifier:") {
			if b, err := os.ReadFile(path + ":Zone.Identifier"); err == nil {
				lines = append(lines, "mark of the web: "+describeZone(string(b)))
			}
		}
	}
	return lines
}

func filesTime(ft windows.Filetime) string {
	return timeFromFiletime(ft.Nanoseconds())
}

func sidName(sid *windows.SID) string {
	account, domain, _, err := sid.LookupAccount("")
	if err != nil {
		return sid.String()
	}
	if domain != "" {
		return domain + `\` + account
	}
	return account
}

// aclSummary is the Get-Acl view: one line per entry, rights in the words
// File Explorer uses.
func aclSummary(acl *windows.ACL) []string {
	var lines []string
	for i := uint32(0); i < uint32(acl.AceCount); i++ {
		var ace *windows.ACCESS_ALLOWED_ACE
		if windows.GetAce(acl, i, &ace) != nil {
			continue
		}
		kind := "allow"
		switch ace.Header.AceType {
		case windows.ACCESS_ALLOWED_ACE_TYPE:
		case windows.ACCESS_DENIED_ACE_TYPE:
			kind = "DENY"
		default:
			continue
		}
		sid := (*windows.SID)(unsafe.Pointer(&ace.SidStart))
		inherited := ""
		if ace.Header.AceFlags&windows.INHERITED_ACE != 0 {
			inherited = " (inherited)"
		}
		lines = append(lines, fmt.Sprintf("acl: %s %s %s%s", kind, sidName(sid), accessRights(uint32(ace.Mask)), inherited))
	}
	return lines
}

// alternateStreams lists the named data streams (":name:$DATA size").
func alternateStreams(path string) []string {
	k32 := windows.NewLazySystemDLL("kernel32.dll")
	first, next := k32.NewProc("FindFirstStreamW"), k32.NewProc("FindNextStreamW")
	if first.Find() != nil || next.Find() != nil {
		return nil
	}
	type streamData struct {
		Size int64
		Name [windows.MAX_PATH + 36]uint16
	}
	p, _ := windows.UTF16PtrFromString(longPathForAPI(path))
	var d streamData
	h, _, _ := first.Call(uintptr(unsafe.Pointer(p)), 0, uintptr(unsafe.Pointer(&d)), 0)
	if windows.Handle(h) == windows.InvalidHandle {
		return nil
	}
	defer windows.FindClose(windows.Handle(h))
	var out []string
	for {
		name := windows.UTF16ToString(d.Name[:])
		if name != "::$DATA" {
			out = append(out, fmt.Sprintf("%s %d bytes", name, d.Size))
		}
		if r, _, _ := next.Call(h, uintptr(unsafe.Pointer(&d))); r == 0 {
			break
		}
	}
	return out
}

func reparseTarget(path string) string {
	t, err := os.Readlink(path)
	if err != nil {
		return ""
	}
	return t
}

// ── Recycle Bin ────────────────────────────────────────────────────────────

// recycleFunc sends paths to the Recycle Bin. A variable so tests never fill
// the person's real bin.
var recycleFunc = shellRecycle

// shellRecycle uses SHFileOperationW with FOF_ALLOWUNDO. FOF_WANTNUKEWARNING
// makes Windows WARN instead of silently deleting for good when an item
// cannot go to the bin (too big, or a drive with no bin). A drive with no bin
// at all is refused before that (SHQueryRecycleBinW fails on it).
func shellRecycle(paths []string) error {
	shell32 := windows.NewLazySystemDLL("shell32.dll")
	query := shell32.NewProc("SHQueryRecycleBinW")
	op := shell32.NewProc("SHFileOperationW")
	for _, p := range paths {
		if len(p) >= windows.MAX_PATH {
			return fmt.Errorf("%s is longer than %d characters, which the Recycle Bin cannot take; nothing was deleted", p, windows.MAX_PATH)
		}
		root, _ := windows.UTF16PtrFromString(filepath.VolumeName(p) + `\`)
		info := struct {
			Size  uint32
			Bytes int64
			Items int64
		}{}
		info.Size = uint32(unsafe.Sizeof(info)) // 24 on 64-bit: SHQUERYRBINFO is naturally aligned there
		if r, _, _ := query.Call(uintptr(unsafe.Pointer(root)), uintptr(unsafe.Pointer(&info))); r != 0 {
			return fmt.Errorf("%s has no Recycle Bin (error 0x%x); nothing was deleted. permanent=true deletes for good", filepath.VolumeName(p), r)
		}
	}
	var from []uint16
	for _, p := range paths {
		u, err := windows.UTF16FromString(p)
		if err != nil {
			return err
		}
		from = append(from, u...) // keeps each string's NUL
	}
	from = append(from, 0)
	const (
		foDelete           = 3
		fofSilent          = 0x0004
		fofNoConfirmation  = 0x0010
		fofAllowUndo       = 0x0040
		fofNoErrorUI       = 0x0400
		fofWantNukeWarning = 0x4000
	)
	sh := struct {
		Hwnd                 uintptr
		Func                 uint32
		From                 *uint16
		To                   *uint16
		Flags                uint16
		AnyOperationsAborted int32
		NameMappings         uintptr
		ProgressTitle        *uint16
	}{Func: foDelete, From: &from[0], Flags: fofSilent | fofNoConfirmation | fofAllowUndo | fofNoErrorUI | fofWantNukeWarning}
	if r, _, _ := op.Call(uintptr(unsafe.Pointer(&sh))); r != 0 {
		return fmt.Errorf("the Recycle Bin refused (SHFileOperation 0x%x)", r)
	}
	if sh.AnyOperationsAborted != 0 {
		return errors.New("the move to the Recycle Bin was cancelled")
	}
	for _, p := range paths {
		if _, err := os.Lstat(p); err == nil {
			return fmt.Errorf("%s is still there after the move to the Recycle Bin", p)
		}
	}
	return nil
}
