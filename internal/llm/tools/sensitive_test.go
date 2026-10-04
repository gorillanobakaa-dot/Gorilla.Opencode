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

// GORILLA OVERRIDE (2026-09-01): the Windows hole, kept closed.
//
// filepath.IsAbs is false for "/home/user/.ssh/id_rsa" on Windows — Windows
// calls that rooted-but-volume-relative. The guard therefore joined it onto the
// project directory, found the result inside the workspace (which is exempt),
// and allowed the read. A path being rooted must never be mistaken for a path
// being relative, on any platform: that is the difference between "somewhere
// else on this machine" and "inside the project the user chose".
func TestRootedPathsAreNotTreatedAsProjectRelative(t *testing.T) {
	for _, p := range []string{
		"/home/gorilla/.ssh/id_rsa",
		"/root/.aws/credentials",
		"/etc/ssl/private/server.key",
	} {
		if why := RefuseSensitiveRead(p); why == "" {
			t.Errorf("ALLOWED %s — a rooted path was resolved as if it were inside the project, "+
				"which exempts it from the credential guard entirely", p)
		}
	}
}

// The exemption itself must still work: a fixture inside the project is readable
// even when its name looks alarming. Removing that would cost the tool its job.
func TestWorkspaceFixturesAreStillExempt(t *testing.T) {
	wd := config.WorkingDirectory()
	if wd == "" {
		t.Skip("no workspace")
	}
	p := filepath.Join(wd, "testdata", "server.pem")
	if why := RefuseSensitiveRead(p); why != "" {
		t.Errorf("refused a file inside the workspace: %s\n%s", p, why)
	}
}

// GORILLA OVERRIDE (2026-10-04): the names taken from comparing this list with
// alibaba/open-code-review's secret patterns.
func TestRegistryTokensAndEnvFilesOutsideTheProjectAreRefused(t *testing.T) {
	for _, p := range []string{
		"/home/gorilla/_netrc",
		"/home/gorilla/.npmrc",
		"/home/gorilla/.pypirc",
		"/home/gorilla/.dockercfg",
		"/home/gorilla/other-project/.env",
		"/home/gorilla/other-project/.env.production",
		"/home/gorilla/other-project/.ENV.local",
	} {
		if why := RefuseSensitiveRead(p); why == "" {
			t.Errorf("ALLOWED a credential read: %s", p)
		}
	}
}

// CAPABILITY GUARD. The template spellings hold placeholders and are what a
// person is told to copy from; a name that merely starts with "env" is not an
// environment file at all.
func TestEnvTemplatesAndLookalikesStayReadable(t *testing.T) {
	for _, p := range []string{
		"/home/gorilla/other-project/.env.example",
		"/home/gorilla/other-project/.env.sample",
		"/home/gorilla/other-project/.env.template",
		"/home/gorilla/other-project/.envrc.md",
		"/home/gorilla/other-project/environment.md",
		"/home/gorilla/other-project/env.go",
	} {
		if why := RefuseSensitiveRead(p); why != "" {
			t.Errorf("REGRESSION: refused a harmless file %q: %s", p, why)
		}
	}
}

// And inside the project the user's own .env is still theirs to open.
func TestAnEnvFileInsideTheWorkspaceIsStillExempt(t *testing.T) {
	wd := config.WorkingDirectory()
	if wd == "" {
		t.Skip("no workspace")
	}
	if why := RefuseSensitiveRead(filepath.Join(wd, ".env")); why != "" {
		t.Errorf("refused a file inside the workspace: %s", why)
	}
}
