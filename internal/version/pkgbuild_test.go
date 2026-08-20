package version

// GORILLA OVERRIDE (2026-08-20): packaging/PKGBUILD is the ONE packaging recipe
// that is published, and it is the install route the README offers Arch users
// first. Nothing checked it, so it sat at pkgver=0.1.85 while releases reached
// 0.1.111 - twenty-six behind. Following the project's own instructions built a
// version from July.
//
// A stale PKGBUILD is invisible by construction: it is still VALID. makepkg
// succeeds, the package installs, and only the version string betrays it. That
// is the same shape as the release-notes glob found the same day - a thing that
// keeps passing after the world moved under it.
//
// These tests read the tracked file only. They cannot reach build-deb.sh or
// build-arch.sh, which are local tooling, so they pin what is checkable from
// the repository alone.

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
)

func readPKGBUILD(t *testing.T) string {
	t.Helper()
	b, err := os.ReadFile("../../packaging/PKGBUILD")
	if err != nil {
		t.Skipf("no PKGBUILD: %v", err)
	}
	return string(b)
}

// latestReleaseNotesVersion is the newest release the repository documents.
// ReleaseNotes/ is the authority because a release page is written for every
// release; git tags are not readable from a test and Compiled.Builds is ignored.
func latestReleaseNotesVersion(t *testing.T) string {
	t.Helper()
	entries, err := os.ReadDir("../../ReleaseNotes")
	if err != nil {
		t.Skipf("no ReleaseNotes: %v", err)
	}
	var vers []string
	for _, e := range entries {
		n := e.Name()
		if !strings.HasPrefix(n, "GITHUB-RELEASE-NOTES-v") || !strings.HasSuffix(n, ".md") {
			continue
		}
		vers = append(vers, strings.TrimSuffix(strings.TrimPrefix(n, "GITHUB-RELEASE-NOTES-"), ".md"))
	}
	if len(vers) == 0 {
		t.Fatal("no release pages found - this guard is not looking at anything")
	}
	// atLeast is the NUMERIC comparison from release_screenshots_test.go. Sorting
	// these as strings puts v0.1.83 after v0.1.103, which is the exact bug that
	// file already records.
	sort.Slice(vers, func(i, j int) bool { return atLeast(vers[j], vers[i]) })
	return strings.TrimPrefix(vers[len(vers)-1], "v")
}

func TestPKGBUILDTracksTheCurrentRelease(t *testing.T) {
	src := readPKGBUILD(t)
	m := regexp.MustCompile(`(?m)^pkgver=(.+)$`).FindStringSubmatch(src)
	if m == nil {
		t.Fatal("PKGBUILD has no pkgver= line")
	}
	got, want := strings.TrimSpace(m[1]), latestReleaseNotesVersion(t)
	if got != want {
		t.Errorf("PKGBUILD pkgver is %s but the newest documented release is %s.\n"+
			"  This file is install route 1 in the README, so it is what an Arch user\n"+
			"  actually builds. A stale version here is still a VALID PKGBUILD - makepkg\n"+
			"  succeeds and the package installs - which is why it went twenty-six\n"+
			"  releases without being noticed. Bump pkgver AND regenerate sha256sums.", got, want)
	}
}

// The checksum must be real. SKIP disables integrity checking altogether, which
// on a project whose SECURITY.md invites people to verify their download is the
// "transparent in theory, closed door in practice" failure PHILOSOPHY.md exists
// to prevent. It was SKIP from the day the file was written, under a comment
// telling the reader to regenerate it.
func TestPKGBUILDChecksumIsNotSKIP(t *testing.T) {
	src := readPKGBUILD(t)
	m := regexp.MustCompile(`(?m)^sha256sums=\((.*)\)$`).FindStringSubmatch(src)
	if m == nil {
		t.Fatal("PKGBUILD has no sha256sums= line")
	}
	sum := strings.Trim(strings.TrimSpace(m[1]), "'\"")
	if sum == "SKIP" {
		t.Error("sha256sums is SKIP - makepkg will accept whatever arrives from the network.\n" +
			"  Regenerate with `makepkg -g`, or sha256sum the tagged tarball directly.")
	}
	if len(sum) != 64 {
		t.Errorf("sha256sums=%q is %d chars; a sha256 is 64 hex characters", sum, len(sum))
	}
}

// python is a HARD runtime dependency: internal/llm/tools/find.go refuses to run
// without it, and find replaced ls, grep and glob. This declared only lynx, so
// pacman reported a clean install and the agent then had no search at all.
//
// The name is checked deliberately. Arch ships it as `python`; `python3` is the
// DEBIAN name and does not exist as an Arch package, so transplanting the .deb
// string verbatim turns a silent gap into a package that cannot install.
func TestPKGBUILDDeclaresPythonByItsArchName(t *testing.T) {
	src := readPKGBUILD(t)
	m := regexp.MustCompile(`(?m)^depends=\((.*)\)$`).FindStringSubmatch(src)
	if m == nil {
		t.Fatal("PKGBUILD has no depends= line")
	}
	deps := m[1]
	if !strings.Contains(deps, "'python'") {
		t.Errorf("depends=(%s) does not include 'python'.\n"+
			"  find.go returns \"the find tool needs python3 (its search engine is a Python\n"+
			"  program embedded in this binary)\" and refuses. Without it the agent ships\n"+
			"  with no search tool at all.", deps)
	}
	if strings.Contains(deps, "'python3'") {
		t.Errorf("depends=(%s) uses 'python3', which is the DEBIAN package name.\n"+
			"  Arch has no such package, so this makes the PKGBUILD uninstallable - a\n"+
			"  silent gap turned into a hard failure. Use 'python'.", deps)
	}
}

// Third copy of one bug. The dual-track docs are v<ver>-release-notes.layman.md
// and .developer.md; a glob anchored as *release-notes.md matches NEITHER, so
// the package ships every older release's notes and none for the release it is.
// The full-looking directory is what hides it.
func TestPKGBUILDShipsTheSplitReleaseNotes(t *testing.T) {
	src := readPKGBUILD(t)
	if strings.Contains(src, "Changelogs/*release-notes.md") {
		t.Error("PKGBUILD globs Changelogs/*release-notes.md, which matches neither\n" +
			"  v<ver>-release-notes.layman.md nor .developer.md. Use *release-notes*.md.\n" +
			"  Verify by grepping the built package for the SPECIFIC version, never for\n" +
			"  the word \"release-notes\" - sixty historical files make that read as a pass.")
	}
	if !strings.Contains(src, "Changelogs/*release-notes*.md") {
		t.Error("PKGBUILD no longer collects the release notes at all")
	}
}

// pfind is the search engine. The same bytes are embedded in the binary, so the
// packaged copy is not what makes the tool work - it exists so users get a CLI
// of their own and so the engine is inspectable as a plain file. Absent from
// both Arch recipes until 2026-08-20 while the .deb had shipped it since
// 2026-08-17.
func TestPKGBUILDShipsPfind(t *testing.T) {
	src := readPKGBUILD(t)
	if !strings.Contains(src, "internal/llm/tools/pfind.py") {
		t.Error("PKGBUILD does not install internal/llm/tools/pfind.py")
	}
	if !strings.Contains(src, `"${pkgdir}/usr/bin/pfind"`) {
		t.Error("PKGBUILD does not create the /usr/bin/pfind wrapper")
	}
	if _, err := os.Stat(filepath.Join("../../internal/llm/tools/pfind.py")); err != nil {
		t.Errorf("PKGBUILD references pfind.py but it is not in the tree: %v", err)
	}
}
