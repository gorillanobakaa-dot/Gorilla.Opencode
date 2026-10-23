package forensics

// GORILLA (2026-10-22): job 2 of the night run, binary_inspect extras for PE:
// the export directory, the VS_VERSIONINFO resource and the Authenticode
// signer names. Read-only and stdlib-only like the rest of the package. The
// version strings and the certificate names are written by whoever built or
// signed the file, and nothing here checks a signature: the output says so.

import (
	"debug/pe"
	"encoding/binary"
	"fmt"
	"io"
	"strings"
	"unicode/utf16"
)

const (
	peDirExport   = pe.IMAGE_DIRECTORY_ENTRY_EXPORT
	peDirResource = pe.IMAGE_DIRECTORY_ENTRY_RESOURCE
	peDirSecurity = pe.IMAGE_DIRECTORY_ENTRY_SECURITY

	rtVersion             = 16
	winCertTypePKCSSigned = 0x0002
	maxExportEntries      = 1 << 20 // a claim above this is a broken or hostile table
	maxCertBlock          = 16 << 20
)

// notVerified is said next to every signer name: the names are read, never checked.
const notVerified = "NOT verified (chain and hash not checked)"

// peRead returns up to n bytes at rva, from the section that holds it.
func peRead(f *pe.File, rva uint32, n int) []byte {
	for _, s := range f.Sections {
		size := s.VirtualSize
		if s.Size > size {
			size = s.Size
		}
		if rva < s.VirtualAddress || rva >= s.VirtualAddress+size {
			continue
		}
		off := rva - s.VirtualAddress
		if off >= s.Size {
			return nil // in the zero-filled tail
		}
		if rest := int(s.Size - off); n > rest {
			n = rest
		}
		buf := make([]byte, n)
		k, _ := s.ReadAt(buf, int64(off))
		return buf[:k]
	}
	return nil
}

func peCString(f *pe.File, rva uint32) string {
	b := peRead(f, rva, 512)
	if i := strings.IndexByte(string(b), 0); i >= 0 {
		b = b[:i]
	}
	return string(b)
}

// peExports lists the export directory: ordinal, RVA or forwarder, name.
func peExports(f *pe.File, dirs []pe.DataDirectory) (dll string, lines []string, err error) {
	if !peDir(dirs, peDirExport) {
		return "", nil, nil
	}
	d := dirs[peDirExport]
	h := peRead(f, d.VirtualAddress, 40)
	if len(h) < 40 {
		return "", nil, fmt.Errorf("export directory unreadable")
	}
	le := binary.LittleEndian
	dll = peCString(f, le.Uint32(h[12:]))
	base, nFuncs, nNames := le.Uint32(h[16:]), le.Uint32(h[20:]), le.Uint32(h[24:])
	if nFuncs > maxExportEntries || nNames > maxExportEntries {
		return dll, nil, fmt.Errorf("export table claims %d functions and %d names: not believed", nFuncs, nNames)
	}
	funcs := peRead(f, le.Uint32(h[28:]), int(nFuncs)*4)
	names := peRead(f, le.Uint32(h[32:]), int(nNames)*4)
	ords := peRead(f, le.Uint32(h[36:]), int(nNames)*2)
	byIndex := map[uint32][]string{}
	for i := 0; i*4+4 <= len(names) && i*2+2 <= len(ords); i++ {
		idx := uint32(le.Uint16(ords[i*2:]))
		byIndex[idx] = append(byIndex[idx], peCString(f, le.Uint32(names[i*4:])))
	}
	for i := uint32(0); int(i)*4+4 <= len(funcs); i++ {
		rva := le.Uint32(funcs[i*4:])
		if rva == 0 {
			continue // unused ordinal
		}
		ord := base + i
		where := fmt.Sprintf("0x%x", rva)
		// An RVA inside the export directory is not code: it is the text "DLL.Function".
		if rva >= d.VirtualAddress && rva < d.VirtualAddress+d.Size {
			where = "-> " + clean(peCString(f, rva))
		}
		name := "(no name, ordinal only)"
		if n := byIndex[i]; len(n) > 0 {
			name = strings.Join(n, ", ")
		}
		lines = append(lines, fmt.Sprintf("ord %d %s %s", ord, where, name))
	}
	return dll, lines, nil
}

// ── version info ────────────────────────────────────────────────────────────

// versionKeys are the StringFileInfo values reported, in this order.
var versionKeys = []string{"FileVersion", "ProductVersion", "CompanyName", "ProductName", "OriginalFilename", "FileDescription"}

type versionInfo struct {
	Strings map[string]string
	Fixed   string // the numeric VS_FIXEDFILEINFO versions
}

// peVersionInfo finds the first RT_VERSION resource and parses it.
func peVersionInfo(f *pe.File, dirs []pe.DataDirectory) (versionInfo, bool) {
	if !peDir(dirs, peDirResource) {
		return versionInfo{}, false
	}
	root := dirs[peDirResource].VirtualAddress
	le := binary.LittleEndian
	// Three levels: type, name, language. Type is matched to RT_VERSION; the
	// first name and the first language are taken.
	off, want := uint32(0), int64(rtVersion)
	for level := 0; level < 3; level++ {
		h := peRead(f, root+off, 16)
		if len(h) < 16 {
			return versionInfo{}, false
		}
		n := int(le.Uint16(h[12:])) + int(le.Uint16(h[14:]))
		ents := peRead(f, root+off+16, n*8)
		next, found := uint32(0), false
		for i := 0; i*8+8 <= len(ents); i++ {
			id, to := le.Uint32(ents[i*8:]), le.Uint32(ents[i*8+4:])
			if want >= 0 && (id&0x80000000 != 0 || int64(id) != want) {
				continue
			}
			next, found = to, true
			break
		}
		if !found {
			return versionInfo{}, false
		}
		want = -1
		if level < 2 {
			if next&0x80000000 == 0 {
				return versionInfo{}, false // a leaf where a folder belongs
			}
			off = next &^ 0x80000000
			continue
		}
		if next&0x80000000 != 0 {
			return versionInfo{}, false
		}
		de := peRead(f, root+next, 16)
		if len(de) < 16 {
			return versionInfo{}, false
		}
		size := le.Uint32(de[4:])
		if size > 1<<20 {
			size = 1 << 20
		}
		data := peRead(f, le.Uint32(de[0:]), int(size)) // OffsetToData is an RVA
		vi := parseVersionInfo(data)
		return vi, len(vi.Strings) > 0 || vi.Fixed != ""
	}
	return versionInfo{}, false
}

