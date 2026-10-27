//go:build windows

package tools

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// makeDirLink makes a junction (no administrator rights needed).
func makeDirLink(link, target string) error {
	out, err := exec.Command("cmd", "/c", "mklink", "/J", link, target).CombinedOutput()
	if err != nil {
		return fmt.Errorf("%v %s", err, out)
	}
	return nil
}

// Mark of the Web and the ACL, as Get-Item / Get-Acl show them.
func TestFilesInfoMarkOfTheWebAndAcl(t *testing.T) {
	f := filepath.Join(t.TempDir(), "download.zip")
	mustWrite(t, f, "zip")
	if err := os.WriteFile(f+":Zone.Identifier", []byte("[ZoneTransfer]\r\nZoneId=3\r\nHostUrl=https://example.org/download.zip\r\n"), 0o644); err != nil {
		t.Skip("no alternate data streams on this volume:", err)
	}
	r, err := filesCall(t, &recordingPermissions{}, map[string]any{"action": "info", "path": f})
	if err != nil || r.IsError {
		t.Fatalf("%v %s", err, r.Content)
	}
	for _, want := range []string{"stream: :Zone.Identifier:$DATA", "mark of the web: ZoneId=3 (the Internet)", "acl: allow ", "created: ", "owner: "} {
		if !strings.Contains(r.Content, want) {
			t.Errorf("info lacks %q:\n%s", want, r.Content)
		}
	}
}

// Hidden, System and NotIndexed set and cleared through SetFileAttributes.
func TestFilesAttributesWindowsBits(t *testing.T) {
	f := filepath.Join(t.TempDir(), "x.txt")
	mustWrite(t, f, "x")
	t.Cleanup(func() { _ = writeAttrs(f, 0) })
	perms := &recordingPermissions{grant: true}
	if r, err := filesCall(t, perms, map[string]any{"action": "attributes", "path": f, "set": "Hidden,System,NotIndexed", "clear": "Archive"}); err != nil || r.IsError {
		t.Fatalf("%v %s", err, r.Content)
	}
	a, _ := readAttrs(f)
	if a&(attrHidden|attrSystem|attrNotIndexed) != attrHidden|attrSystem|attrNotIndexed || a&attrArchive != 0 {
		t.Errorf("attributes %s", attrString(a))
	}
}

// The real Recycle Bin, only when asked for (it puts a file in the bin of
// whoever runs it): GORILLA_TEST_REAL_RECYCLE=1.
func TestFilesRealRecycleBin(t *testing.T) {
	if os.Getenv("GORILLA_TEST_REAL_RECYCLE") != "1" {
		t.Skip("set GORILLA_TEST_REAL_RECYCLE=1 to send one temp file to the real Recycle Bin")
	}
	f := filepath.Join(t.TempDir(), "gorilla-recycle-probe.txt")
	mustWrite(t, f, "probe")
	if err := shellRecycle([]string{f}); err != nil {
		t.Fatal(err)
	}
	if exists(f) {
		t.Fatal("still there")
	}
}

// robocopy exit 16 in the dry run (a destination drive that does not exist):
// an error, nothing asked, nothing changed.
func TestFilesRobocopyFailureStopsBeforeAsking(t *testing.T) {
	src := filepath.Join(t.TempDir(), "src")
	mustWrite(t, filepath.Join(src, "a.txt"), "a")
	perms := &recordingPermissions{grant: true}
	r, err := filesCall(t, perms, map[string]any{"action": "mirror", "path": src, "dest": `Q:\gorilla-no-such-drive\dst`})
	if err != nil || !r.IsError || !strings.Contains(r.Content, "nothing was changed") || len(perms.calls) != 0 {
		t.Errorf("%v %s (asked %d)", err, r.Content, len(perms.calls))
	}
}
