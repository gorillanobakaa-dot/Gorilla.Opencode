package tools

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/opencode-ai/opencode/internal/config"
	"github.com/opencode-ai/opencode/internal/permission"
)

// Every fixture lives in t.TempDir(); nothing here touches a real file of the
// person running the tests, and the Recycle Bin is stubbed.

func filesCall(t *testing.T, perms *recordingPermissions, params map[string]any) (ToolResponse, error) {
	t.Helper()
	return filesCallIn(t, "files-test", perms, params)
}

func filesCallIn(t *testing.T, session string, perms *recordingPermissions, params map[string]any) (ToolResponse, error) {
	t.Helper()
	// A real robocopy run keeps its log in the temp folder for the person;
	// in a test that folder is the test's own, so nothing is left behind.
	tmp := t.TempDir()
	for _, v := range []string{"TMP", "TEMP", "TMPDIR"} {
		t.Setenv(v, tmp)
	}
	ctx := context.WithValue(context.Background(), SessionIDContextKey, session)
	ctx = context.WithValue(ctx, MessageIDContextKey, "m1")
	in, _ := json.Marshal(params)
	return NewFilesTool(perms).Run(ctx, ToolCall{Name: FilesToolName, Input: string(in)})
}

func mustWrite(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func exists(p string) bool { _, err := os.Lstat(p); return err == nil }

func readString(t *testing.T, p string) string {
	t.Helper()
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatalf("read %s: %v", p, err)
	}
	return string(b)
}

// mirrorFixture: src has a.txt, sub/ș.txt; dst has extra.txt, old/y.txt and a
// stale a.txt.
func mirrorFixture(t *testing.T) (src, dst string) {
	root := t.TempDir()
	src, dst = filepath.Join(root, "src"), filepath.Join(root, "dst")
	mustWrite(t, filepath.Join(src, "a.txt"), "new a")
	mustWrite(t, filepath.Join(src, "sub", "ș.txt"), "non-ascii")
	mustWrite(t, filepath.Join(dst, "extra.txt"), "extra")
	mustWrite(t, filepath.Join(dst, "old", "y.txt"), "old")
	mustWrite(t, filepath.Join(dst, "a.txt"), "stale")
	old := time.Now().Add(-48 * time.Hour)
	_ = os.Chtimes(filepath.Join(dst, "a.txt"), old, old)
	return src, dst
}

// The owner's line `robocopy "C:\SourceDir" "D:\DestDir" /MIR /MT:16 /Z`:
// the preview names what is copied AND deleted, the prompt is irreversible,
// and afterwards dest is exactly the source.
func TestFilesMirrorPreviewsAsksThenMirrors(t *testing.T) {
	src, dst := mirrorFixture(t)

	r, err := filesCall(t, &recordingPermissions{}, map[string]any{"action": "mirror", "path": src, "dest": dst, "dry_run": true})
	if err != nil || r.IsError {
		t.Fatalf("dry run: %v %s", err, r.Content)
	}
	for _, want := range []string{"DRY RUN", "extra.txt", "y.txt", "DELETED for good", "NOT to the Recycle Bin", "a.txt"} {
		if !strings.Contains(r.Content, want) {
			t.Errorf("dry run preview lacks %q:\n%s", want, r.Content)
		}
	}
	if !exists(filepath.Join(dst, "extra.txt")) || readString(t, filepath.Join(dst, "a.txt")) != "stale" {
		t.Fatal("the dry run changed the destination")
	}

	deny := &recordingPermissions{grant: false}
	if _, err := filesCall(t, deny, map[string]any{"action": "mirror", "path": src, "dest": dst}); !errors.Is(err, permission.ErrorPermissionDenied) {
		t.Fatalf("denied mirror: err = %v", err)
	}
	if !exists(filepath.Join(dst, "extra.txt")) {
		t.Fatal("a denied mirror deleted a file")
	}
	if len(deny.calls) != 1 {
		t.Fatalf("asked %d times", len(deny.calls))
	}
	c := deny.calls[0]
	if c.Irreversible == "" || !strings.Contains(c.Irreversible, "for good") {
		t.Errorf("mirror prompt not irreversible: %q", c.Irreversible)
	}
	if !strings.Contains(c.Description, "extra.txt") || !strings.Contains(c.Description, "DELETED") {
		t.Errorf("the prompt does not show the deletes:\n%s", c.Description)
	}

	allow := &recordingPermissions{grant: true}
	r, err = filesCall(t, allow, map[string]any{"action": "mirror", "path": src, "dest": dst, "threads": 16, "restartable": true})
	if err != nil || r.IsError {
		t.Fatalf("mirror: %v %s", err, r.Content)
	}
	if exists(filepath.Join(dst, "extra.txt")) || exists(filepath.Join(dst, "old")) {
		t.Errorf("extras survived the mirror:\n%s", r.Content)
	}
	if readString(t, filepath.Join(dst, "a.txt")) != "new a" || readString(t, filepath.Join(dst, "sub", "ș.txt")) != "non-ascii" {
		t.Errorf("dest is not the source:\n%s", r.Content)
	}
	if !strings.Contains(r.Content, "DONE") {
		t.Errorf("result:\n%s", r.Content)
	}
	if runtime.GOOS == "windows" && !strings.Contains(r.Content, "robocopy exit") {
		t.Errorf("robocopy exit code not decoded:\n%s", r.Content)
	}
}

