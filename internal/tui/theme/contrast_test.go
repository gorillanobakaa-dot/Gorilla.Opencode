package theme

import (
	"testing"

	"github.com/charmbracelet/lipgloss"
)

// GORILLA (2026-11-15, owner): red text on a red status bar was unreadable.
// Every foreground/background pair the interface draws from a theme must reach
// WCAG AA for normal text (4.5:1), in the dark and the light variant of every
// built-in theme.

func TestEveryThemedPairMeetsWCAGAA(t *testing.T) {
	type pair struct {
		name   string
		fg, bg func(Theme) lipgloss.AdaptiveColor
	}
	pairs := []pair{
		{"text on background", Theme.Text, Theme.Background},
		{"muted text on background", Theme.TextMuted, Theme.Background},
		{"text on darker background", Theme.Text, Theme.BackgroundDarker},
		{"primary on background", Theme.Primary, Theme.Background},
		{"secondary on background (> prompt)", Theme.Secondary, Theme.Background},
		{"error on background", Theme.Error, Theme.Background},
		{"warning on background", Theme.Warning, Theme.Background},
		{"success on background", Theme.Success, Theme.Background},
		{"info on background", Theme.Info, Theme.Background},
		{"error on darker background", Theme.Error, Theme.BackgroundDarker},
		{"warning on darker background", Theme.Warning, Theme.BackgroundDarker},
		{"info on darker background", Theme.Info, Theme.BackgroundDarker},
		{"background on secondary (model chip)", Theme.Background, Theme.Secondary},
		{"background on warning (helpers chip)", Theme.Background, Theme.Warning},
		{"background on error (cost chip)", Theme.Background, Theme.Error},
		{"darker on muted (help chip)", Theme.BackgroundDarker, Theme.TextMuted},
		{"secondary background on text (tokens chip)", Theme.BackgroundSecondary, Theme.Text},
	}
	for _, name := range AvailableThemes() {
		th := GetTheme(name)
		for _, p := range pairs {
			for _, dark := range []bool{true, false} {
				pick := func(c lipgloss.AdaptiveColor) string {
					if dark {
						return c.Dark
					}
					return c.Light
				}
				fg, bg := pick(p.fg(th)), pick(p.bg(th))
				r, err := Ratio(fg, bg)
				if err != nil {
					t.Errorf("%s %s: %v", name, p.name, err)
					continue
				}
				if r < 4.5 {
					t.Errorf("theme %s (dark=%v): %s is %s on %s = %.2f:1, below 4.5:1", name, dark, p.name, fg, bg, r)
				}
			}
		}
	}
}

func TestContrastCheckCanFail(t *testing.T) {
	r, err := Ratio("#FF0000", "#FF3333")
	if err != nil || r >= 4.5 {
		t.Fatalf("red on red must fail the check, got %.2f (%v)", r, err)
	}
	if r, _ := Ratio("#000000", "#FFFFFF"); r < 20.9 {
		t.Fatalf("black on white is 21:1, got %.2f", r)
	}
}
