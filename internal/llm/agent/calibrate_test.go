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

	declared := map[string]int{}
	for _, c := range config.LoadoutComponents {
		declared[c.ID] = c.Tokens
	}

	CalibrateLoadout(nil, nil, nil, nil, nil) // no LSP clients at all

	for _, c := range config.LoadoutComponents {
		// Per-server rows come from the user's config at load time and have no
		// schema of their own to measure.
		if strings.HasPrefix(c.ID, "lsp.") {
			continue
		}
		if got := config.ComponentTokens(c); got == declared[c.ID] && declared[c.ID] != 0 {
			t.Errorf("%s still reports its hand-written estimate (%d) after calibration — the figure shown in /context is a guess",
				c.ID, got)
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
