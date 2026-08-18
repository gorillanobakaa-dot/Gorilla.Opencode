package dialog

// GORILLA OVERRIDE (2026-08-17): the invariant CLAUDE.md states in capitals —
// NO LINE IN THE FRAME MAY BE WIDER THAN THE TERMINAL — asserted for the
// dialogs that were not covered by TestNewDialogsFitNarrowTerminals.
//
// It was not covered, and all three were broken: /context floored its width UP
// to 100 columns (106 with chrome) whatever the terminal, and the two /osint
// screens floored to 80 and 70. On an 80-column terminal every one of them drew
// outside the window.
//
// Why this is a correctness bug and not a cosmetic one: bubbletea's inline
// renderer erases the previous frame by moving the cursor up by the number of
// LOGICAL lines it drew. A line wider than the terminal occupies two PHYSICAL
// rows but counts as one, so the erase under-reaches by one row per wrapped
// line, every render — and the un-erased rows are stranded in the scrollback.
// That is the same failure class an Arch tester reported on v0.1.87.

import (
	"fmt"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/opencode-ai/opencode/internal/config"
	"github.com/opencode-ai/opencode/internal/llm/agent"
	"github.com/opencode-ai/opencode/internal/session"
)

// frameFitSizes spans a cramped SSH window, the common 80x24 default, a laptop
// and the project's 1600x900 reference screen.
//
// WHY THIS DOES NOT REUSE THE screentest HARNESS: screentest.Render draws into a
// cell grid that CLIPS over-wide lines exactly as a real terminal does (proved by
// its own TestGridClipsLikeATerminal). That is right for what it measures, but it
// destroys the evidence for this bug before any assertion can see it —
// TestNoDialogOverflowsTheWidth asks whether a rendered row exceeds the terminal
// width, which after clipping is impossible by construction. The over-wide frame
// has to be caught in the STRING, before it reaches a grid.
var frameFitSizes = [][2]int{{60, 20}, {80, 24}, {100, 30}, {130, 42}}

func TestDialogFramesNeverExceedTheTerminal(t *testing.T) {
	if _, err := config.Load(t.TempDir(), false); err != nil {
		t.Fatalf("config.Load: %v", err)
	}

	for _, sz := range frameFitSizes {
		w, h := sz[0], sz[1]
		views := map[string]string{}

		lo := NewLoadoutDialogCmp().(*loadoutDialogCmp)
		lo.Update(tea.WindowSizeMsg{Width: w, Height: h})
		views["/context"] = lo.View()

		gate := NewOsintDialogCmp("a question")
		gate.SetSize(w, h)
		views["/osint gate"] = gate.View()

		page := NewOsintPageCmp()
		page.SetSize(w, h)
		views["/osint page"] = page.View()

		// The recovery picker, both states. Empty is not a cosmetic case: it is
		// what someone sees the first time they reach for this, and a stranded
		// row there lands on a user who has already lost a run once.
		empty := NewOsintRecoverCmp(nil)
		empty.SetSize(w, h)
		views["/osint --recover (empty)"] = empty.View()

		full := NewOsintRecoverCmp(recoverFixture())
		full.SetSize(w, h)
		views["/osint --recover"] = full.View()

		// The sessions manager, in the three states that differ in height: a
		// full list, an empty search, and a pending erase (which swaps in a red
		// confirmation line).
		mgr := NewSessionsCmp()
		mgr.SetStore(sessionsFixture(), "s3")
		mgr.SetSize(w, h)
		views["/sessions"] = mgr.View()

		empty2 := NewSessionsCmp()
		empty2.SetStore(SessionsStore{}, "")
		empty2.SetSize(w, h)
		views["/sessions (empty)"] = empty2.View()

		confirm := NewSessionsCmp()
		confirm.SetStore(sessionsFixture(), "")
		confirm.SetSize(w, h)
		confirm.Update(tea.KeyMsg{Type: tea.KeyCtrlD})
		views["/sessions (confirming)"] = confirm.View()

		for name, v := range views {
			if got := lipgloss.Width(v); got > w {
				t.Errorf("%s at %dx%d: frame is %d columns wide — wider than the terminal, which strands rows in scrollback", name, w, h, got)
			}
			// Belt and braces: assert every individual line too, because
			// lipgloss.Width reports the maximum and a single stray line is
			// enough to break the erase.
			for i, line := range strings.Split(v, "\n") {
				if got := lipgloss.Width(line); got > w {
					t.Errorf("%s at %dx%d: line %d is %d columns", name, w, h, i+1, got)
					break
				}
			}
			// HEIGHT is asserted only from the standard terminal upwards.
			// Terminals shorter than that are owned by knownSmallOverflow in
			// screen_invariants_test.go — a ratchet that records what each
			// dialog still needs and fails in BOTH directions, which is a
			// better instrument for a slow squeeze than a flat assertion here.
			// Width has no such ratchet and needs none: drawing outside the
			// window is never acceptable at any size.
			if h >= standardHeight {
				if got := lipgloss.Height(v); got > h {
					t.Errorf("%s at %dx%d: frame is %d rows tall — a frame taller than the window scrolls the terminal", name, w, h, got)
				}
			}
		}
	}
}

