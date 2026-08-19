package startup

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func extrasFixture() []ExtraRow {
	return []ExtraRow{
		{ID: "extras-reasoning-generate", Name: "Ask the model to think out loud",
			What: "the model works through the problem step by step", Costs: true, On: false},
		{ID: "extras-reasoning-show", Name: "Show that thinking on screen",
			What: "the reasoning appears in the conversation", On: true},
		{ID: "extras-toolcalls-show", Name: "Show tool calls and their results",
			What: "you see each command the agent runs, and what came back", On: true},
	}
}

func newExtras(w int) *extrasModel {
	return &extrasModel{width: w, rows: extrasFixture()}
}

// The whole reason this screen exists: the setting that spends more must be
// marked, unmistakably, next to the thing it applies to.
func TestTheCostlyRowIsMarkedAndTheFreeOnesSaySoToo(t *testing.T) {
	v := newExtras(80).View()

	if !strings.Contains(v, "COSTS EXTRA") {
		t.Error("the row that makes the model generate more is not marked as costing anything")
	}
	if strings.Count(v, "COSTS EXTRA") != 2 {
		// Once on the row, once as the heading of the explanation.
		t.Errorf("expected the marker on the one costly row plus its explanation heading, found %d", strings.Count(v, "COSTS EXTRA"))
	}
	// And the free ones must be labelled free. A user told "extras cost money"
	// would otherwise assume all of them do and switch off the free ones,
	// losing information for no saving whatsoever.
	if n := strings.Count(v, "(free)"); n != 2 {
		t.Errorf("expected 2 rows labelled free, found %d — an unlabelled free row reads as a costly one", n)
	}
}

// The explanation must describe a real resource cost and must NOT invent a price.
// No rate is published for the models configured here, and a NIM free tier or a
// local Ollama bills no money at all — a warning that is provably false on the
// user's own machine teaches them to ignore the ones that are true.
func TestTheCostExplanationNamesNoPrice(t *testing.T) {
	v := newExtras(80).View()

	for _, forbidden := range []string{"$", "USD", "cents", "per month", "€"} {
		if strings.Contains(v, forbidden) {
			t.Errorf("the explanation quotes a price (%q) that we have no data for", forbidden)
		}
	}
	// It must still be concrete about what is actually consumed.
	for _, want := range []string{"more CPU", "allowance", "longer"} {
		if !strings.Contains(v, want) {
			t.Errorf("the explanation does not mention %q, so the real cost is not conveyed", want)
		}
	}
	// And it must be honest about the absence of a figure.
	if !strings.Contains(v, "No figure is shown") {
		t.Error("the screen does not explain why no price is given, which reads as an oversight")
	}
}

// Space toggles the highlighted row and nothing else.
func TestSpaceTogglesOnlyTheSelectedRow(t *testing.T) {
	m := newExtras(80)
	before := []bool{m.rows[0].On, m.rows[1].On, m.rows[2].On}

	m.Update(tea.KeyMsg{Type: tea.KeySpace, Runes: []rune(" ")})

	if m.rows[0].On == before[0] {
		t.Error("space did not toggle the selected row")
	}
	if m.rows[1].On != before[1] || m.rows[2].On != before[2] {
		t.Error("space changed a row that was not selected")
	}
}

// Escape must not be read as consent. Recording a decision nobody made is
// exactly what this screen exists to prevent.
func TestEscapeIsNotTakenAsAnAnswer(t *testing.T) {
	m := newExtras(80)
	m.Update(tea.KeyMsg{Type: tea.KeyEsc})

	if !m.quit {
		t.Fatal("esc did not register as quitting")
	}
	if m.accepted {
		t.Error("esc was recorded as acceptance — the caller would persist a choice the user never made")
	}
}

// Enter accepts, and the answers come back.
func TestEnterAcceptsAndReturnsTheRows(t *testing.T) {
	m := newExtras(80)
	m.Update(tea.KeyMsg{Type: tea.KeySpace, Runes: []rune(" ")}) // turn reasoning on
	m.Update(tea.KeyMsg{Type: tea.KeyEnter})

	if !m.accepted || m.quit {
		t.Fatalf("enter did not accept (accepted=%v quit=%v)", m.accepted, m.quit)
	}
	if !m.rows[0].On {
		t.Error("the toggled choice was not carried through to the result")
	}
}

// Navigation must not run off either end of the list.
func TestNavigationStaysInBounds(t *testing.T) {
	m := newExtras(80)
	for i := 0; i < 10; i++ {
		m.Update(tea.KeyMsg{Type: tea.KeyUp})
	}
	if m.sel != 0 {
		t.Errorf("selection went above the first row: %d", m.sel)
	}
	for i := 0; i < 10; i++ {
		m.Update(tea.KeyMsg{Type: tea.KeyDown})
	}
	if m.sel != len(m.rows)-1 {
		t.Errorf("selection went past the last row: %d of %d", m.sel, len(m.rows))
	}
}

// No line may exceed the terminal width, or the explanation is clipped at the
// screen edge — and this screen is only useful if it can be read.
func TestNoLineExceedsTheTerminalWidth(t *testing.T) {
	for _, w := range []int{40, 60, 80, 100, 120} {
		m := newExtras(w)
		for i, l := range strings.Split(m.View(), "\n") {
			// Compare rune counts: the styling is ANSI and does not occupy cells.
			if n := len([]rune(stripANSI(l))); n > w {
				t.Errorf("width=%d: line %d is %d columns:\n%q", w, i, n, l)
			}
		}
	}
}

// The bullet list must stay a bullet list. strings.Fields discards leading
// whitespace, and an earlier version silently flattened "  * a provider…" to
// "* a provider…", so the cost breakdown stopped reading as a list.
func TestBulletIndentSurvivesWrapping(t *testing.T) {
	const bullet = "  * a free tier such as NVIDIA NIM — no money, but your allowance runs down faster and you may start hitting request limits"

	lines := wrapTo(bullet, 60)
	if len(lines) < 2 {
		t.Fatalf("expected the long bullet to wrap, got %d line(s)", len(lines))
	}
	if !strings.HasPrefix(lines[0], "  * ") {
		t.Errorf("the bullet marker and its indent were lost: %q", lines[0])
	}
	// Continuations align under the text, not under the bullet character.
	for i, l := range lines[1:] {
		if !strings.HasPrefix(l, "    ") {
			t.Errorf("continuation %d is not indented under the text: %q", i, l)
		}
		if strings.Contains(l, "*") {
			t.Errorf("continuation %d repeated the bullet: %q", i, l)
		}
	}
}

func TestWrapToBasics(t *testing.T) {
	if got := wrapTo("", 20); len(got) != 1 {
		t.Errorf("empty input produced %d lines", len(got))
	}
	if got := wrapTo("short", 40); len(got) != 1 || got[0] != "short" {
		t.Errorf("short input wrapped: %q", got)
	}
	// A word longer than the width cannot be broken, but must not be dropped.
	long := strings.Repeat("x", 50)
	joined := strings.Join(wrapTo(long, 10), "")
	if !strings.Contains(joined, long) {
		t.Error("an over-long word was lost rather than overflowing")
	}
}

// stripANSI removes escape sequences so widths can be measured in cells.
func stripANSI(s string) string {
	var b strings.Builder
	inEsc := false
	for _, r := range s {
		switch {
		case r == 0x1b:
			inEsc = true
		case inEsc && (r == 'm' || r == 'K'):
			inEsc = false
		case !inEsc:
			b.WriteRune(r)
		}
	}
	return b.String()
}
