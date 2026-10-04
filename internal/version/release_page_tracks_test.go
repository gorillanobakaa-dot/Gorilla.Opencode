package version

// GORILLA OVERRIDE (2026-10-04): the plain-language track goes ON the release
// page. Not behind a link.
//
// v0.1.135, v0.1.136 and v0.1.137 were first published with a complete,
// validated layman document attached to the release and hidden from the page:
//
//	## Full notes
//	- `v0.1.137-release-notes.layman.md` — plain English
//
// The link was relative, so on a release page it did not even open. The reader
// the document was written for — someone who has never opened a terminal,
// deciding on this page whether to download anything — was the one reader who
// would never see it. PHILOSOPHY.md has a name for that: "transparent in theory
// ... a closed door in practice", and a rule: "No one should have to trust a
// summary they cannot verify."
//
// The writer had the philosophy, the guide and the generator, and did it anyway
// by copying the shape of an earlier page (0.1.130 to 0.1.134 have the same
// section). A rule that depends on being remembered is not a rule, so this is
// the rule. `fieldkit release-page compose` builds a page that passes it;
// scripts/release/build_release_page.py is the procedure.
//
// Guarded from v0.1.135. The earlier pages are left as they are: rewriting
// history is not the job, stopping the next one is.

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

const tracksGuardFrom = "v0.1.135"

var (
	wsRe          = regexp.MustCompile(`\s+`)
	headingRe     = regexp.MustCompile(`^#{1,5} `)
	pageLinkRe    = regexp.MustCompile(`(^|[^!])\[([^\]]*)\]\(([^)\s]+)\)`)
	fullNotesRe   = regexp.MustCompile(`(?im)^#{1,3} +full notes\b`)
	whatItIsRe    = regexp.MustCompile(`(?im)^#{2,3} .*\bwhat (?:is this|this is|it is)\b`)
	shouldYouRe   = regexp.MustCompile(`(?im)^#{2,3} .*\bshould you\b`)
	whyMattersRe  = regexp.MustCompile(`(?im)^#{2,3} .*\bwhy (?:this|it) matters\b`)
	absoluteLink  = regexp.MustCompile(`^(?:https?://|#|mailto:)`)
	notesFileLink = regexp.MustCompile(`release-notes\.(?:layman|developer)\.md`)
)

// trackBody is a rendered track as it must appear on the page: without its own
// title block, and with every heading one level down so it sits under the
// page's section for it. Whitespace is collapsed so wrapping does not matter.
func trackBody(md string) string {
	lines := strings.Split(strings.ReplaceAll(md, "\r\n", "\n"), "\n")
	start := 0
	if len(lines) > 0 && strings.HasPrefix(lines[0], "# ") {
		start = 1
		for start < len(lines) && !strings.HasPrefix(lines[start], "## ") {
			start++
		}
	}
	fenced := false
	var out []string
	for _, l := range lines[start:] {
		if strings.HasPrefix(strings.TrimLeft(l, " \t"), "```") {
			fenced = !fenced
		}
		if !fenced && headingRe.MatchString(l) {
			l = "#" + l
		}
		out = append(out, l)
	}
	return strings.TrimSpace(wsRe.ReplaceAllString(strings.Join(out, "\n"), " "))
}

