package tools

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"

	"strings"
	"time"

	"github.com/opencode-ai/opencode/internal/permission"
)

// Windows file attribute bits (winnt.h FILE_ATTRIBUTE_*), the same numbers
// on every OS so the names and the tests are shared.
const (
	attrReadOnly     uint32 = 0x1
	attrHidden       uint32 = 0x2
	attrSystem       uint32 = 0x4
	attrDirectory    uint32 = 0x10
	attrArchive      uint32 = 0x20
	attrNormal       uint32 = 0x80
	attrTemporary    uint32 = 0x100
	attrSparse       uint32 = 0x200
	attrReparsePoint uint32 = 0x400
	attrCompressed   uint32 = 0x800
	attrOffline      uint32 = 0x1000
	attrNotIndexed   uint32 = 0x2000
	attrEncrypted    uint32 = 0x4000
	attrIntegrity    uint32 = 0x8000
	attrNoScrub      uint32 = 0x20000
	attrPinned       uint32 = 0x80000
	attrUnpinned     uint32 = 0x100000
	attrRecallOnData uint32 = 0x400000
)

// attrNames in the order PowerShell prints them.
var attrNames = []struct {
	name string
	bit  uint32
}{
	{"ReadOnly", attrReadOnly}, {"Hidden", attrHidden}, {"System", attrSystem}, {"Directory", attrDirectory},
	{"Archive", attrArchive}, {"Temporary", attrTemporary}, {"SparseFile", attrSparse},
	{"ReparsePoint", attrReparsePoint}, {"Compressed", attrCompressed}, {"Offline", attrOffline},
	{"NotContentIndexed", attrNotIndexed}, {"Encrypted", attrEncrypted}, {"IntegrityStream", attrIntegrity},
	{"NoScrubData", attrNoScrub}, {"Pinned", attrPinned}, {"Unpinned", attrUnpinned},
	{"RecallOnDataAccess", attrRecallOnData},
}

func attrString(a uint32) string {
	var out []string
	for _, n := range attrNames {
		if a&n.bit != 0 {
			out = append(out, n.name)
		}
	}
	if len(out) == 0 {
		return "Normal"
	}
	return strings.Join(out, ", ")
}

// parseAttrs reads "ReadOnly, Hidden" (any case, "+"/space/comma separated).
// normal is true for "Normal": clear every attribute that can be set.
func parseAttrs(s string) (bits uint32, normal bool, err error) {
	for _, w := range strings.FieldsFunc(s, func(r rune) bool { return r == ',' || r == '+' || r == ' ' || r == '|' }) {
		lw := strings.ToLower(w)
		switch lw {
		case "normal":
			normal = true
			continue
		case "notindexed", "notcontentindexed":
			bits |= attrNotIndexed
			continue
		case "isreadonly", "readonly", "read-only":
			bits |= attrReadOnly
			continue
		}
		found := false
		for _, n := range attrNames {
			if strings.EqualFold(n.name, w) {
				bits |= n.bit
				found = true
			}
		}
		if !found {
			return 0, false, fmt.Errorf("unknown attribute %q", w)
		}
	}
	return bits, normal, nil
}

// attrTargets is path itself, or with recurse on a folder everything below it
// whose name matches pattern (Get-ChildItem -Recurse: the folder itself is not
// one of them). Links are listed, never entered.
func attrTargets(root, pattern string, recurse bool) []string {
	fi, err := os.Lstat(root)
	if err != nil || !fi.IsDir() || !recurse {
		return []string{root}
	}
	var out []string
	_ = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil || path == root {
			return nil
		}
		if nameMatches(pattern, d.Name()) {
			out = append(out, path)
		}
		if isLinkEntry(d) && d.IsDir() {
			return filepath.SkipDir
		}
		return nil
	})
	return out
}

