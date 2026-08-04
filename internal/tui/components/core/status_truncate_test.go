package core

import (
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/charmbracelet/x/ansi"
)

// THE BUG: the status line truncated with msg[:infoWidth] — a BYTE offset used
// against a COLUMN count. Pure ASCII hid it. The provider errors that land here
// now carry "—" and "⟨⟩" (3 bytes each), so a cut landing mid-rune stopped being
// a curiosity and became routine, emitting invalid UTF-8 into the frame.
//
// Asserted against the truncation helper the status bar uses, at every cut point
// across a multi-byte string, because the bug only appears at specific offsets —
// a single-width test would have passed against it.
func TestStatusTruncationNeverSplitsARune(t *testing.T) {
	msg := "Llama 3.3 70B isn't enabled for your account (HTTP 404 — your key is fine). " +
		"Pick another with /models.  ⟨POST \"https://integrate.api.nvidia.com/v1\": 404⟩"

	for width := 1; width <= len(msg); width++ {
		got := truncateStatusMsg(msg, width)
		if !utf8.ValidString(got) {
			t.Fatalf("width %d produced invalid UTF-8: %q", width, got)
		}
		if strings.ContainsRune(got, utf8.RuneError) {
			t.Fatalf("width %d produced a replacement char, i.e. a severed rune: %q", width, got)
		}
	}
}

// The old code compared len(msg) — bytes — against a column budget, so a message
// full of multi-byte runes was reported as far longer than it displays and was
// truncated when it would have fitted. Proving the two units genuinely disagree
// keeps this test from passing for the wrong reason.
func TestByteLengthAndDisplayWidthActuallyDiffer(t *testing.T) {
	const msg = "HTTP 404 — your key is fine ⟨POST⟩"

	bytes := len(msg)
	cols := ansi.StringWidth(msg)
	if bytes == cols {
		t.Fatalf("this test string has no multi-byte runes (%d bytes, %d cols), so it "+
			"cannot demonstrate the bug it exists to pin", bytes, cols)
	}

	// At a width between the two, byte-slicing would have truncated while the
	// message actually fits on screen.
	if fits := truncateStatusMsg(msg, cols); fits != msg {
		t.Errorf("a message that exactly fits its column budget was still truncated:\n%q", fits)
	}
}
