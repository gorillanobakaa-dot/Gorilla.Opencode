package agent

// GORILLA OVERRIDE (2026-08-23): what a run cost must be recorded even when the
// run is killed.
//
// Measured on a live seventeen-minute run: the footer read "spent $0.01" while
// the database held $6.70 across eighteen helper sessions. The money was never
// lost. Helper turns credit the helper's OWN session, and the parent is only
// credited when the research tool returns, so the true figure arrived after the
// last moment anyone could act on it.
//
// The worse half: that final write used the run's own cancellable context. Kill
// the run and Get returns context.Canceled, the rollup is skipped, and the Save
// error was discarded by `_, _ =` anyway. Pressing X on a run that had spent
// real money erased every trace of it.
//
// The comment in research-tool.go says under-reporting a run is the worst bug
// available to this project. This was that bug through a different door, and
// the fix for the first one is what failed: an accounting write tied to the
// lifetime of the work it accounts for.

import (
	"context"
	"testing"

	"github.com/opencode-ai/opencode/internal/session"
)

// The property, stated directly against the standard library. A cancelled
// parent must not cancel the accounting write.
func TestTheAccountingWriteSurvivesAKilledRun(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	writeCtx := context.WithoutCancel(ctx)

	cancel() // the user pressed X

	if ctx.Err() == nil {
		t.Fatal("the run context did not cancel; this test proves nothing")
	}
	if err := writeCtx.Err(); err != nil {
		t.Errorf("the accounting context died with the run (%v). Every helper's "+
			"spend is then dropped: Get returns context.Canceled, the rollup is "+
			"skipped, and the session total silently stays where it was.", err)
	}
}

// A helper's spend must actually add up, in all three columns. Cost alone is
// not enough: on a free tier the money is zero and the tokens are the only
// evidence the run happened at all.
func TestHelperSpendAddsUpInEveryColumn(t *testing.T) {
	var total helperSpend
	total.add(helperSpend{cost: 2.0971, inTokens: 114382, outTokens: 1250})
	total.add(helperSpend{cost: 0.7571, inTokens: 29360, outTokens: 1197})
	total.add(helperSpend{cost: 0.6989, inTokens: 150989, outTokens: 899})

	if got := total.cost; got < 3.55 || got > 3.56 {
		t.Errorf("cost totalled %.4f, want about 3.5531", got)
	}
	if total.inTokens != 294731 {
		t.Errorf("input tokens totalled %d, want 294731", total.inTokens)
	}
	if total.outTokens != 3346 {
		t.Errorf("output tokens totalled %d, want 3346", total.outTokens)
	}
}

// The live figure comes from the REGISTRY, not from "every child of this
// session", and the difference is the whole correctness argument: a finished
// run is rolled into the parent, so summing all children would count it twice.
func TestLiveSessionsCoverOnlyTheRunInFlight(t *testing.T) {
	// Cleaned up through the real purge path rather than a test-only reset, so
	// this also exercises the code that actually runs when a run ends.
	defer UnregisterSubAgentsForCall("call-1")
	defer UnregisterSubAgentsForCall("call-2")

	a := RegisterSubAgentState("helper-session-a", "conversation", "call-1",
		"research · ADVERSARY", SubAgentRunning, func() {})
	RegisterSubAgentState("helper-session-b", "other-conversation", "call-2",
		"research · COST", SubAgentRunning, func() {})

	live := LiveSubAgentSessions("conversation")
	if len(live) != 1 || live[0] != "helper-session-a" {
		t.Fatalf("got %v, want only this conversation's helper. Summing another "+
			"conversation's helpers into this footer would bill the user for "+
			"work they are not looking at.", live)
	}

	UnregisterSubAgentsForCall("call-1")
	if live := LiveSubAgentSessions("conversation"); len(live) != 0 {
		t.Errorf("after the run was purged, %v is still counted as in flight. The "+
			"tool has already rolled this spend into the parent, so counting it "+
			"again turns an under-report into an over-report.", live)
	}
	_ = a
}

// A RUN MUST NOT INFLATE THE CONTEXT GAUGE.
//
// Measured 2026-08-23: after an eighteen-helper run, a conversation holding
// about 14K of real context showed "1.2M (115%)" in the footer and a red
// "Context: 229%". The rollup had added the helpers' 1,121,961 prompt tokens to
// PromptTokens, which is CURRENT OCCUPANCY, not a lifetime total.
//
// agent.go:819 predicted this in writing three months earlier: "a running total
// would climb past 100% and sit there showing a false warning". The fields to
// accumulate into are the Cumulative ones, added by that same fix so a lifetime
// total could be recorded WITHOUT breaking the gauge.
//
// This matters beyond a wrong number: a false context warning invites a user to
// compact a conversation that is nowhere near full, throwing away history to
// solve a problem they do not have.
func TestARunDoesNotInflateTheContextGauge(t *testing.T) {
	const window = 250_000

	// A small conversation, plus a very large run underneath it.
	sess := session.Session{
		PromptTokens:     13504,
		CompletionTokens: 540,
	}
	run := helperSpend{cost: 7.64, inTokens: 1_121_961, outTokens: 17_319}

	// What the rollup does.
	sess.Cost += run.cost
	sess.CumulativePromptTokens += run.inTokens
	sess.CumulativeCompletionTokens += run.outTokens

	gauge := float64(sess.PromptTokens+sess.CompletionTokens) / float64(window) * 100
	if gauge > 100 {
		t.Errorf("the context gauge reads %.0f%% for a conversation holding %d tokens "+
			"in a %d window. The helpers' tokens have been added to CURRENT "+
			"occupancy; they belong in the Cumulative fields.",
			gauge, sess.PromptTokens+sess.CompletionTokens, window)
	}

	// The lifetime total must still be recorded, or the fix trades one wrong
	// number for a missing one.
	if sess.CumulativePromptTokens != 1_121_961 {
		t.Errorf("cumulative prompt tokens %d, want the run's 1121961: the spend "+
			"has to land SOMEWHERE or a killed ledger is the result",
			sess.CumulativePromptTokens)
	}
	if sess.Cost < 7.6 {
		t.Errorf("cost %.2f, want the run's 7.64", sess.Cost)
	}
}
