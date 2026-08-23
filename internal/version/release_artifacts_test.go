package version

// GORILLA OVERRIDE (2026-08-23): a release must ship EVERY format, not the one
// the releaser happened to remember.
//
// The Arch package (.pkg.tar.zst) shipped on every release from well before
// v0.1.96 through v0.1.111, and then stopped. v0.1.112, v0.1.113, v0.1.114 and
// v0.1.115 all went out without it. FOUR CONSECUTIVE RELEASES, and nobody
// noticed until the owner asked where it had gone.
//
// It went unnoticed for the same reason the stale pkgver did: a release missing
// one format is still a completely valid release. The page loads, the .deb
// installs, the checksums verify, `gh release view` looks healthy. Nothing about
// it reads as broken unless you already know what should be there. The evidence
// was sitting in Compiled.Builds/ the whole time, which jumps straight from
// 0.1.111 to 0.1.115: nobody even BUILT one for the three in between.
//
// It matters because packaging/PKGBUILD tells Arch users, in writing, to
// "Download gorilla-opencode-${pkgver}-1-x86_64.pkg.tar.zst from the GitHub
// release page". For four releases that instruction pointed at a file that was
// not there. The project's own documentation was the thing that was wrong.
//
// WHY THIS TEST IS SHAPED THE WAY IT IS. A test cannot ask GitHub what a release
// contains without a network and a token, and a test that needs those is a test
// that gets skipped. So it checks the step BEFORE upload, where the mistake is
// actually made, using a rule that needs neither:
//
//	If you built ANY artifact for a version, you must have built ALL of them.
//
// A fresh clone has no build outputs and skips cleanly. A machine mid-release
// has some, and that is exactly when a missing format is a bug rather than an
// absence. That rule catches the real failure ("built the .deb, forgot the Arch
// package") without inventing a reason to fail on somebody's laptop.
//
// The published side is covered too, but only when explicitly asked for, by
// TestPublishedReleaseCarriesEveryArtifact below.

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// releaseArtifact is one format a release must carry.
//
// The name templates are the CONVENTION AS PUBLISHED, taken from the assets on
// v0.1.110 and v0.1.114 rather than invented here. They differ from each other
// on purpose (dpkg wants underscores and a bare version, pacman wants hyphens
// and a pkgrel, the raw binary carries a leading v) and getting one wrong
// produces a file nobody's download script finds.
var releaseArtifacts = []struct {
	label    string
	template string // %s is the bare version, e.g. 0.1.115
	built    string // how to produce it, printed in the failure
}{
	{
		label:    "Debian package",
		template: "gorilla-opencode_%s_amd64.deb",
		built:    "scripts/build-deb.sh %s",
	},
	{
		label:    "Arch/CachyOS package",
		template: "gorilla-opencode-%s-1-x86_64.pkg.tar.zst",
		built:    "scripts/build-arch.sh %s",
	},
	{
		label:    "stripped linux binary",
		template: "gorilla-opencode-v%s-linux-amd64",
		built:    `go build -ldflags "-s -w -X github.com/opencode-ai/opencode/internal/version.Version=v%s" -o gorilla-opencode . && cp gorilla-opencode Compiled.Builds/gorilla-opencode-v%s-linux-amd64`,
	},
	{
		label:    "checksums",
		template: "SHA256SUMS-v%s.txt",
		built:    "sha256sum the three above > Compiled.Builds/SHA256SUMS-v%s.txt",
	},
}

const compiledBuilds = "../../Compiled.Builds"