// The Go engine mirrors the same way (it is what runs off Windows).
func TestFilesGoEngineMirror(t *testing.T) {
	src, dst := mirrorFixture(t)
	op := syncOp{Action: "mirror", Src: src, Dst: dst, Threads: 4}
	plan, err := goPlan(op)
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Copies) != 2 || plan.Overwrites != 1 || len(plan.Deletes) != 1 || len(plan.DeleteDirs) != 1 {
		t.Fatalf("plan: %+v", plan)
	}
	if _, err := goRun(op); err != nil {
		t.Fatal(err)
	}
	if exists(filepath.Join(dst, "extra.txt")) || exists(filepath.Join(dst, "old")) ||
		readString(t, filepath.Join(dst, "sub", "ș.txt")) != "non-ascii" {
		t.Error("go engine did not mirror")
	}
	again, _ := goPlan(op)
	if len(again.Copies)+again.deletes() != 0 {
		t.Errorf("a second mirror still has work: %+v", again)
	}
}

// /PURGE alone: extras go, nothing is copied.
func TestFilesPurgeDeletesOnlyExtras(t *testing.T) {
	src, dst := mirrorFixture(t)
	r, err := filesCall(t, &recordingPermissions{grant: true}, map[string]any{"action": "purge", "path": src, "dest": dst})
	if err != nil || r.IsError {
		t.Fatalf("purge: %v %s", err, r.Content)
	}
	if exists(filepath.Join(dst, "extra.txt")) || exists(filepath.Join(dst, "old")) {
		t.Errorf("extras survived:\n%s", r.Content)
	}
	if exists(filepath.Join(dst, "sub")) || readString(t, filepath.Join(dst, "a.txt")) != "stale" {
		t.Errorf("purge copied something:\n%s", r.Content)
	}
}

// /MOVE deletes source files and folders after copying; /MOV only files.
func TestFilesMoveAndMoveFiles(t *testing.T) {
	for _, action := range []string{"move", "move_files"} {
		t.Run(action, func(t *testing.T) {
			root := t.TempDir()
			src, dst := filepath.Join(root, "src"), filepath.Join(root, "dst")
			mustWrite(t, filepath.Join(src, "a.txt"), "a")
			mustWrite(t, filepath.Join(src, "deep", "b.txt"), "b")
			perms := &recordingPermissions{grant: true}
			r, err := filesCall(t, perms, map[string]any{"action": action, "path": src, "dest": dst})
			if err != nil || r.IsError {
				t.Fatalf("%v %s", err, r.Content)
			}
			if len(perms.calls) != 1 || !strings.Contains(perms.calls[0].Irreversible, "from the source") {
				t.Errorf("prompt: %+v", perms.calls)
			}
			if readString(t, filepath.Join(dst, "deep", "b.txt")) != "b" {
				t.Error("not copied")
			}
			if exists(filepath.Join(src, "a.txt")) || exists(filepath.Join(src, "deep", "b.txt")) {
				t.Errorf("source files left:\n%s", r.Content)
			}
			if action == "move_files" && !exists(filepath.Join(src, "deep")) {
				t.Error("move_files (/MOV) removed a folder")
			}
			if action == "move" && exists(filepath.Join(src, "deep")) {
				t.Error("move (/MOVE) left the emptied folder")
			}
		})
	}
}

// 128 threads (robocopy's /MT ceiling) on a 2,000-file tree.
func TestFilesCopy128ThreadsOn2000Files(t *testing.T) {
	root := t.TempDir()
	src, dst := filepath.Join(root, "src"), filepath.Join(root, "dst")
	for i := 0; i < 2000; i++ {
		mustWrite(t, filepath.Join(src, fmt.Sprintf("d%02d", i%20), fmt.Sprintf("f%04d.txt", i)), fmt.Sprint(i))
	}
	perms := &recordingPermissions{grant: true}
	r, err := filesCall(t, perms, map[string]any{"action": "copy", "path": src, "dest": dst, "threads": 128})
	if err != nil || r.IsError {
		t.Fatalf("%v %s", err, r.Content)
	}
	n := 0
	_ = filepath.WalkDir(dst, func(_ string, d os.DirEntry, _ error) error {
		if d != nil && !d.IsDir() {
			n++
		}
		return nil
	})
	if n != 2000 {
		t.Errorf("copied %d of 2000 files", n)
	}
	if !strings.Contains(r.Content, "2000 file(s) were copied") {
		t.Errorf("result:\n%s", r.Content)
	}
	if runtime.GOOS == "windows" && !strings.Contains(r.Content, "128 threads") {
		t.Errorf("not run with 128 threads:\n%s", r.Content)
	}
	if perms.calls[0].Irreversible != "" {
		t.Errorf("a copy of new files was marked irreversible: %q", perms.calls[0].Irreversible)
	}
}

func TestFilesThreadsDefaultAndCap(t *testing.T) {
	want := runtime.NumCPU()
	if want > 128 {
		want = 128
	}
	for in, out := range map[int]int{0: want, -3: want, 1: 1, 64: 64, 128: 128, 129: 128, 1000: 128} {
		if got := filesThreads(in); got != out {
			t.Errorf("filesThreads(%d) = %d, want %d", in, got, out)
		}
	}
}

