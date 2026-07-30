package tui

import (
	"reflect"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/opencode-ai/opencode/internal/tui/components/chat"
	"github.com/opencode-ai/opencode/internal/tui/page"
)

// stubStatus stands in for the status bar. The real one reaches into the app; all
// this needs to be is a known number of rows.
type stubStatus struct{ rows int }

func (s stubStatus) Init() tea.Cmd                       { return nil }
func (s stubStatus) Update(tea.Msg) (tea.Model, tea.Cmd) { return s, nil }
func (s stubStatus) View() string {
	return strings.TrimSuffix(strings.Repeat("status\n", s.rows), "\n")
}
func (s stubStatus) SetSize(int, int) tea.Cmd { return nil }
func (s stubStatus) GetSize() (int, int)      { return 0, s.rows }

// tallPage is a page whose body is deliberately enormous — the shape of the bug
// this file guards against.
//
// It pads its lines out to cols because overlays are clamped to the width of what
// they are drawn over (a real fix: an unclamped overlay used to be handed back
// untouched and spill past the screen). A 6-column stub body would shrink the
// overlay to 6 columns and make an assertion about the overlay's content fail for
// reasons that have nothing to do with this file.
// The two views carry DIFFERENT labels and different heights on purpose. An
// earlier version of this stub returned the same thing for both, which made the
// two branches indistinguishable by either height or content — the test passed
// while proving nothing.
type tallPage struct {
	rows       int // the full-screen body
	footerRows int // what the inline frame is allowed to show
	cols       int
}

func (p tallPage) Init() tea.Cmd                       { return nil }
func (p tallPage) Update(tea.Msg) (tea.Model, tea.Cmd) { return p, nil }

func (p tallPage) block(label string, n int) string {
	line := label
	if p.cols > len(label) {
		line = label + strings.Repeat(" ", p.cols-len(label))
	}
	rows := make([]string, n)
	for i := range rows {
		rows[i] = line
	}
	return strings.Join(rows, "\n")
}

func (p tallPage) View() string { return p.block("bodyline", p.rows) }
func (p tallPage) FooterView(maxRows int) string {
	// Honours the budget the same way the real page does: shed rows rather than
	// overflow, since overflowing is the bug being guarded against.
	n := p.footerRows
	if maxRows > 0 && n > maxRows {
		n = maxRows
	}
	return p.block("footerline", n)
}

// A page that offers no footer at all — the log viewer's shape.
type plainPage struct{}

func (p plainPage) Init() tea.Cmd                       { return nil }
func (p plainPage) Update(tea.Msg) (tea.Model, tea.Cmd) { return p, nil }
func (p plainPage) View() string                        { return strings.Repeat("log line\n", 200) }

// With the alternate screen off and nothing open, the frame must be the footer —
// not the full-screen layout. Painting a window's worth of panels over text that
// has already been printed into the terminal cannot be undone, and cannot be
// erased cleanly either.
func TestFrameIsOnlyTheFooterWhenNothingIsOpen(t *testing.T) {
	a := appModel{
		width: 100, height: 30,
		scrollback:  true,
		currentPage: page.ChatPage,
		pages:       map[page.PageID]tea.Model{page.ChatPage: tallPage{rows: 24, footerRows: 2, cols: 100}},
		status:      stubStatus{rows: 1},
	}

	view := a.View()
	if n := strings.Count(view, "footerline"); n != 2 {
		t.Errorf("footer content appears %d times, want 2:\n%q", n, view)
	}
	if strings.Contains(view, "bodyline") {
		t.Error("the full-screen body leaked into the inline frame; it is 24 rows tall " +
			"and would overflow the window, breaking every later erase")
	}
	if !strings.Contains(view, "status") {
		t.Error("the status line is missing; it is the only always-visible chrome")
	}
	if rows := lipgloss.Height(view); rows != 3 {
		t.Errorf("frame is %d rows, want 3 (2 footer + 1 status). Anything larger "+
			"means the full-screen layout leaked into the inline frame", rows)
	}
}

// The same model with an overlay open must render the full layout, because the
// overlay is drawn on the alternate screen where a whole screen exists.
//
// The overlay used here is the pending sign-in URL rather than a dialog flag,
// because it renders from a plain string. Setting a show* flag would send View()
// into a dialog component that a hand-built appModel never constructed, and the
// nil panic that follows says nothing about the branch under test.
func TestFrameIsFullScreenWhileAnOverlayIsOpen(t *testing.T) {
	a := appModel{
		width: 100, height: 30,
		scrollback:  true,
		currentPage: page.ChatPage,
		pages:       map[page.PageID]tea.Model{page.ChatPage: tallPage{rows: 24, footerRows: 2, cols: 100}},
		status:      stubStatus{rows: 1},
		loginURL:    "https://example.invalid/device",
	}

	if !a.anyOverlayOpen() {
		t.Fatal("the pending sign-in URL did not register as an overlay, so this test " +
			"proves nothing")
	}
	view := a.View()
	if rows := lipgloss.Height(view); rows <= 3 {
		t.Errorf("frame is only %d rows with an overlay open; it should be rendering "+
			"the full layout, since the overlay gets the alternate screen", rows)
	}
	// Not an exact count: the overlay is PLACED OVER the body, so it covers some of
	// those rows. What matters is that the body was rendered at all — the footer-only
	// branch would have produced none of it — and that the overlay is on top.
	if !strings.Contains(view, "bodyline") {
		t.Errorf("no full-screen body in the frame, so the footer-only branch was taken "+
			"while an overlay was open; got:\n%q", view)
	}
	if !strings.Contains(view, "example.invalid") {
		t.Error("the overlay itself is not in the frame")
	}
}