// TestAReleaseIsAllFormatsOrNone fails when a version has SOME of its artifacts
// built but not all: the exact state a half-finished release leaves behind.
func TestAReleaseIsAllFormatsOrNone(t *testing.T) {
	ver := latestReleaseNotesVersion(t)
	bare := strings.TrimPrefix(ver, "v")

	if _, err := os.Stat(compiledBuilds); err != nil {
		t.Skipf("no Compiled.Builds (a fresh clone; nothing has been built here): %v", err)
	}

	var present, missing []string
	for _, a := range releaseArtifacts {
		name := fmt.Sprintf(a.template, bare)
		if _, err := os.Stat(filepath.Join(compiledBuilds, name)); err == nil {
			present = append(present, a.label)
			continue
		}
		how := strings.ReplaceAll(a.built, "%s", bare)
		missing = append(missing, fmt.Sprintf("  %-24s %s\n      build it: %s", a.label, name, how))
	}

	// Nothing built for this version: not a release machine. Say nothing.
	if len(present) == 0 {
		t.Skipf("nothing built for %s here; not mid-release", ver)
	}
	if len(missing) == 0 {
		t.Logf("%s: all %d artifacts present", ver, len(releaseArtifacts))
		return
	}

	t.Errorf("%s is a PARTIAL release: %d of %d artifacts built.\n\n"+
		"Built: %s\n\nMissing:\n%s\n"+
		"  A release missing one format is still a valid-looking release: the page loads,\n"+
		"  the other packages install, the checksums verify. That is why the Arch package\n"+
		"  vanished for FOUR consecutive releases (v0.1.112 through v0.1.115) before anyone\n"+
		"  noticed. packaging/PKGBUILD tells Arch users in writing to download the\n"+
		"  .pkg.tar.zst from the release page, so a missing one makes the project's own\n"+
		"  documentation wrong.",
		ver, len(present), len(releaseArtifacts),
		strings.Join(present, ", "), strings.Join(missing, "\n"))
}

// TestPublishedReleaseCarriesEveryArtifact checks the real release page. It
// needs the network and a gh token, so it runs ONLY when asked:
//
//	CHECK_PUBLISHED_RELEASE=1 go test ./internal/version/ -run TestPublishedRelease
//
// Opt-in rather than automatic on purpose: a test that fails on a train is a
// test people learn to ignore, and an ignored guard is worse than none.
func TestPublishedReleaseCarriesEveryArtifact(t *testing.T) {
	if os.Getenv("CHECK_PUBLISHED_RELEASE") != "1" {
		t.Skip("set CHECK_PUBLISHED_RELEASE=1 to check the published release page")
	}
	ver := latestReleaseNotesVersion(t)
	bare := strings.TrimPrefix(ver, "v")

	if _, err := exec.LookPath("gh"); err != nil {
		t.Skipf("gh not installed: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	// The TAG carries a leading v; latestReleaseNotesVersion returns it bare.
	// Passing the bare form makes gh exit 1, which this function then read as
	// "not published yet" and skipped: a guard that silently passes on its own
	// bug. It did exactly that on first run, 2026-08-23, against a release that
	// was live and complete.
	tag := "v" + bare
	cmd := exec.CommandContext(ctx, "gh", "release", "view", tag, "--json", "assets")
	cmd.Dir = "../.."
	out, err := cmd.Output()
	if err != nil {
		t.Skipf("could not read release %s (offline, or not published yet): %v", tag, err)
	}

	var payload struct {
		Assets []struct {
			Name string `json:"name"`
		} `json:"assets"`
	}
	if err := json.Unmarshal(out, &payload); err != nil {
		t.Fatalf("unreadable gh output: %v", err)
	}
	have := map[string]bool{}
	for _, a := range payload.Assets {
		have[a.Name] = true
	}

	for _, a := range releaseArtifacts {
		name := fmt.Sprintf(a.template, bare)
		if !have[name] {
			t.Errorf("release %s is missing %s (%s).\n"+
				"  Upload it: gh release upload %s Compiled.Builds/%s",
				tag, a.label, name, tag, name)
		}
	}
}

// The templates must stay distinct. Two artifacts resolving to one filename
// would make this whole guard pass while a format was missing, which is the
// failure it exists to catch wearing a different hat.
func TestArtifactNameTemplatesAreDistinct(t *testing.T) {
	seen := map[string]string{}
	for _, a := range releaseArtifacts {
		name := fmt.Sprintf(a.template, "0.1.115")
		if prev, dup := seen[name]; dup {
			t.Errorf("%q and %q both resolve to %s", prev, a.label, name)
		}
		seen[name] = a.label
	}
	if len(seen) != len(releaseArtifacts) {
		t.Errorf("%d templates produced %d names", len(releaseArtifacts), len(seen))
	}
}
