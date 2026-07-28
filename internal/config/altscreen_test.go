package config

import "testing"

// A fresh install must land OUTSIDE the alternate screen. That buffer has no
// scrollback, so anything drawn in it cannot be scrolled back to, selected or
// copied — the default decides whether the program behaves like a terminal
// program at all, and nobody clicking an icon will find a setting to fix it.
func TestAlternateScreenDefaultsOff(t *testing.T) {
	if _, err := Load(t.TempDir(), false); err != nil {
		t.Fatalf("Load: %v", err)
	}
	if AlternateScreenEnabled() {
		t.Error("a fresh config draws on the alternate screen; the conversation would " +
			"have no scrollback, so the wheel, Select-All and copy would all do nothing")
	}

	// And the registry must agree with the code, or /settings would advertise a
	// default the program does not honour.
	found := false
	for _, s := range Settings {
		if s.ID != "alternateScreen" {
			continue
		}
		found = true
		if s.Default != false {
			t.Errorf("/settings advertises default %v for alternateScreen; the code uses false", s.Default)
		}
		if !s.Restart {
			t.Error("alternateScreen is not marked as needing a restart, but the buffer is " +
				"entered once at startup — the setting would appear to do nothing")
		}
	}
	if !found {
		t.Fatal("alternateScreen is not in the settings registry, so it is unreachable " +
			"from /settings and this test's other assertions are vacuous")
	}
}

// Wanting the wheel and actually requesting mouse events are different questions.
// Without the alternate screen the TERMINAL scrolls the conversation, because the
// conversation is in its scrollback; asking for mouse events there would take a
// working wheel away and break drag-to-select in exchange for nothing.
func TestMouseEventsAreOnlyRequestedOnTheAlternateScreen(t *testing.T) {
	if _, err := Load(t.TempDir(), false); err != nil {
		t.Fatalf("Load: %v", err)
	}

	cases := []struct {
		altScreen, wheel, wantRequest bool
		why                           string
	}{
		{false, false, false, "nothing asked for, nothing requested"},
		{false, true, false, "wheel wanted but the terminal already scrolls; requesting would break selection for nothing"},
		{true, false, false, "on a separate screen but the wheel was not asked for"},
		{true, true, true, "on a separate screen and the wheel was asked for: the only case that buys anything"},
	}
	for _, c := range cases {
		if err := SetAlternateScreen(c.altScreen); err != nil {
			t.Fatalf("SetAlternateScreen: %v", err)
		}
		if err := SetMouseWheel(c.wheel); err != nil {
			t.Fatalf("SetMouseWheel: %v", err)
		}
		if got := RequestMouseEvents(); got != c.wantRequest {
			t.Errorf("altScreen=%v wheel=%v: RequestMouseEvents()=%v, want %v — %s",
				c.altScreen, c.wheel, got, c.wantRequest, c.why)
		}
		// The stored preference must survive either way: a user who asked for the
		// wheel should still have it recorded when they turn the alternate screen on.
		if got := MouseWheelEnabled(); got != c.wheel {
			t.Errorf("altScreen=%v wheel=%v: the preference itself changed to %v; "+
				"RequestMouseEvents must gate the request, not rewrite the answer",
				c.altScreen, c.wheel, got)
		}
	}
}

// Both settings must round-trip through the file. An explicit true has to survive
// a restart, and omitempty on a bool is the trap that would silently drop it —
// safe here only because the default is false, which this pins down.
func TestAlternateScreenPersists(t *testing.T) {
	dir := t.TempDir()
	if _, err := Load(dir, false); err != nil {
		t.Fatalf("Load: %v", err)
	}
	if err := SetAlternateScreen(true); err != nil {
		t.Fatalf("SetAlternateScreen: %v", err)
	}

	if _, err := Load(dir, false); err != nil {
		t.Fatalf("reload: %v", err)
	}
	if !AlternateScreenEnabled() {
		t.Error("alternateScreen=true did not survive a reload; the setting would " +
			"silently revert on every restart, which is exactly when it is read")
	}
}
