package forensics

import (
	"archive/tar"
	"archive/zip"
	"bufio"
	"bytes"
	"compress/bzip2"
	"compress/gzip"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"
)

// Archive caps. Each one is reported when hit.
const (
	maxListed      = 200
	maxFound       = 100
	DefaultMaxCopy = 256 << 20 // largest entry Extract writes
)

// streamReadCap bounds the decompressed bytes read while walking a compressed
// stream. A variable only so a test can lower it.
var streamReadCap int64 = 2 << 30

// archiveEntry is one member of an archive.
type archiveEntry struct {
	name   string
	size   int64 // uncompressed; -1 when unknown
	kind   string
	method string // zip: stored/deflate
}

const (
	kindFile = "file"
	kindDir  = "dir"
	kindLink = "link"
)

// walker visits every entry in order. open reads the current entry's bytes
// and is valid only during the visit. visit returns true to stop.
type walker func(visit func(e archiveEntry, open func() (io.Reader, error)) bool) error

// openedArchive is an archive ready to walk.
type openedArchive struct {
	format string
	note   string
	walk   walker
	// capped is set when a compressed stream was cut off at streamReadCap.
	capped *bool
	close  func()
}

// ListArchive lists the entries of a zip (jar, xpi, omni.ja), tar, tar.gz,
// tar.bz2, gzip or bzip2 file. A non-empty pattern turns it into a find:
// a glob (*, ?, [) is matched against the full name and the base name,
// anything else is a case-insensitive substring.
func ListArchive(p string, pattern string) (string, error) {
	a, err := openArchive(p)
	if err != nil {
		return "", err
	}
	defer a.close()
	st, _ := os.Stat(p)

	var (
		rows               []string
		total, files, dirs int
		matched            int
		sum                int64
		sizeUnknown        bool
	)
	limit := maxListed
	if pattern != "" {
		limit = maxFound
	}
	err = a.walk(func(e archiveEntry, _ func() (io.Reader, error)) bool {
		total++
		switch e.kind {
		case kindDir:
			dirs++
		default:
			files++
		}
		if e.size >= 0 {
			sum += e.size
		} else {
			sizeUnknown = true
		}
		if pattern != "" && !matchEntry(e.name, pattern) {
			return false
		}
		matched++
		if len(rows) < limit {
			rows = append(rows, entryRow(e))
		}
		return false
	})
	if err != nil {
		return "", err
	}

	var b strings.Builder
	fmt.Fprintf(&b, "archive: %s (%d bytes), format %s\n", filepath.Base(p), st.Size(), a.format)
	if a.note != "" {
		b.WriteString(a.note + "\n")
	}
	approx := ""
	if sizeUnknown {
		approx = " (some sizes unknown)"
	}
	fmt.Fprintf(&b, "entries: %d (%d files, %d folders), %d bytes uncompressed%s\n", total, files, dirs, sum, approx)
	if pattern != "" {
		fmt.Fprintf(&b, "matching %q: %d\n", pattern, matched)
	}
	b.WriteString("bytes, name (folders end in /)\n")
	for _, r := range rows {
		b.WriteString("  " + r + "\n")
	}
	if matched > len(rows) {
		fmt.Fprintf(&b, "  ... %d more not shown (capped at %d; use a pattern to narrow)\n", matched-len(rows), limit)
	}
	if *a.capped {
		fmt.Fprintf(&b, "stream capped: stopped after %d decompressed bytes; the listing is incomplete\n", int64(streamReadCap))
	}
	return b.String(), nil
}

func entryRow(e archiveEntry) string {
	name := clean(e.name)
	size := fmt.Sprintf("%10d", e.size)
	if e.size < 0 {
		size = fmt.Sprintf("%10s", "?")
	}
	switch e.kind {
	case kindDir:
		if !strings.HasSuffix(name, "/") {
			name += "/"
		}
	case kindLink:
		name += " (link)"
	}
	return size + " " + name
}

func matchEntry(name, pattern string) bool {
	if strings.ContainsAny(pattern, "*?[") {
		trimmed := strings.TrimSuffix(name, "/")
		if ok, _ := path.Match(pattern, trimmed); ok {
			return true
		}
		ok, _ := path.Match(pattern, path.Base(trimmed))
		return ok
	}
	return strings.Contains(strings.ToLower(name), strings.ToLower(pattern))
}

