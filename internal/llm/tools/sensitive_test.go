package tools

// GORILLA OVERRIDE (2026-08-18): credential-read refusal.
//
// The asymmetry under test: refusing an SSH private key costs nothing, because
// nothing legitimate asks for one. Refusing project files would cost everything,
// because reading them is the job. Both halves are asserted — the second is the
// capability guard.

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/opencode-ai/opencode/internal/config"
)

func TestCredentialPathsOutsideTheProjectAreRefused(t *testing.T) {
	for _, p := range []string{
		"/home/gorilla/.ssh/id_rsa",
		"/home/gorilla/.ssh/id_ed25519",
		"/home/gorilla/.aws/credentials",
		"/home/gorilla/.gnupg/secring.gpg",
		"/home/gorilla/.netrc",
		"/home/gorilla/certs/server.pem",
		"/home/gorilla/secret.key",
		"/home/gorilla/.config/gorilla-opencode/config.json",
		"/home/gorilla/.kube/config",
	} {
		if why := RefuseSensitiveRead(p); why == "" {
			t.Errorf("ALLOWED a credential read: %s", p)
		} else if !strings.Contains(why, "Refusing") {
			t.Errorf("refusal for %s does not explain itself: %q", p, why)
		}
	}
}

// CAPABILITY GUARD. Ordinary source files must be readable, including ones whose
// names would look alarming out of context.
func TestOrdinaryProjectFilesAreStillReadable(t *testing.T) {
	wd := config.WorkingDirectory()
	if wd == "" {
		t.Skip("no workspace")
	}
	for _, p := range []string{
		"main.go", "internal/config/config.go", "README.md",
		"testdata/key.pem",       // a fixture INSIDE the project is fine
		"internal/.ssh-notes.md", // not a .ssh component
		filepath.Join(wd, "go.mod"),
	} {
		if why := RefuseSensitiveRead(p); why != "" {
			t.Errorf("REGRESSION: refused an ordinary project file %q: %s", p, why)
		}
	}
}

// The directory match is per path COMPONENT, so a lookalike name is not caught.
func TestLookalikeDirectoryNamesAreNotCaught(t *testing.T) {
	for _, p := range []string{"/home/gorilla/my.ssh-notes/readme.md", "/home/gorilla/sshconfig/x.md"} {
		if why := RefuseSensitiveRead(p); why != "" {
			t.Errorf("a lookalike name was wrongly refused: %q -> %s", p, why)
		}
	}
}
