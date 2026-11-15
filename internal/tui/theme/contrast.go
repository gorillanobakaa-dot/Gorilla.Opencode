package theme

import (
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// GORILLA (2026-11-15, owner): red text on a red status bar was unreadable.
// Every theme colour that is drawn as text, or as a chip behind text, is
// brought to WCAG AA (4.5:1, normal text) against the three backgrounds of its
// theme, in both variants. A theme author's palette is kept as far as it
// passes; a colour that fails is mixed toward white (dark variant) or black
// (light variant) in 4% steps until it passes, so its hue is kept.

// MinContrast is WCAG AA for normal text.
const MinContrast = 4.5

func parseHex(hex string) (r, g, b float64, err error) {
	h := strings.TrimPrefix(hex, "#")
	if len(h) != 6 {
		return 0, 0, 0, fmt.Errorf("not a #rrggbb colour: %q", hex)
	}
	var c [3]float64
	for i := 0; i < 3; i++ {
		v, e := strconv.ParseUint(h[i*2:i*2+2], 16, 8)
		if e != nil {
			return 0, 0, 0, e
		}
		c[i] = float64(v)
	}
	return c[0], c[1], c[2], nil
}

func luminance(hex string) (float64, error) {
	r, g, b, err := parseHex(hex)
	if err != nil {
		return 0, err
	}
	lin := func(v float64) float64 {
		f := v / 255
		if f <= 0.03928 {
			return f / 12.92
		}
		return math.Pow((f+0.055)/1.055, 2.4)
	}
	return 0.2126*lin(r) + 0.7152*lin(g) + 0.0722*lin(b), nil
}

// Ratio is the WCAG contrast ratio of two #rrggbb colours.
func Ratio(a, b string) (float64, error) {
	la, err := luminance(a)
	if err != nil {
		return 0, err
	}
	lb, err := luminance(b)
	if err != nil {
		return 0, err
	}
	if la < lb {
		la, lb = lb, la
	}
	return (la + 0.05) / (lb + 0.05), nil
}

func passes(fg string, bgs []string) bool {
	for _, bg := range bgs {
		if r, err := Ratio(fg, bg); err != nil || r < MinContrast {
			return false
		}
	}
	return true
}

// EnsureContrast returns fg unchanged when it reaches 4.5:1 on every bg, else
// the nearest mix of fg toward the pole (white or black) that does.
func EnsureContrast(fg string, bgs []string, towardWhite bool) string {
	if _, _, _, err := parseHex(fg); err != nil || passes(fg, bgs) {
		return fg
	}
	r, g, b, _ := parseHex(fg)
	pole := 0.0
	if towardWhite {
		pole = 255
	}
	for i := 1; i <= 25; i++ {
		f := float64(i) * 0.04
		mix := func(v float64) int { return int(math.Round(v + (pole-v)*f)) }
		c := fmt.Sprintf("#%02x%02x%02x", mix(r), mix(g), mix(b))
		if passes(c, bgs) {
			return c
		}
	}
	if towardWhite {
		return "#ffffff"
	}
	return "#000000"
}

type baseHolder interface{ base() *BaseTheme }

func (t *BaseTheme) base() *BaseTheme { return t }

// ensureAA brings the text and chip colours of a theme to AA. Themes that do
// not embed BaseTheme are left alone.
func ensureAA(th Theme) {
	h, ok := th.(baseHolder)
	if !ok {
		return
	}
	b := h.base()
	fix := func(c *lipgloss.AdaptiveColor) {
		c.Dark = EnsureContrast(c.Dark, []string{b.BackgroundColor.Dark, b.BackgroundDarkerColor.Dark, b.BackgroundSecondaryColor.Dark}, true)
		c.Light = EnsureContrast(c.Light, []string{b.BackgroundColor.Light, b.BackgroundDarkerColor.Light, b.BackgroundSecondaryColor.Light}, false)
	}
	for _, c := range []*lipgloss.AdaptiveColor{&b.TextColor, &b.TextMutedColor, &b.PrimaryColor, &b.SecondaryColor,
		&b.ErrorColor, &b.WarningColor, &b.SuccessColor, &b.InfoColor} {
		fix(c)
	}
}