// ExtractEntry copies ONE regular-file entry out of an archive into destDir,
// keeping its relative path. It refuses, without writing anything, an entry
// name that is absolute or climbs out (".."), a name Windows would reinterpret
// (drive letters, ":" streams, device names, trailing dots), links and
// folders, an existing file, a destination path that passes through a link,
// and an entry larger than maxBytes. destDir must already be the caller's
// checked, link-resolved workspace folder.
func ExtractEntry(archivePath, entryName, destDir string, maxBytes int64) (string, error) {
	rel, err := SafeEntryPath(entryName)
	if err != nil {
		return "", err
	}
	a, err := openArchive(archivePath)
	if err != nil {
		return "", err
	}
	defer a.close()

	var (
		result  string
		opErr   error
		found   bool
		similar []string
	)
	want := normEntryName(entryName)
	walkErr := a.walk(func(e archiveEntry, open func() (io.Reader, error)) bool {
		if normEntryName(e.name) != want {
			if len(similar) < 5 && strings.Contains(strings.ToLower(e.name), strings.ToLower(path.Base(want))) {
				similar = append(similar, e.name)
			}
			return false
		}
		found = true
		switch {
		case e.kind == kindDir:
			opErr = fmt.Errorf("%s is a folder; extract the files inside it one at a time", entryName)
		case e.kind != kindFile:
			opErr = fmt.Errorf("%s is a %s, not a regular file; links are never extracted", entryName, e.kind)
		case e.size > maxBytes:
			opErr = fmt.Errorf("%s is %d bytes, over the %d-byte extraction cap", entryName, e.size, maxBytes)
		default:
			result, opErr = writeEntry(open, rel, destDir, maxBytes)
		}
		return true
	})
	if walkErr != nil {
		return "", walkErr
	}
	if !found {
		msg := fmt.Sprintf("no entry named %q in %s", entryName, filepath.Base(archivePath))
		if len(similar) > 0 {
			msg += "; similar: " + clean(strings.Join(similar, ", "))
		}
		return "", errors.New(msg + " (names are exact; action=find searches)")
	}
	return result, opErr
}

// normEntryName makes "./a/b" and "a/b" the same entry, as tar writers differ.
func normEntryName(n string) string {
	return strings.TrimPrefix(strings.ReplaceAll(n, "\\", "/"), "./")
}

// SafeEntryPath turns an archive entry name into a relative slash path that
// cannot leave the folder it is joined to, or says why it refuses.
func SafeEntryPath(name string) (string, error) {
	n := normEntryName(name)
	refuse := func(why string) (string, error) {
		return "", fmt.Errorf("refused entry name %q: %s", clean(name), why)
	}
	switch {
	case n == "":
		return refuse("empty")
	case strings.HasPrefix(n, "/"):
		return refuse("absolute path")
	case strings.HasSuffix(n, "/"):
		return refuse("it is a folder")
	case strings.Contains(n, ":"):
		return refuse("':' is a drive letter or an alternate data stream on Windows")
	}
	for _, r := range n {
		if r < 0x20 || r == 0x7f {
			return refuse("control character in the name")
		}
	}
	for _, seg := range strings.Split(n, "/") {
		switch {
		case seg == "..":
			return refuse("'..' climbs out of the destination (path traversal)")
		case seg == "" || seg == ".":
			return refuse("empty path segment")
		case strings.HasSuffix(seg, ".") || strings.HasSuffix(seg, " "):
			return refuse("Windows silently drops a trailing dot or space")
		case isWindowsDevice(seg):
			return refuse("'" + seg + "' is a Windows device name")
		}
	}
	return n, nil
}

func isWindowsDevice(seg string) bool {
	base := strings.ToUpper(seg)
	if i := strings.IndexByte(base, '.'); i >= 0 {
		base = base[:i]
	}
	switch base {
	case "CON", "PRN", "AUX", "NUL", "CONIN$", "CONOUT$":
		return true
	}
	if len(base) == 4 && (strings.HasPrefix(base, "COM") || strings.HasPrefix(base, "LPT")) && base[3] >= '0' && base[3] <= '9' {
		return true
	}
	return false
}

