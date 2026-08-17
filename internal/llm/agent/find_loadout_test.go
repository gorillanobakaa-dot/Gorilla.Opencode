package agent

import (
	"testing"

	"github.com/opencode-ai/opencode/internal/config"
	"github.com/opencode-ai/opencode/internal/llm/tools"
)

func taskToolNames() map[string]bool {
	names := map[string]bool{}
	for _, t := range TaskAgentTools(nil, nil) {
		names[t.Info().Name] = true
	}
	return names
}

// TestFindToolIsInTheContextLoadoutAndToggles is the user-facing contract for
// the find tool's /context row: it is registered, it is ON by default, turning
// it off really removes it from the agent's toolbox, and turning it back on
// really restores it. The registry row is what /context renders; the
// TaskAgentTools round trip is what the model actually receives.
func TestFindToolIsInTheContextLoadoutAndToggles(t *testing.T) {
	if _, err := config.Load(t.TempDir(), false); err != nil {
		t.Fatalf("Load: %v", err)
	}

	var row *config.LoadoutComponent
	for i := range config.LoadoutComponents {
		if config.LoadoutComponents[i].ID == "tool.find" {
			row = &config.LoadoutComponents[i]
		}
		// The three retired tools must NOT have rows: a /context entry for a
		// tool that can never be registered would be a dead switch.
		switch config.LoadoutComponents[i].ID {
		case "tool.ls", "tool.grep", "tool.glob":
			t.Errorf("retired component %s still has a /context row", config.LoadoutComponents[i].ID)
		}
	}
	if row == nil {
		t.Fatal("tool.find is not in the /context loadout registry")
	}
	if !row.Default {
		t.Error("tool.find must be ON by default")
	}
	if !row.Critical {
		t.Error("tool.find must be marked critical — without it the agent cannot see the tree")
	}

	if !taskToolNames()[tools.FindToolName] {
		t.Fatal("find missing from TaskAgentTools while enabled")
	}

	// Flip OFF — and restore afterwards even if an assertion fails, because
	// loadout state persists and leaks between tests otherwise.
	config.ToggleLoadout("tool.find")
	defer func() {
		if !config.LoadoutEnabled("tool.find") {
			config.ToggleLoadout("tool.find")
		}
	}()
	if taskToolNames()[tools.FindToolName] {
		t.Error("find still handed to the agent after /context disabled it")
	}

	// Flip back ON.
	config.ToggleLoadout("tool.find")
	if !taskToolNames()[tools.FindToolName] {
		t.Error("find not restored after re-enabling in /context")
	}
}

// TestStaleRetiredLoadoutKeysAreHarmless: a user upgrading from a build that
// had tool.ls/tool.grep/tool.glob rows may carry their disabled state in
// loadout.json. Those orphaned keys must not crash anything or affect find.
func TestStaleRetiredLoadoutKeysAreHarmless(t *testing.T) {
	if _, err := config.Load(t.TempDir(), false); err != nil {
		t.Fatalf("Load: %v", err)
	}
	for _, stale := range []string{"tool.ls", "tool.grep", "tool.glob"} {
		config.ToggleLoadout(stale) // writes a disabled entry for a component that no longer exists
	}
	defer func() {
		for _, stale := range []string{"tool.ls", "tool.grep", "tool.glob"} {
			if !config.LoadoutEnabled(stale) {
				config.ToggleLoadout(stale)
			}
		}
	}()

	if !config.LoadoutEnabled("tool.find") {
		t.Error("stale retired keys disabled tool.find")
	}
	if !taskToolNames()[tools.FindToolName] {
		t.Error("stale retired keys removed find from the toolbox")
	}
}