// The width helper must never return more than the terminal can hold, and must
// still answer sensibly before the first WindowSizeMsg arrives.
func TestDialogWidthNeverExceedsTerminal(t *testing.T) {
	const chrome = 6
	for _, term := range []int{10, 26, 40, 80, 100, 200, 400} {
		got := dialogWidth(term, 104, chrome)
		if term >= 26 && got+chrome > term {
			t.Errorf("dialogWidth(%d) = %d, which needs %d columns", term, got, got+chrome)
		}
		if got > 104 {
			t.Errorf("dialogWidth(%d) = %d, above the preferred cap", term, got)
		}
	}
	if got := dialogWidth(0, 104, chrome); got != 104 {
		t.Errorf("unknown terminal size should fall back to the preferred width, got %d", got)
	}
}

// recoverFixture is more runs than fit on screen, with the longest real
// question this project has actually asked — the 2026-08-17 identity run, whose
// prompt ran to several hundred characters.
func recoverFixture() []agent.RecoverableRun {
	long := "Establish or refute whether the local \"friend\" kelexine who designed the private " +
		"Rust tool findx (verified local note dated 2026-07-22 on the user's machine) is the " +
		"same person as the public developer Franklin Kelechi who holds the @kelexine GitHub, " +
		"HuggingFace, SourceForge, GitLab, dev.to, Mastodon and XDA accounts."
	runs := make([]agent.RecoverableRun, 0, 12)
	for i := 0; i < 12; i++ {
		runs = append(runs, agent.RecoverableRun{
			CallID:   "call_0123456789abcdef",
			Question: long,
			When:     time.Date(2026, 8, 17, 21, 30, 0, 0, time.UTC),
			Lanes:    17,
			Tokens:   507935,
		})
	}
	return runs
}

// sessionsFixture is more conversations than fit, with a title long enough to
// wrap if anything forgets to truncate, and sizes spanning the formatter's
// bytes/KB/MB branches.
func sessionsFixture() SessionsStore {
	long := "Establish or refute whether the local friend kelexine who designed the private Rust tool findx is the same person as the public developer Franklin Kelechi"
	// A REAL title from the developer's store, containing U+FFFC (object
	// replacement character). It is one rune and two display columns, and it
	// wrapped the row on the first live run because fitLine counted runes.
	// Emoji and CJK behave the same way and are far more likely in the hands of
	// the people this is built for.
	wide := "\ufffcDSML\ufffcparameter name=\"agents\" 中文 😀 " + long
	rows := make([]SessionRow, 0, 20)
	for i := 0; i < 20; i++ {
		title := long
		if i%3 == 0 {
			title = wide
		}
		rows = append(rows, SessionRow{
			Session: session.Session{
				ID:        fmt.Sprintf("s%d", i),
				Title:     title,
				CreatedAt: time.Date(2026, 8, 17, 21, 30, 0, 0, time.UTC).Unix() - int64(i*3600),
			},
			Bytes: int64(i) * 500_000,
			Msgs:  int64(i) * 7,
		})
	}
	return SessionsStore{
		Rows: rows, TotalBytes: 4_774_160, FileBytes: 9_826_688,
		Sessions: 20, Helpers: 45, Msg: 578,
	}
}
