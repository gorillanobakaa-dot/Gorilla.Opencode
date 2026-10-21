package forensics

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/flate"
	"compress/gzip"
	"encoding/binary"
	"hash/crc32"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// ── Mozilla optimized jar writer (test only) ────────────────────────────────
//
// Mirrors python/mozbuild/mozpack/mozjar.py JarWriter.finish with
// optimize=True: a 4-byte little-endian preload length, the central directory
// (entry offsets already shifted past it), the end record with
// cdir_offset = 4, the local headers and data, and the end record again.
// GORILLA_FORENSICS_FIXTURES=1 go test -run TestWriteOmniJaFixture
// regenerates testdata/omni.ja from omniJaEntries.

type jarEntry struct {
	name    string
	data    []byte
	deflate bool
}

var omniJaEntries = []jarEntry{
	{name: "chrome.manifest", data: []byte("content gorilla gorilla/content/\n")},
	{name: "res/"},
	{name: "modules/Gorilla.sys.mjs", data: []byte(strings.Repeat("export const GORILLA = \"omni\";\n", 40)), deflate: true},
	{name: "defaults/pref/gorilla.js", data: []byte("pref(\"gorilla.fixture\", true);\n"), deflate: true},
}

func writeOptimizedJar(entries []jarEntry) []byte {
	type built struct {
		e      jarEntry
		body   []byte
		method uint16
		crc    uint32
		local  []byte
	}
	var bs []built
	for _, e := range entries {
		b := built{e: e, body: e.data, crc: crc32.ChecksumIEEE(e.data)}
		if e.deflate {
			var buf bytes.Buffer
			w, _ := flate.NewWriter(&buf, 9)
			w.Write(e.data)
			w.Close()
			b.body, b.method = buf.Bytes(), 8
		}
		var lh bytes.Buffer
		le := func(v any) { binary.Write(&lh, binary.LittleEndian, v) }
		le(uint32(0x04034b50))
		le(uint16(10)) // version needed
		le(uint16(0))  // flags
		le(b.method)
		le(uint16(0))      // time
		le(uint16(0x0021)) // date 1980-01-01
		le(b.crc)
		le(uint32(len(b.body)))
		le(uint32(len(e.data)))
		le(uint16(len(e.name)))
		le(uint16(0))
		lh.WriteString(e.name)
		b.local = lh.Bytes()
		bs = append(bs, b)
	}
	central := func(shift uint32) []byte {
		var cd bytes.Buffer
		le := func(v any) { binary.Write(&cd, binary.LittleEndian, v) }
		off := shift
		for _, b := range bs {
			le(uint32(0x02014b50))
			le(uint16(10)) // made by
			le(uint16(10)) // needed
			le(uint16(0))
			le(b.method)
			le(uint16(0))
			le(uint16(0x0021))
			le(b.crc)
			le(uint32(len(b.body)))
			le(uint32(len(b.e.data)))
			le(uint16(len(b.e.name)))
			le(uint16(0)) // extra
			le(uint16(0)) // comment
			le(uint16(0)) // disk
			le(uint16(0)) // internal attrs
			le(uint32(0)) // external attrs
			le(off)
			cd.WriteString(b.e.name)
			off += uint32(len(b.local) + len(b.body))
		}
		return cd.Bytes()
	}
	cdSize := len(central(0))
	end := func() []byte {
		var e bytes.Buffer
		le := func(v any) { binary.Write(&e, binary.LittleEndian, v) }
		le(uint32(0x06054b50))
		le(uint16(0))
		le(uint16(0))
		le(uint16(len(bs)))
		le(uint16(len(bs)))
		le(uint32(cdSize))
		le(uint32(4)) // cdir_offset: right after the preload length
		le(uint16(0))
		return e.Bytes()
	}
	head := uint32(4 + cdSize + 22)
	var out bytes.Buffer
	binary.Write(&out, binary.LittleEndian, head+uint32(len(bs[0].local)+len(bs[0].body)))
	out.Write(central(head))
	out.Write(end())
	for _, b := range bs {
		out.Write(b.local)
		out.Write(b.body)
	}
	out.Write(end())
	return out.Bytes()
}

func TestWriteOmniJaFixture(t *testing.T) {
	if os.Getenv("GORILLA_FORENSICS_FIXTURES") != "1" {
		t.Skip("set GORILLA_FORENSICS_FIXTURES=1 to regenerate testdata/omni.ja")
	}
	if err := os.WriteFile("testdata/omni.ja", writeOptimizedJar(omniJaEntries), 0o644); err != nil {
		t.Fatal(err)
	}
}

// ── omni.ja ─────────────────────────────────────────────────────────────────

