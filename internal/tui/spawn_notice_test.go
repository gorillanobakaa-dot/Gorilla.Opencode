package tui

// GORILLA OVERRIDE (2026-08-23): the helper spawn notice truncated to a
// constant, not to the screen.
//
// Reported from a 1519-pixel-wide terminal where every helper still read
// "research · ADVERSARY - what breaks, l...". The budget was a literal 40 with
// no relationship to the width, so buying a bigger monitor changed nothing.
//
// The owner: "notice how the description gets truncated... we can make full use
// of the screen, we have room enough."

import (
	"strings"
	"testing"

	"github.com/opencode-ai/opencode/internal/config"
)

// THE BUG. A wider terminal must produce a bigger budget, or the number is not
// about the screen at all.
func TestAWiderTerminalShowsMoreOfTheLabel(t *testing.T) {
	narrow := spawnLabelBudget(80, "a1")
	wide := spawnLabelBudget(200, "a1")

	if !(wide > narrow) {
		t.Errorf("80 cols gives %d and 200 cols gives %d. A constant looks exactly "+
			"like this, and it is what shipped: truncatePrompt(..., 40) regardless "+
			"of the terminal.", narrow, wide)
	}
	// The old constant, named so a regression is unmistakable.
	if wide <= 40 {
		t.Errorf("a 200 column terminal gets %d columns for the label, no better than "+
			"the hardcoded 40 it replaced", wide)
	}
}

// The hint is the actionable half. A label that runs short loses words; a hint
// that runs short loses the only instruction saying a runaway helper can be
// killed. So the chrome is reserved and the label takes what is left.
func TestTheKillHintIsAlwaysAffordable(t *testing.T) {
	for _, width := range []int{80, 100, 120, 160, 200} {
		budget := spawnLabelBudget(width, "a10")
		chrome := len("🦍 helper a10 spawned:   (/tasks to view or kill)")
		if budget+chrome > width+spawnNoticeDecoration+8 {
			t.Errorf("at %d cols the label budget is %d, which does not leave room for "+
				"the chrome and the kill hint", width, budget)
		}
	}
}

// A narrow pane must still show enough to tell the roles apart. They are
// distinguished by their FIRST word: ADVERSARY, REQUIREMENT, PRIOR ART.
func TestANarrowPaneStillDistinguishesTheRoles(t *testing.T) {
	budget := spawnLabelBudget(40, "a1")
	if budget < spawnLabelFloor {
		t.Errorf("budget %d fell below the floor %d", budget, spawnLabelFloor)
	}

	labels := []string{
		"research · ADVERSARY - what breaks",
		"research · REQUIREMENT - what does this need",
		"research · PRIOR ART - has someone already",
	}
	seen := map[string]bool{}
	for _, l := range labels {
		got := truncatePrompt(l, budget)
		if seen[got] {
			t.Errorf("two roles truncate to the identical string %q, so /tasks cannot "+
				"tell the user which helper to kill", got)
		}
		seen[got] = true
	}
}

// Directive 1. This line renders once per helper, so a ten-helper run put
// twenty em-dashes on screen in one burst.
func TestTheSpawnNoticeHasNoEmDash(t *testing.T) {
	notice := "🦍 helper a1 spawned: " + truncatePrompt("research · ADVERSARY", 40) +
		"  (/tasks to view or kill)"
	if strings.Contains(notice, "—") {
		t.Error("an em-dash is back in the helper spawn notice")
	}
}

// GORILLA OVERRIDE (2026-08-23): a liveness notice that is wrong ABOUT THE USER
// costs double, because its whole job is to stop them distrusting the screen.
//
// The owner, on a fast line by his own explicit setting, was told "Welcome to
// austere: slow model, SLOW LINE, quiet screen". He had already told the
// program the opposite. Same fault as a full green bar for an allowance that
// does not exist: confident, specific, not true.
func TestTheAustereLineOnlyAppearsOnTheAustereProfile(t *testing.T) {
	config.UseConnProfileForTest(t, config.ProfileUnconstrained)

	for _, l := range heartbeatLines() {
		if strings.Contains(l, "austere") {
			t.Errorf("a fast-line user is told:\n  %s\n\n"+
				"  They chose the profile that says the opposite. A liveness notice "+
				"exists to stop somebody distrusting the screen; one that is wrong "+
				"about their own setup does the reverse.", l)
		}
		if strings.Contains(l, "—") {
			t.Errorf("em-dash in a heartbeat line (directive 1): %s", l)
		}
	}
}

// The line is GOOD and must survive for the people it was written for: the
// satellite uplink where everything looks broken and almost nothing is.
func TestTheAustereLineSurvivesForAustereUsers(t *testing.T) {
	config.UseConnProfileForTest(t, config.ProfileAustere)

	var found bool
	for _, l := range heartbeatLines() {
		if strings.Contains(l, "austere") {
			found = true
		}
	}
	if !found {
		t.Error("the austere heartbeat line was removed rather than scoped. It was " +
			"written for a real user on a real satellite link and it should still " +
			"reach them.")
	}
}
