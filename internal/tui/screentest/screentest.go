// Package screentest renders a component into a real terminal cell grid so tests
// can assert on what a user would actually SEE.
//
// GORILLA OVERRIDE: this is the one idea worth stealing from OpenAI's Codex, whose
// Rust TUI drives every widget through a VT100 parser and snapshots the resulting
// screen — 613 golden files across 3,234 tests. The equivalent here costs no new
// dependency: x/cellbuf is already in the module graph and is a genuine terminal
// cell buffer, so SetContent parses the ANSI a component emits and lays it out at a
// fixed width and height exactly as a terminal would.
//
// Why string assertions were not enough. Every display bug this project has
// shipped fell into one of three shapes, and only the first is reliably catchable
// by matching strings:
//
//  1. Missing text. strings.Contains finds it.
//  2. Text present but INVISIBLE, because the foreground ended up equal to the
//     background. That was the /help bug: the selected row rendered as a blank
//     line while its explanation still showed below. It took three attempts to
//     write a string-matching test for it — the first passed against the bug, the
//     second asserted on the wrong escape sequence — because the question "is this
//     visible?" is about a cell's two colours, not about the byte stream.
//  3. Text that overflows the terminal and is clipped at the edge. Counting
//     lipgloss.Width per line catches some of it, but lipgloss WRAPS rather than
//     overflows, so an over-long string shows up as extra height instead. A grid
//     with a fixed height makes both visible as the same thing: cells that did not
//     fit.
//
// The grid answers all three directly, which is why it exists.
package screentest

import (
	"image/color"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/cellbuf"
	"github.com/muesli/termenv"
)

// Force full colour output for the whole test binary.
//
// This is not a nicety: without it the package is WORSE than useless. lipgloss
// detects colour support from the output device, a test process has no terminal,
// so every Render() returns its input with all styling stripped — verified, a
// styled string came back as the literal "hello" with no escape at all. Every cell
// then reports a nil foreground and a nil background, every legibility check
// passes, and a suite full of colour assertions goes green while the bug it was
// written for sails through.
//
// Setting it in init() rather than asking callers to remember is deliberate. The
// failure mode of forgetting is a silent false pass, which is the one outcome a
// test harness must never have.
func init() {
	lipgloss.SetColorProfile(termenv.TrueColor)
}

// Screen is a rendered component laid out at a fixed size.
type Screen struct {
	buf           *cellbuf.Buffer
	width, height int
	// rendered is the number of rows the view actually wanted, which can exceed
	// height — that IS the overflow, and it is lost once the grid is filled.
	rendered int
}

// Render lays a view out at the given terminal size.
//
// height is what the terminal has, NOT what the component asked for. A component
// wanting more rows than that is the bug being hunted, so the difference is
// recorded rather than hidden by growing the grid.
func Render(view string, width, height int) *Screen {
	rows := strings.Split(strings.TrimSuffix(view, "\n"), "\n")

	buf := cellbuf.NewBuffer(width, height)
	cellbuf.SetContent(buf, view)

	return &Screen{buf: buf, width: width, height: height, rendered: len(rows)}
}

// Size reports the grid the component was laid out in.
func (s *Screen) Size() (width, height int) { return s.width, s.height }

// RowsWanted is how many rows the view emitted. Greater than the terminal height
// means content was pushed off the screen.
func (s *Screen) RowsWanted() int { return s.rendered }

// Overflows reports whether the view asked for more rows than the terminal has.
//
// This is the failure the /help dialog shipped: a fixed chrome guess rendered 15
// lines into a 10-line terminal. The user sees a truncated dialog with no
// indication that anything is missing.
func (s *Screen) Overflows() bool { return s.rendered > s.height }

// Text returns the visible characters of one row, trailing blanks trimmed.
func (s *Screen) Text(y int) string {
	var b strings.Builder
	for x := 0; x < s.width; x++ {
		c := s.buf.Cell(x, y)
		if c == nil {
			b.WriteByte(' ')
			continue
		}
		// A double-width grapheme occupies two cells: the rune in the first, and a
		// zero-width placeholder in the second. Emitting a space for the
		// placeholder split "日本" into "日 本", so any assertion about a CJK or
		// emoji label was quietly wrong. Skip placeholders; they are not columns of
		// their own.
		if c.Width == 0 {
			continue
		}
		if c.Rune == 0 {
			b.WriteByte(' ')
			continue
		}
		b.WriteRune(c.Rune)
	}
	return strings.TrimRight(b.String(), " ")
}