func TestOmniJaFixtureIsTheOptimizedLayout(t *testing.T) {
	raw, err := os.ReadFile("testdata/omni.ja")
	if err != nil {
		t.Fatal(err)
	}
	// The layout claim the rest of this file rests on: not a local header
	// first, but a preload length and then the central directory.
	if bytes.HasPrefix(raw, magicZip) || !bytes.Equal(raw[4:8], magicZipCentral) {
		t.Fatalf("testdata/omni.ja is not in the optimized layout: % x", raw[:8])
	}
	// Go's own reader opens it: the evidence that no custom reader is needed.
	zr, err := zip.OpenReader("testdata/omni.ja")
	if err != nil {
		t.Fatalf("archive/zip cannot open the optimized layout: %v", err)
	}
	zr.Close()
}

func TestOmniJaListFindExtract(t *testing.T) {
	out, err := ListArchive("testdata/omni.ja", "")
	if err != nil {
		t.Fatal(err)
	}
	wantAll(t, out,
		"format zip",
		"layout: Mozilla optimized jar (central directory first",
		"entries: 4 (3 files, 1 folders)",
		"chrome.manifest",
		"res/",
	)
	found, err := ListArchive("testdata/omni.ja", "*.mjs")
	if err != nil {
		t.Fatal(err)
	}
	wantAll(t, found, `matching "*.mjs": 1`, "modules/Gorilla.sys.mjs")
	if strings.Contains(found, "chrome.manifest") {
		t.Errorf("find listed a non-match:\n%s", found)
	}
	sub, _ := ListArchive("testdata/omni.ja", "PREF")
	wantAll(t, sub, `matching "PREF": 1`, "defaults/pref/gorilla.js")

	dest := t.TempDir()
	msg, err := ExtractEntry("testdata/omni.ja", "modules/Gorilla.sys.mjs", dest, DefaultMaxCopy)
	if err != nil {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(filepath.Join(dest, "modules", "Gorilla.sys.mjs"))
	if string(got) != string(omniJaEntries[2].data) {
		t.Errorf("extracted bytes differ (%d vs %d)", len(got), len(omniJaEntries[2].data))
	}
	wantAll(t, msg, "extracted modules/Gorilla.sys.mjs", "sha256 ")
}

// The end-record search in archive/zip looks back at most ~64 KiB from the end
// of the file, so an optimized jar bigger than that is the case that would
// break a reader relying on the FRONT end record. Built at test time so the
// checked-in fixture stays small.
func TestLargeOptimizedJarStillOpens(t *testing.T) {
	big := make([]byte, 200<<10)
	for i := range big {
		big[i] = byte(i*7 + i>>8)
	}
	entries := append([]jarEntry{{name: "big.bin", data: big}}, omniJaEntries...)
	p := filepath.Join(t.TempDir(), "big-omni.ja")
	os.WriteFile(p, writeOptimizedJar(entries), 0o644)
	out, err := ListArchive(p, "")
	if err != nil {
		t.Fatal(err)
	}
	wantAll(t, out, "entries: 5", "Mozilla optimized jar")
	dest := t.TempDir()
	if _, err := ExtractEntry(p, "big.bin", dest, DefaultMaxCopy); err != nil {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(filepath.Join(dest, "big.bin"))
	if !bytes.Equal(got, big) {
		t.Error("big.bin came out different")
	}
}

// ── tar, gzip, bzip2 ────────────────────────────────────────────────────────

func TestTarBzip2Fixture(t *testing.T) {
	out, err := ListArchive("testdata/fx.tar.bz2", "")
	if err != nil {
		t.Fatal(err)
	}
	wantAll(t, out, "format tar.bz2", "docs/", "docs/readme.txt", "notes.txt", "(link)")
	dest := t.TempDir()
	if _, err := ExtractEntry("testdata/fx.tar.bz2", "./notes.txt", dest, DefaultMaxCopy); err != nil {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(filepath.Join(dest, "notes.txt"))
	if string(got) != "second file\n" {
		t.Errorf("notes.txt = %q", got)
	}
}

func TestSingleBzip2Fixture(t *testing.T) {
	out, err := ListArchive("testdata/single.txt.bz2", "")
	if err != nil {
		t.Fatal(err)
	}
	wantAll(t, out, "bzip2 (single compressed file)", "? single.txt")
	dest := t.TempDir()
	if _, err := ExtractEntry("testdata/single.txt.bz2", "single.txt", dest, DefaultMaxCopy); err != nil {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(filepath.Join(dest, "single.txt"))
	if string(got) != "GORILLA_BZIP2_SINGLE\n" {
		t.Errorf("single.txt = %q", got)
	}
}

func TestTarGzAndPlainGzip(t *testing.T) {
	dir := t.TempDir()
	var tb bytes.Buffer
	tw := tar.NewWriter(&tb)
	tw.WriteHeader(&tar.Header{Name: "a/b.txt", Mode: 0o644, Size: 3, Typeflag: tar.TypeReg})
	tw.Write([]byte("abc"))
	tw.Close()
	var gzb bytes.Buffer
	gw := gzip.NewWriter(&gzb)
	gw.Write(tb.Bytes())
	gw.Close()
	tgz := filepath.Join(dir, "x.tgz")
	os.WriteFile(tgz, gzb.Bytes(), 0o644)
	out, err := ListArchive(tgz, "")
	if err != nil {
		t.Fatal(err)
	}
	wantAll(t, out, "format tar.gz", "3 a/b.txt")

	var one bytes.Buffer
	gw = gzip.NewWriter(&one)
	gw.Name = "inner-name.log"
	gw.Write([]byte("hello gzip"))
	gw.Close()
	gz := filepath.Join(dir, "whatever.gz")
	os.WriteFile(gz, one.Bytes(), 0o644)
	out, err = ListArchive(gz, "")
	if err != nil {
		t.Fatal(err)
	}
	wantAll(t, out, "gzip (single compressed file)", "10 inner-name.log", "gzip trailer")
}

func TestStreamCapIsReported(t *testing.T) {
	old := streamReadCap
	streamReadCap = 2048
	defer func() { streamReadCap = old }()
	var tb bytes.Buffer
	tw := tar.NewWriter(&tb)
	for i := 0; i < 20; i++ {
		tw.WriteHeader(&tar.Header{Name: strings.Repeat("f", i+1), Mode: 0o644, Size: 512, Typeflag: tar.TypeReg})
		tw.Write(make([]byte, 512))
	}
	tw.Close()
	var gzb bytes.Buffer
	gw := gzip.NewWriter(&gzb)
	gw.Write(tb.Bytes())
	gw.Close()
	p := filepath.Join(t.TempDir(), "bomb.tar.gz")
	os.WriteFile(p, gzb.Bytes(), 0o644)
	out, err := ListArchive(p, "")
	if err != nil {
		t.Fatal(err)
	}
	wantAll(t, out, "stream capped: stopped after 2048")
}

func TestUnsupportedFormatsSaySo(t *testing.T) {
	p := filepath.Join(t.TempDir(), "x.xz")
	os.WriteFile(p, []byte{0xfd, '7', 'z', 'X', 'Z', 0, 1, 2}, 0o644)
	if _, err := ListArchive(p, ""); err == nil || !strings.Contains(err.Error(), "it is xz") {
		t.Errorf("xz: err = %v", err)
	}
}

// ── refusals ────────────────────────────────────────────────────────────────

func TestSafeEntryPath(t *testing.T) {
	ok := map[string]string{
		"a.txt":            "a.txt",
		"./dir/a.txt":      "dir/a.txt",
		`dir\sub\a.txt`:    "dir/sub/a.txt",
		"chrome/en-US/x.p": "chrome/en-US/x.p",
	}
	for in, want := range ok {
		if got, err := SafeEntryPath(in); err != nil || got != want {
			t.Errorf("SafeEntryPath(%q) = %q, %v; want %q", in, got, err, want)
		}
	}
	bad := []string{
		"../evil.txt", "a/../../evil.txt", `..\evil.txt`, `a\..\..\evil.txt`,
		"/etc/passwd", `\Windows\x`, "C:/Windows/x", "c:evil", "file.txt:stream",
		"CON", "aux.txt", "dir/COM1.log", "lpt9", "trailing.", "trailing ",
		"", "dir/", "a//b", "a/./b", "a\x00b", "a\nb",
	}
	for _, in := range bad {
		if got, err := SafeEntryPath(in); err == nil {
			t.Errorf("SafeEntryPath(%q) accepted as %q", in, got)
		}
	}
	// Each rule on its own, by its reason: several names trip more than one
	// rule (".." also ends in a dot), and a test that only asks "refused?"
	// cannot tell when one of them stops working (fieldkit mutate showed it).
	reasons := map[string]string{
		"../evil.txt":      "path traversal",
		"a/../../evil.txt": "path traversal",
		"/etc/passwd":      "absolute path",
		`\Windows\x`:       "absolute path",
		"C:/Windows/x":     "drive letter",
		"aux.txt":          "device name",
		"trailing.":        "trailing dot",
		"a//b":             "empty path segment",
	}
	for in, want := range reasons {
		if _, err := SafeEntryPath(in); err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("SafeEntryPath(%q): err = %v, want reason %q", in, err, want)
		}
	}
}

// An archive whose entries try to climb out must not write a single byte
// outside the destination, whatever the entry name.
func TestTraversalEntriesWriteNothing(t *testing.T) {
	root := t.TempDir()
	dest := filepath.Join(root, "dest")
	os.Mkdir(dest, 0o755)
	evil := []string{"../evil1.txt", "a/../../evil2.txt", `..\evil3.txt`, "/evil4.txt", "C:/evil5.txt"}
	var zb bytes.Buffer
	zw := zip.NewWriter(&zb)
	for _, n := range evil {
		w, err := zw.CreateHeader(&zip.FileHeader{Name: n, Method: zip.Store})
		if err != nil {
			t.Fatal(err)
		}
		w.Write([]byte("pwned"))
	}
	zw.Close()
	zp := filepath.Join(root, "evil.zip")
	os.WriteFile(zp, zb.Bytes(), 0o644)

	var tb bytes.Buffer
	tw := tar.NewWriter(&tb)
	for _, n := range evil {
		tw.WriteHeader(&tar.Header{Name: n, Mode: 0o644, Size: 5, Typeflag: tar.TypeReg})
		tw.Write([]byte("pwned"))
	}
	tw.WriteHeader(&tar.Header{Name: "link", Linkname: "../../outside", Typeflag: tar.TypeSymlink})
	tw.Close()
	tp := filepath.Join(root, "evil.tar")
	os.WriteFile(tp, tb.Bytes(), 0o644)

	for _, arc := range []string{zp, tp} {
		for _, n := range evil {
			if msg, err := ExtractEntry(arc, n, dest, DefaultMaxCopy); err == nil {
				t.Errorf("%s: %q extracted: %s", filepath.Base(arc), n, msg)
			}
		}
	}
	if _, err := ExtractEntry(tp, "link", dest, DefaultMaxCopy); err == nil || !strings.Contains(err.Error(), "links are never extracted") {
		t.Errorf("symlink entry: err = %v", err)
	}
	// Nothing anywhere under root except the two archives and the empty dest.
	var stray []string
	filepath.Walk(root, func(p string, fi os.FileInfo, err error) error {
		if err == nil && !fi.IsDir() && p != zp && p != tp {
			stray = append(stray, p)
		}
		return nil
	})
	if len(stray) > 0 {
		t.Errorf("files written: %v", stray)
	}
}

func TestExtractRefusals(t *testing.T) {
	dest := t.TempDir()
	if _, err := ExtractEntry("testdata/omni.ja", "res/", dest, DefaultMaxCopy); err == nil {
		t.Error("folder entry extracted")
	}
	if _, err := ExtractEntry("testdata/omni.ja", "modules/Gorilla.sys.mjs", dest, 100); err == nil || !strings.Contains(err.Error(), "extraction cap") {
		t.Errorf("over-cap entry: err = %v", err)
	}
	if _, err := ExtractEntry("testdata/omni.ja", "chrome.manifest", dest, DefaultMaxCopy); err != nil {
		t.Fatal(err)
	}
	if _, err := ExtractEntry("testdata/omni.ja", "chrome.manifest", dest, DefaultMaxCopy); err == nil || !strings.Contains(err.Error(), "already exists") {
		t.Errorf("overwrite: err = %v", err)
	}
	_, err := ExtractEntry("testdata/omni.ja", "Gorilla.sys.mjs", dest, DefaultMaxCopy)
	if err == nil || !strings.Contains(err.Error(), "similar: modules/Gorilla.sys.mjs") {
		t.Errorf("missing entry: err = %v", err)
	}
}

// A link already inside the destination must not carry the write elsewhere.
func TestExtractRefusesLinkedFolderInDestination(t *testing.T) {
	root := t.TempDir()
	dest := filepath.Join(root, "dest")
	outside := filepath.Join(root, "outside")
	os.Mkdir(dest, 0o755)
	os.Mkdir(outside, 0o755)
	link := filepath.Join(dest, "modules")
	if err := os.Symlink(outside, link); err != nil {
		// Windows without Developer Mode refuses symlinks; a junction needs no
		// privilege and is the link a hostile project would plant there.
		if runtime.GOOS != "windows" {
			t.Skipf("cannot create a symlink here: %v", err)
		}
		if out, err := exec.Command("cmd", "/c", "mklink", "/J", link, outside).CombinedOutput(); err != nil {
			t.Skipf("cannot create a junction here: %v %s", err, out)
		}
	}
	if _, err := ExtractEntry("testdata/omni.ja", "modules/Gorilla.sys.mjs", dest, DefaultMaxCopy); err == nil {
		t.Error("extracted through a linked folder")
	}
	if ents, _ := os.ReadDir(outside); len(ents) > 0 {
		t.Errorf("wrote outside: %v", ents)
	}
}
