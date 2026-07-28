package styles

import (
	"github.com/charmbracelet/lipgloss"
	"github.com/opencode-ai/opencode/internal/config"
	"github.com/opencode-ai/opencode/internal/tui/theme"
)

var (
	ImageBakcground = "#212121"
)

// PanelBackground is the colour to fill a panel or a line with.
//
// GORILLA OVERRIDE: it is TRANSPARENT — the terminal's own background — whenever
// the program is not drawing on the alternate screen.
//
// The reasoning is about what we control. On the alternate screen the program owns
// every cell, so filling them with the theme's background produces a complete,
// deliberate-looking surface. Outside it we own only the cells we write, and the
// terminal's background shows through everything else: the rows above the
// conversation, the startup padding, anything the shell left on screen, and every
// span we forget to style. The result is a coloured slab with holes punched in it,
// which reads as unfinished rather than as a theme. Measured example: the footer's
// field separators were raw strings, so a 100-column line changed background at
// column 19.
//
// Painting nothing removes the entire class of defect rather than the instances of
// it — there is no such thing as a half-painted transparent background. The text
// keeps every one of the theme's foreground colours, so the palette survives; only
// the fills go. This is what Gemini CLI and Claude Code do, and it is why neither
// ever looks patchy.
//
// The alternative was to set the terminal's own background with OSC 11, which was
// verified to work here (queried black, set #282a36, queried it back and got
// 2828/2a2a/3636). Rejected because it changes a setting the user owns and does not
// change back if the program is killed hard.
//
// Deliberate highlights — a selected row, a warning, an error bar — are NOT routed
// through here. Those carry meaning and must stay painted in both modes.
func PanelBackground() lipgloss.TerminalColor {
	if config.AlternateScreenEnabled() {
		return theme.CurrentTheme().Background()
	}
	// NoColor emits no escape at all, which is precisely "whatever the terminal is".
	return lipgloss.NoColor{}
}

// Style generation functions that use the current theme

// BaseStyle returns the base style with background and foreground colors
func BaseStyle() lipgloss.Style {
	t := theme.CurrentTheme()
	return lipgloss.NewStyle().
		Background(PanelBackground()).
		Foreground(t.Text())
}

// Regular returns a basic unstyled lipgloss.Style
func Regular() lipgloss.Style {
	return lipgloss.NewStyle()
}

// Bold returns a bold style
func Bold() lipgloss.Style {
	return Regular().Bold(true)
}

// Padded returns a style with horizontal padding
func Padded() lipgloss.Style {
	return Regular().Padding(0, 1)
}

// Border returns a style with a normal border
func Border() lipgloss.Style {
	t := theme.CurrentTheme()
	return Regular().
		Border(lipgloss.NormalBorder()).
		BorderForeground(t.BorderNormal())
}

// ThickBorder returns a style with a thick border
func ThickBorder() lipgloss.Style {
	t := theme.CurrentTheme()
	return Regular().
		Border(lipgloss.ThickBorder()).
		BorderForeground(t.BorderNormal())
}

// DoubleBorder returns a style with a double border
func DoubleBorder() lipgloss.Style {
	t := theme.CurrentTheme()
	return Regular().
		Border(lipgloss.DoubleBorder()).
		BorderForeground(t.BorderNormal())
}

// FocusedBorder returns a style with a border using the focused border color
func FocusedBorder() lipgloss.Style {
	t := theme.CurrentTheme()
	return Regular().
		Border(lipgloss.NormalBorder()).
		BorderForeground(t.BorderFocused())
}

// DimBorder returns a style with a border using the dim border color
func DimBorder() lipgloss.Style {
	t := theme.CurrentTheme()
	return Regular().
		Border(lipgloss.NormalBorder()).
		BorderForeground(t.BorderDim())
}

// PrimaryColor returns the primary color from the current theme
func PrimaryColor() lipgloss.AdaptiveColor {
	return theme.CurrentTheme().Primary()
}

// SecondaryColor returns the secondary color from the current theme
func SecondaryColor() lipgloss.AdaptiveColor {
	return theme.CurrentTheme().Secondary()
}

// AccentColor returns the accent color from the current theme
func AccentColor() lipgloss.AdaptiveColor {
	return theme.CurrentTheme().Accent()
}

// ErrorColor returns the error color from the current theme
func ErrorColor() lipgloss.AdaptiveColor {
	return theme.CurrentTheme().Error()
}

// WarningColor returns the warning color from the current theme
func WarningColor() lipgloss.AdaptiveColor {
	return theme.CurrentTheme().Warning()
}

// SuccessColor returns the success color from the current theme
func SuccessColor() lipgloss.AdaptiveColor {
	return theme.CurrentTheme().Success()
}

// InfoColor returns the info color from the current theme
func InfoColor() lipgloss.AdaptiveColor {
	return theme.CurrentTheme().Info()
}

// TextColor returns the text color from the current theme
func TextColor() lipgloss.AdaptiveColor {
	return theme.CurrentTheme().Text()
}

// TextMutedColor returns the muted text color from the current theme
func TextMutedColor() lipgloss.AdaptiveColor {
	return theme.CurrentTheme().TextMuted()
}

// TextEmphasizedColor returns the emphasized text color from the current theme
func TextEmphasizedColor() lipgloss.AdaptiveColor {
	return theme.CurrentTheme().TextEmphasized()
}

// BackgroundColor returns the background color from the current theme
func BackgroundColor() lipgloss.AdaptiveColor {
	return theme.CurrentTheme().Background()
}

// BackgroundSecondaryColor returns the secondary background color from the current theme
func BackgroundSecondaryColor() lipgloss.AdaptiveColor {
	return theme.CurrentTheme().BackgroundSecondary()
}

// BackgroundDarkerColor returns the darker background color from the current theme
func BackgroundDarkerColor() lipgloss.AdaptiveColor {
	return theme.CurrentTheme().BackgroundDarker()
}

// BorderNormalColor returns the normal border color from the current theme
func BorderNormalColor() lipgloss.AdaptiveColor {
	return theme.CurrentTheme().BorderNormal()
}

// BorderFocusedColor returns the focused border color from the current theme
func BorderFocusedColor() lipgloss.AdaptiveColor {
	return theme.CurrentTheme().BorderFocused()
}

// BorderDimColor returns the dim border color from the current theme
func BorderDimColor() lipgloss.AdaptiveColor {
	return theme.CurrentTheme().BorderDim()
}
