package tui

import (
	"os"
	"strings"
	"testing"
)

// THIS FILE IS A LOCK, not a unit test. It is the codebase equivalent of
// chattr +i on the quota meter and the banana ladder.
//
// WHY IT EXISTS. On 2026-08-19 a codebase-wide ASCII sweep (commit 5e4cd97, 81
// files, 657 insertions) replaced the meter's solid body [████░░░░] with
// [####....]. The owner's actual request had been to fix misaligned lines in
// the PROMPT. The sweep matched on "looks like a box character" across the whole
// tree, took two days of deliberate design with it, and then added its own guard
// so the change could not be reversed by accident.
//
// So this is the counter-guard. If a future sweep wants to change any of it, it
// has to delete this file deliberately and explain why in the commit - which is
// the whole point. An accident cannot do it; only a decision can.
//
// WHAT IS PROTECTED AND WHY EACH ONE MATTERS:
//   - the SOLID glyphs. A block reads as a meter; a # reads as text. The colour
//     gradient needs a body to sit in or it is just tinted punctuation.
//   - the gradient endpoints. Red at empty, green at full, through yellow. The
//     bar is a thermometer, and a thermometer that is not red at the bottom is
//     not communicating anything.
//   - all nine banana rungs, verbatim. They are the project's voice. "Rationing
//     mode: sniff the banana, don't eat it" is not decoration - it tells someone
//     on a metered free tier exactly how worried to be, in one line they will
//     actually read.

func TestQuotaBarKeepsItsSolidBody(t *testing.T) {
	full := quotaBar(1.0, 10)
	empty := quotaBar(0.0, 10)

	if !strings.Contains(full, "█") {
		t.Errorf("the filled cell is no longer U+2588 FULL BLOCK: %q\n"+
			"A solid block reads as a METER. A '#' reads as TEXT. If you are "+
			"here because an ASCII sweep flagged this file, read the comment at "+
			"the top of quotaBar: this panel prints to SCROLLBACK, not the "+
			"one-row frame the ASCII rule protects.", full)
	}
	if !strings.Contains(empty, "░") {
		t.Errorf("the empty cell is no longer U+2591 LIGHT SHADE: %q\n"+
			"U+2591 is East Asian NEUTRAL - it never carried a width risk at "+
			"all, and was swept up by pattern-matching rather than by the rule's "+
			"own rationale.", empty)
	}
	for _, bad := range []string{"#", "."} {
		if strings.Contains(full, bad) || strings.Contains(empty, bad) {
			t.Errorf("the meter has been reduced to ASCII %q again", bad)
		}
	}
}

func TestQuotaGradientStillRunsRedToGreen(t *testing.T) {
	// Endpoints, not the whole ramp: red at empty, green at full. If these two
	// move, the bar has stopped being a thermometer.
	if got := quotaHexColor(0.0); !strings.EqualFold(got, "#FF0000") {
		t.Errorf("empty end is %s, want #FF0000 (red)", got)
	}
	if got := quotaHexColor(1.0); !strings.EqualFold(got, "#00FF00") {
		t.Errorf("full end is %s, want #00FF00 (green)", got)
	}
	if got := quotaHexColor(0.5); !strings.EqualFold(got, "#FFFF00") {
		t.Errorf("midpoint is %s, want #FFFF00 (yellow)", got)
	}
	// And the cells must actually DIFFER across the bar, or the gradient has
	// been flattened to one colour while still technically being a gradient
	// function.
	if barCellColor(0, 24) == barCellColor(23, 24) {
		t.Error("first and last cell are the same colour; the gradient is flat")
	}
}

// All nine rungs, verbatim. The tier boundaries are tested elsewhere; this
// protects the WORDS.
func TestAllNineBananaRungsSurvive(t *testing.T) {
	want := []string{
		"Loaded up on bananas... let's go nuts.",
		"You're halfway through your bananas...",
		"Running low on bananas...",
		"Yeah... just a few bananas left.",
		"Banana emergency! Scraping the peel...",
		"This is not a drill. The barrel has a bottom and I can see it.",
		"Rationing mode: sniff the banana, don't eat it.",
		"Last banana spotted. Nobody make any sudden prompts.",
		"Zero bananas. Even the peel is gone.",
	}
	src, err := os.ReadFile("quota_panel.go")
	if err != nil {
		t.Fatal(err)
	}
	body := string(src)
	for _, w := range want {
		if !strings.Contains(body, w) {
			t.Errorf("a banana rung was removed or reworded: %q\n"+
				"These are the project's voice on a metered free tier. Changing "+
				"one is a decision, not a tidy-up - make it deliberately and say "+
				"so in the commit.", w)
		}
	}
	// And they must still be reachable, not just present in the file.
	seen := map[string]bool{}
	for _, f := range []float64{1.0, 0.7, 0.55, 0.3, 0.15, 0.08, 0.04, 0.02, 0.0} {
		seen[bananaStatus(f)] = true
	}
	if len(seen) < 7 {
		t.Errorf("only %d distinct banana messages are reachable across the "+
			"range; the ladder has been collapsed", len(seen))
	}
}
