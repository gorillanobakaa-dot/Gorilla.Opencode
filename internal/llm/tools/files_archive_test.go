package tools

import (
	"archive/zip"
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"testing"

	"github.com/opencode-ai/opencode/internal/permission"
)

// The owner, on GPT-6-Luna (2026-10-27): "list all your files and their
// attributes in the working folder" was answered with the attribute flags
// only. One list call with details=true now carries owner, created and the
// permissions of every entry, and a plain list says that switch exists.
func TestFilesListDetailsAnswersListAllYourFilesAndTheirAttributes(t *testing.T) {
	dir := t.TempDir()
	mustWrite(t, filepath.Join(dir, "a.txt"), "a")
	mustWrite(t, filepath.Join(dir, "sub", "b.txt"), "b")
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
	if plain := list(nil); !strings.Contains(plain, "details=true") {
		t.Errorf("a plain listing does not name details=true:\n%s", plain)
	}
	got := list(map[string]any{"details": true, "recurse": true})
	want := []string{"owner: "}
	if runtime.GOOS == "windows" {
		want = append(want, "created: ", "acl: ")
	} else {
		want = append(want, "mode: ")
	}
	for _, name := range []string{"a.txt", "sub", "b.txt"} {
		var block string
		lines := strings.Split(got, "\n")
		for i, l := range lines {
			if strings.HasSuffix(strings.TrimSpace(l), name) && i+1 < len(lines) {
				block = lines[i+1]
			}
		}
		for _, w := range want {
			if !strings.Contains(block, w) {
				t.Errorf("%s: details line lacks %q: %q\n%s", name, w, block, got)
			}
		}
	}
	if !strings.Contains(filesDescription, "owner, created, permissions") ||
		!strings.Contains(filesDescription, "streams and Mark of the Web") ||
		!strings.Contains(filesDescription, "details=true") {
		t.Errorf("the description does not say what info/details cover:\n%s", filesDescription)
	}
}

func zipNames(t *testing.T, p string) map[string]uint16 {
	t.Helper()
	zr, err := zip.OpenReader(p)
	if err != nil {
		t.Fatalf("open %s: %v", p, err)
	}
	defer zr.Close()
	out := map[string]uint16{}
	for _, f := range zr.File {
		out[f.Name] = f.Method
	}
	return out
}

func onlyEntry(t *testing.T, dir string) string {
	t.Helper()
	es, _ := os.ReadDir(dir)
	var names []string
	for _, e := range es {
		names = append(names, e.Name())
	}
	sort.Strings(names)
	return strings.Join(names, ",")
}