// parseVersionInfo reads a VS_VERSIONINFO block: the fixed numeric versions and
// the StringFileInfo strings (the first table's value wins for each key).
func parseVersionInfo(data []byte) versionInfo {
	vi := versionInfo{Strings: map[string]string{}}
	walkVersionBlock(data, 0, nil, func(path []string, key string, typ uint16, value []byte) {
		switch {
		case len(path) == 0 && key == "VS_VERSION_INFO" && len(value) >= 52 &&
			binary.LittleEndian.Uint32(value) == 0xFEEF04BD:
			le := binary.LittleEndian
			v := func(ms, ls uint32) string {
				return fmt.Sprintf("%d.%d.%d.%d", ms>>16, ms&0xffff, ls>>16, ls&0xffff)
			}
			vi.Fixed = "file " + v(le.Uint32(value[8:]), le.Uint32(value[12:])) +
				", product " + v(le.Uint32(value[16:]), le.Uint32(value[20:]))
		case len(path) == 3 && path[1] == "StringFileInfo" && typ == 1:
			if _, seen := vi.Strings[key]; !seen {
				vi.Strings[key] = utf16z(value)
			}
		}
	})
	return vi
}

// walkVersionBlock walks the nested {wLength, wValueLength, wType, key, value,
// children} blocks. Every field is 32-bit aligned from the start of the resource.
func walkVersionBlock(data []byte, p int, path []string, fn func(path []string, key string, typ uint16, value []byte)) {
	if len(path) > 4 {
		return // the format nests three deep; deeper is not a version resource
	}
	le := binary.LittleEndian
	if p+6 > len(data) {
		return
	}
	length, vlen, typ := int(le.Uint16(data[p:])), int(le.Uint16(data[p+2:])), le.Uint16(data[p+4:])
	end := p + length
	if length < 6 || end > len(data) {
		return
	}
	k := p + 6
	var key []uint16
	for ; k+2 <= end; k += 2 {
		c := le.Uint16(data[k:])
		if c == 0 {
			k += 2
			break
		}
		key = append(key, c)
	}
	v := align4(k)
	if typ == 1 {
		vlen *= 2 // text values count UTF-16 units, not bytes
	}
	vend := v + vlen
	if vend > end {
		vend = end
	}
	var value []byte
	if v < vend {
		value = data[v:vend]
	}
	name := string(utf16.Decode(key))
	fn(path, name, typ, value)
	for c := align4(vend); c+6 <= end; {
		cl := int(le.Uint16(data[c:]))
		if cl < 6 {
			break
		}
		walkVersionBlock(data[:end], c, append(path[:len(path):len(path)], name), fn)
		c = align4(c + cl)
	}
}

func align4(n int) int { return (n + 3) &^ 3 }

// utf16z decodes little-endian UTF-16 up to the first NUL.
func utf16z(b []byte) string {
	var u []uint16
	for i := 0; i+2 <= len(b); i += 2 {
		c := binary.LittleEndian.Uint16(b[i:])
		if c == 0 {
			break
		}
		u = append(u, c)
	}
	return string(utf16.Decode(u))
}

func writeVersionInfo(b *strings.Builder, vi versionInfo) {
	b.WriteString("version info (written by whoever built the file):\n")
	for _, k := range versionKeys {
		if v, ok := vi.Strings[k]; ok {
			fmt.Fprintf(b, "  %s: %s\n", k, clean(v))
		}
	}
	if vi.Fixed != "" {
		fmt.Fprintf(b, "  numeric: %s\n", vi.Fixed)
	}
}

// ── Authenticode ────────────────────────────────────────────────────────────

// peSignature reports the signer names in the WIN_CERTIFICATE table. The
// security directory's address is a file offset, not an RVA.
func peSignature(b *strings.Builder, r io.ReaderAt, dirs []pe.DataDirectory) {
	if !peDir(dirs, peDirSecurity) {
		b.WriteString("signature: none\n")
		return
	}
	d := dirs[peDirSecurity]
	size := d.Size
	if size > maxCertBlock {
		size = maxCertBlock
	}
	tbl := make([]byte, size)
	n, _ := r.ReadAt(tbl, int64(d.VirtualAddress))
	tbl = tbl[:n]
	le := binary.LittleEndian
	found := false
	for p := 0; p+8 <= len(tbl); {
		l := int(le.Uint32(tbl[p:]))
		if l < 8 || p+l > len(tbl) {
			break
		}
		ctype := le.Uint16(tbl[p+6:])
		if ctype == winCertTypePKCSSigned {
			found = true
			fmt.Fprintf(b, "signature: Authenticode, %s\n", notVerified)
			writeCMSSigners(b, tbl[p+8:p+l])
		}
		p = align8(p + l)
	}
	if !found {
		fmt.Fprintf(b, "signature: certificate table present but holds no PKCS#7 signature; %s\n", notVerified)
	}
}

func align8(n int) int { return (n + 7) &^ 7 }
