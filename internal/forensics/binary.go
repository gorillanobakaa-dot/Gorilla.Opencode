// Package forensics reads executables and archives without running them.
//
// GORILLA (2026-10-21): phase 1 of the binary and forensics toolkit (PLAN.md,
// "binary and forensics toolkit"). On Linux the harness leaned on readelf,
// objdump, nm, strings and unzip; Windows and a fresh macOS 10.13 have none of
// them, so the model was half blind there. Everything here is the Go standard
// library only: no cgo, no dependency, no network, no process started, nothing
// written except the one archive entry Extract is asked for. It must also build
// with the patched go1.24 legacy-darwin toolchain, so nothing newer than 1.24.
//
// Output is sized for the smallest model the harness targets (Gemma 4 E2B on
// CPU): every list is capped and every cap says so, with the way to narrow.
package forensics

import (
	"bytes"
	"debug/elf"
	"debug/macho"
	"debug/pe"
	"encoding/binary"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Parts of a binary report. Summary is the default and the cheapest.
const (
	PartSummary  = "summary"
	PartImports  = "imports"
	PartSymbols  = "symbols"
	PartStrings  = "strings"
	PartSections = "sections"
	PartExports  = "exports"
)

// stringScanCap is how much of a file a default strings scan reads. A variable only so a test can
// lower it; full=true reads the whole file.
var stringScanCap int64 = 256 << 20

// Caps. Each one is reported when hit, never applied silently.
const (
	maxSections  = 40
	maxLibs      = 40
	maxSymbols   = 150
	maxStrings   = 150
	maxStringLen = 160
	entropyCap   = 32 << 20 // bytes read per section for entropy
	highEntropy  = 7.2      // bits/byte: packed, compressed or encrypted
)

// Options selects what Inspect reports.
type Options struct {
	Part   string // summary (default), sections, imports, exports, symbols, strings
	Filter string // case-insensitive substring for symbols, imports and strings
	MinLen int    // shortest string reported (default 6)
	// Full lifts every cap: for a person or a large model. The small defaults are for a small
	// local model; with Full the caller (the tool) spills an oversized result to a file.
	Full bool
}

// limit is n, or no limit with Full.
func (o Options) limit(n int) int {
	if o.Full {
		return math.MaxInt32
	}
	return n
}

// Inspect reports on the executable at path. It never runs it.
func Inspect(path string, o Options) (string, error) {
	if o.Part == "" {
		o.Part = PartSummary
	}
	switch o.Part {
	case PartSummary, PartSections, PartImports, PartExports, PartSymbols, PartStrings:
	default:
		return "", fmt.Errorf("unknown part %q: use summary, sections, imports, exports, symbols or strings", o.Part)
	}
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	st, err := f.Stat()
	if err != nil {
		return "", err
	}
	if st.IsDir() {
		return "", fmt.Errorf("%s is a folder, not a file", path)
	}

	var b strings.Builder
	fmt.Fprintf(&b, "file: %s (%d bytes)\n", filepath.Base(path), st.Size())
	if o.Part == PartStrings {
		// Strings work on any file, so a format it cannot parse is no reason to refuse.
		writeStrings(&b, f, st.Size(), o)
		return b.String(), nil
	}

	magic := make([]byte, 8)
	n, _ := f.ReadAt(magic, 0)
	magic = magic[:n]
	switch {
	case bytes.HasPrefix(magic, []byte(elf.ELFMAG)):
		err = inspectELF(&b, f, o)
	case bytes.HasPrefix(magic, []byte("MZ")):
		err = inspectPE(&b, f, o)
	case isMachO(magic):
		err = inspectMachO(&b, f, o)
	case len(magic) >= 4 && binary.BigEndian.Uint32(magic) == macho.MagicFat:
		err = inspectFat(&b, f, o)
	default:
		return "", fmt.Errorf("not an ELF, PE or Mach-O file (first bytes % x); part=strings works on any file", magic)
	}
	if err != nil {
		return "", err
	}
	return b.String(), nil
}

func isMachO(m []byte) bool {
	if len(m) < 4 {
		return false
	}
	le, be := binary.LittleEndian.Uint32(m), binary.BigEndian.Uint32(m)
	for _, v := range []uint32{macho.Magic32, macho.Magic64} {
		if le == v || be == v {
			return true
		}
	}
	return false
}

// ── ELF ─────────────────────────────────────────────────────────────────────

func inspectELF(b *strings.Builder, r io.ReaderAt, o Options) error {
	f, err := elf.NewFile(r)
	if err != nil {
		return fmt.Errorf("ELF header present but unreadable: %w", err)
	}
	defer f.Close()
	bits := "32"
	if f.Class == elf.ELFCLASS64 {
		bits = "64"
	}
	order := "little-endian"
	if f.Data == elf.ELFDATA2MSB {
		order = "big-endian"
	}
	fmt.Fprintf(b, "format: ELF%s %s, type %s, machine %s, OS/ABI %s\n", bits, order, elfType(f.Type), f.Machine, f.OSABI)

	switch o.Part {
	case PartSummary:
		fmt.Fprintf(b, "entry: 0x%x\n", f.Entry)
		for _, p := range f.Progs {
			if p.Type == elf.PT_INTERP {
				data, _ := io.ReadAll(io.LimitReader(p.Open(), 4096))
				fmt.Fprintf(b, "interpreter: %s\n", clean(strings.TrimRight(string(data), "\x00")))
			}
		}
		libs, _ := f.ImportedLibraries()
		writeList(b, "needed libraries", libs, o.limit(maxLibs), "")
		rpath, _ := f.DynString(elf.DT_RPATH)
		runpath, _ := f.DynString(elf.DT_RUNPATH)
		if len(rpath)+len(runpath) > 0 {
			fmt.Fprintf(b, "rpath: %s; runpath: %s\n", orDash(rpath), orDash(runpath))
		}
		syms, symErr := f.Symbols()
		dyn, _ := f.DynamicSymbols()
		if symErr != nil {
			fmt.Fprintf(b, "symbols: stripped (no .symtab); %d dynamic\n", len(dyn))
		} else {
			fmt.Fprintf(b, "symbols: %d in .symtab, %d dynamic (part=symbols lists them)\n", len(syms), len(dyn))
		}
		if n := len(elfExports(f)); n > 0 {
			fmt.Fprintf(b, "exports: %d (part=exports lists them)\n", n)
		}
		writeELFSections(b, f, o)
	case PartSections:
		writeELFSections(b, f, o)
	case PartImports:
		libs, _ := f.ImportedLibraries()
		writeList(b, "needed libraries", libs, o.limit(maxLibs), o.Filter)
		is, _ := f.ImportedSymbols()
		var lines []string
		for _, s := range is {
			l := s.Name
			if s.Version != "" {
				l += "@" + s.Version
			}
			if s.Library != "" {
				l += " (" + s.Library + ")"
			}
			lines = append(lines, l)
		}
		writeList(b, "imported symbols", lines, o.limit(maxSymbols), o.Filter)
	case PartExports:
		writeList(b, "exported dynamic symbols", elfExports(f), o.limit(maxSymbols), o.Filter)
	case PartSymbols:
		var lines []string
		syms, err := f.Symbols()
		if err != nil {
			b.WriteString("no .symtab (stripped); dynamic symbols follow\n")
		}
		dyn, _ := f.DynamicSymbols()
		for _, s := range append(syms, dyn...) {
			if s.Name == "" {
				continue
			}
			kind := "?"
			switch elf.ST_TYPE(s.Info) {
			case elf.STT_FUNC:
				kind = "func"
			case elf.STT_OBJECT:
				kind = "data"
			case elf.STT_SECTION, elf.STT_FILE:
				continue
			}
			if s.Section == elf.SHN_UNDEF {
				kind = "undef"
			}
			lines = append(lines, fmt.Sprintf("0x%x %s %s", s.Value, kind, s.Name))
		}
		writeList(b, "symbols", lines, o.limit(maxSymbols), o.Filter)
	}
	return nil
}

// elfExports lists the dynamic symbols a library offers others: defined, global
// or weak, and code, data or thread-local.
func elfExports(f *elf.File) []string {
	dyn, _ := f.DynamicSymbols()
	var lines []string
	for _, s := range dyn {
		if s.Section == elf.SHN_UNDEF || s.Name == "" {
			continue
		}
		bind := elf.ST_BIND(s.Info)
		if bind != elf.STB_GLOBAL && bind != elf.STB_WEAK {
			continue
		}
		kind := ""
		switch elf.ST_TYPE(s.Info) {
		case elf.STT_FUNC:
			kind = "func"
		case elf.STT_OBJECT:
			kind = "data"
		case elf.STT_TLS:
			kind = "tls"
		case elf.STT_GNU_IFUNC:
			kind = "ifunc"
		default:
			continue
		}
		if bind == elf.STB_WEAK {
			kind += ",weak"
		}
		name := s.Name
		if s.Version != "" {
			name += "@" + s.Version
		}
		lines = append(lines, fmt.Sprintf("0x%x %s %s", s.Value, kind, name))
	}
	return lines
}

func elfType(t elf.Type) string {
	switch t {
	case elf.ET_EXEC:
		return "EXEC (fixed-address executable)"
	case elf.ET_DYN:
		return "DYN (shared library or PIE executable)"
	case elf.ET_REL:
		return "REL (object file)"
	case elf.ET_CORE:
		return "CORE (core dump)"
	}
	return t.String()
}

func writeELFSections(b *strings.Builder, f *elf.File, o Options) {
	var rows []string
	for _, s := range f.Sections {
		if s.Type == elf.SHT_NULL {
			continue
		}
		flags := ""
		if s.Flags&elf.SHF_EXECINSTR != 0 {
			flags += "x"
		}
		if s.Flags&elf.SHF_WRITE != 0 {
			flags += "w"
		}
		ent := "-"
		if s.Type != elf.SHT_NOBITS && s.Size > 0 {
			ent = entropyOf(s.Open(), s.Size)
		}
		rows = append(rows, fmt.Sprintf("%-22s %10d %5s %s", clean(s.Name), s.Size, ent, flags))
	}
	writeSectionRows(b, rows, o)
}

// ── PE ──────────────────────────────────────────────────────────────────────

func inspectPE(b *strings.Builder, r io.ReaderAt, o Options) error {
	f, err := pe.NewFile(r)
	if err != nil {
		return fmt.Errorf("MZ header present but not a readable PE: %w", err)
	}
	defer f.Close()
	kind, entry, base, subsys := "PE32", uint64(0), uint64(0), uint16(0)
	var dirs []pe.DataDirectory
	switch oh := f.OptionalHeader.(type) {
	case *pe.OptionalHeader32:
		entry, base, subsys, dirs = uint64(oh.AddressOfEntryPoint), uint64(oh.ImageBase), oh.Subsystem, oh.DataDirectory[:oh.NumberOfRvaAndSizes]
	case *pe.OptionalHeader64:
		kind = "PE32+ (64-bit)"
		entry, base, subsys, dirs = uint64(oh.AddressOfEntryPoint), oh.ImageBase, oh.Subsystem, oh.DataDirectory[:oh.NumberOfRvaAndSizes]
	}
	role := "EXE"
	if f.Characteristics&pe.IMAGE_FILE_DLL != 0 {
		role = "DLL"
	}
	fmt.Fprintf(b, "format: %s %s, machine %s, subsystem %s\n", kind, role, peMachine(f.Machine), peSubsystem(subsys))

	imports := map[string][]string{}
	var dlls []string
	if syms, err := f.ImportedSymbols(); err == nil {
		for _, s := range syms {
			fn, dll, _ := strings.Cut(s, ":")
			if _, seen := imports[dll]; !seen {
				dlls = append(dlls, dll)
			}
			imports[dll] = append(imports[dll], fn)
		}
	}

	switch o.Part {
	case PartSummary:
		fmt.Fprintf(b, "entry RVA: 0x%x, image base: 0x%x\n", entry, base)
		if ts := f.TimeDateStamp; ts != 0 {
			fmt.Fprintf(b, "link time stamp: %s (may be zeroed or faked)\n", time.Unix(int64(ts), 0).UTC().Format("2006-01-02 15:04 UTC"))
		}
		var rows []string
		for _, d := range dlls {
			rows = append(rows, fmt.Sprintf("%s (%d functions)", d, len(imports[d])))
		}
		writeList(b, "imported DLLs", rows, o.limit(maxLibs), "")
		if peDir(dirs, pe.IMAGE_DIRECTORY_ENTRY_DELAY_IMPORT) {
			b.WriteString("delay-load imports: present, not listed\n")
		}
		if dll, ex, err := peExports(f, dirs); err != nil {
			fmt.Fprintf(b, "exports: %s\n", clean(err.Error()))
		} else if len(ex) > 0 {
			fmt.Fprintf(b, "exports: %d as %s (part=exports lists them)\n", len(ex), orDash([]string{dll}))
		}
		if vi, ok := peVersionInfo(f, dirs); ok {
			writeVersionInfo(b, vi)
		}
		peSignature(b, r, dirs)
		if peDir(dirs, pe.IMAGE_DIRECTORY_ENTRY_COM_DESCRIPTOR) {
			b.WriteString(".NET: yes (managed code)\n")
		}
		writePESections(b, f, o)
	case PartSections:
		writePESections(b, f, o)
	case PartImports:
		var lines []string
		for _, d := range dlls {
			for _, fn := range imports[d] {
				lines = append(lines, d+"!"+fn)
			}
		}
		writeList(b, "imported functions", lines, o.limit(maxSymbols), o.Filter)
	case PartExports:
		dll, ex, err := peExports(f, dirs)
		if err != nil {
			return err
		}
		if dll != "" {
			fmt.Fprintf(b, "export name: %s\n", clean(dll))
		}
		writeList(b, "exported functions (ordinal, RVA or -> forwarder, name)", ex, o.limit(maxSymbols), o.Filter)
	case PartSymbols:
		var lines []string
		for _, s := range f.Symbols {
			lines = append(lines, fmt.Sprintf("0x%x sect%d %s", s.Value, s.SectionNumber, s.Name))
		}
		if len(lines) == 0 {
			b.WriteString("no COFF symbols (normal for a linked EXE/DLL); part=imports lists what it calls\n")
			return nil
		}
		writeList(b, "COFF symbols", lines, o.limit(maxSymbols), o.Filter)
	}
	return nil
}

func peDir(dirs []pe.DataDirectory, i int) bool {
	return i < len(dirs) && dirs[i].VirtualAddress != 0 && dirs[i].Size != 0
}

func writePESections(b *strings.Builder, f *pe.File, o Options) {
	var rows []string
	for _, s := range f.Sections {
		flags := ""
		if s.Characteristics&pe.IMAGE_SCN_MEM_EXECUTE != 0 {
			flags += "x"
		}
		if s.Characteristics&pe.IMAGE_SCN_MEM_WRITE != 0 {
			flags += "w"
		}
		ent := "-"
		if s.Size > 0 {
			ent = entropyOf(s.Open(), uint64(s.Size))
		}
		rows = append(rows, fmt.Sprintf("%-22s %10d %5s %s", clean(s.Name), s.Size, ent, flags))
	}
	writeSectionRows(b, rows, o)
}

func peMachine(m uint16) string {
	switch m {
	case pe.IMAGE_FILE_MACHINE_AMD64:
		return "x86_64"
	case pe.IMAGE_FILE_MACHINE_I386:
		return "i386"
	case pe.IMAGE_FILE_MACHINE_ARM64:
		return "arm64"
	case pe.IMAGE_FILE_MACHINE_ARMNT:
		return "arm (thumb-2)"
	}
	return fmt.Sprintf("0x%x", m)
}

func peSubsystem(s uint16) string {
	switch s {
	case pe.IMAGE_SUBSYSTEM_WINDOWS_GUI:
		return "GUI"
	case pe.IMAGE_SUBSYSTEM_WINDOWS_CUI:
		return "console"
	case pe.IMAGE_SUBSYSTEM_NATIVE:
		return "native (driver)"
	case pe.IMAGE_SUBSYSTEM_EFI_APPLICATION, pe.IMAGE_SUBSYSTEM_EFI_BOOT_SERVICE_DRIVER, pe.IMAGE_SUBSYSTEM_EFI_RUNTIME_DRIVER:
		return "EFI"
	}
	return fmt.Sprintf("%d", s)
}

// ── Mach-O ──────────────────────────────────────────────────────────────────

// Load commands debug/macho does not decode.
const (
	lcVersionMinMacOSX = 0x24
	lcBuildVersion     = 0x32
	lcCodeSignature    = 0x1d
	platformMacOS      = 1
)

// The MacBook Air this toolkit was asked for runs macOS 10.13 (High Sierra) on
// an Intel CPU; 10.13.6 is High Sierra's last release, so a binary that needs
// 10.13.4 still loads there and one that needs 10.14 does not.
const legacyMacOS = "10.13.6"

func inspectMachO(b *strings.Builder, r io.ReaderAt, o Options) error {
	f, err := macho.NewFile(r)
	if err != nil {
		return fmt.Errorf("Mach-O magic present but unreadable: %w", err)
	}
	defer f.Close()
	b.WriteString("format: Mach-O thin\n")
	writeMachOSlice(b, f, o)
	return nil
}

func inspectFat(b *strings.Builder, r io.ReaderAt, o Options) error {
	ff, err := macho.NewFatFile(r)
	if err != nil {
		// 0xCAFEBABE is also a Java class file.
		return fmt.Errorf("starts with CAFEBABE but is not a readable fat Mach-O (a Java .class file has the same magic): %w", err)
	}
	defer ff.Close()
	fmt.Fprintf(b, "format: Mach-O fat (universal), %d slices\n", len(ff.Arches))
	for _, a := range ff.Arches {
		fmt.Fprintf(b, "\n== slice %s (offset %d, %d bytes) ==\n", machoCPU(a.Cpu), a.Offset, a.Size)
		writeMachOSlice(b, a.File, o)
	}
	return nil
}

func writeMachOSlice(b *strings.Builder, f *macho.File, o Options) {
	switch o.Part {
	case PartSummary:
		fmt.Fprintf(b, "arch: %s, type: %s, flags: %s\n", machoCPU(f.Cpu), machoType(f.Type), machoFlags(f.Flags))
		min, sdk, src := machoVersions(f)
		if min == "" {
			b.WriteString("minimum macOS: not recorded (no version load command)\n")
		} else {
			fmt.Fprintf(b, "minimum macOS: %s (sdk %s, from %s)\n", min, sdk, src)
		}
		fmt.Fprintf(b, "macOS %s (MacBook Air 2011, Intel) check: %s\n", legacyMacOS, legacyVerdict(f.Cpu, min, src))
		libs, _ := f.ImportedLibraries()
		writeList(b, "libraries", libs, o.limit(maxLibs), "")
		var rp []string
		for _, l := range f.Loads {
			if r, ok := l.(*macho.Rpath); ok {
				rp = append(rp, r.Path)
			}
		}
		if len(rp) > 0 {
			fmt.Fprintf(b, "rpath: %s\n", clean(strings.Join(rp, "; ")))
		}
		machoSignature(b, f)
		if f.Symtab != nil {
			fmt.Fprintf(b, "symbols: %d (part=symbols lists them)\n", len(f.Symtab.Syms))
		}
		if ex, src := machoExports(f); len(ex) > 0 {
			fmt.Fprintf(b, "exports: %d from the %s (part=exports lists them)\n", len(ex), src)
		}
		writeMachOSections(b, f, o)
	case PartSections:
		writeMachOSections(b, f, o)
	case PartImports:
		libs, _ := f.ImportedLibraries()
		writeList(b, "libraries", libs, o.limit(maxLibs), o.Filter)
		is, _ := f.ImportedSymbols()
		writeList(b, "imported symbols", is, o.limit(maxSymbols), o.Filter)
	case PartExports:
		ex, src := machoExports(f)
		writeList(b, "exported symbols ("+src+")", ex, o.limit(maxSymbols), o.Filter)
	case PartSymbols:
		var lines []string
		if f.Symtab != nil {
			for _, s := range f.Symtab.Syms {
				kind := "def"
				if s.Sect == 0 {
					kind = "undef"
				}
				lines = append(lines, fmt.Sprintf("0x%x %s %s", s.Value, kind, s.Name))
			}
		}
		writeList(b, "symbols", lines, o.limit(maxSymbols), o.Filter)
	}
}

// machoVersions reads the minimum OS and SDK from LC_BUILD_VERSION (macOS 10.14
// SDK and later) or LC_VERSION_MIN_MACOSX (older).
func machoVersions(f *macho.File) (min, sdk, src string) {
	for _, l := range f.Loads {
		raw := l.Raw()
		if len(raw) < 8 {
			continue
		}
		switch f.ByteOrder.Uint32(raw) {
		case lcVersionMinMacOSX:
			if len(raw) >= 16 && min == "" {
				min, sdk, src = machoVer(f.ByteOrder.Uint32(raw[8:])), machoVer(f.ByteOrder.Uint32(raw[12:])), "LC_VERSION_MIN_MACOSX"
			}
		case lcBuildVersion:
			if len(raw) >= 20 && f.ByteOrder.Uint32(raw[8:]) == platformMacOS {
				min, sdk, src = machoVer(f.ByteOrder.Uint32(raw[12:])), machoVer(f.ByteOrder.Uint32(raw[16:])), "LC_BUILD_VERSION"
			}
		}
	}
	return
}

// machoVer decodes the xxxx.yy.zz nibble encoding Apple uses.
func machoVer(v uint32) string {
	s := fmt.Sprintf("%d.%d", v>>16, (v>>8)&0xff)
	if p := v & 0xff; p != 0 {
		s += fmt.Sprintf(".%d", p)
	}
	return s
}

// legacyVerdict says whether the header allows this slice to load on the Air.
// Only the header: a library the binary links may still be missing on 10.13.
//
// Which load command recorded the minimum matters. LC_BUILD_VERSION arrived
// with the 10.14 SDK, and 10.13 predates it: measured 2026-10-21, Gorilla
// OpenCode 0.1.149-dev for the Air (patched go1.24) carries minos 11.0 there
// and runs on the Air. So only LC_VERSION_MIN_MACOSX is a verdict on 10.13;
// a higher LC_BUILD_VERSION is reported as unproven, never as a refusal.
func legacyVerdict(cpu macho.Cpu, min, src string) string {
	if cpu != macho.CpuAmd64 && cpu != macho.Cpu386 {
		return "NO: " + machoCPU(cpu) + " slice, the Air is Intel"
	}
	if min == "" {
		return "UNKNOWN: no minimum recorded (header does not forbid it)"
	}
	if versionLE(min, legacyMacOS) {
		return "OK by header (needs " + min + ")"
	}
	if src == "LC_BUILD_VERSION" {
		return "UNPROVEN: LC_BUILD_VERSION says " + min + ", but 10.13 predates that load command " +
			"(Gorilla's own 10.13 build says 11.0 there and runs); only a run on the Air proves it"
	}
	return "NO: needs macOS " + min
}

func versionLE(a, b string) bool {
	pa, pb := strings.Split(a, "."), strings.Split(b, ".")
	for i := 0; i < 3; i++ {
		var x, y int
		if i < len(pa) {
			fmt.Sscanf(pa[i], "%d", &x)
		}
		if i < len(pb) {
			fmt.Sscanf(pb[i], "%d", &y)
		}
		if x != y {
			return x < y
		}
	}
	return true
}

func writeMachOSections(b *strings.Builder, f *macho.File, o Options) {
	var rows []string
	for _, s := range f.Sections {
		ent := "-"
		const zerofill = 0x1
		if s.Flags&0xff != zerofill && s.Size > 0 {
			ent = entropyOf(s.Open(), s.Size)
		}
		rows = append(rows, fmt.Sprintf("%-22s %10d %5s", clean(s.Seg+","+s.Name), s.Size, ent))
	}
	writeSectionRows(b, rows, o)
}

func machoCPU(c macho.Cpu) string {
	switch c {
	case macho.CpuAmd64:
		return "x86_64"
	case macho.Cpu386:
		return "i386"
	case macho.CpuArm64:
		return "arm64"
	case macho.CpuArm:
		return "arm"
	case macho.CpuPpc:
		return "ppc"
	case macho.CpuPpc64:
		return "ppc64"
	}
	return fmt.Sprintf("cpu 0x%x", uint32(c))
}

func machoType(t macho.Type) string {
	switch t {
	case macho.TypeExec:
		return "executable"
	case macho.TypeDylib:
		return "dylib"
	case macho.TypeBundle:
		return "bundle"
	case macho.TypeObj:
		return "object"
	}
	return fmt.Sprintf("%d", t)
}

func machoFlags(fl uint32) string {
	var out []string
	if fl&macho.FlagPIE != 0 {
		out = append(out, "PIE")
	}
	if fl&macho.FlagTwoLevel != 0 {
		out = append(out, "two-level")
	}
	if fl&macho.FlagAllowStackExecution != 0 {
		out = append(out, "EXECUTABLE-STACK")
	}
	if len(out) == 0 {
		return "-"
	}
	return strings.Join(out, ",")
}

// ── shared output helpers ───────────────────────────────────────────────────

func writeSectionRows(b *strings.Builder, rows []string, o Options) {
	fmt.Fprintf(b, "sections (%d): name, bytes, entropy bits/byte (>%.1f = packed/compressed/encrypted), x=exec w=write\n", len(rows), highEntropy)
	for i, r := range rows {
		if i == o.limit(maxSections) {
			fmt.Fprintf(b, "  ... %d more sections not shown (capped at %d; full=true lists all)\n", len(rows)-maxSections, maxSections)
			break
		}
		b.WriteString("  " + r + "\n")
	}
}

// writeList prints a capped, optionally filtered list and says when it capped.
func writeList(b *strings.Builder, title string, items []string, max int, filter string) {
	if filter != "" {
		lf := strings.ToLower(filter)
		var kept []string
		for _, it := range items {
			if strings.Contains(strings.ToLower(it), lf) {
				kept = append(kept, it)
			}
		}
		fmt.Fprintf(b, "%s matching %q: %d of %d\n", title, filter, len(kept), len(items))
		items = kept
	} else {
		fmt.Fprintf(b, "%s: %d\n", title, len(items))
	}
	for i, it := range items {
		if i == max {
			fmt.Fprintf(b, "  ... %d more not shown (capped at %d; use filter to narrow, or full=true)\n", len(items)-max, max)
			break
		}
		b.WriteString("  " + clean(it) + "\n")
	}
}

func orDash(s []string) string {
	if len(s) == 0 {
		return "-"
	}
	return clean(strings.Join(s, ":"))
}

// clean strips control characters and caps length: names inside a binary are
// written by whoever built it, so they must not be able to shape the output.
func clean(s string) string {
	var out strings.Builder
	for _, r := range s {
		switch {
		case r < 0x20 || r == 0x7f:
			out.WriteRune('?')
		case r >= 0x200b && r <= 0x200f, r >= 0x202a && r <= 0x202e, r >= 0x2066 && r <= 0x2069, r == 0xfeff:
			// zero-width and bidi controls: they let text lie about its own shape
		default:
			out.WriteRune(r)
		}
		if out.Len() > maxStringLen*2 {
			out.WriteString("...")
			break
		}
	}
	return out.String()
}
