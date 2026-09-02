package config

import "testing"

// The bug this guards: the low-bandwidth preset switched seven components off,
// wrote them to disk, and nothing ever put them back. A user lost /review to it
// and the only symptom was a model saying it had no such tool.
func TestLowBandwidthTrimCanBeUndone(t *testing.T) {
	if _, err := Load(t.TempDir(), false); err != nil {
		t.Fatalf("Load: %v", err)
	}
	ResetLoadout()
	// Restore afterwards: loadout state is package-global, and leaving it
	// trimmed made a neighbouring test see a preset that could not subtract
	// any further.
	defer ResetLoadout()

	// Precondition: review ships on, or this test proves nothing.
	if !LoadoutEnabled("tool.review") {
		t.Fatal("premise broken: tool.review does not ship enabled")
	}

	ApplyLowBandwidthLoadout()
	if LoadoutEnabled("tool.review") {
		t.Fatal("the trim did not switch tool.review off")
	}
	if n := LowBandwidthUndoCount(); n == 0 {
		t.Fatal("the trim recorded nothing to undo, so it is still one-way")
	}

	restored := UndoLowBandwidthLoadout()
	if restored == 0 {
		t.Fatal("undo restored nothing")
	}
	if !LoadoutEnabled("tool.review") {
		t.Error("tool.review is still off after undo — the exact failure this fixes")
	}
	if n := LowBandwidthUndoCount(); n != 0 {
		t.Errorf("undo left %d entries; it should be spent", n)
	}
}

// Undo must not switch on something the user turned off deliberately. Someone
// who disabled web search on purpose, then applied the preset, should not find
// it back on afterwards.
func TestUndoDoesNotResurrectTheUsersOwnChoices(t *testing.T) {
	if _, err := Load(t.TempDir(), false); err != nil {
		t.Fatalf("Load: %v", err)
	}
	ResetLoadout()
	// Restore afterwards: loadout state is package-global, and leaving it
	// trimmed made a neighbouring test see a preset that could not subtract
	// any further.
	defer ResetLoadout()

	// The user switches web search off themselves.
	if LoadoutEnabled("tool.websearch") {
		ToggleLoadout("tool.websearch")
	}
	if LoadoutEnabled("tool.websearch") {
		t.Fatal("could not switch tool.websearch off for the test")
	}

	ApplyLowBandwidthLoadout()
	UndoLowBandwidthLoadout()

	if LoadoutEnabled("tool.websearch") {
		t.Error("undo switched on something the user had disabled deliberately")
	}
	// But review, which the trim really did turn off, must come back.
	if !LoadoutEnabled("tool.review") {
		t.Error("undo failed to restore what the trim actually changed")
	}
}

// A second trim must not overwrite the record with an empty one, which would
// silently make the first trim permanent.
func TestSecondTrimDoesNotDestroyThePendingUndo(t *testing.T) {
	if _, err := Load(t.TempDir(), false); err != nil {
		t.Fatalf("Load: %v", err)
	}
	ResetLoadout()
	// Restore afterwards: loadout state is package-global, and leaving it
	// trimmed made a neighbouring test see a preset that could not subtract
	// any further.
	defer ResetLoadout()

	ApplyLowBandwidthLoadout()
	first := LowBandwidthUndoCount()
	if first == 0 {
		t.Fatal("first trim recorded nothing")
	}
	// Everything is already off, so this changes nothing.
	ApplyLowBandwidthLoadout()
	if got := LowBandwidthUndoCount(); got != first {
		t.Errorf("undo record went %d -> %d after a no-op trim; the way back was lost", first, got)
	}
	if UndoLowBandwidthLoadout() == 0 {
		t.Error("nothing could be restored after two trims")
	}
	if !LoadoutEnabled("tool.review") {
		t.Error("tool.review not restored after two trims")
	}
}

// The guarantee the original carried, still true with the recording added.
func TestLowBandwidthStillOnlySubtracts(t *testing.T) {
	if _, err := Load(t.TempDir(), false); err != nil {
		t.Fatalf("Load: %v", err)
	}
	ResetLoadout()
	// Restore afterwards: loadout state is package-global, and leaving it
	// trimmed made a neighbouring test see a preset that could not subtract
	// any further.
	defer ResetLoadout()
	before := LoadoutActiveTokens()
	after := ApplyLowBandwidthLoadout()
	if after > before {
		t.Errorf("the trim raised the per-turn cost %d -> %d", before, after)
	}
}