// The owner, on GPT-6-Luna (2026-10-27): "archive all the files in the
// working folder". The model hand-wrote .NET ZipFile, put the zip INSIDE the
// folder and asked nothing. Now: one action, asked first, the zip beside the
// folder, and the reply names the defaults used and the options.
func TestFilesArchiveAllTheFilesInTheWorkingFolder(t *testing.T) {
	root := t.TempDir()
	work := filepath.Join(root, "work")
	mustWrite(t, filepath.Join(work, "a.txt"), strings.Repeat("a", 4000))
	mustWrite(t, filepath.Join(work, "sub", "b.txt"), "b")

	deny := &recordingPermissions{grant: false}
	if _, err := filesCall(t, deny, map[string]any{"action": "archive", "path": work}); !errors.Is(err, permission.ErrorPermissionDenied) {
		t.Fatalf("denied archive: err = %v", err)
	}
	if len(deny.calls) != 1 {
		t.Fatalf("asked %d times, want once", len(deny.calls))
	}
	if d := deny.calls[0].Description; !strings.Contains(d, "defaults used") || !strings.Contains(d, "options:") {
		t.Errorf("the question does not show defaults and options:\n%s", d)
	}
	if got := onlyEntry(t, root); got != "work" {
		t.Fatalf("a denied archive wrote something: %s", got)
	}

	allow := &recordingPermissions{grant: true}
	r, err := filesCall(t, allow, map[string]any{"action": "archive", "path": work})
	if err != nil || r.IsError {
		t.Fatalf("archive: %v %s", err, r.Content)
	}
	for _, want := range []string{"DONE", "format zip, level 6, no password",
		"defaults used (not asked for): format zip, level 6 (normal), no password, saved beside the folder, not inside it",
		"options: format=zip|7z|tar.xz", "password=", "dest="} {
		if !strings.Contains(r.Content, want) {
			t.Errorf("reply lacks %q:\n%s", want, r.Content)
		}
	}
	if got := onlyEntry(t, work); got != "a.txt,sub" {
		t.Errorf("the archive went inside the folder it packs: %s", got)
	}
	es, _ := os.ReadDir(root)
	var zipPath string
	for _, e := range es {
		if strings.HasPrefix(e.Name(), "work-") && strings.HasSuffix(e.Name(), ".zip") {
			zipPath = filepath.Join(root, e.Name())
		}
	}
	if zipPath == "" {
		t.Fatalf("no work-<date>.zip beside the folder: %s", onlyEntry(t, root))
	}
	names := zipNames(t, zipPath)
	for _, n := range []string{"work/a.txt", "work/sub/", "work/sub/b.txt"} {
		if _, ok := names[n]; !ok {
			t.Errorf("zip lacks %s: %v", n, names)
		}
	}
	if names["work/a.txt"] != zip.Deflate {
		t.Errorf("a.txt method %d, want Deflate", names["work/a.txt"])
	}
	if exists(zipPath + ".partial") {
		t.Error("the .partial file was left behind")
	}

	// Refused before asking: dest inside the folder, an existing dest, a
	// password on a format that cannot carry one, an unknown level.
	for _, c := range []map[string]any{
		{"dest": filepath.Join(work, "in.zip")},
		{"dest": zipPath},
		{"format": "7z", "password": "pw"},
		{"level": 12},
		{"format": "rar"},
	} {
		perms := &recordingPermissions{grant: true}
		params := map[string]any{"action": "archive", "path": work}
		for k, v := range c {
			params[k] = v
		}
		r, err := filesCall(t, perms, params)
		if err != nil || !r.IsError || len(perms.calls) != 0 {
			t.Errorf("%v: not refused before asking (err %v, asked %d): %s", c, err, len(perms.calls), r.Content)
		}
	}
	if exists(filepath.Join(work, "in.zip")) {
		t.Error("an archive was written inside the folder")
	}

	// A dest folder gets the default name; level 0 stores.
	out := filepath.Join(root, "out")
	if err := os.Mkdir(out, 0o755); err != nil {
		t.Fatal(err)
	}
	r, err = filesCall(t, allow, map[string]any{"action": "archive", "path": work, "dest": out, "level": 0})
	if err != nil || r.IsError || strings.Contains(r.Content, "level 6 (normal)") {
		t.Fatalf("level 0 into a folder: %v %s", err, r.Content)
	}
	es, _ = os.ReadDir(out)
	if len(es) != 1 || zipNames(t, filepath.Join(out, es[0].Name()))["work/a.txt"] != zip.Store {
		t.Errorf("level 0 did not store into %s: %v", out, es)
	}
}

// 7z, tar.xz and zip with a password go to Windows' own tar.exe; each file
// starts with its format's signature, and the password zip's entries are
// AES-encrypted (method 99), never stored in the clear.
func TestFilesArchiveFormatsWithWindowsTar(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("bsdtar is System32\\tar.exe on Windows; elsewhere it is optional")
	}
	root := t.TempDir()
	work := filepath.Join(root, "work")
	mustWrite(t, filepath.Join(work, "secret.txt"), "the plain text inside")
	allow := &recordingPermissions{grant: true}
	for _, c := range []struct {
		format, password, dest string
		magic                  []byte
	}{
		{"7z", "", "w.7z", []byte("7z\xBC\xAF\x27\x1C")},
		{"tar.xz", "", "w", []byte("\xFD7zXZ\x00")},
		{"zip", "pw-123", "w.zip", []byte("PK\x03\x04")},
	} {
		params := map[string]any{"action": "archive", "path": work, "format": c.format, "dest": filepath.Join(root, c.dest)}
		if c.password != "" {
			params["password"] = c.password
		}
		r, err := filesCall(t, allow, params)
		if err != nil || r.IsError {
			t.Fatalf("%s: %v %s", c.format, err, r.Content)
		}
		if strings.Contains(r.Content, "pw-123") {
			t.Errorf("the reply echoes the password:\n%s", r.Content)
		}
		p := filepath.Join(root, c.dest)
		if c.format == "tar.xz" {
			p += ".tar.xz"
		}
		b, err := os.ReadFile(p)
		if err != nil || !bytes.HasPrefix(b, c.magic) {
			t.Fatalf("%s: %s is not a %s file (%v)", c.format, p, c.format, err)
		}
		if c.password != "" {
			if bytes.Contains(b, []byte("the plain text inside")) {
				t.Error("the password zip holds the text in the clear")
			}
			if m := zipNames(t, p)["work/secret.txt"]; m != 99 {
				t.Errorf("password zip entry method %d, want 99 (AES)", m)
			}
		}
	}
	for _, call := range allow.calls {
		if strings.Contains(call.Description, "pw-123") {
			t.Errorf("the permission prompt shows the password:\n%s", call.Description)
		}
	}
}