// With the alternate screen ON, nothing about the frame changes — this mode must
// be entirely opt-out.
func TestAlternateScreenModeStillRendersTheFullLayout(t *testing.T) {
	a := appModel{
		width: 100, height: 30,
		scrollback:  false,
		currentPage: page.ChatPage,
		pages:       map[page.PageID]tea.Model{page.ChatPage: tallPage{rows: 24, footerRows: 2, cols: 100}},
		status:      stubStatus{rows: 1},
	}
	if rows := lipgloss.Height(a.View()); rows <= 3 {
		t.Errorf("frame is %d rows on the alternate screen; the full layout should be "+
			"drawn exactly as before", rows)
	}
}

// A page with no footer must contribute nothing but the status line. Drawing its
// whole body inline is precisely the overflow this mode avoids — the log viewer
// is 200 lines long.
func TestPageWithoutAFooterContributesOnlyTheStatusLine(t *testing.T) {
	a := appModel{
		width: 100, height: 30,
		scrollback:  true,
		currentPage: page.LogsPage,
		pages:       map[page.PageID]tea.Model{page.LogsPage: plainPage{}},
		status:      stubStatus{rows: 1},
	}

	view := a.View()
	if strings.Contains(view, "log line") {
		t.Error("the log viewer's whole body was drawn into the inline frame; it is " +
			"200 lines long and would overflow any window")
	}
	if rows := lipgloss.Height(view); rows != 1 {
		t.Errorf("frame is %d rows, want 1 (status only)", rows)
	}
}

// Opening a dialog must enter the alternate screen and closing it must leave.
// Without this the dialog paints a whole screen into a short inline frame, and
// bubbletea's erase — which counts logical lines — then lands in the wrong place
// on every subsequent redraw.
func TestOpeningAndClosingADialogSwitchesBuffers(t *testing.T) {
	base := func() appModel {
		return appModel{
			width: 100, height: 30,
			scrollback:  true,
			currentPage: page.ChatPage,
			pages:       map[page.PageID]tea.Model{page.ChatPage: tallPage{rows: 24, footerRows: 2, cols: 100}},
			status:      stubStatus{rows: 1},
			loadedPages: map[page.PageID]bool{page.ChatPage: true},
		}
	}

	// Opening: a dialog flag is now set, none was before.
	opened := base()
	opened.showQuit = true
	if got := cmdKind(opened.bufferCmd(false)); got != "enter" {
		t.Errorf("opening a dialog produced %q, want the alternate screen to be entered", got)
	}

	// Closing: no flag set now, one was before.
	if got := cmdKind(base().bufferCmd(true)); got != "exit" {
		t.Errorf("closing a dialog produced %q, want the alternate screen to be left", got)
	}

	// No change, either way: no buffer command, or buffers would be switched on
	// every keystroke — which flickers and scrolls the printed conversation away.
	if got := cmdKind(base().bufferCmd(false)); got != "none" {
		t.Errorf("no overlay change (none open) produced %q", got)
	}
	if got := cmdKind(opened.bufferCmd(true)); got != "none" {
		t.Errorf("no overlay change (still open) produced %q", got)
	}

	// And with the alternate screen already in use, buffers are never switched.
	always := base()
	always.scrollback = false
	always.showQuit = true
	if got := cmdKind(always.bufferCmd(false)); got != "none" {
		t.Errorf("on the alternate screen a dialog produced %q; it is already there", got)
	}
}

// cmdKind identifies a buffer command by comparing function identity, because
// bubbletea's alt-screen message types are unexported.
func cmdKind(cmd tea.Cmd) string {
	if cmd == nil {
		return "none"
	}
	switch reflect.ValueOf(cmd).Pointer() {
	case reflect.ValueOf(tea.EnterAltScreen).Pointer():
		return "enter"
	case reflect.ValueOf(tea.ExitAltScreen).Pointer():
		return "exit"
	}
	return "other"
}

// The footer frame must be STABLE — always the same height regardless of whether a
// reply is streaming. The old contract was "at most half the window"; the new
// contract is "exactly the reserved rows, always", because a frame that shrinks when
// streaming ends causes bubbletea's cursor-up erase to over-reach and wipe the
// scrollback.
//
// The floor is chat.FooterReservedRows (transcript block) + 1 (status). The page
// clips whatever it would show to the budget, so no single component grows past
// the reserved allocation.
func TestFooterIsGivenAHardRowBudget(t *testing.T) {
	for _, height := range []int{6, 10, 24, 40} {
		a := appModel{
			width: 100, height: height,
			scrollback:  true,
			currentPage: page.ChatPage,
			// Asks for far more rows than any budget would allow.
			pages:  map[page.PageID]tea.Model{page.ChatPage: tallPage{rows: 24, footerRows: 40, cols: 100}},
			status: stubStatus{rows: 1},
		}
		view := a.View()
		rows := lipgloss.Height(view)

		// The hard maximum: the budget is half the window OR the reserved rows
		// floor, whichever is larger, plus the status line. This is exact because
		// tallPage.FooterView clips to its budget and we add exactly 1 for status.
		budget := height / 2
		minBudget := chat.FooterReservedRows + 2
		if budget < minBudget {
			budget = minBudget
		}
		limit := budget + 1 // +1 for the status line
		if rows > limit {
			t.Errorf("height %d: frame is %d rows, limit %d. The footer overflowed its "+
				"budget, which breaks the renderer's line-erase arithmetic",
				height, rows, limit)
		}
		if !strings.Contains(view, "status") {
			t.Errorf("height %d: the status line was shed; it is not optional", height)
		}
	}
}