func (t filesTool) attributes(ctx context.Context, p filesParams) (ToolResponse, error) {
	if _, err := os.Lstat(p.Path); err != nil {
		return NewTextErrorResponse(err.Error()), nil
	}
	targets := attrTargets(p.Path, p.Pattern, p.Recurse)
	if strings.TrimSpace(p.Set) == "" && strings.TrimSpace(p.Clear) == "" {
		var b strings.Builder
		var lines []string
		for _, f := range targets {
			a, err := readAttrs(f)
			if err != nil {
				lines = append(lines, fmt.Sprintf("%s: %v", f, err))
				continue
			}
			lines = append(lines, fmt.Sprintf("%s: %s", f, attrString(a)))
		}
		writeListN(&b, "", lines, p.Full, filesListLines)
		return NewTextResponse(b.String()), nil
	}
	set, setNormal, err := parseAttrs(p.Set)
	if err != nil {
		return NewTextErrorResponse(err.Error()), nil
	}
	clear, _, err := parseAttrs(p.Clear)
	if err != nil {
		return NewTextErrorResponse(err.Error()), nil
	}
	if setNormal {
		clear |= settableAttrs
	}
	if bad := (set | clear) &^ settableAttrs; bad != 0 {
		return NewTextErrorResponse(fmt.Sprintf("cannot set or clear %s here; settable: %s. "+
			"Compressed and Encrypted need compact.exe / cipher.exe on Windows.", attrString(bad), attrString(settableAttrs))), nil
	}
	type change struct {
		path     string
		old, new uint32
	}
	var changes []change
	var lines []string
	for _, f := range targets {
		old, err := readAttrs(f)
		if err != nil {
			continue
		}
		nw := (old &^ clear) | set
		if nw != old {
			changes = append(changes, change{f, old, nw})
			lines = append(lines, fmt.Sprintf("%s: %s -> %s", f, attrString(old), attrString(nw)))
		}
	}
	var b strings.Builder
	fmt.Fprintf(&b, "DRY RUN (nothing changed yet): %d of %d item(s) change\n", len(changes), len(targets))
	writeList(&b, "  ", lines, p.Full)
	preview := b.String()
	if p.DryRun || len(changes) == 0 {
		return NewTextResponse(preview), nil
	}
	if !t.ask(ctx, "attributes", p.Path, "attributes:"+p.Path, preview, "", lines) {
		return ToolResponse{}, permission.ErrorPermissionDenied
	}
	var failed []string
	for _, c := range changes {
		if err := writeAttrs(c.path, c.new); err != nil {
			failed = append(failed, fmt.Sprintf("%s: %v", c.path, err))
		}
	}
	out := strings.Replace(preview, "DRY RUN (nothing changed yet)", "DONE", 1)
	if len(failed) > 0 {
		var fb strings.Builder
		fmt.Fprintf(&fb, "%sFAILED (%d):\n", out, len(failed))
		writeList(&fb, "  ! ", failed, p.Full)
		return NewTextErrorResponse(fb.String()), nil
	}
	return NewTextResponse(out), nil
}

// ── info ───────────────────────────────────────────────────────────────────

func filesInfo(p filesParams) (ToolResponse, error) {
	fi, err := os.Lstat(p.Path)
	if err != nil {
		return NewTextErrorResponse(err.Error()), nil
	}
	kind := "file"
	switch {
	case fi.Mode()&os.ModeSymlink != 0:
		kind = "symbolic link"
	case fi.Mode()&os.ModeIrregular != 0:
		kind = "junction or other reparse point"
	case fi.IsDir():
		kind = "folder"
	}
	lines := []string{"path: " + p.Path, "type: " + kind}
	if fi.IsDir() {
		if entries, err := os.ReadDir(p.Path); err == nil {
			lines = append(lines, fmt.Sprintf("entries: %d (top level)", len(entries)))
		}
	} else {
		lines = append(lines, "size: "+filesHumanBytes(fi.Size()))
	}
	lines = append(lines, "modified: "+fi.ModTime().Format(time.RFC3339))
	if a, err := readAttrs(p.Path); err == nil {
		lines = append(lines, "attributes: "+attrString(a))
	}
	if t := reparseTarget(p.Path); t != "" {
		lines = append(lines, "points to: "+t)
	}
	lines = append(lines, platformInfo(p.Path)...)
	if p.Hash && fi.Mode().IsRegular() {
		if h, err := fileSHA256(p.Path); err == nil {
			lines = append(lines, "sha256: "+h)
		} else {
			lines = append(lines, "sha256: "+err.Error())
		}
	}
	return NewTextResponse(strings.Join(lines, "\n")), nil
}

