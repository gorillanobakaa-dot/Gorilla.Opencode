//go:build windows

package tui

import "testing"

func TestWindowsFontPreferencesRespectDefaultsAndEnv(t *testing.T) {
	t.Setenv("GORILLA_FONT", "")
	t.Setenv("GORILLA_FONT_SIZE", "")
	fonts := preferredWindowsFonts()
	if len(fonts) == 0 || fonts[0] != "Cascadia Mono" {
		t.Fatalf("default first font = %q, want Cascadia Mono", fonts)
	}
	if sz := preferredWindowsFontSize(); sz != 18 {
		t.Fatalf("default font size = %d, want 18", sz)
	}

	t.Setenv("GORILLA_FONT", "JetBrains Mono")
	t.Setenv("GORILLA_FONT_SIZE", "20")
	fonts = preferredWindowsFonts()
	if fonts[0] != "JetBrains Mono" {
		t.Fatalf("override first font = %q, want JetBrains Mono", fonts[0])
	}
	if sz := preferredWindowsFontSize(); sz != 20 {
		t.Fatalf("override font size = %d, want 20", sz)
	}

	// Out-of-range sizes fall back safely to default 18.
	t.Setenv("GORILLA_FONT_SIZE", "999")
	if sz := preferredWindowsFontSize(); sz != 18 {
		t.Fatalf("out-of-range font size = %d, want 18", sz)
	}

	// Face name UTF-16 round-trip and truncation safety (LF_FACESIZE = 32).
	if got := decodeFaceName(encodeFaceName("Cascadia Mono")); got != "Cascadia Mono" {
		t.Fatalf("face name round-trip = %q", got)
	}
	long := decodeFaceName(encodeFaceName("AVeryLongFontNameThatExceedsThirtyTwoCharacters"))
	if len(long) != lfFacesize-1 {
		t.Fatalf("long face name len = %d, want %d", len(long), lfFacesize-1)
	}

	// Opt-out disables any Win32 call.
	t.Setenv("GORILLA_NO_FONT_OVERRIDE", "1")
	if got := EnsureConsoleFont(); got != "" {
		t.Fatalf("EnsureConsoleFont with opt-out returned %q, want empty", got)
	}
}
