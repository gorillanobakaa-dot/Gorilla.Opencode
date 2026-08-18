package agent

import (
	"strings"
	"testing"

	"github.com/opencode-ai/opencode/internal/config"
)

// Every switchable component must end up with a MEASURED cost rather than the
// hand-written estimate, because /context presents these to the user as a budget and
// a guess dressed as a measurement is worse than an obvious approximation.
//
// diagnostics used to be exempt when no LSP clients were configured — the call was
// guarded on len(lspClients) > 0. But a tool's SCHEMA is static: the clients affect
// what it returns when called, not what it costs to declare. So with every language
// server switched off, which is supported and is the developer's own setup, that one
// row showed an estimate while every other row showed a real figure.
func TestCalibrationCoversEveryComponentWithNoLSPClients(t *testing.T) {
	if _, err := config.Load(t.TempDir(), false); err != nil {
		t.Fatalf("Load: %v", err)
	}

	// GORILLA FIX (2026-08-18): this used to assert "calibrated != hand-written",
	// which is a PROXY for "calibration ran" — and it fails exactly when the
	// hand-written estimate happens to be right. tool.review measured 475 and
	// the estimate was corrected to 475, at which point the test declared the
	// figure a guess. Same shape as the limit-in-the-wrong-unit trap: a proxy
	// breaks precisely in the case it was meant to reward.
	//
	// Stamp a sentinel that no real schema can produce, then assert calibration
	// overwrote it. That measures the thing itself.
	const sentinel = -424242
	for _, c := range config.LoadoutComponents {
		if strings.HasPrefix(c.ID, "lsp.") {
			continue
		}
		config.SetLoadoutTokens(c.ID, sentinel)
	}

	CalibrateLoadout(nil, nil, nil, nil, nil) // no LSP clients at all

	for _, c := range config.LoadoutComponents {
		// Per-server rows come from the user's config at load time and have no
		// schema of their own to measure.
		if strings.HasPrefix(c.ID, "lsp.") {
			continue
		}
		got := config.ComponentTokens(c)
		if got == sentinel {
			t.Errorf("%s was never calibrated — the figure shown in /context is whatever was hand-written",
				c.ID)
		}
		if got < 0 {
			t.Errorf("%s calibrated to %d; a negative token cost is not a measurement", c.ID, got)
		}
	}
}

// Calibration must never take the program down. It runs during startup, before there
// is any UI to report an error to, so a panic would turn a cosmetic number into a
// launch failure. Nil dependencies are exactly what the recover in CalibrateLoadout
// is for.
func TestCalibrationSurvivesNilDependencies(t *testing.T) {
	if _, err := config.Load(t.TempDir(), false); err != nil {
		t.Fatalf("Load: %v", err)
	}
	CalibrateLoadout(nil, nil, nil, nil, nil)
}