// Rule 3: refused before anything is asked or run.
func TestFilesRefusesRootsAndNesting(t *testing.T) {
	root := t.TempDir()
	src := filepath.Join(root, "src")
	mustWrite(t, filepath.Join(src, "a.txt"), "a")
	home, _ := os.UserHomeDir()
	volume := string(filepath.Separator)
	if runtime.GOOS == "windows" {
		volume = filepath.VolumeName(root) + `\`
	}
	cases := []map[string]any{
		{"action": "mirror", "path": src, "dest": filepath.Join(src, "inner")},
		{"action": "copy", "path": src, "dest": filepath.Join(src, "inner")},
		{"action": "mirror", "path": src, "dest": root},
		{"action": "mirror", "path": src, "dest": src},
		{"action": "mirror", "path": src, "dest": volume},
		{"action": "purge", "path": src, "dest": volume},
		{"action": "mirror", "path": src, "dest": home},
		{"action": "move", "path": volume, "dest": filepath.Join(root, "x")},
	}
	for _, c := range cases {
		perms := &recordingPermissions{grant: true}
		r, err := filesCall(t, perms, c)
		if err != nil || !r.IsError || !strings.Contains(r.Content, "refused") || len(perms.calls) != 0 {
			t.Errorf("%v: not refused before asking: %v %s", c, err, r.Content)
		}
	}
	if !exists(filepath.Join(src, "a.txt")) {
		t.Fatal("a refusal changed the source")
	}
}

func TestFilesRobocopyArgs(t *testing.T) {
	op := syncOp{Action: "mirror", Src: `C:\s`, Dst: `C:\d`, Threads: 16, Restartable: true}
	prev := strings.Join(robocopyArgs(op, true, `C:\l.log`), " ")
	real := strings.Join(robocopyArgs(op, false, `C:\l.log`), " ")
	for _, want := range []string{"/MIR", "/XJ", "/L", "/UNILOG:C:\\l.log", "/R:2", "/W:2"} {
		if !strings.Contains(prev, want) {
			t.Errorf("preview lacks %s: %s", want, prev)
		}
	}
	if strings.Contains(prev, "/MT") || strings.Contains(real, "/L ") || !strings.Contains(real, "/MT:16") || !strings.Contains(real, "/Z") || !strings.Contains(real, "/XJ") {
		t.Errorf("real: %s", real)
	}
	purge := strings.Join(robocopyArgs(syncOp{Action: "purge", Src: "a", Dst: "b"}, false, ""), " ")
	for _, want := range []string{"/PURGE", "/XC", "/XN", "/XO", "/XL"} {
		if !strings.Contains(purge, want) || strings.Contains(purge, "/MIR") {
			t.Errorf("purge: %s", purge)
		}
	}
	for action, want := range map[string]string{"copy": "/E", "move": "/MOVE", "move_files": "/MOV"} {
		args := robocopyArgs(syncOp{Action: action, Src: "a", Dst: "b", Pattern: "*.txt"}, false, "")
		if args[2] != "*.txt" || !strings.Contains(strings.Join(args, " "), " "+want+" ") {
			t.Errorf("%s: %v", action, args)
		}
	}
}

const sampleRobocopyLog = "\r\n" +
	"\t                   1\tC:\\s\\\r\n" +
	"\t*EXTRA Dir        -1\tC:\\d\\old\\\r\n" +
	"\t  *EXTRA File \t\t       2 2026/10/27 12:01:42\tC:\\d\\old\\y.txt\r\n" +
	"\t    New File  \t\t       5 2026/10/27 12:01:42\tC:\\s\\a.txt\r\n" +
	"\t    Newer     \t\t      10 2026/10/27 12:01:42\tC:\\s\\b.txt\r\n" +
	"\t  New Dir          1\tC:\\s\\sub\\\r\n" +
	"\t    New File  \t\t       7 2026/10/27 12:01:42\tC:\\s\\sub\\ș.txt\r\n" +
	"2026/10/27 12:01:43 ERROR 5 (0x00000005) Copying File C:\\s\\locked.txt\r\n" +
	"Access is denied.\r\n"

func TestFilesParseRobocopyLog(t *testing.T) {
	p := parseRobocopyLog(sampleRobocopyLog, "mirror")
	if len(p.Copies) != 3 || p.Bytes != 22 || p.Overwrites != 1 {
		t.Errorf("copies %v bytes %d overwrites %d", p.Copies, p.Bytes, p.Overwrites)
	}
	if len(p.Deletes) != 1 || len(p.DeleteDirs) != 1 || len(p.NewDirs) != 1 {
		t.Errorf("deletes %v dirs %v new %v", p.Deletes, p.DeleteDirs, p.NewDirs)
	}
	if p.Copies[2] != `C:\s\sub\ș.txt` {
		t.Errorf("non-ASCII path: %q", p.Copies[2])
	}
	if len(p.Errors) != 1 || !strings.Contains(p.Errors[0], "Access is denied") {
		t.Errorf("errors: %v", p.Errors)
	}
	m := parseRobocopyLog(sampleRobocopyLog, "move_files")
	if len(m.Deletes) != 1+3 {
		t.Errorf("move: the copied files are deleted from the source: %v", m.Deletes)
	}
}

func TestFilesDecodeUnilog(t *testing.T) {
	b := []byte{0xFF, 0xFE}
	for _, r := range "ș\tx" {
		b = append(b, byte(r), byte(r>>8))
	}
	if got := decodeUnilog(b); got != "ș\tx" {
		t.Errorf("got %q", got)
	}
	if got := decodeUnilog([]byte("\uFEFFplain")); got != "plain" {
		t.Errorf("got %q", got)
	}
}

// Copy-Item / Move-Item on one file, and Move-Item -Force over an existing one.
func TestFilesSingleFileCopyAndMove(t *testing.T) {
	root := t.TempDir()
	f := filepath.Join(root, "File.txt")
	mustWrite(t, f, "one")
	backup := filepath.Join(root, "Backup")
	if err := os.Mkdir(backup, 0o755); err != nil {
		t.Fatal(err)
	}
	perms := &recordingPermissions{grant: true}
	if r, err := filesCall(t, perms, map[string]any{"action": "copy", "path": f, "dest": backup}); err != nil || r.IsError {
		t.Fatalf("copy: %v %s", err, r.Content)
	}
	if readString(t, filepath.Join(backup, "File.txt")) != "one" || !exists(f) {
		t.Fatal("copy into a folder")
	}
	mustWrite(t, filepath.Join(root, "Dest", "File.txt"), "old")
	r, err := filesCall(t, perms, map[string]any{"action": "move", "path": f, "dest": filepath.Join(root, "Dest", "File.txt")})
	if err != nil || r.IsError {
		t.Fatalf("move: %v %s", err, r.Content)
	}
	if exists(f) || readString(t, filepath.Join(root, "Dest", "File.txt")) != "one" {
		t.Error("move over an existing file")
	}
	if last := perms.calls[len(perms.calls)-1]; !strings.Contains(last.Irreversible, "overwrites") || !strings.Contains(last.Description, "REPLACES") {
		t.Errorf("overwrite not said: %+v", last)
	}
}

// A read-only file in the destination is replaced, as robocopy does.
func TestFilesCopyOverReadOnlyDestination(t *testing.T) {
	root := t.TempDir()
	src, dst := filepath.Join(root, "src"), filepath.Join(root, "dst")
	mustWrite(t, filepath.Join(src, "r.txt"), "fresh")
	mustWrite(t, filepath.Join(dst, "r.txt"), "stale!")
	if err := writeAttrs(filepath.Join(dst, "r.txt"), attrReadOnly); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = writeAttrs(filepath.Join(dst, "r.txt"), 0) })
	r, err := filesCall(t, &recordingPermissions{grant: true}, map[string]any{"action": "copy", "path": src, "dest": dst})
	if err != nil || r.IsError {
		t.Fatalf("%v %s", err, r.Content)
	}
	if readString(t, filepath.Join(dst, "r.txt")) != "fresh" {
		t.Errorf("read-only destination not replaced:\n%s", r.Content)
	}
}

// The owner's `$_.Name -replace 'oldText', 'newText'`.
func TestFilesRenameReplace(t *testing.T) {
	dir := t.TempDir()
	for _, n := range []string{"oldText-1.txt", "oldText-2.txt", "keep.txt"} {
		mustWrite(t, filepath.Join(dir, n), n)
	}
	r, err := filesCall(t, &recordingPermissions{grant: true}, map[string]any{"action": "rename", "path": dir, "find": "oldText", "replace": "newText"})
	if err != nil || r.IsError {
		t.Fatalf("%v %s", err, r.Content)
	}
	for _, n := range []string{"newText-1.txt", "newText-2.txt", "keep.txt"} {
		if !exists(filepath.Join(dir, n)) {
			t.Errorf("%s missing:\n%s", n, r.Content)
		}
	}
	if !strings.Contains(r.Content, "2 file(s) renamed") {
		t.Errorf("result:\n%s", r.Content)
	}
}

// The owner's `"{0:D2}_{1}" -f $script:i++, $_.Name`.
func TestFilesRenameCounterPrefix(t *testing.T) {
	dir := t.TempDir()
	for _, n := range []string{"b.jpg", "a.jpg", "c.jpg"} {
		mustWrite(t, filepath.Join(dir, n), n)
	}
	perms := &recordingPermissions{grant: true}
	r, _ := filesCall(t, perms, map[string]any{"action": "rename", "path": dir, "template": "{n:D2}_{name}", "dry_run": true})
	if !strings.Contains(r.Content, "a.jpg -> 01_a.jpg") || exists(filepath.Join(dir, "01_a.jpg")) || len(perms.calls) != 0 {
		t.Fatalf("dry run:\n%s", r.Content)
	}
	if r, err := filesCall(t, perms, map[string]any{"action": "rename", "path": dir, "template": "{n:D2}_{name}"}); err != nil || r.IsError {
		t.Fatalf("%v %s", err, r.Content)
	}
	for _, n := range []string{"01_a.jpg", "02_b.jpg", "03_c.jpg"} {
		if !exists(filepath.Join(dir, n)) {
			t.Errorf("%s missing", n)
		}
	}
	if got := newName("Photo One.JPG", nil, "", "title", "{stem}-{n:D3}{ext}", 7); got != "Photo One-007.Jpg" {
		t.Errorf("title + suffix counter: %q", got)
	}
	if got := newName("Report.TXT", nil, "", "lower", "", 1); got != "report.txt" {
		t.Errorf("lower: %q", got)
	}
}

func TestFilesRenameCollisionsRefusedBeforeAnythingMoves(t *testing.T) {
	dir := t.TempDir()
	for _, n := range []string{"a1.txt", "a2.txt", "b1.txt", "x.txt"} {
		mustWrite(t, filepath.Join(dir, n), n)
	}
	cases := []map[string]any{
		{"find": `\d`, "replace": ""},              // a1, a2 -> a.txt twice
		{"find": `^b1`, "replace": "x"},            // b1.txt -> x.txt, which exists
		{"find": `^x`, "replace": "con"},           // reserved name
		{"find": `\.txt$`, "replace": ".txt."},     // trailing dot
		{"find": `^b1\.txt$`, "replace": "fresh."}, // trailing dot on a name nobody has
		{"find": `^a1`, "replace": "a/1"},          // separator
		{"find": `(`, "replace": ""},               // bad regex
		{"template": "same.txt", "pattern": "*"},   // every file to one name
	}
	for _, c := range cases {
		c["action"], c["path"] = "rename", dir
		perms := &recordingPermissions{grant: true}
		r, err := filesCall(t, perms, c)
		if err != nil || !r.IsError || len(perms.calls) != 0 {
			t.Errorf("%v: not refused before asking: %v %s", c, err, r.Content)
		}
	}
	for _, n := range []string{"a1.txt", "a2.txt", "b1.txt", "x.txt"} {
		if !exists(filepath.Join(dir, n)) {
			t.Fatalf("%s moved during a refused rename", n)
		}
	}
}

// 1.txt -> 2.txt while 2.txt -> 3.txt: a name taken by a file that is itself
// being renamed is not a collision. And Rename-Item on one file.
func TestFilesRenameChainAndSingleFile(t *testing.T) {
	dir := t.TempDir()
	mustWrite(t, filepath.Join(dir, "1.txt"), "one")
	mustWrite(t, filepath.Join(dir, "2.txt"), "two")
	perms := &recordingPermissions{grant: true}
	if r, err := filesCall(t, perms, map[string]any{"action": "rename", "path": dir, "template": "{n}.txt", "start": 2}); err != nil || r.IsError {
		t.Fatalf("%v %s", err, r.Content)
	}
	if readString(t, filepath.Join(dir, "2.txt")) != "one" || readString(t, filepath.Join(dir, "3.txt")) != "two" || exists(filepath.Join(dir, "1.txt")) {
		t.Error("chain rename")
	}
	if r, err := filesCall(t, perms, map[string]any{"action": "rename", "path": filepath.Join(dir, "3.txt"), "dest": "NewName.txt"}); err != nil || r.IsError {
		t.Fatalf("%v %s", err, r.Content)
	}
	if readString(t, filepath.Join(dir, "NewName.txt")) != "two" {
		t.Error("single rename")
	}
}

// Set-ItemProperty -Name Attributes -Value ReadOnly / Normal, and
// $file.IsReadOnly = $false.
func TestFilesAttributesReadOnlyToggle(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "File.txt")
	mustWrite(t, f, "x")
	t.Cleanup(func() { _ = writeAttrs(f, 0) })
	perms := &recordingPermissions{grant: true}
	if r, err := filesCall(t, perms, map[string]any{"action": "attributes", "path": f, "set": "ReadOnly"}); err != nil || r.IsError {
		t.Fatalf("%v %s", err, r.Content)
	}
	if a, _ := readAttrs(f); a&attrReadOnly == 0 {
		t.Fatal("not read-only")
	}
	r, _ := filesCall(t, perms, map[string]any{"action": "attributes", "path": f})
	if !strings.Contains(r.Content, "ReadOnly") {
		t.Errorf("get: %s", r.Content)
	}
	if err := os.WriteFile(f, []byte("y"), 0o644); err == nil {
		t.Error("a read-only file could be written")
	}
	if r, err := filesCall(t, perms, map[string]any{"action": "attributes", "path": f, "clear": "IsReadOnly"}); err != nil || r.IsError {
		t.Fatalf("%v %s", err, r.Content)
	}
	if a, _ := readAttrs(f); a&attrReadOnly != 0 {
		t.Fatal("still read-only")
	}
	_, _ = filesCall(t, perms, map[string]any{"action": "attributes", "path": f, "set": "ReadOnly"})
	if r, err := filesCall(t, perms, map[string]any{"action": "attributes", "path": f, "set": "Normal"}); err != nil || r.IsError {
		t.Fatalf("Normal: %v %s", err, r.Content)
	}
	if a, _ := readAttrs(f); a&attrReadOnly != 0 {
		t.Error("Normal kept ReadOnly")
	}
	asked := len(perms.calls)
	_, _ = filesCall(t, perms, map[string]any{"action": "attributes", "path": f})
	if len(perms.calls) != asked {
		t.Error("reading attributes asked for permission")
	}
	if r, _ := filesCall(t, perms, map[string]any{"action": "attributes", "path": f, "set": "Encrypted"}); !r.IsError {
		t.Errorf("Encrypted accepted: %s", r.Content)
	}
}

// Recursive set with a preview of how many items change.
func TestFilesAttributesRecursivePreview(t *testing.T) {
	dir := t.TempDir()
	mustWrite(t, filepath.Join(dir, "a.log"), "a")
	mustWrite(t, filepath.Join(dir, "sub", "b.log"), "b")
	mustWrite(t, filepath.Join(dir, "sub", "c.txt"), "c")
	t.Cleanup(func() {
		for _, f := range []string{"a.log", "sub/b.log", "sub/c.txt"} {
			_ = writeAttrs(filepath.Join(dir, f), 0)
		}
	})
	perms := &recordingPermissions{grant: true}
	r, _ := filesCall(t, perms, map[string]any{"action": "attributes", "path": dir, "recurse": true, "pattern": "*.log", "set": "ReadOnly", "dry_run": true})
	if !strings.Contains(r.Content, "2 of 2 item(s) change") || len(perms.calls) != 0 {
		t.Fatalf("preview:\n%s", r.Content)
	}
	if a, _ := readAttrs(filepath.Join(dir, "a.log")); a&attrReadOnly != 0 {
		t.Fatal("dry run changed an attribute")
	}
	_, _ = filesCall(t, perms, map[string]any{"action": "attributes", "path": dir, "recurse": true, "pattern": "*.log", "set": "ReadOnly"})
	if a, _ := readAttrs(filepath.Join(dir, "sub", "b.log")); a&attrReadOnly == 0 {
		t.Error("sub/b.log not set")
	}
	if a, _ := readAttrs(filepath.Join(dir, "sub", "c.txt")); a&attrReadOnly != 0 {
		t.Error("c.txt set though the pattern excludes it")
	}
}

func TestFilesParseAttrs(t *testing.T) {
	bits, normal, err := parseAttrs("ReadOnly, hidden+NotIndexed")
	if err != nil || normal || bits != attrReadOnly|attrHidden|attrNotIndexed {
		t.Errorf("%x %v %v", bits, normal, err)
	}
	if _, normal, _ := parseAttrs("Normal"); !normal {
		t.Error("Normal")
	}
	if _, _, err := parseAttrs("Sparkly"); err == nil {
		t.Error("unknown attribute accepted")
	}
	if got := attrString(attrReadOnly | attrArchive); got != "ReadOnly, Archive" {
		t.Error(got)
	}
	if attrString(0) != "Normal" {
		t.Error("zero")
	}
}

// Get-Item details: size, hard links, hash; read-only, so no prompt.
func TestFilesInfo(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "File.txt")
	mustWrite(t, f, "abc")
	if err := os.Link(f, filepath.Join(dir, "second-name.txt")); err != nil {
		t.Skip("no hard links here:", err)
	}
	perms := &recordingPermissions{}
	r, err := filesCall(t, perms, map[string]any{"action": "info", "path": f, "hash": true})
	if err != nil || r.IsError {
		t.Fatalf("%v %s", err, r.Content)
	}
	for _, want := range []string{"size: 3 B", "hard links: 2", "owner: ",
		"sha256: ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad"} {
		if !strings.Contains(r.Content, want) {
			t.Errorf("info lacks %q:\n%s", want, r.Content)
		}
	}
	if len(perms.calls) != 0 {
		t.Error("info asked for permission")
	}
}

func TestFilesDescribeZoneAndRights(t *testing.T) {
	z := describeZone("[ZoneTransfer]\r\nZoneId=3\r\nHostUrl=https://example.org/x.zip\r\n")
	if !strings.Contains(z, "ZoneId=3 (the Internet)") || !strings.Contains(z, "HostUrl=https://example.org/x.zip") {
		t.Error(z)
	}
	for mask, want := range map[uint32]string{0x1F01FF: "FullControl", 0x1301BF: "Modify", 0x1200A9: "ReadAndExecute", 0x120089: "Read", 0x10000000: "FullControl", 0x1: "special (0x1)"} {
		if got := accessRights(mask); got != want {
			t.Errorf("accessRights(%#x) = %s, want %s", mask, got, want)
		}
	}
}

// Get-ChildItem -File / -Directory / -Hidden / -Recurse.
func TestFilesList(t *testing.T) {
	dir := t.TempDir()
	mustWrite(t, filepath.Join(dir, "visible.txt"), "v")
	mustWrite(t, filepath.Join(dir, "sub", "deep.txt"), "d")
	hidden := filepath.Join(dir, ".secret")
	mustWrite(t, hidden, "h")
	if runtime.GOOS == "windows" {
		if err := writeAttrs(hidden, attrHidden); err != nil {
			t.Fatal(err)
		}
	}
	list := func(extra map[string]any) string {
		params := map[string]any{"action": "list", "path": dir}
		for k, v := range extra {
			params[k] = v
		}
		r, err := filesCall(t, &recordingPermissions{}, params)
		if err != nil || r.IsError {
			t.Fatalf("%v %s", err, r.Content)
		}
		return r.Content
	}
	plain := list(nil)
	if !strings.Contains(plain, "visible.txt") || strings.Contains(plain, ".secret") || strings.Contains(plain, "deep.txt") ||
		!strings.Contains(plain, "1 hidden/system not shown") {
		t.Errorf("plain:\n%s", plain)
	}
	if h := list(map[string]any{"hidden": true}); !strings.Contains(h, ".secret") || !strings.Contains(h, "Hidden") {
		t.Errorf("hidden:\n%s", h)
	}
	if rec := list(map[string]any{"recurse": true, "kind": "file"}); !strings.Contains(rec, "deep.txt") || strings.Contains(rec, "<DIR>") {
		t.Errorf("recurse files:\n%s", rec)
	}
	if dirs := list(map[string]any{"kind": "dir"}); !strings.Contains(dirs, "sub") || strings.Contains(dirs, "visible.txt") {
		t.Errorf("dirs:\n%s", dirs)
	}
}

// stubRecycle replaces the Recycle Bin for one test.
func stubRecycle(t *testing.T) *[]string {
	t.Helper()
	var got []string
	old := recycleFunc
	recycleFunc = func(paths []string) error {
		got = append(got, paths...)
		for _, p := range paths {
			if err := forceRemoveAll(p); err != nil {
				return err
			}
		}
		return nil
	}
	t.Cleanup(func() { recycleFunc = old })
	return &got
}

// Remove-Item goes to the Recycle Bin by default; *.tmp with a pattern.
func TestFilesDeleteGoesToTheRecycleBin(t *testing.T) {
	recycled := stubRecycle(t)
	dir := t.TempDir()
	mustWrite(t, filepath.Join(dir, "a.tmp"), "1")
	mustWrite(t, filepath.Join(dir, "keep.txt"), "2")
	mustWrite(t, filepath.Join(dir, "sub", "b.tmp"), "3")
	perms := &recordingPermissions{grant: true}
	r, err := filesCall(t, perms, map[string]any{"action": "delete", "path": dir, "pattern": "*.tmp", "recurse": true})
	if err != nil || r.IsError {
		t.Fatalf("%v %s", err, r.Content)
	}
	if len(*recycled) != 2 || exists(filepath.Join(dir, "a.tmp")) || !exists(filepath.Join(dir, "keep.txt")) {
		t.Errorf("recycled %v:\n%s", *recycled, r.Content)
	}
	if perms.calls[0].Irreversible != "" || !strings.Contains(perms.calls[0].Description, "Recycle Bin") {
		t.Errorf("prompt: %+v", perms.calls[0])
	}
	// A folder with content needs recurse, like Remove-Item.
	sub := filepath.Join(dir, "sub2")
	mustWrite(t, filepath.Join(sub, "x"), "x")
	if r, _ := filesCall(t, perms, map[string]any{"action": "delete", "path": sub}); !r.IsError || !exists(sub) {
		t.Errorf("non-empty folder without recurse: %s", r.Content)
	}
}

// permanent=true: irreversible prompt, read-only files too (-Force); refused
// when the person asked for the trash.
func TestFilesDeletePermanent(t *testing.T) {
	recycled := stubRecycle(t)
	dir := t.TempDir()
	locked := filepath.Join(dir, "LockedFile.txt")
	mustWrite(t, locked, "x")
	if err := writeAttrs(locked, attrReadOnly); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = writeAttrs(locked, 0) })

	NoteUserRequest("files-trash", "move LockedFile.txt to the recycle bin please")
	perms := &recordingPermissions{grant: true}
	r, _ := filesCallIn(t, "files-trash", perms, map[string]any{"action": "delete", "path": locked, "permanent": true})
	if !r.IsError || !strings.Contains(r.Content, "NOT RUN") || len(perms.calls) != 0 || !exists(locked) {
		t.Fatalf("permanent delete after a trash request: %s", r.Content)
	}

	r, err := filesCall(t, perms, map[string]any{"action": "delete", "path": locked, "permanent": true})
	if err != nil || r.IsError {
		t.Fatalf("%v %s", err, r.Content)
	}
	if exists(locked) || len(*recycled) != 0 {
		t.Error("not deleted for good")
	}
	if !strings.Contains(perms.calls[0].Irreversible, PermanentDeleteWhy) {
		t.Errorf("prompt: %+v", perms.calls[0])
	}
}

func TestFilesDeleteRefusesRootsAndWorkingDirectory(t *testing.T) {
	home, _ := os.UserHomeDir()
	for _, p := range []string{home, filepath.VolumeName(home) + string(filepath.Separator), filepathWorkingDir()} {
		perms := &recordingPermissions{grant: true}
		r, _ := filesCall(t, perms, map[string]any{"action": "delete", "path": p, "recurse": true, "dry_run": true})
		if !r.IsError || len(perms.calls) != 0 {
			t.Errorf("%s: %s", p, r.Content)
		}
	}
}

// Paths past MAX_PATH with non-ASCII names: copy, rename and attributes.
func TestFilesLongPathsAndNonASCII(t *testing.T) {
	root := t.TempDir()
	deep := filepath.Join(root, "src")
	for len(deep) < 300 {
		deep = filepath.Join(deep, "ăîșțâ-ünïcödé-folder")
	}
	f := filepath.Join(deep, "fișier-ü.txt")
	mustWrite(t, f, "long")
	dst := filepath.Join(root, "dst")
	perms := &recordingPermissions{grant: true}
	if r, err := filesCall(t, perms, map[string]any{"action": "copy", "path": filepath.Join(root, "src"), "dest": dst}); err != nil || r.IsError {
		t.Fatalf("copy: %v %s", err, r.Content)
	}
	rel, _ := filepath.Rel(filepath.Join(root, "src"), f)
	copied := filepath.Join(dst, rel)
	if len(copied) <= 260 || readString(t, copied) != "long" {
		t.Fatalf("long path copy (%d chars)", len(copied))
	}
	if r, err := filesCall(t, perms, map[string]any{"action": "attributes", "path": copied, "set": "ReadOnly"}); err != nil || r.IsError {
		t.Fatalf("attributes: %v %s", err, r.Content)
	}
	t.Cleanup(func() { _ = writeAttrs(copied, 0) })
	if r, err := filesCall(t, perms, map[string]any{"action": "rename", "path": filepath.Dir(copied), "template": "{n:D2}_{name}"}); err != nil || r.IsError {
		t.Fatalf("rename: %v %s", err, r.Content)
	}
	if !exists(filepath.Join(filepath.Dir(copied), "01_fișier-ü.txt")) {
		t.Error("long path rename")
	}
}

func TestFilesUnknownActionAndMissingPath(t *testing.T) {
	if r, _ := filesCall(t, &recordingPermissions{}, map[string]any{"action": "explode", "path": t.TempDir()}); !r.IsError {
		t.Error("unknown action accepted")
	}
	if r, _ := filesCall(t, &recordingPermissions{}, map[string]any{"action": "info"}); !r.IsError {
		t.Error("missing path accepted")
	}
	if r, _ := filesCall(t, &recordingPermissions{}, map[string]any{"action": "copy", "path": t.TempDir()}); !r.IsError {
		t.Error("copy without dest accepted")
	}
}

func filepathWorkingDir() string { return config.WorkingDirectory() }

// A link inside the source that points back at the source is never followed
// (/XJ; the Go walk does not enter links): the copy ends, and the link's
// content is not copied.
func TestFilesJunctionLoopIsNotFollowed(t *testing.T) {
	root := t.TempDir()
	src, dst := filepath.Join(root, "src"), filepath.Join(root, "dst")
	mustWrite(t, filepath.Join(src, "a.txt"), "a")
	if err := makeDirLink(filepath.Join(src, "loop"), src); err != nil {
		t.Skip("cannot make a folder link here:", err)
	}
	done := make(chan struct{})
	var r ToolResponse
	var err error
	go func() {
		defer close(done)
		r, err = filesCall(t, &recordingPermissions{grant: true}, map[string]any{"action": "copy", "path": src, "dest": dst})
	}()
	select {
	case <-done:
	case <-time.After(60 * time.Second):
		t.Fatal("copy did not finish: the loop was followed")
	}
	if err != nil || r.IsError {
		t.Fatalf("%v %s", err, r.Content)
	}
	if !exists(filepath.Join(dst, "a.txt")) || exists(filepath.Join(dst, "loop", "a.txt")) {
		t.Errorf("loop content copied:\n%s", r.Content)
	}
	// Deleting the source with recurse removes the link, never what it points to.
	victim := filepath.Join(root, "victim")
	mustWrite(t, filepath.Join(victim, "precious.txt"), "p")
	holder := filepath.Join(root, "holder")
	if err := os.Mkdir(holder, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := makeDirLink(filepath.Join(holder, "link"), victim); err != nil {
		t.Fatal(err)
	}
	if r, err := filesCall(t, &recordingPermissions{grant: true}, map[string]any{"action": "delete", "path": holder, "recurse": true, "permanent": true}); err != nil || r.IsError {
		t.Fatalf("%v %s", err, r.Content)
	}
	if !exists(filepath.Join(victim, "precious.txt")) {
		t.Fatal("deleting a folder deleted what a link inside it points to")
	}
}

// The root rules on their own: in the end-to-end test the nesting rule fires
// first, because the temp folder is on the same drive and under the profile.
func TestFilesRefuseRootDirectly(t *testing.T) {
	home, _ := os.UserHomeDir()
	elsewhere := filepath.Join(string(filepath.Separator), "gorilla-files-test-nowhere")
	if runtime.GOOS == "windows" {
		elsewhere = `Z:\gorilla-files-test-nowhere`
		if why := refuseSync("mirror", elsewhere, `C:\`); !strings.Contains(why, "drive root") {
			t.Errorf("mirror to the C: root: %q", why)
		}
	}
	for action, args := range map[string][2]string{
		"mirror": {elsewhere, home}, "purge": {elsewhere, home}, "move": {home, elsewhere}, "move_files": {home, elsewhere},
	} {
		if why := refuseSync(action, args[0], args[1]); !strings.Contains(why, "user profile") {
			t.Errorf("%s %v: %q", action, args, why)
		}
	}
	if why := refuseSync("copy", home, elsewhere); why != "" {
		t.Errorf("copying FROM the profile is allowed: %q", why)
	}
	if why := refuseRoot("destination", t.TempDir()); why != "" {
		t.Errorf("an ordinary folder refused: %q", why)
	}
}

// The Go engine never enters a link (Windows runs robocopy in the tools, so
// this is the only place its own rule runs there).
func TestFilesGoEngineSkipsLinks(t *testing.T) {
	root := t.TempDir()
	src := filepath.Join(root, "src")
	mustWrite(t, filepath.Join(src, "a.txt"), "a")
	if err := makeDirLink(filepath.Join(src, "loop"), src); err != nil {
		t.Skip("cannot make a folder link here:", err)
	}
	plan, err := goPlan(syncOp{Action: "copy", Src: src, Dst: filepath.Join(root, "dst")})
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Copies) != 1 || len(plan.NewDirs) != 0 {
		t.Errorf("the link was entered: %+v", plan)
	}
}
