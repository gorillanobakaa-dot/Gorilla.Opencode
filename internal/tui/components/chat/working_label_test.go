package chat

// GORILLA OVERRIDE (2026-08-18): tests for the working indicator's clock and
// its cold-start escalation.
//
// The failure this prevents is silent by nature: a mute "Thinking..." spinner
// while a model warms up, indistinguishable from a hang. Measured that day,
// warm NVIDIA NIM models answered in under a second and a cold one took 12–19s,
// which is why the threshold sits at 12s and why only the pre-first-token
// phases carry the reassurance.

import (
	"strings"
	"testing"
	"time"
)

func TestWorkingLabelIsAlwaysAShortCounter(t *testing.T) {
	m := &messagesCmp{}

	// Early on: phase plus a counter, so the screen is visibly ticking.
	if got := m.workingLabel("Thinking...", 3*time.Second); got != "Thinking... (3s)" {
		t.Errorf("early label should be a plain counter, got %q", got)
	}

	// The footer is exactly one row and lipgloss WRAPS, so the label must stay
	// short at any elapsed — the long explanation lives in a toast, not here. A
	// label that grew a sentence would become a second row and corrupt the
	// scrollback erase. Guard the length hard.
	long := m.workingLabel("Thinking...", 9999*time.Second)
	if len(long) > 32 {
		t.Errorf("the footer label grew long enough to risk wrapping: %q (%d chars)", long, len(long))
	}
	if strings.Contains(long, "warming up") {
		t.Errorf("the explanation leaked into the one-row footer: %q", long)
	}
}

// The toast decision is a pure predicate so it can be tested without the event
// loop. It fires past the threshold, only for pre-token phases.
func TestColdStartWarnDecision(t *testing.T) {
	if shouldColdStartWarn("Thinking...", 3*time.Second) {
		t.Error("warned too early")
	}
	if !shouldColdStartWarn("Thinking...", coldStartHint) {
		t.Error("did not warn once the threshold was reached")
	}
	if !shouldColdStartWarn("Generating...", 30*time.Second) {
		t.Error("did not warn on a long pre-token generate")
	}
	for _, phase := range []string{"Waiting for tool response...", "Building tool call...", ""} {
		if shouldColdStartWarn(phase, 5*time.Minute) {
			t.Errorf("wrongly warned on a non-pre-token phase %q — that would call a tool wait a model warm-up", phase)
		}
	}
}

// A tool-wait phase still gets a plain counter in the footer — it is a real
// wait worth showing — it just must not get the model warm-up sentence (that is
// asserted by TestColdStartWarnDecision).
func TestToolWaitPhasesStillCount(t *testing.T) {
	m := &messagesCmp{}
	for _, phase := range []string{"Waiting for tool response...", "Building tool call..."} {
		if got := m.workingLabel(phase, 30*time.Second); !strings.Contains(got, "30s") {
			t.Errorf("phase %q lost its counter: %q", phase, got)
		}
	}
}

// The very first frame of a phase has no stamped start time; it must read as the
// bare phase name, never "(0s)" noise or a panic on the zero time.
func TestZeroElapsedIsJustThePhaseName(t *testing.T) {
	m := &messagesCmp{}
	if got := m.workingLabel("Generating...", 0); got != "Generating..." {
		t.Errorf("zero elapsed should be the bare phase, got %q", got)
	}
	if got := m.elapsedInPhase(); got != 0 {
		t.Errorf("an unstamped clock should read 0, got %v", got)
	}
}

// An empty phase produces no text at all — the footer stays blank rather than
// rendering a stray spinner with nothing beside it.
func TestEmptyPhaseIsBlank(t *testing.T) {
	m := &messagesCmp{}
	if got := m.workingLabel("", 40*time.Second); got != "" {
		t.Errorf("an empty phase should render nothing, got %q", got)
	}
}
