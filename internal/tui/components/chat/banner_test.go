package chat

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/opencode-ai/opencode/internal/config"
	"github.com/opencode-ai/opencode/internal/tui/screentest"
	"github.com/opencode-ai/opencode/internal/version"
)

// The program's name, version and folder used to live in the sidebar. With the
// sidebar gone they had nowhere to be, and a user searching the frame for them —
// even at a smaller font — finds nothing. The banner is where they went.
func TestSessionBannerNamesTheProgramAndItsVersion(t *testing.T) {
	if _, err := config.Load(t.TempDir(), false); err != nil {
		t.Fatalf("Load: %v", err)
	}
	got := SessionBanner(100)
	if strings.TrimSpace(got) == "" {
		t.Fatal("no banner at all; the version is then unreachable from the interface")
	}
	for _, want := range []string{"Gorilla OpenCode", version.Version} {
		if !strings.Contains(got, want) {
			t.Errorf("banner does not mention %q:\n%s", want, got)
		}
	}
	// And it must say the thing an upgrading user cannot otherwise discover.
	if !strings.Contains(got, "copy it") {
		t.Errorf("banner does not mention that the conversation is copyable:\n%s", got)
	}
}

// Printed output cannot be withdrawn, so nothing prints at an unknown width.
func TestSessionBannerRefusesUnknownWidth(t *testing.T) {
	for _, w := range []int{0, -20} {
		if got := SessionBanner(w); got != "" {
			t.Errorf("width %d produced %q", w, got)
		}
	}
}

// Every line must fit, or the terminal wraps it into rows the renderer never
// counted and every later erase lands wrong.
func TestSessionBannerFitsItsWidth(t *testing.T) {
	if _, err := config.Load(t.TempDir(), false); err != nil {
		t.Fatalf("Load: %v", err)
	}
	for _, w := range []int{30, 60, 100} {
		for i, line := range strings.Split(SessionBanner(w), "\n") {
			if got := lipgloss.Width(line); got > w {
				t.Errorf("width %d: line %d is %d columns wide", w, i, got)
			}
		}
	}
}

// The footer's right-hand side was empty; the name and version now sit there. It
// must be pinned to the right edge and must not collide with the left-hand fields.
func TestFooterCarriesTheVersionOnTheRight(t *testing.T) {
	if _, err := config.Load(t.TempDir(), false); err != nil {
		t.Fatalf("Load: %v", err)
	}
	// GORILLA OVERRIDE (2026-09-01): pin the working directory to something
	// short, so this measures the LAYOUT rather than the length of a temp path.
	//
	// joinWithTrailer drops the trailer rather than truncating it when the
	// left-hand fields would collide — deliberately, and documented there: half
	// a version string is worse than none. t.TempDir() on Windows returns
	// eighty-odd columns under AppData\Local\Temp, which on its own pushed the
	// footer past 140 and took the version with it. The test was measuring the
	// temp directory, not the footer.
	config.Get().WorkingDir = filepath.Join("short", "wd")
	cmp := infoFor(sessionForBanner(), nil)

	const wide = 140
	view := cmp.CompactView(wide)
	first := strings.Split(view, "\n")[0]
	if !strings.Contains(first, version.Version) {
		t.Errorf("the footer's first line does not carry the version:\n%q", first)
	}
	if w := lipgloss.Width(first); w != wide {
		t.Errorf("first line is %d columns, want exactly %d", w, wide)
	}
	// Right-pinned means it STARTS near the right edge. Asserting only that it ends
	// there passes trivially, because the line is padded to full width afterwards —
	// a version sitting immediately after the fields would still "end" at the edge.
	plain := screentest.Render(view, wide, lipgloss.Height(view)).Text(0)
	trailer := "Gorilla OpenCode " + version.Version
	at := strings.Index(plain, "Gorilla OpenCode")
	if at < 0 {
		t.Fatalf("the trailer is missing entirely:\n%q", plain)
	}
	if want := wide - len(trailer) - 1; at < want {
		t.Errorf("the trailer starts at column %d but the right edge would put it at "+
			"%d; it is sitting next to the fields rather than pinned right:\n%q",
			at, want, plain)
	}

	// On a narrow footer the trailer is dropped rather than colliding or truncated.
	narrow := cmp.CompactView(40)
	if strings.Contains(strings.Split(narrow, "\n")[0], "Gorilla OpenCode") {
		t.Error("a 40-column footer still tries to show the name; half a version " +
			"string is worse than none")
	}
	for i, line := range strings.Split(narrow, "\n") {
		if got := lipgloss.Width(line); got != 40 {
			t.Errorf("narrow line %d is %d columns, want 40", i, got)
		}
	}
}
