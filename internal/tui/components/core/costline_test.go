package core

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
	"github.com/opencode-ai/opencode/internal/session"
)

func TestCostDetailSaysWhyAndWhatIsMissing(t *testing.T) {
	s := session.Session{Cost: 10, CostReread: 7.2, LastCallCost: 0.0564, CallsUnpriced: 3}
	got := costDetail(s)
	for _, want := range []string{"72% re-reading", "last call $0.056", "3 call(s) unpriced"} {
		if !strings.Contains(got, want) {
			t.Errorf("costDetail = %q, missing %q", got, want)
		}
	}
	if costDetail(session.Session{}) != "" {
		t.Error("a session with no calls must add nothing to the chip")
	}
}

// One warning per step: $5 at $5.10, nothing more until $10, then $10 (or
// higher, said once, when a single call jumps several steps).
func TestCostWarningOncePerStep(t *testing.T) {
	s := session.Session{PromptTokens: 312_000, LastCallCost: 0.07}
	msg, step := costWarning(s, 4.99, 5, 0)
	if msg != "" || step != 0 {
		t.Fatalf("warned below the threshold: %q", msg)
	}
	msg, step = costWarning(s, 5.10, 5, step)
	if !strings.Contains(msg, "passed $5.00") || !strings.Contains(msg, "312K tokens") || step != 1 {
		t.Fatalf("first warning = %q step %d", msg, step)
	}
	if again, st := costWarning(s, 9.99, 5, step); again != "" || st != 1 {
		t.Fatalf("warned twice for the same step: %q", again)
	}
	msg, step = costWarning(s, 16, 5, step)
	if !strings.Contains(msg, "passed $15.00") || step != 3 {
		t.Fatalf("jump past two steps = %q step %d", msg, step)
	}
	if off, _ := costWarning(s, 1000, 0, 0); off != "" {
		t.Fatal("threshold 0 must switch the warning off")
	}
	if !overCostThreshold(5, 5) || overCostThreshold(4.99, 5) || overCostThreshold(100, 0) {
		t.Fatal("red chip threshold wrong")
	}
}

// On a wide line the chip says why; on the 80-column MacBook Air the detail is
// the first thing to go, and the dollar figure stays.
func TestCostDetailGivesWayOnANarrowLine(t *testing.T) {
	s := session.Session{ID: "s1", Cost: 4.82, CostReread: 1.83, LastCallCost: 0.056, CallsPriced: 69}
	wide := statusCmp{width: 240, session: s}
	if got := ansi.Strip(wide.chrome().tokens); !strings.Contains(got, "38% re-reading") || !strings.Contains(got, "~$4.82 est") {
		t.Errorf("wide chip = %q", got)
	}
	narrow := statusCmp{width: 80, session: s}
	got := ansi.Strip(narrow.chrome().tokens)
	if strings.Contains(got, "re-reading") || !strings.Contains(got, "~$4.82 est") {
		t.Errorf("80-column chip = %q: the detail should give way, the cost should stay", got)
	}
}
