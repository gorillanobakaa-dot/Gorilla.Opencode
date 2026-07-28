package layout

import "github.com/charmbracelet/lipgloss"

// FitHeight renders a component with successively smaller row budgets until the
// result actually fits the height available, and returns what fitted.
//
// GORILLA OVERRIDE: this replaces a pattern that has now produced the same bug
// twice. Components were computing their capacity arithmetically —
//
//	const commandHelpFixedLines = 2 + 2 + 3 // border, padding, title, subtitle, blank
//	budget := m.height - commandHelpFixedLines - len(detailBlock)
//
// — which is a GUESS at how much space the frame takes. Every such constant is a
// standing bet that nobody will add a line to the header, change the border, or
// alter the padding. The bet was already lost: /help shipped rendering 15 rows into
// a 10-row terminal, was "fixed" by measuring the detail block, and a cell-grid test
// then found it still wanting 13 rows in an 8-row terminal, because the constant
// itself was wrong and a minimum-rows floor silently overrode the height limit.
//
// Measuring removes the class. The renderer is asked for a view at some row count,
// the result is measured, and if it does not fit the count comes down. Whatever the
// chrome happens to be — border, padding, headers, footers, a wrapped explanation —
// it is counted because it is rendered, not because someone predicted it.
//
// render must produce a view using AT MOST rows content rows. It is called at least
// once and at most maxRows-minRows+1 times; on a terminal that cannot fit even
// minRows the smallest view is returned, because showing a clipped dialog beats
// showing none.
func FitHeight(available, maxRows, minRows int, render func(rows int) string) (view string, rows int) {
	if minRows < 1 {
		minRows = 1
	}
	if maxRows < minRows {
		maxRows = minRows
	}

	rows = maxRows
	for {
		view = render(rows)
		// available <= 0 means the size is not known yet: render at full size and
		// let the caller resize when a WindowSizeMsg arrives. Clamping to a guess
		// here is what produced invisible dialogs before.
		if available <= 0 || lipgloss.Height(view) <= available || rows <= minRows {
			return view, rows
		}
		// Come down by the overshoot rather than one row at a time, so a badly
		// oversized first attempt converges immediately instead of re-rendering
		// once per excess line.
		over := lipgloss.Height(view) - available
		rows -= max(1, over)
		if rows < minRows {
			rows = minRows
		}
	}
}

// Fitter remembers the row count that last fitted, so a component that is
// re-rendered without changing size does not pay for the search again.
//
// GORILLA OVERRIDE: FitHeight alone made dialogs 2.4x to 6.5x more expensive to
// render — /context went from 1.02ms to 6.64ms per View() — because Bubble Tea calls
// View() on every keystroke AND every streamed token, while FitHeight re-ran its
// whole search each time. Measured, not assumed: 3 internal renders per View() for
// /context, 2 for /help.
//
// The cache is verify-then-reuse rather than trust-the-cache. The remembered count
// is rendered once and CHECKED; only if it no longer fits is the full search run.
// So content that changed shape between frames — a longer explanation, a scroll
// note appearing — cannot slip through and overflow. A change of terminal size or
// item count invalidates it outright, because a bigger terminal should be allowed to
// show more rows and a cache that only ever shrinks would never notice.
type Fitter struct {
	rows     int
	forAvail int
	forMax   int
	forKey   uint64
	valid    bool
}

// Fit renders at the remembered row count when the inputs are unchanged and it still
// fits, and searches otherwise.
//
// key must capture everything OTHER than size that changes the rendered height —
// typically the selection, because a per-selection explanation block varies in
// length. Getting this wrong is not a performance bug but a correctness one: a first
// version keyed on size alone, and because the check only asked "does the remembered
// count still fit" and never "could more fit now", one tight selection locked in a
// small list for the rest of the session and two commands became UNREACHABLE while
// scrolling. Caught by an existing reachability test.
func (f *Fitter) Fit(available, maxRows, minRows int, key uint64, render func(rows int) string) string {
	if f.valid && f.forAvail == available && f.forMax == maxRows && f.forKey == key {
		if view := render(f.rows); available <= 0 || lipgloss.Height(view) <= available {
			return view
		}
	}
	view, rows := FitHeight(available, maxRows, minRows, render)
	f.rows, f.forAvail, f.forMax, f.forKey, f.valid = rows, available, maxRows, key, true
	return view
}
