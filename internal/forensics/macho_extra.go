package forensics

// GORILLA (2026-10-22): job 2 of the night run, binary_inspect extras for
// Mach-O: exported symbols (the dyld export trie, else the symbol table) and
// the code signature's identifier and CMS signer names. Read from __LINKEDIT
// through debug/macho's segment reader, so a fat slice works the same way.

import (
	"debug/macho"
	"encoding/binary"
	"fmt"
	"strings"
)

const (
	lcDyldInfo        = 0x22
	lcDyldInfoOnly    = 0x80000022
	lcDyldExportsTrie = 0x80000033

	exportKindMask     = 0x03
	exportKindTLS      = 0x01
	exportKindAbsolute = 0x02
	exportWeak         = 0x04
	exportReexport     = 0x08
	exportStubResolver = 0x10

	csMagicEmbedded = 0xfade0cc0 // SuperBlob
	csMagicCodeDir  = 0xfade0c02
	csMagicCMS      = 0xfade0b01
	csSlotCodeDir   = 0
	csSlotCMS       = 0x10000
	csFlagAdhoc     = 0x2

	nStab = 0xe0
	nType = 0x0e
	nExt  = 0x01

	maxLinkedit  = 64 << 20
	maxTrieNodes = 1 << 20
)

// linkedit reads size bytes at file offset off (relative to the slice) from __LINKEDIT.
func linkedit(f *macho.File, off, size uint32) ([]byte, error) {
	seg := f.Segment("__LINKEDIT")
	if seg == nil {
		return nil, fmt.Errorf("no __LINKEDIT segment")
	}
	if uint64(off) < seg.Offset || uint64(off)+uint64(size) > seg.Offset+seg.Filesz || size > maxLinkedit {
		return nil, fmt.Errorf("data at %d+%d lies outside __LINKEDIT", off, size)
	}
	buf := make([]byte, size)
	if _, err := seg.ReadAt(buf, int64(uint64(off)-seg.Offset)); err != nil {
		return nil, err
	}
	return buf, nil
}

// machoExports returns the exported symbols and where they came from.
func machoExports(f *macho.File) (lines []string, src string) {
	for _, l := range f.Loads {
		raw := l.Raw()
		if len(raw) < 16 {
			continue
		}
		var off, size uint32
		switch f.ByteOrder.Uint32(raw) {
		case lcDyldInfo, lcDyldInfoOnly:
			if len(raw) >= 48 {
				off, size = f.ByteOrder.Uint32(raw[40:]), f.ByteOrder.Uint32(raw[44:])
			}
		case lcDyldExportsTrie:
			off, size = f.ByteOrder.Uint32(raw[8:]), f.ByteOrder.Uint32(raw[12:])
		default:
			continue
		}
		if size == 0 {
			continue
		}
		data, err := linkedit(f, off, size)
		if err != nil {
			return []string{"export trie unreadable: " + err.Error()}, "export trie"
		}
		return walkExportTrie(data), "export trie"
	}
	if f.Symtab != nil {
		for _, s := range f.Symtab.Syms {
			if s.Type&nStab == 0 && s.Type&nExt != 0 && s.Type&nType != 0 {
				lines = append(lines, fmt.Sprintf("0x%x %s", s.Value, s.Name))
			}
		}
	}
	return lines, "symbol table, external and defined"
}

func uleb(b []byte, p int) (uint64, int, bool) {
	var v uint64
	for shift := uint(0); p < len(b); shift += 7 {
		c := b[p]
		p++
		if shift < 64 {
			v |= uint64(c&0x7f) << shift
		}
		if c&0x80 == 0 {
			return v, p, true
		}
	}
	return 0, p, false
}

func cstr(b []byte, p int) (string, int, bool) {
	for i := p; i < len(b); i++ {
		if b[i] == 0 {
			return string(b[p:i]), i + 1, true
		}
	}
	return "", len(b), false
}

