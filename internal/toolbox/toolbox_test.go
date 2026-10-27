package toolbox

import (
	"bytes"
	"compress/gzip"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"testing/fstest"
	"time"
)

// A fake toolbox: no real helper is needed, so these run on every platform and
// in a plain `go test` as well as with -tags toolbox.
const plat = "windows-amd64"

func fakeBox(t *testing.T, files map[string][]byte, pins map[string]string) (*Box, string) {
	t.Helper()
	fsys := fstest.MapFS{}
	for name, data := range files {
		fsys[plat+"/"+name] = &fstest.MapFile{Data: data}
	}
	root := t.TempDir()
	return New(fsys, plat, pins, func() (string, error) { return root, nil }), root
}

func gz(t *testing.T, b []byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	w := gzip.NewWriter(&buf)
	if _, err := w.Write(b); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func TestParsePinsHash(t *testing.T) {
	sum := strings.Repeat("ab", 32)
	pins, err := ParsePins("# comment\n\n" + strings.ToUpper(sum) + "  windows-amd64/sd.exe\n" + sum + " *windows-amd64/x.exe\n")
	if err != nil {
		t.Fatal(err)
	}
	if pins["windows-amd64/sd.exe"] != sum || pins["windows-amd64/x.exe"] != sum {
		t.Fatalf("pins = %v", pins)
	}
	for _, bad := range []string{
		"abc  windows-amd64/sd.exe",                         // not a sha256
		strings.Repeat("zz", 32) + "  windows-amd64/sd.exe", // not hex
		sum + "  a  b", // three fields
		sum + "  windows-amd64/sd.exe\n" + sum + "  windows-amd64/sd.exe", // pinned twice
	} {
		if _, err := ParsePins(bad); err == nil {
			t.Errorf("ParsePins(%q) accepted a bad list", bad)
		}
	}
}

func TestDefaultPinsParseHash(t *testing.T) {
	pins, err := ParsePins(pinsText)
	if err != nil {
		t.Fatalf("the committed SHA256SUMS does not parse: %v", err)
	}
	for _, want := range []string{"pfind.exe", "sd.exe", "ast-grep.exe", "coreutils.exe", "difft.exe", "hyperfine.exe", "ninja.exe"} {
		if _, ok := pins["windows-amd64/"+want]; !ok {
			t.Errorf("SHA256SUMS has no line for windows-amd64/%s", want)
		}
	}
	if _, ok := pins["windows-amd64/sg.exe"]; ok {
		t.Error("sg.exe is upstream's deprecated 50 MB alias of ast-grep and must not be shipped")
	}
	if _, err := Default(); err != nil {
		t.Fatal(err)
	}
}

func TestExtractWritesHashNamedFolderWithPlainName(t *testing.T) {
	data := []byte("helper bytes")
	sum := hashBytes(data)
	b, root := fakeBox(t, map[string][]byte{"sd.exe": data}, map[string]string{plat + "/sd.exe": sum})
	p, err := b.Path("sd")
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(root, "gorilla-opencode", "toolbox", sum, "sd.exe")
	if p != want {
		t.Fatalf("Path = %s, want %s", p, want)
	}
	got, err := os.ReadFile(p)
	if err != nil || !bytes.Equal(got, data) {
		t.Fatalf("extracted bytes = %q, %v", got, err)
	}
	if runtime.GOOS != "windows" {
		for _, d := range []string{filepath.Dir(p), filepath.Dir(filepath.Dir(p))} {
			st, err := os.Stat(d)
			if err != nil || st.Mode().Perm()&0o077 != 0 {
				t.Errorf("%s mode %v, want 0700", d, st.Mode().Perm())
			}
		}
	}
	// No temp file is left behind.
	entries, _ := os.ReadDir(filepath.Dir(p))
	if len(entries) != 1 {
		t.Errorf("extract folder holds %d entries, want only sd.exe", len(entries))
	}
}

func TestExtractGzipCopyHashIsOfUncompressedBytes(t *testing.T) {
	data := bytes.Repeat([]byte("compressible "), 1000)
	sum := hashBytes(data)
	b, _ := fakeBox(t, map[string][]byte{"ninja.exe.gz": gz(t, data)}, map[string]string{plat + "/ninja.exe": sum})
	if names := b.Names(); len(names) != 1 || names[0] != "ninja" {
		t.Fatalf("Names = %v", names)
	}
	p, err := b.Path("ninja")
	if err != nil {
		t.Fatal(err)
	}
	if got, _ := os.ReadFile(p); !bytes.Equal(got, data) {
		t.Fatal("the extracted file is not the decompressed helper")
	}
}

func TestHashMismatchEmbeddedIsRefused(t *testing.T) {
	b, root := fakeBox(t, map[string][]byte{"sd.exe": []byte("tampered build")},
		map[string]string{plat + "/sd.exe": hashBytes([]byte("the pinned build"))})
	_, err := b.Path("sd")
	if err == nil || !strings.Contains(err.Error(), "refusing") {
		t.Fatalf("err = %v, want a refusal", err)
	}
	if _, serr := os.Stat(filepath.Join(root, "gorilla-opencode")); !os.IsNotExist(serr) {
		t.Error("a refused helper must not be extracted at all")
	}
}

func TestHashMissingPinIsRefused(t *testing.T) {
	b, _ := fakeBox(t, map[string][]byte{"sd.exe": []byte("x")}, map[string]string{})
	if _, err := b.Path("sd"); err == nil || !strings.Contains(err.Error(), "not pinned") {
		t.Fatalf("err = %v, want 'not pinned'", err)
	}
}

func TestHashCheckedBeforeEveryExecRewritesTamperedFile(t *testing.T) {
	data := []byte("good helper")
	b, _ := fakeBox(t, map[string][]byte{"sd.exe": data}, map[string]string{plat + "/sd.exe": hashBytes(data)})
	p, err := b.Path("sd")
	if err != nil {
		t.Fatal(err)
	}
	// Same size, other bytes, old timestamp: only a hash notices.
	if err := os.WriteFile(p, []byte("evil helper"), 0o700); err != nil {
		t.Fatal(err)
	}
	old := time.Now().Add(-time.Hour)
	_ = os.Chtimes(p, old, old)
	p2, err := b.Path("sd")
	if err != nil || p2 != p {
		t.Fatalf("Path after tamper = %s, %v", p2, err)
	}
	if got, _ := os.ReadFile(p); !bytes.Equal(got, data) {
		t.Fatalf("tampered file was handed out again: %q", got)
	}
	if err := os.Remove(p); err != nil {
		t.Fatal(err)
	}
	if _, err := b.Path("sd"); err != nil {
		t.Fatal(err)
	}
	if got, _ := os.ReadFile(p); !bytes.Equal(got, data) {
		t.Fatal("a deleted extract was not restored")
	}
}

func TestExtractNotEmbedded(t *testing.T) {
	var nilBox *Box
	if _, err := nilBox.Path("sd"); !errors.Is(err, ErrNotEmbedded) {
		t.Errorf("nil box: %v", err)
	}
	empty := New(nil, plat, nil, CacheRoot)
	if _, err := empty.Path("sd"); !errors.Is(err, ErrNotEmbedded) {
		t.Errorf("no fs: %v", err)
	}
	b, _ := fakeBox(t, map[string][]byte{}, map[string]string{})
	if _, err := b.Path("sd"); !errors.Is(err, ErrNotEmbedded) {
		t.Errorf("absent helper: %v", err)
	}
	if dirs, err := empty.Dirs(); len(dirs) != 0 || err != nil {
		t.Errorf("Dirs of an empty box = %v, %v", dirs, err)
	}
}

func TestExtractRefusesSharedCacheDir(t *testing.T) {
	data := []byte("h")
	pins := map[string]string{plat + "/sd.exe": hashBytes(data)}
	fsys := fstest.MapFS{plat + "/sd.exe": &fstest.MapFile{Data: data}}
	var root string
	if runtime.GOOS == "windows" {
		// Outside the profile and writable by every local user.
		public := os.Getenv("PUBLIC")
		if public == "" {
			public = `C:\Users\Public`
		}
		root = filepath.Join(public, fmt.Sprintf("gorilla-toolbox-test-%d", time.Now().UnixNano()))
	} else {
		root = t.TempDir()
		if err := os.Chmod(root, 0o777); err != nil {
			t.Fatal(err)
		}
		shared := filepath.Join(root, "gorilla-opencode")
		if err := os.Mkdir(shared, 0o777); err != nil {
			t.Fatal(err)
		}
		if err := os.Chmod(shared, 0o777); err != nil {
			t.Fatal(err)
		}
	}
	b := New(fsys, plat, pins, func() (string, error) { return root, nil })
	_, err := b.Path("sd")
	if err == nil {
		t.Fatal("extracted into a folder other users can write to")
	}
	if runtime.GOOS == "windows" {
		if _, serr := os.Stat(root); !os.IsNotExist(serr) {
			os.RemoveAll(root) // made by this test only
			t.Fatal("a folder was created outside the profile before the check")
		}
	}
}

func TestMentioned(t *testing.T) {
	b, _ := fakeBox(t, map[string][]byte{"sd.exe": nil, "ast-grep.exe": nil, "README.md": nil}, nil)
	cases := map[string][]string{
		"sd foo bar file.txt":             {"sd"},
		"Get-Content x | sd.exe a b":      {"sd"},
		"& 'ast-grep' run -p x":           {"ast-grep"},
		"cd src; ast-grep scan && sd x y": {"ast-grep", "sd"},
		"echo sdx; csd; mysd":             nil,
		"git status":                      nil,
	}
	for cmd, want := range cases {
		got := b.Mentioned(cmd)
		if fmt.Sprint(got) != fmt.Sprint(want) {
			t.Errorf("Mentioned(%q) = %v, want %v", cmd, got, want)
		}
	}
}

func TestVerifyMentionedExtractsOnlyNamedHelpers(t *testing.T) {
	a, c := []byte("a"), []byte("c")
	b, root := fakeBox(t, map[string][]byte{"sd.exe": a, "ninja.exe": c},
		map[string]string{plat + "/sd.exe": hashBytes(a), plat + "/ninja.exe": hashBytes(c)})
	if err := b.VerifyMentioned("sd x y"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, "gorilla-opencode", "toolbox", hashBytes(a), "sd.exe")); err != nil {
		t.Error("sd was not extracted")
	}
	if _, err := os.Stat(filepath.Join(root, "gorilla-opencode", "toolbox", hashBytes(c))); !os.IsNotExist(err) {
		t.Error("ninja was extracted although the command does not call it")
	}
}

func TestPrependPath(t *testing.T) {
	sep := string(os.PathListSeparator)
	key := "PATH"
	if runtime.GOOS == "windows" {
		key = "Path"
	}
	env := PrependPath([]string{"A=1", key + "=old"}, []string{"d1", "d2"})
	if env[1] != key+"=d1"+sep+"d2"+sep+"old" {
		t.Errorf("env = %v", env)
	}
	if got := PrependPath([]string{"A=1"}, []string{"d1"}); got[len(got)-1] != "PATH=d1" {
		t.Errorf("missing PATH: %v", got)
	}
	if got := PrependPath([]string{"PATH=x"}, nil); got[0] != "PATH=x" {
		t.Errorf("no dirs: %v", got)
	}
}

func TestIsToolboxFile(t *testing.T) {
	root, err := CacheRoot()
	if err != nil {
		t.Skip(err)
	}
	if !IsToolboxFile(filepath.Join(root, "gorilla-opencode", "toolbox", "abc", "pfind.exe")) {
		t.Error("a file in the toolbox folder is not recognized")
	}
	if IsToolboxFile(filepath.Join(root, "gorilla-opencode", "pfind.exe")) {
		t.Error("a file outside the toolbox folder is recognized")
	}
}
