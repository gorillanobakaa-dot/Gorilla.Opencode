package version

import (
	"bytes"
	"context"
	"encoding/binary"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// scripts/build-darwin-legacy.sh ends by checking that the macOS 10.13 binary
// does not import SecTrustEvaluateWithError (macOS 10.14+). It used to grep the
// whole file, and the binary embeds the project's LESSONS, one of which quotes
// "Symbol not found: _SecTrustEvaluateWithError": every good binary failed
// (seen on the Linux build, 2026-10-23). The check now reads the import names
// from the __LINKEDIT segment only. This runs that check, cut out of the
// script, on fake Mach-O files.

func darwinCheck(t *testing.T) string {
	t.Helper()
	src, err := os.ReadFile("../../scripts/build-darwin-legacy.sh")
	if err != nil {
		t.Fatal(err)
	}
	text := string(src)
	verify := strings.Index(text, "# Verify:")
	if verify < 0 {
		t.Fatal("no '# Verify:' block in the script")
	}
	open := "<<'PY'\n"
	start := strings.Index(text[verify:], open)
	if start < 0 {
		t.Fatal("no Python check after '# Verify:'")
	}
	start += verify + len(open)
	end := strings.Index(text[start:], "\nPY\n")
	if end < 0 {
		t.Fatal("end of the Python check not found")
	}
	return text[start : start+end+1]
}

// fakeMachO builds a 64-bit Mach-O header with a __TEXT and a __LINKEDIT
// segment holding the given bytes, plus any extra load command numbers.
func fakeMachO(text, linkedit []byte, extra ...uint32) []byte {
	le := binary.LittleEndian
	seg := func(name string, off, size uint64) []byte {
		b := make([]byte, 72)
		le.PutUint32(b[0:], 0x19)
		le.PutUint32(b[4:], 72)
		copy(b[8:24], name)
		le.PutUint64(b[40:], off)
		le.PutUint64(b[48:], size)
		return b
	}
	ncmds := 2 + len(extra)
	data := uint64(32 + 72*2 + 8*len(extra))
	var cmds []byte
	cmds = append(cmds, seg("__TEXT", data, uint64(len(text)))...)
	cmds = append(cmds, seg("__LINKEDIT", data+uint64(len(text)), uint64(len(linkedit)))...)
	for _, c := range extra {
		b := make([]byte, 8)
		le.PutUint32(b[0:], c)
		le.PutUint32(b[4:], 8)
		cmds = append(cmds, b...)
	}
	h := make([]byte, 32)
	le.PutUint32(h[0:], 0xfeedfacf)
	le.PutUint32(h[16:], uint32(ncmds))
	le.PutUint32(h[20:], uint32(len(cmds)))
	out := append(h, cmds...)
	out = append(out, text...)
	return append(out, linkedit...)
}

func TestDarwinBuildImportCheck(t *testing.T) {
	py := ""
	for _, name := range []string{"python3", "python"} {
		if p, err := exec.LookPath(name); err == nil {
			py = p
			break
		}
	}
	if py == "" {
		t.Skip("no python on PATH")
	}
	check := darwinCheck(t)
	lesson := []byte("dyld: Symbol not found: _SecTrustEvaluateWithError\x00")
	good := []byte("\x00_SecTrustEvaluate\x00_SecTrustCreateWithCertificates\x00")
	bad := []byte("\x00_SecTrustEvaluate\x00_SecTrustEvaluateWithError\x00")
	cases := []struct {
		name string
		bin  []byte
		fail string // "" means the check must pass
	}{
		{"lesson text in __TEXT, good imports", fakeMachO(lesson, good), ""},
		{"the 10.14 import in __LINKEDIT", fakeMachO(nil, bad), "still imports SecTrustEvaluateWithError"},
		{"no import names found", fakeMachO(lesson, []byte("\x00_other\x00")), "import names were not found"},
		{"chained fixups", fakeMachO(nil, good, 0x80000034), "LC_DYLD_CHAINED_FIXUPS"},
		{"not a Mach-O file", []byte("#!/bin/sh\n" + strings.Repeat("x", 40)), "not a 64-bit Mach-O"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			bin := filepath.Join(t.TempDir(), "gorilla-opencode-mac")
			if err := os.WriteFile(bin, c.bin, 0o644); err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
			defer cancel()
			cmd := exec.CommandContext(ctx, py, "-I", "-", bin)
			cmd.Stdin = strings.NewReader(check)
			out, err := cmd.CombinedOutput()
			if c.fail == "" {
				if err != nil {
					t.Fatalf("a good binary failed: %v\n%s", err, out)
				}
				return
			}
			if err == nil || !bytes.Contains(out, []byte(c.fail)) {
				t.Fatalf("want a failure naming %q, got %v\n%s", c.fail, err, out)
			}
		})
	}
}
