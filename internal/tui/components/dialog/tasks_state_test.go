package dialog

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/opencode-ai/opencode/internal/llm/agent"
)

// THE BUG, reported 2026-08-14 with two screenshots: the user selected 10
// helpers, /tasks showed 4, and the status bar said "4 helper(s)". Nothing was
// wrong with the run — 10 lanes were selected and 6 were queued behind a
// 4-slot semaphore — but a helper only entered the registry once it WON a slot,
// so the six were invisible AND unkillable.
//
// These tests hold the fix: every state is distinguishable, every row fits, and
// the header can never imply a number the run does not have.

var allStates = []agent.SubAgentState{
	agent.SubAgentQueued, agent.SubAgentRunning, agent.SubAgentDone,
	agent.SubAgentFailed, agent.SubAgentKilled,
}

// Every marker must be the SAME display width. A variable-width badge puts a
// row over the terminal width, and an over-wide line is the documented root
// cause of the footer marching down the screen (CLAUDE.md).
func TestEveryStateMarkerIsTheSameWidth(t *testing.T) {
	want := lipgloss.Width(allStates[0].Marker())
	if want == 0 {
		t.Fatal("markers measure as zero width; the row arithmetic cannot work")
	}
	for _, s := range allStates {
		if got := lipgloss.Width(s.Marker()); got != want {
			t.Errorf("%s marker is %d cells, %s is %d — rows will not align and a line can exceed the width",
				s.Label(), got, allStates[0].Label(), want)
		}
	}
}

// The constant the row arithmetic depends on must match what a marker really
// measures. If someone swaps in a wider glyph, this is where it must fail.
func TestRowPrefixConstantMatchesTheRealMarkerWidth(t *testing.T) {
	markerCells := lipgloss.Width(agent.SubAgentRunning.Marker())
	// 1 + id(4) + 1 + marker + 1 + label(7) + 1 + elapsed(6) + 2
	want := 1 + 4 + 1 + markerCells + 1 + 7 + 1 + 6 + 2
	if taskRowPrefixCells != want {
		t.Errorf("taskRowPrefixCells is %d but a real row prefix measures %d "+
			"(marker is %d cells) — the prompt will be truncated to the wrong length",
			taskRowPrefixCells, want, markerCells)
	}
}

// NO LINE IN THE FRAME MAY BE WIDER THAN THE TERMINAL. Emoji are two cells, and
// the pair is four; getting that wrong is precisely the class of bug that took
// three releases to find last time.
func TestTaskRowsNeverExceedTheWidth(t *testing.T) {
	for _, termWidth := range []int{80, 100, 120, 176} {
		m := NewTasksDialogCmp()
		m.Update(tea.WindowSizeMsg{Width: termWidth, Height: 40})

		// Build a row for every state with a prompt far too long to fit.
		long := strings.Repeat("REQUIREMENT — what does the target actually demand ", 8)
		for _, s := range allStates {
			line := " a10  " + s.Marker() + " " + s.Label() + "  2m5s  " +
				truncate(long, termWidth-tasksSidePadding-taskRowPrefixCells)
			if got := lipgloss.Width(line); got > termWidth {
				t.Errorf("term %d, state %s: row measures %d cells — over the terminal width, "+
					"which strands footer debris on every render",
					termWidth, s.Label(), got)
			}
		}
	}
}

// The state word must always ship with the glyph. On a machine with no emoji
// font every marker renders as an identical box, and this list is how the user
// decides what to kill.
func TestEveryStateHasADistinctWordNotJustAGlyph(t *testing.T) {
	seen := map[string]bool{}
	for _, s := range allStates {
		l := s.Label()
		if l == "" || l == "UNKNOWN" {
			t.Errorf("state %d has no usable label", int(s))
		}
		if seen[l] {
			t.Errorf("label %q is used by two states — they are indistinguishable without colour", l)
		}
		seen[l] = true
		if lipgloss.Width(l) > 7 {
			t.Errorf("label %q is %d cells, wider than the 7 the row reserves", l, lipgloss.Width(l))
		}
	}
}

// The header must itemise, so "4" can never again be read as "only 4 exist".
func TestHeaderCountsEachStateSeparately(t *testing.T) {
	tasks := []agent.SubAgentInfo{
		{ID: "a1", State: agent.SubAgentRunning},
		{ID: "a2", State: agent.SubAgentRunning},
		{ID: "a3", State: agent.SubAgentQueued},
		{ID: "a4", State: agent.SubAgentQueued},
		{ID: "a5", State: agent.SubAgentQueued},
		{ID: "a6", State: agent.SubAgentDone},
		{ID: "a7", State: agent.SubAgentFailed},
	}
	got := taskCountSummary(tasks)
	for _, want := range []string{"2 running", "3 queued", "1 done", "1 failed"} {
		if !strings.Contains(got, want) {
			t.Errorf("header %q does not report %q", got, want)
		}
	}
	// The old header could only ever say one number. If the summary collapses
	// back to that, a throttled run looks like a small one again.
	if !strings.Contains(got, ",") {
		t.Errorf("header %q reports a single figure; queued helpers are invisible again", got)
	}
	if got := taskCountSummary(nil); !strings.Contains(got, "none") {
		t.Errorf("empty list should say so plainly, got %q", got)
	}
}

// Only live helpers cost anything. A finished row lingers so the user can see
// it landed, but counting it as active would overstate what is running.
func TestOnlyQueuedAndRunningCountAsLive(t *testing.T) {
	live := map[agent.SubAgentState]bool{
		agent.SubAgentQueued: true, agent.SubAgentRunning: true,
		agent.SubAgentDone: false, agent.SubAgentFailed: false, agent.SubAgentKilled: false,
	}
	for s, want := range live {
		if s.Live() != want {
			t.Errorf("%s.Live() = %v, want %v", s.Label(), s.Live(), want)
		}
	}
}