func TestEveryReleasePageCarriesBothTracksInFull(t *testing.T) {
	dir := "../../ReleaseNotes"
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Skipf("no ReleaseNotes directory: %v", err)
	}
	checked := 0
	for _, e := range entries {
		name := e.Name()
		if !strings.HasPrefix(name, "GITHUB-RELEASE-NOTES-") || !strings.HasSuffix(name, ".md") {
			continue
		}
		ver := strings.TrimSuffix(strings.TrimPrefix(name, "GITHUB-RELEASE-NOTES-"), ".md")
		if !atLeast(ver, tracksGuardFrom) {
			continue
		}
		checked++
		raw, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			t.Errorf("%s: %v", name, err)
			continue
		}
		page := strings.ReplaceAll(string(raw), "\r\n", "\n")
		flat := wsRe.ReplaceAllString(page, " ")

		for _, track := range []string{"layman", "developer"} {
			path := filepath.Join("../../Changelogs", ver+"-release-notes."+track+".md")
			md, err := os.ReadFile(path)
			if err != nil {
				t.Errorf("%s: the %s track %s does not exist. Every release has both.", name, track, path)
				continue
			}
			if !strings.Contains(flat, trackBody(string(md))) {
				t.Errorf("%s does not carry the %s track in full.\n"+
					"  The text of %s was not found on the page. Attaching the file to the release, or\n"+
					"  linking to it, is the closed door: the reader this was written for will not open it.\n"+
					"  Build the page with: python scripts/release/build_release_page.py . %s",
					name, track, path, ver)
			}
		}

		if fullNotesRe.MatchString(page) {
			t.Errorf("%s has a \"Full notes\" section. The notes go on the page, not behind it.", name)
		}

		// The opening: what this is, whether to bother, why it matters — before
		// a changelog, because a reader who does not know what the program is
		// cannot use anything that follows.
		for label, re := range map[string]*regexp.Regexp{
			"what the program is":        whatItIsRe,
			"whether to download it":     shouldYouRe,
			"why it matters to a person": whyMattersRe,
		} {
			loc := re.FindStringIndex(page)
			if loc == nil {
				t.Errorf("%s never tells the reader %s (no such heading).", name, label)
				continue
			}
			if line := strings.Count(page[:loc[0]], "\n") + 1; label == "what the program is" && line > 40 {
				t.Errorf("%s does not say %s until line %d; it must open the page.", name, label, line)
			}
		}

		for _, m := range pageLinkRe.FindAllStringSubmatch(page, -1) {
			text, target := m[2], m[3]
			switch {
			case notesFileLink.MatchString(target) || notesFileLink.MatchString(text):
				t.Errorf("%s: link [%s](%s) sends the reader to the notes file instead of putting the notes on the page.", name, text, target)
			case !absoluteLink.MatchString(target):
				t.Errorf("%s: link [%s](%s) is relative. It does not open from a GitHub release page.", name, text, target)
			}
		}

		li := strings.Index(page, "# In plain language")
		di := strings.Index(page, "# For developers")
		if li < 0 || di < 0 || di < li {
			t.Errorf("%s: the plain-language part must be on the page and must come before the developer part.", name)
		}
	}
	if checked == 0 {
		t.Skip("no release page at or after " + tracksGuardFrom)
	}
	t.Logf("checked %d release page(s)", checked)
}

// The guard must be able to fail. This is the page that was published, in
// miniature: a summary, then a link to the document.
func TestTheTracksGuardRejectsThePageThatWasPublished(t *testing.T) {
	layman := "# Title\n\n**Date:** x\n\n---\n\n## Why This Release Exists\n\nA full explanation with an analogy.\n"
	published := "# Release\n\n## Why this release exists\n\nA summary.\n\n## Full notes\n\n- [`notes.layman.md`](Changelogs/v0.1.137-release-notes.layman.md) — plain English\n"
	if strings.Contains(wsRe.ReplaceAllString(published, " "), trackBody(layman)) {
		t.Error("a page that only links to the layman track was accepted as carrying it")
	}
	if !fullNotesRe.MatchString(published) {
		t.Error("the Full notes section was not recognised")
	}
	m := pageLinkRe.FindStringSubmatch(published)
	if m == nil || absoluteLink.MatchString(m[3]) || !notesFileLink.MatchString(m[3]) {
		t.Error("the relative link to the notes file was not recognised")
	}
	if got := trackBody(layman); got != "### Why This Release Exists A full explanation with an analogy." {
		t.Errorf("trackBody = %q", got)
	}
}
