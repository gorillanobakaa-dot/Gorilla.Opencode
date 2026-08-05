package message

import "testing"

// THE BUG (v0.1.68 shipped it): a provider failure was recorded with
// FinishReasonError and the provider's own text, and then immediately
// overwritten with FinishReasonCanceled by the caller — because AddFinish
// REMOVES any existing finish part before appending. Both the reason and the
// details were lost.
//
// Observed 2026-08-05 against a live NVIDIA 404: the status bar showed
// "Yi Large isn't enabled for your account (HTTP 404 …)" while the transcript
// said "Canceled — no answer was produced", and every finish row in the session
// database read reason=canceled, details empty.
//
// This test pins the destructive property itself, so the agent-side guard has
// something concrete to protect.
func TestASecondAddFinishDestroysTheFirstAndItsDetails(t *testing.T) {
	var m Message
	m.AddFinish(FinishReasonError, "NVIDIA said: 404, model not enabled for this account")

	// Simulate the caller that used to run unconditionally after an error.
	m.AddFinish(FinishReasonCanceled)

	var finishes int
	var got Finish
	for _, p := range m.Parts {
		if f, ok := p.(Finish); ok {
			finishes++
			got = f
		}
	}
	if finishes != 1 {
		t.Fatalf("expected exactly one finish part, got %d", finishes)
	}
	if got.Reason != FinishReasonCanceled {
		t.Fatalf("setup wrong: expected the second call to win, got %q", got.Reason)
	}
	if got.Details != "" {
		t.Fatalf("details unexpectedly survived; this test no longer describes the hazard")
	}

	// The point, stated as an assertion a future reader cannot miss: overwriting
	// is silent and total. Callers must not re-finish a message that already
	// carries a failure they did not produce.
	t.Log("confirmed: a second AddFinish silently discards the earlier reason AND details")
}

// The details must survive when nothing overwrites them — otherwise the guard
// in the agent would be protecting something that never worked anyway.
func TestDetailsSurviveWhenNotOverwritten(t *testing.T) {
	const detail = "Yi Large isn't enabled for your account (HTTP 404 — your key is fine)."
	var m Message
	m.AddFinish(FinishReasonError, detail)

	for _, p := range m.Parts {
		if f, ok := p.(Finish); ok {
			if f.Reason != FinishReasonError {
				t.Errorf("reason = %q, want error", f.Reason)
			}
			if f.Details != detail {
				t.Errorf("details = %q, want %q", f.Details, detail)
			}
			return
		}
	}
	t.Fatal("no finish part recorded")
}
