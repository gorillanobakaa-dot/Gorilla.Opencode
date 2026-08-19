package version

// GORILLA OVERRIDE (2026-08-19): a release page must EMBED its screenshots, not
// link to them.
//
// This is directive 13, and it has now had to be said more than once — which
// means writing it down did not work and it needs to be mechanical.
//
// v0.1.103 shipped a release page carrying the words "Everything you need is on
// this page, printed in full" directly above a link that sent the reader
// somewhere else for the screenshots. Both halves of that are the failure:
// CLAUDE.md's release checklist says the page carries the full documentation
// INLINE because "a release page that says 'see the docs' is the closed door
// PHILOSOPHY.md argues against", and directive 13 says screenshots are part of
// the deliverable rather than decoration.
//
// Two details this test also pins, because both were got wrong at least once:
//   - Relative image paths DO NOT RENDER in a GitHub release body. They must be
//     absolute raw.githubusercontent.com URLs, pinned to the tag so the image
//     never changes under a published page.
//   - Every image must be wrapped in a link to itself, so a reader can reach
//     the full-resolution original from a page that displays it scaled down.

import (
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

// atLeast compares dotted versions NUMERICALLY.
//
// Written after the first version of this test compared them as STRINGS, which
// puts v0.1.83 after v0.1.103 because "8" > "1" — so the guard swept up thirty
// historical releases it was never meant to judge. A lexical compare on a
// dotted version is always wrong and always looks right on the examples you
// happen to try.
func atLeast(v, floor string) bool {
	parse := func(s string) []int {
		s = strings.TrimPrefix(s, "v")
		parts := strings.Split(s, ".")
		out := make([]int, 0, len(parts))
		for _, p := range parts {
			n, _ := strconv.Atoi(p)
			out = append(out, n)
		}
		return out
	}
	a, b := parse(v), parse(floor)
	for i := 0; i < len(a) && i < len(b); i++ {
		if a[i] != b[i] {
			return a[i] > b[i]
		}
	}
	return len(a) >= len(b)
}

var (
	imageRe     = regexp.MustCompile(`!\[([^\]]*)\]\(([^)]+)\)`)
	linkedImgRe = regexp.MustCompile(`\[!\[([^\]]*)\]\(([^)]+)\)\]\(([^)]+)\)`)
)

// releasesExemptFromScreenshots are the ones with nothing to photograph, each
// with its reason. An exemption without a reason is how a rule becomes
// decoration.
var releasesExemptFromScreenshots = map[string]string{}

func TestEveryReleasePageEmbedsItsScreenshots(t *testing.T) {
	dir := "../../ReleaseNotes"
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Skipf("no ReleaseNotes directory: %v", err)
	}

	// Only guard releases from the point the rule was made mechanical.
	// Rewriting history is not the job; stopping the next one is.
	const guardFrom = "v0.1.103"

	checked := 0
	for _, e := range entries {
		name := e.Name()
		if !strings.HasPrefix(name, "GITHUB-RELEASE-NOTES-") || !strings.HasSuffix(name, ".md") {
			continue
		}
		ver := strings.TrimSuffix(strings.TrimPrefix(name, "GITHUB-RELEASE-NOTES-"), ".md")
		if !atLeast(ver, guardFrom) {
			continue
		}
		if why, ok := releasesExemptFromScreenshots[ver]; ok {
			t.Logf("%s exempt: %s", ver, why)
			continue
		}
		checked++

		body, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			t.Errorf("%s: %v", name, err)
			continue
		}
		text := string(body)

		imgs := imageRe.FindAllStringSubmatch(text, -1)
		if len(imgs) == 0 {
			t.Errorf("%s embeds no screenshots.\n"+
				"  A release page is where somebody decides whether to download this. Directive 13: "+
				"screenshots are part of the deliverable, not decoration, and a release without them "+
				"is incomplete work. Linking to docs/SCREENSHOTS.md is the 'see the docs' closed door.",
				name)
			continue
		}

		for _, m := range imgs {
			alt, src := m[1], m[2]
			if !strings.HasPrefix(src, "https://raw.githubusercontent.com/") {
				t.Errorf("%s: image %q uses %q.\n"+
					"  Relative and blob paths DO NOT RENDER in a GitHub release body. Use an absolute "+
					"raw.githubusercontent.com URL pinned to the tag.", name, alt, src)
			}
			if strings.Contains(src, "/main/") {
				t.Errorf("%s: image %q points at /main/. Pin it to the tag, or the picture on a "+
					"published page changes when the file does.", name, alt)
			}
			if len(strings.Fields(alt)) < 6 {
				t.Errorf("%s: alt text %q is %d words. It must describe what the image PROVES, "+
					"not what it is called.", name, alt, len(strings.Fields(alt)))
			}
			if strings.Contains(text, "width=") {
				t.Errorf("%s: a width attribute is present. Never thumbnail — full size, and let "+
					"the host render it wide.", name)
			}
		}

		// Every image must be clickable through to its full-resolution original.
		linked := linkedImgRe.FindAllStringSubmatch(text, -1)
		if len(linked) != len(imgs) {
			t.Errorf("%s: %d image(s) but only %d wrapped in a link to the original.\n"+
				"  Required form: [![alt](path)](path)", name, len(imgs), len(linked))
		}
		for _, m := range linked {
			if m[2] != m[3] {
				t.Errorf("%s: image %q links to %q rather than to itself", name, m[1], m[3])
			}
		}
	}

	if checked == 0 {
		t.Fatalf("guarded no release pages at or after %s — the guard is not looking at anything", guardFrom)
	}
	t.Logf("checked %d release page(s)", checked)
}

// The page must not claim to be complete while sending the reader elsewhere.
func TestAReleasePageDoesNotClaimCompletenessAndThenLinkAway(t *testing.T) {
	dir := "../../ReleaseNotes"
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Skip("no ReleaseNotes directory")
	}
	for _, e := range entries {
		name := e.Name()
		if !strings.HasPrefix(name, "GITHUB-RELEASE-NOTES-v0.1.103") {
			continue
		}
		body, _ := os.ReadFile(filepath.Join(dir, name))
		text := string(body)
		if !strings.Contains(text, "printed in full") {
			continue
		}
		if strings.Contains(text, "Screenshots of everything in this release:") {
			t.Errorf("%s says it is printed in full and then links away for the screenshots", name)
		}
	}
}

// The comparison itself, because getting it wrong is what made the first run of
// the guard sweep up thirty releases it was not meant to judge.
func TestVersionComparisonIsNumericNotLexical(t *testing.T) {
	if !atLeast("v0.1.103", "v0.1.103") {
		t.Error("a version is not at least itself")
	}
	if !atLeast("v0.1.104", "v0.1.103") {
		t.Error("v0.1.104 did not compare as later than v0.1.103")
	}
	if atLeast("v0.1.83", "v0.1.103") {
		t.Error("v0.1.83 compared as later than v0.1.103 — this is the lexical bug")
	}
	if atLeast("v0.1.99", "v0.1.103") {
		t.Error("v0.1.99 compared as later than v0.1.103")
	}
}
