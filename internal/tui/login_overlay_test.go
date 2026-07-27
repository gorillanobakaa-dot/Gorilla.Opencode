package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// A real Google OAuth URL, which is the length that broke the first version.
const testAuthURL = "https://accounts.google.com/o/oauth2/v2/auth?access_type=offline&client_id=681255809395-oo8ft2oprdrnp9e3aqf6av3hmdib135j.apps.googleusercontent.com&prompt=consent&redirect_uri=http%3A%2F%2F127.0.0.1%3A41234%2Foauth2callback&response_type=code&scope=https%3A%2F%2Fwww.googleapis.com%2Fauth%2Fcloud-platform&state=eca13fe48822ac57ead4787779e8de35815093beaf9399160d6ac432e4250f6d"

// The reported bug: "escape does not get rid of the message". The clear was
// written into the keys.Quit branch, so esc never reached it.
func TestEscapeDismissesTheLoginOverlay(t *testing.T) {
	a := &appModel{width: 100, height: 34, loginURL: testAuthURL}

	if !a.tryDismissLoginOverlay(tea.KeyMsg{Type: tea.KeyEsc}) {
		t.Fatal("esc was not handled while the overlay was up")
	}
	if a.loginURL != "" {
		t.Errorf("overlay still showing after esc: %q", a.loginURL)
	}
}

// It must not swallow keys it has no business consuming — the overlay is not
// modal, and the user carries on typing while sign-in completes.
func TestLoginOverlayOnlyConsumesEscape(t *testing.T) {
	a := &appModel{width: 100, height: 34, loginURL: testAuthURL}

	for _, msg := range []tea.Msg{
		tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("h")},
		tea.KeyMsg{Type: tea.KeyEnter},
		tea.KeyMsg{Type: tea.KeyUp},
		tea.WindowSizeMsg{Width: 80, Height: 24},
	} {
		if a.tryDismissLoginOverlay(msg) {
			t.Errorf("consumed %T %v, which belongs to the editor or a dialog", msg, msg)
		}
	}
	if a.loginURL == "" {
		t.Error("the overlay was cleared by a key that is not esc")
	}
}

// With no overlay up, esc must fall through to whatever else wants it.
func TestEscapeIgnoredWhenNoOverlay(t *testing.T) {
	a := &appModel{width: 100, height: 34}
	if a.tryDismissLoginOverlay(tea.KeyMsg{Type: tea.KeyEsc}) {
		t.Error("claimed esc with no overlay showing, stealing it from the page")
	}
}

// The black bars in the reported screenshot. lipgloss does not pad the short
// lines of a multi-line render, so any line narrower than the widest one leaves
// unpainted cells that show as terminal black. Every line must be identical
// width — that is what makes the box a rectangle.
func TestLoginOverlayLinesAreUniformWidth(t *testing.T) {
	for _, w := range []int{40, 60, 80, 100, 120, 200} {
		a := &appModel{width: w, height: 40, loginURL: testAuthURL}

		lines := strings.Split(a.loginURLOverlay(), "\n")
		if len(lines) < 3 {
			t.Fatalf("width=%d: overlay rendered only %d lines", w, len(lines))
		}
		want := lipgloss.Width(lines[0])
		for i, l := range lines {
			if got := lipgloss.Width(l); got != want {
				t.Errorf("width=%d: line %d is %d columns, first line is %d — the difference renders as a black bar:\n%q",
					w, i, got, want, l)
			}
		}
	}
}

// And the box must fit the terminal, or it is clipped at the screen edge — which
// is what made the unwrapped URL unreadable as well as unpastable.
func TestLoginOverlayFitsTerminalWidth(t *testing.T) {
	for _, w := range []int{40, 60, 80, 100, 120, 200} {
		a := &appModel{width: w, height: 40, loginURL: testAuthURL}
		for i, l := range strings.Split(a.loginURLOverlay(), "\n") {
			if got := lipgloss.Width(l); got > w {
				t.Errorf("width=%d: line %d is %d columns wide", w, i, got)
			}
		}
	}
}

// The whole URL has to survive. Wrapping is acceptable; losing characters is not
// — a truncated auth URL is worthless.
func TestLoginOverlayPreservesTheWholeURL(t *testing.T) {
	a := &appModel{width: 80, height: 40, loginURL: testAuthURL}

	// Strip the border/padding and rejoin: the URL's characters must all be
	// present, in order, once the box furniture is removed.
	var joined strings.Builder
	for _, l := range strings.Split(a.loginURLOverlay(), "\n") {
		clean := strings.Map(func(r rune) rune {
			if strings.ContainsRune("╭╮╰╯│─", r) {
				return -1
			}
			return r
		}, l)
		joined.WriteString(strings.TrimSpace(clean))
	}
	flat := joined.String()

	if !strings.Contains(flat, testAuthURL) {
		t.Errorf("the URL is not recoverable from the rendered overlay.\nWanted:\n%s\nGot:\n%s", testAuthURL, flat)
	}
}

// A short terminal must shed prose, never URL characters.
func TestLoginOverlayShedsProseNotURLOnShortTerminals(t *testing.T) {
	tall := &appModel{width: 80, height: 40, loginURL: testAuthURL}
	short := &appModel{width: 80, height: 12, loginURL: testAuthURL}

	tallView, shortView := tall.loginURLOverlay(), short.loginURLOverlay()
	if lipgloss.Height(shortView) >= lipgloss.Height(tallView) {
		t.Errorf("short terminal did not shed anything: %d vs %d lines",
			lipgloss.Height(shortView), lipgloss.Height(tallView))
	}
	// The title survives, so the box is still identifiable.
	if !strings.Contains(shortView, "Sign in with Google") {
		t.Error("the title was shed, leaving an unlabelled box")
	}
	// Count URL characters present in each; the short one must not have fewer.
	countURLChars := func(v string) int {
		n := 0
		for _, chunk := range hardWrap(testAuthURL, 20) {
			if strings.Contains(strings.ReplaceAll(v, "\n", ""), chunk) {
				n += len(chunk)
			}
		}
		return n
	}
	if countURLChars(shortView) < countURLChars(tallView) {
		t.Error("URL characters were dropped to fit the height; a partial auth URL is worthless")
	}
}

func TestHardWrap(t *testing.T) {
	if got := hardWrap("abcdefghij", 4); len(got) != 3 || got[0] != "abcd" || got[2] != "ij" {
		t.Errorf("hardWrap = %q, want [abcd efgh ij]", got)
	}
	if got := hardWrap("abc", 10); len(got) != 1 || got[0] != "abc" {
		t.Errorf("short input should be one chunk, got %q", got)
	}
	if got := hardWrap("", 10); got != nil {
		t.Errorf("empty input should wrap to nothing, got %q", got)
	}
	// Must break mid-token: a URL is one word, so a word-wrapper would leave it
	// on a single over-wide line, which is the original bug.
	if got := hardWrap(testAuthURL, 40); len(got) < 5 {
		t.Errorf("a %d-char URL wrapped to only %d lines of 40", len(testAuthURL), len(got))
	}
	for i, chunk := range hardWrap(testAuthURL, 40) {
		if n := len([]rune(chunk)); n > 40 {
			t.Errorf("chunk %d is %d runes, over the 40 limit", i, n)
		}
	}
}
