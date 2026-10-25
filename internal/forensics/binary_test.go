package forensics

import (
	"bytes"
	"debug/macho"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// The fixtures are built from testdata/fx.c; testdata/README.md has the
// commands. They are checked in so the tests need no compiler.

func mustInspect(t *testing.T, path string, o Options) string {
	t.Helper()
	out, err := Inspect(path, o)
	if err != nil {
		t.Fatalf("Inspect(%s, %+v): %v", path, o, err)
	}
	return out
}

func wantAll(t *testing.T, out string, wants ...string) {
	t.Helper()
	for _, w := range wants {
		if !strings.Contains(out, w) {
			t.Errorf("output lacks %q:\n%s", w, out)
		}
	}
}

func TestELFFixture(t *testing.T) {
	out := mustInspect(t, "testdata/fx.elf", Options{})
	wantAll(t, out,
		"format: ELF64 little-endian, type EXEC",
		"machine EM_X86_64",
		"interpreter: /lib64/ld-linux-x86-64.so.2",
		"needed libraries: 1\n  libc.so.6",
		"rpath: -; runpath: $ORIGIN/lib",
		" in .symtab, 2 dynamic",
	)
	// .text is executable and has real entropy; .got is all zeros.
	if !regexpLine(out, `\.text\s+\d+\s+[1-7]\.\d\d x`) {
		t.Errorf(".text row lacks entropy or the x flag:\n%s", out)
	}
	wantAll(t, mustInspect(t, "testdata/fx.elf", Options{Part: PartImports}),
		"puts@GLIBC_2.2.5 (libc.so.6)")
	sym := mustInspect(t, "testdata/fx.elf", Options{Part: PartSymbols, Filter: "gorilla"})
	wantAll(t, sym, `symbols matching "gorilla": 2 of`, "func gorilla_marker", "data gorilla_wide")
}

func TestPEFixture(t *testing.T) {
	out := mustInspect(t, "testdata/fx.pe", Options{})
	wantAll(t, out,
		"format: PE32+ (64-bit) EXE, machine x86_64, subsystem console",
		"KERNEL32.dll (",
		"signature: none",
		"sections (",
	)
	if !regexpLine(out, `\.text\s+\d+\s+\d\.\d\d x`) {
		t.Errorf(".text row lacks entropy or the x flag:\n%s", out)
	}
	imp := mustInspect(t, "testdata/fx.pe", Options{Part: PartImports, Filter: "puts"})
	wantAll(t, imp, "api-ms-win-crt-stdio-l1-1-0.dll!puts", `matching "puts": 1 of`)
	wantAll(t, mustInspect(t, "testdata/fx.pe", Options{Part: PartSymbols}), "no COFF symbols")
}

func TestMachOFatFixture(t *testing.T) {
	out := mustInspect(t, "testdata/fx-fat.macho", Options{})
	x86, arm, ok := strings.Cut(out, "== slice arm64")
	if !ok {
		t.Fatalf("no arm64 slice:\n%s", out)
	}
	wantAll(t, out, "format: Mach-O fat (universal), 2 slices")
	wantAll(t, x86,
		"== slice x86_64",
		"minimum macOS: 10.13 (sdk ",
		"from LC_VERSION_MIN_MACOSX",
		"check: OK by header (needs 10.13)",
		"/usr/lib/libSystem.B.dylib",
		"__TEXT,__text",
	)
	wantAll(t, arm,
		"minimum macOS: 11.0",
		"from LC_BUILD_VERSION",
		"check: NO: arm64 slice, the Air is Intel",
		"code signature: present, NOT verified (chain and hash not checked)",
	)
	imp := mustInspect(t, "testdata/fx-fat.macho", Options{Part: PartImports})
	if strings.Count(imp, "_puts") != 2 {
		t.Errorf("want _puts imported by both slices:\n%s", imp)
	}
}

// A thin Mach-O is a different code path from a fat one: cut the x86_64 slice
// out of the fat fixture and inspect it alone.
func TestMachOThinSlice(t *testing.T) {
	f, err := os.Open("testdata/fx-fat.macho")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	ff, err := macho.NewFatFile(f)
	if err != nil {
		t.Fatal(err)
	}
	var thin []byte
	for _, a := range ff.Arches {
		if a.Cpu == macho.CpuAmd64 {
			thin = make([]byte, a.Size)
			if _, err := f.ReadAt(thin, int64(a.Offset)); err != nil {
				t.Fatal(err)
			}
		}
	}
	p := filepath.Join(t.TempDir(), "thin")
	if err := os.WriteFile(p, thin, 0o644); err != nil {
		t.Fatal(err)
	}
	out := mustInspect(t, p, Options{})
	wantAll(t, out, "format: Mach-O thin", "arch: x86_64", "check: OK by header", "code signature: none")
	wantAll(t, mustInspect(t, p, Options{Part: PartSymbols, Filter: "gorilla"}), "def _gorilla_wide")
}

func TestLegacyVerdict(t *testing.T) {
	const minCmd, buildCmd = "LC_VERSION_MIN_MACOSX", "LC_BUILD_VERSION"
	cases := []struct {
		cpu  macho.Cpu
		min  string
		src  string
		want string
	}{
		{macho.CpuAmd64, "10.13", minCmd, "OK"},
		{macho.CpuAmd64, "10.9", minCmd, "OK"},
		{macho.CpuAmd64, "10.13.4", minCmd, "OK"},
		{macho.CpuAmd64, "10.13.6", minCmd, "OK"},
		{macho.CpuAmd64, "10.14", minCmd, "UNPROVEN: LC_VERSION_MIN_MACOSX says 10.14"},
		{macho.CpuAmd64, "11.0", minCmd, "UNPROVEN: LC_VERSION_MIN_MACOSX says 11.0"},
		// Measured 2026-10-25: 10.13.6 ran copies marked LC_VERSION_MIN_MACOSX 10.14
		// and LC_BUILD_VERSION 10.14 / 99.0. A header above 10.13 is never a refusal.
		// 10.13 does not read LC_BUILD_VERSION: Gorilla's own 10.13 build
		// says 11.0 there and runs on the Air.
		{macho.CpuAmd64, "11.0", buildCmd, "UNPROVEN: LC_BUILD_VERSION says 11.0"},
		{macho.CpuAmd64, "10.12", buildCmd, "OK"},
		{macho.CpuAmd64, "", "", "UNKNOWN"},
		{macho.CpuArm64, "10.9", minCmd, "NO: arm64"},
		{macho.Cpu386, "10.6", minCmd, "OK"},
	}
	for _, c := range cases {
		if got := legacyVerdict(c.cpu, c.min, c.src); !strings.HasPrefix(got, c.want) {
			t.Errorf("legacyVerdict(%v, %q, %s) = %q, want prefix %q", c.cpu, c.min, c.src, got, c.want)
		}
	}
	if machoVer(0x000a0d00) != "10.13" || machoVer(0x000a0d06) != "10.13.6" || machoVer(0x000b0000) != "11.0" {
		t.Errorf("machoVer decodes wrongly: %s %s %s", machoVer(0x000a0d00), machoVer(0x000a0d06), machoVer(0x000b0000))
	}
}

func TestStringsFindASCIIAndUTF16(t *testing.T) {
	for _, p := range []string{"testdata/fx.elf", "testdata/fx.pe", "testdata/fx-fat.macho"} {
		out := mustInspect(t, p, Options{Part: PartStrings, Filter: "GORILLA_"})
		if !regexpLine(out, `0x[0-9a-f]+ a GORILLA_FIXTURE_ASCII$`) {
			t.Errorf("%s: ASCII string not found:\n%s", p, out)
		}
		if !regexpLine(out, `0x[0-9a-f]+ w GORILLA_WIDE_16$`) {
			t.Errorf("%s: UTF-16LE string not found:\n%s", p, out)
		}
	}
}

func TestStringsOddOffsetAndMinLength(t *testing.T) {
	// A wide string starting on an odd offset, an ASCII run one short of the
	// minimum, and one exactly at it.
	data := []byte{0xff}
	data = append(data, utf16le("ODDWIDE")...)
	data = append(data, 0xff, 0xff)
	data = append(data, "abcde"...) // 5: under min 6
	data = append(data, 0)
	data = append(data, "abcdef"...) // 6: reported
	data = append(data, 0)
	var got []foundString
	scanStrings(bytes.NewReader(data), 6, 1<<20, func(s foundString) bool { got = append(got, s); return true })
	want := []foundString{{Off: 1, Wide: true, Text: "ODDWIDE"}, {Off: 23, Text: "abcdef"}}
	if len(got) != len(want) {
		t.Fatalf("got %+v, want %+v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("string %d = %+v, want %+v", i, got[i], want[i])
		}
	}
}

func TestStringsCapIsReported(t *testing.T) {
	var data []byte
	for i := 0; i < maxStrings+25; i++ {
		data = append(data, []byte("stringnumber")...)
		data = append(data, 0)
	}
	p := filepath.Join(t.TempDir(), "many.bin")
	os.WriteFile(p, data, 0o644)
	out := mustInspect(t, p, Options{Part: PartStrings})
	wantAll(t, out, "ASCII + UTF-16LE): 175", "... 25 more not shown (capped at 150")
}

// The three users (owner, 2026-10-22): a person and a large model ask for full=true and get every
// line; a small local model gets the capped default. Both are held here.
func TestFullLiftsEveryCap(t *testing.T) {
	var data []byte
	for i := 0; i < maxStrings+25; i++ {
		data = append(data, []byte("stringnumber")...)
		data = append(data, 0)
	}
	p := filepath.Join(t.TempDir(), "many.bin")
	os.WriteFile(p, data, 0o644)
	full := mustInspect(t, p, Options{Part: PartStrings, Full: true})
	if strings.Contains(full, "more not shown") || strings.Count(full, " a stringnumber") != maxStrings+25 {
		t.Errorf("full=true still capped:\n%s", full[len(full)-300:])
	}
	wantAll(t, mustInspect(t, p, Options{Part: PartStrings}), "or full=true")

	old := stringScanCap
	stringScanCap = 64
	defer func() { stringScanCap = old }()
	wantAll(t, mustInspect(t, p, Options{Part: PartStrings}), "scan capped: only the first 64 of")
	if strings.Contains(mustInspect(t, p, Options{Part: PartStrings, Full: true}), "scan capped") {
		t.Error("full=true still stopped the scan")
	}

	if (Options{}).limit(40) != 40 || (Options{Full: true}).limit(40) < 1<<30 {
		t.Error("limit() does not follow Full")
	}
}

func TestEntropy(t *testing.T) {
	zeros := make([]byte, 4096)
	if got := entropyOf(bytes.NewReader(zeros), 4096); got != "0.00" {
		t.Errorf("zeros entropy = %s, want 0.00", got)
	}
	all := make([]byte, 256*16)
	for i := range all {
		all[i] = byte(i)
	}
	if got := entropyOf(bytes.NewReader(all), uint64(len(all))); got != "8.00" {
		t.Errorf("uniform entropy = %s, want 8.00", got)
	}
	// A section larger than what was read is marked as a sample.
	if got := entropyOf(bytes.NewReader(all), uint64(len(all))*2); !strings.HasPrefix(got, "~") {
		t.Errorf("sampled entropy = %s, want ~ prefix", got)
	}
}

func TestListCapAndFilter(t *testing.T) {
	var items []string
	for i := 0; i < 50; i++ {
		items = append(items, "item")
	}
	var b strings.Builder
	writeList(&b, "things", items, 40, "")
	wantAll(t, b.String(), "things: 50", "... 10 more not shown (capped at 40")
	b.Reset()
	writeList(&b, "things", []string{"alpha", "Beta", "gamma"}, 40, "BET")
	wantAll(t, b.String(), `things matching "BET": 1 of 3`, "  Beta")
	if strings.Contains(b.String(), "alpha") {
		t.Errorf("filter let alpha through:\n%s", b.String())
	}
}

func TestRefusesNonBinariesAndUnknownParts(t *testing.T) {
	p := filepath.Join(t.TempDir(), "notes.txt")
	os.WriteFile(p, []byte("just some text here"), 0o644)
	if _, err := Inspect(p, Options{}); err == nil || !strings.Contains(err.Error(), "not an ELF, PE or Mach-O") {
		t.Errorf("text file: err = %v", err)
	}
	// strings still works on it
	wantAll(t, mustInspect(t, p, Options{Part: PartStrings}), "just some text here")
	if _, err := Inspect("testdata/fx.elf", Options{Part: "everything"}); err == nil {
		t.Error("unknown part accepted")
	}
	if _, err := Inspect("testdata", Options{}); err == nil {
		t.Error("folder accepted")
	}
}

func TestCleanStripsControlAndBidi(t *testing.T) {
	got := clean("a\x1b[31mb\u202ec\u200bd")
	if got != "a?[31mbcd" {
		t.Errorf("clean = %q", got)
	}
}

func utf16le(s string) []byte {
	var out []byte
	for _, c := range []byte(s) {
		out = append(out, c, 0)
	}
	return out
}

func regexpLine(out, pattern string) bool {
	re := regexp.MustCompile(pattern)
	for _, l := range strings.Split(out, "\n") {
		if re.MatchString(strings.TrimSpace(l)) {
			return true
		}
	}
	return false
}
