package config

import (
	"slices"
	"sync"
	"testing"
)

// Unknown ids must be ENABLED, matching LoadoutEnabled's rule. A command added in
// a later version must not be silently missing because an old commands.json does
// not mention it.
func TestUnknownCommandIsEnabled(t *testing.T) {
	if !CommandEnabled("a-command-that-does-not-exist-yet") {
		t.Error("unknown command id defaulted to disabled — a newly shipped command would silently vanish")
	}
}

func TestToggleAndResetCommands(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	// Force a re-read against the isolated dir.
	commandsOnce = onceReset()
	t.Cleanup(func() { commandsOnce = onceReset(); ResetCommands() })

	const id = "export"
	if !CommandEnabled(id) {
		t.Fatalf("%s should start enabled", id)
	}

	if enabled := ToggleCommand(id); enabled {
		t.Errorf("ToggleCommand reported enabled=%v after switching off", enabled)
	}
	if CommandEnabled(id) {
		t.Error("command still enabled after toggle")
	}
	if got := DisabledCommands(); !slices.Contains(got, id) {
		t.Errorf("DisabledCommands() = %v, want it to contain %q", got, id)
	}

	if enabled := ToggleCommand(id); !enabled {
		t.Error("second toggle did not re-enable")
	}
	if !CommandEnabled(id) {
		t.Error("command not enabled after toggling back")
	}
	// An enabled command must be ABSENT from the file, not stored as false —
	// keeps commands.json minimal and makes "differs from default" countable.
	if got := DisabledCommands(); slices.Contains(got, id) {
		t.Errorf("re-enabled command still listed as disabled: %v", got)
	}
}

func TestResetCommandsClearsEverything(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	commandsOnce = onceReset()
	t.Cleanup(func() { commandsOnce = onceReset(); ResetCommands() })

	for _, id := range []string{"export", "tasks", "context"} {
		SetCommandDisabled(id, true)
	}
	if got := len(DisabledCommands()); got != 3 {
		t.Fatalf("disabled %d commands, want 3", got)
	}

	ResetCommands()
	if got := DisabledCommands(); len(got) != 0 {
		t.Errorf("after ResetCommands, still disabled: %v", got)
	}
}

// DisabledCommands must be sorted, so /reset's "disabled: a, b, c" summary is
// stable rather than reshuffling between renders.
func TestDisabledCommandsIsSorted(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	commandsOnce = onceReset()
	t.Cleanup(func() { commandsOnce = onceReset(); ResetCommands() })

	for _, id := range []string{"zebra", "alpha", "mike"} {
		SetCommandDisabled(id, true)
	}
	got := DisabledCommands()
	want := []string{"alpha", "mike", "zebra"}
	if !slices.Equal(got, want) {
		t.Errorf("DisabledCommands() = %v, want %v", got, want)
	}
}

// onceReset returns a fresh sync.Once so a test can force commands.json to be
// re-read against an isolated XDG_CONFIG_HOME. The production path deliberately
// reads once per process.
func onceReset() sync.Once { return sync.Once{} }