func writeEntry(open func() (io.Reader, error), rel, destDir string, maxBytes int64) (string, error) {
	target := filepath.Join(destDir, filepath.FromSlash(rel))
	if !within(destDir, target) {
		return "", fmt.Errorf("refused: %s would land outside %s", rel, destDir)
	}
	// Every existing folder between destDir and the file must be a real folder:
	// a link or junction planted there would carry the write somewhere else.
	cur := destDir
	for _, seg := range strings.Split(filepath.Dir(filepath.FromSlash(rel)), string(filepath.Separator)) {
		if seg == "." {
			break
		}
		cur = filepath.Join(cur, seg)
		if fi, err := os.Lstat(cur); err == nil {
			if fi.Mode()&(os.ModeSymlink|os.ModeIrregular) != 0 || !fi.IsDir() {
				return "", fmt.Errorf("refused: %s is a link or not a folder", cur)
			}
		}
	}
	if _, err := os.Lstat(target); err == nil {
		return "", fmt.Errorf("refused: %s already exists; nothing is overwritten", target)
	}
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return "", err
	}
	r, err := open()
	if err != nil {
		return "", err
	}
	out, err := os.OpenFile(target, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		return "", err
	}
	h := sha256.New()
	n, err := io.Copy(io.MultiWriter(out, h), io.LimitReader(r, maxBytes+1))
	cerr := out.Close()
	if err == nil {
		err = cerr
	}
	if err == nil && n > maxBytes {
		err = fmt.Errorf("entry is larger than its header said and over the %d-byte cap", maxBytes)
	}
	if err != nil {
		os.Remove(target)
		return "", fmt.Errorf("extraction failed, partial file removed: %w", err)
	}
	return fmt.Sprintf("extracted %s -> %s (%d bytes, sha256 %x)", rel, target, n, h.Sum(nil)), nil
}

// within reports whether target is dir or below it, on cleaned paths.
func within(dir, target string) bool {
	rel, err := filepath.Rel(filepath.Clean(dir), filepath.Clean(target))
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) && !filepath.IsAbs(rel)
}

// ── opening by content, never by extension ──────────────────────────────────

var (
	magicZip        = []byte("PK\x03\x04")
	magicZipEmpty   = []byte("PK\x05\x06")
	magicZipCentral = []byte("PK\x01\x02")
	magicGzip       = []byte{0x1f, 0x8b}
	magicBzip2      = []byte("BZh")
)

func openArchive(p string) (*openedArchive, error) {
	f, err := os.Open(p)
	if err != nil {
		return nil, err
	}
	st, err := f.Stat()
	if err != nil {
		f.Close()
		return nil, err
	}
	if st.IsDir() {
		f.Close()
		return nil, fmt.Errorf("%s is a folder, not an archive", p)
	}
	head := make([]byte, 512)
	n, _ := f.ReadAt(head, 0)
	head = head[:n]
	capped := false
	a := &openedArchive{capped: &capped, close: func() { f.Close() }}

	switch {
	case bytes.HasPrefix(head, magicZip), bytes.HasPrefix(head, magicZipEmpty),
		len(head) >= 8 && bytes.Equal(head[4:8], magicZipCentral):
		zr, err := zip.NewReader(f, st.Size())
		if err != nil {
			f.Close()
			return nil, fmt.Errorf("zip signature present but unreadable: %w", err)
		}
		a.format = "zip"
		if len(head) >= 8 && bytes.Equal(head[4:8], magicZipCentral) {
			// Mozilla's optimized jar (omni.ja from mozpack's JarWriter with
			// optimize=True): a 4-byte preload length, then the central
			// directory, then the files. Go reads it because the end record's
			// directory offset (4) holds a valid header (archive/zip
			// readDirectoryEnd, "baseOffset = 0" case).
			a.note = fmt.Sprintf("layout: Mozilla optimized jar (central directory first, preload %d bytes)", binary.LittleEndian.Uint32(head))
		}
		a.walk = zipWalker(zr)
		return a, nil
	case bytes.HasPrefix(head, magicGzip):
		gz, err := gzip.NewReader(f)
		if err != nil {
			f.Close()
			return nil, fmt.Errorf("gzip signature present but unreadable: %w", err)
		}
		gz.Multistream(true)
		name := gz.Name
		if name == "" {
			name = strings.TrimSuffix(strings.TrimSuffix(filepath.Base(p), ".gz"), ".tgz")
		}
		// The gzip trailer holds the uncompressed size modulo 4 GiB, which
		// is what gzip -l reports; it is the last member's only.
		size := int64(-1)
		if st.Size() >= 18 {
			var tail [4]byte
			if _, err := f.ReadAt(tail[:], st.Size()-4); err == nil {
				size = int64(binary.LittleEndian.Uint32(tail[:]))
			}
		}
		return streamArchive(a, "gzip", name, size, gz)
	case bytes.HasPrefix(head, magicBzip2):
		name := strings.TrimSuffix(strings.TrimSuffix(filepath.Base(p), ".bz2"), ".tbz2")
		return streamArchive(a, "bzip2", name, -1, bzip2.NewReader(f))
	case isTar(head):
		a.format = "tar"
		a.walk = tarWalker(f, a.capped)
		return a, nil
	}
	f.Close()
	return nil, fmt.Errorf("not a zip/jar/omni.ja, tar, gzip or bzip2 file (%s); xz, zstd, 7z and rar need an external tool", sniffOther(head))
}