// walkExportTrie lists every terminal of a dyld export trie. A loop or a
// truncated node ends the walk and says so.
func walkExportTrie(b []byte) []string {
	type node struct {
		off    int
		prefix string
	}
	var out []string
	seen := map[int]bool{}
	stack := []node{{0, ""}}
	for len(stack) > 0 {
		n := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		if seen[n.off] || len(seen) >= maxTrieNodes || n.off >= len(b) {
			out = append(out, fmt.Sprintf("(export trie damaged at node %d; walk stopped there)", n.off))
			continue
		}
		seen[n.off] = true
		term, p, ok := uleb(b, n.off)
		if !ok {
			out = append(out, "(export trie truncated)")
			continue
		}
		children := p + int(term)
		if term > 0 {
			flags, q, _ := uleb(b, p)
			var line string
			if flags&exportReexport != 0 {
				ord, q2, _ := uleb(b, q)
				imp, _, _ := cstr(b, q2)
				if imp == "" {
					imp = n.prefix
				}
				line = fmt.Sprintf("re-export of %s from library %d: %s", imp, ord, n.prefix)
			} else {
				addr, _, _ := uleb(b, q)
				line = fmt.Sprintf("0x%x %s", addr, n.prefix)
			}
			var tags []string
			switch flags & exportKindMask {
			case exportKindTLS:
				tags = append(tags, "tls")
			case exportKindAbsolute:
				tags = append(tags, "absolute")
			}
			if flags&exportWeak != 0 {
				tags = append(tags, "weak")
			}
			if flags&exportStubResolver != 0 {
				tags = append(tags, "resolver")
			}
			if len(tags) > 0 {
				line += " (" + strings.Join(tags, ", ") + ")"
			}
			out = append(out, line)
		}
		if children >= len(b) {
			continue
		}
		count := int(b[children])
		p = children + 1
		var kids []node
		for i := 0; i < count; i++ {
			edge, q, ok := cstr(b, p)
			if !ok {
				break
			}
			child, q2, ok := uleb(b, q)
			if !ok {
				break
			}
			p = q2
			kids = append(kids, node{int(child), n.prefix + edge})
		}
		for i := len(kids) - 1; i >= 0; i-- { // keep the trie's own order
			stack = append(stack, kids[i])
		}
	}
	return out
}

// machoSignature describes LC_CODE_SIGNATURE: identifier, ad-hoc or not, and
// the CMS signer names when there are any.
func machoSignature(b *strings.Builder, f *macho.File) {
	var off, size uint32
	found := false
	for _, l := range f.Loads {
		raw := l.Raw()
		if len(raw) >= 16 && f.ByteOrder.Uint32(raw) == lcCodeSignature {
			off, size, found = f.ByteOrder.Uint32(raw[8:]), f.ByteOrder.Uint32(raw[12:]), true
		}
	}
	if !found {
		b.WriteString("code signature: none\n")
		return
	}
	fmt.Fprintf(b, "code signature: present, %s\n", notVerified)
	data, err := linkedit(f, off, size)
	if err != nil {
		fmt.Fprintf(b, "  unreadable: %s\n", clean(err.Error()))
		return
	}
	writeCodeSignature(b, data)
}

// writeCodeSignature reads an embedded-signature SuperBlob (big-endian always).
func writeCodeSignature(b *strings.Builder, data []byte) {
	be := binary.BigEndian
	if len(data) < 12 || be.Uint32(data) != csMagicEmbedded {
		b.WriteString("  not an embedded signature blob\n")
		return
	}
	count := int(be.Uint32(data[8:]))
	var cms []byte
	adhoc := false
	for i := 0; i < count && 12+i*8+8 <= len(data); i++ {
		slot, at := be.Uint32(data[12+i*8:]), int(be.Uint32(data[16+i*8:]))
		if at+8 > len(data) {
			continue
		}
		magic, length := be.Uint32(data[at:]), int(be.Uint32(data[at+4:]))
		if length < 8 || at+length > len(data) {
			continue
		}
		blob := data[at : at+length]
		switch {
		case slot == csSlotCodeDir && magic == csMagicCodeDir && len(blob) >= 24:
			adhoc = be.Uint32(blob[12:])&csFlagAdhoc != 0
			if id, _, ok := cstr(blob, int(be.Uint32(blob[20:]))); ok {
				fmt.Fprintf(b, "  identifier: %s (written by whoever signed it)\n", clean(id))
			}
		case slot == csSlotCMS && magic == csMagicCMS:
			cms = blob[8:]
		}
	}
	if len(cms) == 0 {
		if adhoc {
			b.WriteString("  ad-hoc: no signer certificate (anyone can make this signature)\n")
		} else {
			b.WriteString("  no signer certificate in the signature\n")
		}
		return
	}
	writeCMSSigners(b, cms)
}
