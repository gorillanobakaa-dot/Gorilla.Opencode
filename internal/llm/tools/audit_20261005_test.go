package tools

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/opencode-ai/opencode/internal/config"
)

// view shows a Windows file without its carriage returns, so the text a model
// sends back has plain line breaks. The edit must still find it, and must write
// the file's own line endings.
func TestEditTextIsMatchedToTheFilesLineEndings(t *testing.T) {
	crlfFile := "package main\r\n\r\nfunc a() {}\r\nfunc b() {}\r\n"
	asSeen := "func a() {}\nfunc b() {}"
	if strings.Contains(crlfFile, asSeen) {
		t.Fatal("test is not exercising the fault")
	}
	old := matchLineEndings(crlfFile, asSeen)
	if !strings.Contains(crlfFile, old) {
		t.Fatalf("a two-line old_string is still not found in a CRLF file: %q", old)
	}
	replacement := matchLineEndings(crlfFile, "func a() {}\nfunc c() {}\n")
	result := strings.Replace(crlfFile, old, replacement, 1)
	if strings.Contains(strings.ReplaceAll(result, "\r\n", ""), "\n") {
		t.Errorf("the edit left a bare line feed in a CRLF file: %q", result)
	}
	// An LF file stays LF even if the model sends CRLF.
	if got := matchLineEndings("a\nb\n", "a\r\nb"); got != "a\nb" {
		t.Errorf("LF file: %q", got)
	}
	// Idempotent.
	if again := matchLineEndings(crlfFile, old); again != old {
		t.Errorf("applied twice it changed the text: %q", again)
	}
}

func TestASearchPatternThatReachesForCredentialsFromOutsideTheProjectIsRefused(t *testing.T) {
	outside := t.TempDir() // not a workspace root
	for _, glob := range []string{".aws/**", "**/.ssh/*", "*.pem", "**/id_rsa", ".env", "**/.gnupg/**", `.aws\credentials`} {
		if why := refuseSensitiveGlob(glob, outside); why == "" {
			t.Errorf("glob %q from outside the project was allowed", glob)
		}
	}
	for _, glob := range []string{"*.go", "**/*.md", "src/**", "", ".github/**"} {
		if why := refuseSensitiveGlob(glob, outside); why != "" {
			t.Errorf("ordinary glob %q was refused: %s", glob, why)
		}
	}
}

// A shell command that names a path outside the project must present THAT
// path, so auto-approve's "not outside every workspace root" rule can apply.
func TestAShellCommandNamesThePathItReachesOutsideTheProject(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skip(err)
	}
	wd := config.WorkingDirectory()
	want := filepath.Join(home, ".ssh", "authorized_keys")
	cmds := []string{
		`echo x >> ~/.ssh/authorized_keys`,
		`cp key.pub $HOME/.ssh/authorized_keys`,
		`cat a.txt | tee "` + want + `"`,
	}
	if runtime.GOOS == "windows" {
		// Backslashes are path separators only on Windows.
		cmds = append(cmds, `Set-Content $env:USERPROFILE\.ssh\authorized_keys "ssh-ed25519 AAAA"`)
	}
	for _, cmd := range cmds {
		got := bashRequestPath(cmd)
		if !strings.EqualFold(filepath.Clean(got), filepath.Clean(want)) {
			t.Errorf("%q -> %q, want %q", cmd, got, want)
		}
	}
	// Ordinary project work stays the working directory, and the program being
	// RUN is not a place being touched.
	prog := filepath.Join(home, "go", "bin", "tool")
	for _, cmd := range []string{
		"go test ./...", "git status", "ls -la src", "cat ./README.md",
		"curl https://example.org/x", prog + " --version", "echo hi > /dev/null",
	} {
		if got := bashRequestPath(cmd); got != wd {
			t.Errorf("%q -> %q, want the working directory", cmd, got)
		}
	}
}

func TestWindowsTrailingDotsAndSpacesDoNotHideACredentialFile(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Win32 path normalisation")
	}
	home, _ := os.UserHomeDir()
	for _, p := range []string{
		filepath.Join(home, ".ssh.", "id_rsa."),
		filepath.Join(home, ".ssh ", "id_rsa"),
		filepath.Join(home, ".aws", "credentials. "),
	} {
		if why := RefuseSensitiveRead(p); why == "" {
			t.Errorf("%q opens a credential file on Windows and was allowed", p)
		}
	}
	// The program's own key store, whatever the case.
	base := config.ConfigBase()
	if why := RefuseSensitiveRead(filepath.Join(strings.ToUpper(base), "config.json")); why == "" {
		t.Errorf("the program's own config, upper-cased, was allowed")
	}
}

// Keys of saved connections and sign-in tokens must be on the masking list,
// and a key saved AFTER the list was first built must get onto it.
func TestConnectionKeysAndLaterKeysAreMasked(t *testing.T) {
	cfg := safeConfig()
	if cfg == nil {
		t.Skip("no configuration loaded in this test binary")
	}
	resetKnownSecretsForTest()
	t.Cleanup(resetKnownSecretsForTest)
	first := "nvapi-" + strings.Repeat("Qw9_", 12)
	saved := cfg.LocalEndpoints
	t.Cleanup(func() { cfg.LocalEndpoints = saved })
	cfg.LocalEndpoints = append([]config.LocalEndpoint{}, config.LocalEndpoint{Name: "Far", BaseURL: "https://example.org/v1", APIKey: first})

	out, n := MaskSecrets("view", "the key is "+first+" in this file")
	if n != 1 || strings.Contains(out, first) {
		t.Fatalf("a saved connection's key was not masked: %q", out)
	}

	// A second key arrives later. Force the list to be considered old.
	second := "nvapi-" + strings.Repeat("Zx7_", 12)
	cfg.LocalEndpoints = append(cfg.LocalEndpoints, config.LocalEndpoint{Name: "Later", BaseURL: "https://example.net/v1", APIKey: second})
	knownSecretsMu.Lock()
	knownSecretsAt = knownSecretsAt.Add(-11 * time.Second)
	knownSecretsMu.Unlock()
	if out, _ := MaskSecrets("view", "later key "+second); strings.Contains(out, second) {
		t.Errorf("a key saved after the list was built is never masked: %q", out)
	}
}
