package models

import "testing"

// A single-id Antigravity Gemini model (no effort word in the id) still gets a
// low/medium/high meter.
func TestSingleIDGeminiHasEffortMeter(t *testing.T) {
	m := agModel("gemini-4-hydrogen-sonar", "Gemini 4 Hydrogen (Sonar)")
	m.CanReason = true
	if len(m.EffortOptions()) == 0 {
		t.Fatal("gemini-4-hydrogen-sonar has no effort meter")
	}
}
