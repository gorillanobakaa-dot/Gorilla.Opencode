package core

import (
	"testing"

	"github.com/opencode-ai/opencode/internal/tui/styles"
	"github.com/opencode-ai/opencode/internal/tui/theme"
)

// Owner, 2026-11-15: the bar's red chips and the folder in its middle were red
// on red. Every fixed pair the bar draws must reach WCAG AA (4.5:1).
func TestStatusBarFixedPairsMeetAA(t *testing.T) {
	pairs := []struct{ name, fg, bg string }{
		{"white on red, dark", "#FFFFFF", styles.SpeedBg.Dark},
		{"white on red, light", "#FFFFFF", styles.SpeedBg.Light},
		{"white on info blue", "#FFFFFF", "#1E3A8A"},
		{"black on warning yellow", "#000000", "#FFD400"},
		{"white on error black", "#FFFFFF", "#000000"},
	}
	for _, p := range pairs {
		r, err := theme.Ratio(p.fg, p.bg)
		if err != nil || r < theme.MinContrast {
			t.Errorf("%s: %s on %s = %.2f:1 (%v), below 4.5:1", p.name, p.fg, p.bg, r, err)
		}
	}
	// and the check can fail: the old pair.
	if r, _ := theme.Ratio("#FFFFFF", "#FF3333"); r >= theme.MinContrast {
		t.Fatalf("white on #FF3333 is %.2f:1; the check would not have caught the bug", r)
	}
}
