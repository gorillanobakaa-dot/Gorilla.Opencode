package agent

import (
	"os"
	"testing"

	"github.com/opencode-ai/opencode/internal/config/configtest"
)

// GORILLA OVERRIDE (2026-08-17): tests in this package call config.Load, and
// the dossier tests call config.ToggleLoadout — which persists through
// saveLoadout(). Without this guard those writes land in the developer's real
// ~/.config/gorilla-opencode/loadout.json.
//
// That is not hypothetical. It happened on 2026-08-17: the dossier row, which
// ships OFF by design and which the owner had never armed, was found set to
// true in his live config, put there by this package's tests. His own
// screenshot minutes earlier showed it OFF. The file was backed up and the key
// restored by hand.
//
// CLAUDE.md already recorded this failure three times over in other packages
// ("internal/config had a guard; four other packages did not"). This is the
// fourth. The guard panics rather than falling back, because silent damage to
// someone's configuration is worse than a failed test run.
func TestMain(m *testing.M) { os.Exit(configtest.Isolate(m)) }