// Lines returns every row's visible text.
func (s *Screen) Lines() []string {
	out := make([]string, s.height)
	for y := range out {
		out[y] = s.Text(y)
	}
	return out
}

// String is the whole grid, for a failure message or a golden file.
func (s *Screen) String() string { return strings.Join(s.Lines(), "\n") }

// Contains reports whether the text appears anywhere on the grid.
//
// Unlike strings.Contains over a raw view, this only sees what SURVIVED layout —
// a match here means the text is genuinely on screen, not merely in the byte
// stream that was clipped before it got there.
func (s *Screen) Contains(text string) bool {
	return strings.Contains(s.String(), text)
}

// Legible reports whether a cell's character can actually be read: it has content,
// and its foreground differs from its background.
//
// The heart of this package. A cell holding 'x' in grey on grey contains an 'x'
// that nobody can see, and no amount of string matching will tell you.
//
// A nil colour means "the terminal's default", so nil-vs-nil is legible (default
// text on default background) while an explicit colour equal to its own background
// is not.
func (s *Screen) Legible(x, y int) bool {
	c := s.buf.Cell(x, y)
	if c == nil || c.Rune == 0 || c.Rune == ' ' {
		return false // nothing to read
	}
	fg, bg := c.Style.Fg, c.Style.Bg
	if fg == nil && bg == nil {
		return true // both default: readable by definition
	}
	if fg == nil || bg == nil {
		return true // one default, one set — cannot collide
	}
	fr, fgn, fb, fa := fg.RGBA()
	br, bgn, bb, ba := bg.RGBA()
	return fr != br || fgn != bgn || fb != bb || fa != ba
}

// RowLegible reports whether a row has any readable character at all.
//
// Use it on a row that MUST be visible — a selected list item, a heading. A row of
// text whose foreground matches its background returns false, which is exactly the
// /help defect: present in the output, invisible on the screen.
func (s *Screen) RowLegible(y int) bool {
	for x := 0; x < s.width; x++ {
		if s.Legible(x, y) {
			return true
		}
	}
	return false
}

// FindRow returns the index of the first row containing text, or -1.
func (s *Screen) FindRow(text string) int {
	for y := 0; y < s.height; y++ {
		if strings.Contains(s.Text(y), text) {
			return y
		}
	}
	return -1
}

// WidestRow returns the greatest number of occupied columns on any row, and which
// row it was. A value equal to the grid width on a bordered dialog usually means
// the border was clipped rather than fitted.
func (s *Screen) WidestRow() (cols, row int) {
	for y := 0; y < s.height; y++ {
		if n := len([]rune(s.Text(y))); n > cols {
			cols, row = n, y
		}
	}
	return cols, row
}

// BackgroundBreak reports the first column on row y whose background differs from
// column 0's, or -1 if the whole row shares one background.
//
// This catches the defect that a width assertion cannot see: a row that is the
// right length but painted in patches, because some spans were styled and the text
// between them was not. On screen that reads as black rectangles punched through a
// coloured bar — present, correctly sized, and visibly unfinished.
//
// Outside the alternate screen it matters more than it looks. There the terminal's
// own background shows through anything unpainted, so an unstyled separator is not
// a subtle shade difference; it is a hole.
func (s *Screen) BackgroundBreak(y int) int {
	if y < 0 || y >= s.height {
		return -1
	}
	first := s.buf.Cell(0, y)
	if first == nil {
		return -1
	}
	want := first.Style.Bg
	for x := 1; x < s.width; x++ {
		c := s.buf.Cell(x, y)
		if c == nil || c.Width == 0 {
			continue // continuation cell of a wide rune: carries no style of its own
		}
		if !sameColour(c.Style.Bg, want) {
			return x
		}
	}
	return -1
}

// UniformBackground reports whether every row shares a single background colour,
// naming the first row and column that does not.
func (s *Screen) UniformBackground() (ok bool, row, col int) {
	for y := 0; y < s.height; y++ {
		if x := s.BackgroundBreak(y); x >= 0 {
			return false, y, x
		}
	}
	return true, -1, -1
}

func sameColour(a, b color.Color) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	ar, ag, ab, aa := a.RGBA()
	br, bg, bb, ba := b.RGBA()
	return ar == br && ag == bg && ab == bb && aa == ba
}