func fileSHA256(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// timeFromFiletime formats nanoseconds since the Unix epoch.
func timeFromFiletime(ns int64) string {
	return time.Unix(0, ns).Format(time.RFC3339)
}

// accessRights names an NTFS access mask the way File Explorer does.
func accessRights(mask uint32) string {
	for _, r := range []struct {
		name string
		bits uint32
	}{
		{"FullControl", 0x1F01FF}, {"Modify", 0x1301BF}, {"ReadAndExecute", 0x1200A9},
		{"Read", 0x120089}, {"Write", 0x100116},
	} {
		if mask&r.bits == r.bits {
			return r.name
		}
	}
	// Generic rights, as inherited entries often carry them.
	switch {
	case mask&0x10000000 != 0:
		return "FullControl"
	case mask&0x80000000 != 0 && mask&0x40000000 != 0:
		return "Modify"
	case mask&0x80000000 != 0:
		return "Read"
	}
	return fmt.Sprintf("special (0x%X)", mask)
}

// describeZone reads a Zone.Identifier stream (Mark of the Web).
func describeZone(s string) string {
	zones := map[string]string{"0": "this computer", "1": "local intranet", "2": "trusted sites", "3": "the Internet", "4": "restricted sites"}
	var parts []string
	for _, line := range strings.Split(strings.ReplaceAll(s, "\r", ""), "\n") {
		k, v, ok := strings.Cut(strings.TrimSpace(line), "=")
		if !ok {
			continue
		}
		switch strings.ToLower(k) {
		case "zoneid":
			name := zones[v]
			if name == "" {
				name = "unknown"
			}
			parts = append(parts, fmt.Sprintf("ZoneId=%s (%s)", v, name))
		case "hosturl", "referrerurl":
			parts = append(parts, k+"="+v)
		}
	}
	if len(parts) == 0 {
		return "present, unreadable"
	}
	return strings.Join(parts, " ")
}

// ── list ───────────────────────────────────────────────────────────────────

// filesList is Get-ChildItem -Force: files and folders with their attributes,
// hidden and system items included and marked. Searching is find's job; this
// is the view that shows what find skips on purpose.
//
// GORILLA FIX (2026-10-27, job 07): hidden and system items are listed by
// default. They were left out unless hidden=true, so "show the attributes of
// every file here" answered without the two hidden files, the very ones whose
// attributes mattered; the model saw "2 hidden/system not shown" and did not
// ask again. A hidden FOLDER is still not entered without hidden=true (.git
// and the like would flood a recursive listing); the header counts them.
func filesList(p filesParams) (ToolResponse, error) {
	fi, err := os.Stat(p.Path)
	if err != nil {
		return NewTextErrorResponse(err.Error()), nil
	}
	if !fi.IsDir() {
		return filesInfo(p)
	}
	kind := strings.ToLower(strings.TrimSpace(p.Kind))
	var lines []string
	skipped := 0
	_ = filepath.WalkDir(p.Path, func(path string, d fs.DirEntry, err error) error {
		if err != nil || path == p.Path {
			return nil
		}
		hidden := false
		a, aerr := readAttrs(path)
		if aerr == nil {
			hidden = a&(attrHidden|attrSystem) != 0
		}
		descend := p.Recurse && d.IsDir() && !isLinkEntry(d)
		if hidden && d.IsDir() && descend && !p.Hidden {
			skipped++
			descend = false
		}
		show := nameMatches(p.Pattern, d.Name()) &&
			!(kind == "file" && d.IsDir()) && !(kind == "dir" && !d.IsDir())
		if show {
			info, _ := d.Info()
			size := "<DIR>"
			mod := ""
			if info != nil {
				if !d.IsDir() {
					size = fmt.Sprintf("%d", info.Size())
				}
				mod = info.ModTime().Format("2006-01-02 15:04")
			}
			rel, _ := filepath.Rel(p.Path, path)
			lines = append(lines, fmt.Sprintf("%-16s %12s  %-24s %s", mod, size, attrString(a), rel))
		}
		if d.IsDir() && !descend {
			return filepath.SkipDir
		}
		return nil
	})
	var b strings.Builder
	fmt.Fprintf(&b, "%s: %d item(s)", p.Path, len(lines))
	if skipped > 0 {
		fmt.Fprintf(&b, ", %d hidden/system folder(s) listed but not entered (hidden=true enters them)", skipped)
	}
	b.WriteString("\n")
	writeListN(&b, "", lines, p.Full, filesListLines)
	return NewTextResponse(b.String()), nil
}