func isTar(head []byte) bool {
	return len(head) >= 262 && bytes.Equal(head[257:262], []byte("ustar"))
}

func sniffOther(head []byte) string {
	switch {
	case bytes.HasPrefix(head, []byte{0xfd, '7', 'z', 'X', 'Z', 0}):
		return "it is xz"
	case bytes.HasPrefix(head, []byte{0x28, 0xb5, 0x2f, 0xfd}):
		return "it is zstd"
	case bytes.HasPrefix(head, []byte{'7', 'z', 0xbc, 0xaf, 0x27, 0x1c}):
		return "it is 7z"
	case bytes.HasPrefix(head, []byte("Rar!")):
		return "it is rar"
	}
	if len(head) > 8 {
		head = head[:8]
	}
	return fmt.Sprintf("first bytes % x", head)
}

// streamArchive handles a compressed stream: a tar inside it is walked as a
// tar, anything else is one entry of unknown size until it is read.
func streamArchive(a *openedArchive, format, name string, size int64, r io.Reader) (*openedArchive, error) {
	br := bufio.NewReaderSize(&capReader{r: r, left: streamReadCap, capped: a.capped}, 64<<10)
	head, _ := br.Peek(512)
	if isTar(head) {
		a.format = "tar." + map[string]string{"gzip": "gz", "bzip2": "bz2"}[format]
		a.walk = tarWalker(br, a.capped)
		return a, nil
	}
	if size >= 0 {
		a.note = "size: from the gzip trailer (modulo 4 GiB)"
	}
	a.format = format + " (single compressed file)"
	a.walk = func(visit func(archiveEntry, func() (io.Reader, error)) bool) error {
		visit(archiveEntry{name: name, size: size, kind: kindFile}, func() (io.Reader, error) { return br, nil })
		return nil
	}
	return a, nil
}

// capReader stops a decompressor after left bytes and records that it did, so
// a small file that inflates to terabytes cannot hold the tool forever.
type capReader struct {
	r      io.Reader
	left   int64
	capped *bool
}

func (c *capReader) Read(p []byte) (int, error) {
	if c.left <= 0 {
		*c.capped = true
		return 0, io.EOF
	}
	if int64(len(p)) > c.left {
		p = p[:c.left]
	}
	n, err := c.r.Read(p)
	c.left -= int64(n)
	return n, err
}

func zipWalker(zr *zip.Reader) walker {
	return func(visit func(archiveEntry, func() (io.Reader, error)) bool) error {
		for _, zf := range zr.File {
			e := archiveEntry{name: zf.Name, size: int64(zf.UncompressedSize64), kind: kindFile}
			switch {
			case zf.FileInfo().IsDir() || strings.HasSuffix(zf.Name, "/"):
				e.kind = kindDir
			case zf.Mode()&os.ModeSymlink != 0:
				e.kind = kindLink
			}
			switch zf.Method {
			case zip.Store:
				e.method = "stored"
			case zip.Deflate:
				e.method = "deflate"
			default:
				e.method = fmt.Sprintf("method %d", zf.Method)
			}
			var rc io.ReadCloser
			stop := visit(e, func() (io.Reader, error) {
				var err error
				rc, err = zf.Open()
				return rc, err
			})
			if rc != nil {
				rc.Close()
			}
			if stop {
				return nil
			}
		}
		return nil
	}
}

func tarWalker(r io.Reader, capped *bool) walker {
	return func(visit func(archiveEntry, func() (io.Reader, error)) bool) error {
		tr := tar.NewReader(r)
		for {
			h, err := tr.Next()
			if err == io.EOF {
				return nil
			}
			if err != nil {
				if *capped {
					return nil
				}
				return fmt.Errorf("tar read failed: %w", err)
			}
			e := archiveEntry{name: h.Name, size: h.Size}
			switch h.Typeflag {
			case tar.TypeReg, 0: // 0 is the pre-POSIX regular file
				e.kind = kindFile
			case tar.TypeDir:
				e.kind, e.size = kindDir, 0
			case tar.TypeSymlink, tar.TypeLink:
				e.kind, e.size = kindLink, 0
			default:
				e.kind = fmt.Sprintf("special entry (type %q)", h.Typeflag)
			}
			if visit(e, func() (io.Reader, error) { return tr, nil }) {
				return nil
			}
		}
	}
}
