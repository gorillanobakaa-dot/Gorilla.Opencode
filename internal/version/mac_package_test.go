package version

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// scripts/packaging/build_mac_legacy_package.py turns the darwin binary and
// packaging/macos into what "Install Gorilla OpenCode.command" unpacks. It was
// written on 2026-10-19 but left untracked, so a fresh clone could not build the
// Mac release (found from the Linux machine, 2026-10-23). This builds a package
// from a stand-in binary and checks it against what the installer looks for:
// names, execute bits, LF line endings, the checksum line shasum -c reads, the
// version in Info.plist and the icon the plist names.

func findPython(t *testing.T) string {
	t.Helper()
	for _, name := range []string{"python3", "python"} {
		if p, err := exec.LookPath(name); err == nil {
			return p
		}
	}
	t.Skip("no python on PATH")
	return ""
}

func runPackager(t *testing.T, py string, binary []byte, version string) (string, []byte, error) {
	t.Helper()
	dir := t.TempDir()
	bin := filepath.Join(dir, "gorilla-opencode-mac")
	if err := os.WriteFile(bin, binary, 0o755); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(dir, "out")
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	b, err := exec.CommandContext(ctx, py, "-I", "../../scripts/packaging/build_mac_legacy_package.py",
		"../..", bin, version, out).CombinedOutput()
	return out, b, err
}

func TestMacPackageMatchesTheInstaller(t *testing.T) {
	py := findPython(t)
	const v = "9.9.9-test"
	prog := []byte("\xcf\xfa\xed\xfe fake darwin binary, stamped v" + v + "\x00")
	out, msg, err := runPackager(t, py, prog, v)
	if err != nil {
		t.Fatalf("packager failed: %v\n%s", err, msg)
	}
	folder := filepath.Join(out, "gorilla-opencode-"+v+"-mac")
	tgzName := "gorilla-opencode-" + v + "-mac-10.13-x86_64.tar.gz"
	for _, f := range []string{"Install Gorilla OpenCode.command", "Undo Gorilla OpenCode.command", "READ-ME-FIRST.txt", tgzName, "SHA256SUMS-" + v + "-mac.txt"} {
		if _, err := os.Stat(filepath.Join(folder, f)); err != nil {
			t.Errorf("the folder lacks %s", f)
		}
	}

	// The checksum line, as shasum -a 256 -c reads it in the installer.
	tgz, err := os.ReadFile(filepath.Join(folder, tgzName))
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(tgz)
	sums, _ := os.ReadFile(filepath.Join(folder, "SHA256SUMS-"+v+"-mac.txt"))
	if want := hex.EncodeToString(sum[:]) + "  " + tgzName + "\n"; string(sums) != want {
		t.Errorf("SHA256SUMS is %q, want %q", sums, want)
	}

	// The archive: every path the installer names, with the right execute bits.
	install, _ := os.ReadFile("../../packaging/macos/Install Gorilla OpenCode.command")
	top := "gorilla-opencode-" + v + "-mac/"
	entries := map[string]*tar.Header{}
	contents := map[string][]byte{}
	gz, err := gzip.NewReader(bytes.NewReader(tgz))
	if err != nil {
		t.Fatal(err)
	}
	tr := tar.NewReader(gz)
	for {
		h, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		entries[h.Name] = h
		contents[h.Name], _ = io.ReadAll(tr)
	}
	app := top + "Gorilla OpenCode.app/Contents/"
	wantExec := map[string]string{
		top + "gorilla-opencode-mac":               `"$SRC/gorilla-opencode-mac"`,
		app + "MacOS/gorilla-opencode-launch":      `"$SRC/Gorilla OpenCode.app/Contents/MacOS/gorilla-opencode-launch"`,
		app + "Resources/Gorilla OpenCode.command": `"$SRC/Gorilla OpenCode.app/Contents/Resources/Gorilla OpenCode.command"`,
	}
	for name, inInstaller := range wantExec {
		h := entries[name]
		if h == nil {
			t.Errorf("the archive lacks %s", name)
			continue
		}
		if h.Mode&0o111 == 0 {
			t.Errorf("%s is not executable (mode %o)", name, h.Mode)
		}
		if !strings.Contains(string(install), inInstaller) {
			t.Errorf("the installer no longer names %s", inInstaller)
		}
	}
	if !bytes.Equal(contents[top+"gorilla-opencode-mac"], prog) {
		t.Error("the program in the archive is not the binary given")
	}
	for _, name := range []string{app + "MacOS/gorilla-opencode-launch", app + "Resources/Gorilla OpenCode.command"} {
		if bytes.Contains(contents[name], []byte("\r\n")) {
			t.Errorf("%s has CRLF line endings", name)
		}
	}
	plist := string(contents[app+"Info.plist"])
	if !strings.Contains(plist, "<string>"+v+"</string>") || strings.Contains(plist, "@VERSION@") {
		t.Errorf("Info.plist does not carry the version %s", v)
	}
	if !strings.Contains(plist, "<key>CFBundleIconFile</key>\n\t<string>gorilla</string>") {
		t.Error("Info.plist does not name the icon gorilla")
	}
	if icns := contents[app+"Resources/gorilla.icns"]; len(icns) < 8 || string(icns[:4]) != "icns" {
		t.Error("gorilla.icns is missing or not an icns file")
	}

	// The .zip for the release page keeps the execute bit on the two scripts.
	z, err := zip.OpenReader(filepath.Join(out, "gorilla-opencode-"+v+"-mac-10.13.zip"))
	if err != nil {
		t.Fatal(err)
	}
	defer z.Close()
	seen := 0
	for _, f := range z.File {
		if strings.HasSuffix(f.Name, ".command") {
			seen++
			if f.Mode()&0o111 == 0 {
				t.Errorf("%s in the zip is not executable", f.Name)
			}
		}
	}
	if seen != 2 {
		t.Errorf("the zip holds %d .command files, want 2", seen)
	}
}

// A binary without the version stamp is refused: the installer would reject it
// on the Mac, after the download.
func TestMacPackageRefusesAnUnstampedBinary(t *testing.T) {
	py := findPython(t)
	_, msg, err := runPackager(t, py, []byte("no stamp here"), "9.9.9-test")
	if err == nil || !strings.Contains(string(msg), "version stamp") {
		t.Fatalf("want a refusal naming the version stamp, got %v\n%s", err, msg)
	}
}
