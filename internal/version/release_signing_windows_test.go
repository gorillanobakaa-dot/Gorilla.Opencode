package version

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// runSigning runs step 5 of scripts/build-toolbox.ps1 alone on a copy of a
// small file, with the given signing variable.
func runSigning(t *testing.T, thumb string) (string, error, []byte, string) {
	t.Helper()
	script, err := filepath.Abs(filepath.Join("..", "..", "scripts", "build-toolbox.ps1"))
	if err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(t.TempDir(), "release-candidate.exe")
	content := []byte("MZ not a real program, only bytes the signing step must leave alone when off")
	if err := os.WriteFile(target, content, 0o644); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("powershell.exe", "-NoProfile", "-NonInteractive", "-ExecutionPolicy", "Bypass",
		"-File", script, "-SignOnly", target)
	cmd.Env = append(os.Environ(), "GORILLA_SIGN_CERT_SHA1="+thumb)
	out, runErr := cmd.CombinedOutput()
	after, _ := os.ReadFile(target)
	return string(out), runErr, content, string(after)
}

// TestReleaseSigningIsOffWithoutTheOwnersCertificate (gap #24): with no
// certificate configured the release step says "not signed", succeeds, and
// leaves the file as it was.
func TestReleaseSigningIsOffWithoutTheOwnersCertificate(t *testing.T) {
	out, err, before, after := runSigning(t, "")
	if err != nil {
		t.Fatalf("the signing step failed with no certificate configured: %v\n%s", err, out)
	}
	if !strings.Contains(out, "not signed") || !strings.Contains(out, "GORILLA_SIGN_CERT_SHA1") {
		t.Errorf("the step does not say the file is unsigned and why:\n%s", out)
	}
	if !bytes.Equal([]byte(after), before) {
		t.Error("the file changed although signing is off")
	}
}

// TestReleaseSigningFailsClosed: signing that was asked for and cannot
// happen (no such certificate) stops the release instead of shipping an
// unsigned file quietly.
func TestReleaseSigningFailsClosed(t *testing.T) {
	out, err, _, _ := runSigning(t, strings.Repeat("0", 40))
	if err == nil {
		t.Fatalf("a signing request with a certificate that does not exist succeeded:\n%s", out)
	}
	if !strings.Contains(out, "NOT signed") {
		t.Errorf("the failure does not say the exe is not signed:\n%s", out)
	}
}
