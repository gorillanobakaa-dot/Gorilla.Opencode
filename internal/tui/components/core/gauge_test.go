package core

// GORILLA OVERRIDE (2026-08-23): two token facts, one label, one percentage.
//
// Photographed on the owner's screen, both numbers visible at once: the header
// read "context 34.8K (14%)" and the footer read "Context: ⚠(169%)". Same
// moment, same session.
//
// `TotalTokens()` includes the helpers, deliberately, because on 2026-08-17 a
// run burning 507,935 tokens across 17 helper sessions displayed 44,688 and the
// owner had to total the database by hand. That fix was right. Dividing that
// total by the model's CONTEXT WINDOW is not: helper tokens were never in this
// model's window.
//
// Third instance in one day of a TOTAL being used as a GAUGE.

import (
	"strings"
	"testing"
)

// THE PHOTOGRAPHED CASE, with its real numbers.
func TestTheGaugeMeasuresTheWindowNotTheBill(t *testing.T) {
	const window = 250_000
	const context = 34_800  // what is actually in the window
	const spent = 1_467_867 // conversation plus eighteen helper sessions

	out := formatTokensAndCost(context, spent, window, 0)

	if strings.Contains(out, "%") {
		t.Errorf("a warning percentage was printed for a window that is 14%% full:\n"+
			"  %s\n\n"+
			"  The helpers' tokens are being divided by this model's context window. "+
			"They were never in it.", out)
	}
	if !strings.Contains(out, "34.8K") {
		t.Errorf("the context figure is missing or wrong: %s", out)
	}
}

// The spend must still be VISIBLE. Fixing the gauge by hiding the true total
// would re-break the 2026-08-17 fix, which exists because a half-million-token
// run displayed forty-four thousand.
func TestTheRealSpendIsStillShown(t *testing.T) {
	out := formatTokensAndCost(34_800, 1_467_867, 250_000, 0)
	if !strings.Contains(out, "Spent: 1.5M") {
		t.Errorf("the run's real token spend is not on screen:\n  %s\n\n"+
			"  Hiding it to fix the percentage would restore the bug where a run "+
			"across seventeen helper sessions reported only the conversation.", out)
	}
}

// An ordinary chat has no helpers, so the footer must stay as short as it was.
// A second figure that always says the same as the first is noise.
func TestAnOrdinaryChatKeepsTheShortFooter(t *testing.T) {
	out := formatTokensAndCost(12_000, 12_000, 250_000, 0.42)
	if strings.Contains(out, "Spent:") {
		t.Errorf("a chat with no helpers grew a redundant second figure:\n  %s", out)
	}
	if !strings.Contains(out, "~$0.42 est") {
		t.Errorf("the cost estimate is missing or unmarked: %s", out)
	}
}

// A genuinely full window must still warn. Fixing a false alarm by removing the
// alarm is not a fix.
func TestARealFullWindowStillWarns(t *testing.T) {
	out := formatTokensAndCost(230_000, 230_000, 250_000, 0)
	if !strings.Contains(out, "%") {
		t.Errorf("a window at 92%% did not warn:\n  %s", out)
	}
}
