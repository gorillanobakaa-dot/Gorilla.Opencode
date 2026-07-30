package inline

import (
	"strings"
	"unicode/utf8"
)

// A minimal terminal emulator, enough to replay what bubbletea's inline renderer
// actually writes — including SCROLLING, which is the whole point.
//
// WHY THIS EXISTS. Every previous check of the scrollback behaviour ended with a
// human looking at a screenshot. That made each regression cost another session
// to re-find, and it is why the same class of bug has been "fixed" repeatedly
// and kept coming back. Asserting on the raw byte stream is not enough either:
// the bug is not which bytes are emitted, it is where the cursor ends up after
// the terminal has scrolled. Only a model that scrolls can answer that.
//
// The sequence set is not guessed. It is the set the renderer was measured
// emitting: CUU/CUD/CUF/CUB, CHA, EL, ED, CUP, plus \r and \n. SGR colours and
// DEC private modes (?25l, ?2004h, mouse) are parsed and ignored — they move
// nothing. Anything unrecognised is skipped rather than printed, so a future
// bubbletea that emits something new degrades to a missing effect rather than to
// garbage cells that would look like a bug in the program under test.
type term struct {
	w, h       int
	screen     [][]rune // exactly h rows of w cells
	scrollback []string // rows that scrolled off the top
	cx, cy     int
}

func newTerm(w, h int) *term {
	t := &term{w: w, h: h}
	t.screen = make([][]rune, h)
	for i := range t.screen {
		t.screen[i] = blankRow(w)
	}
	return t
}

func blankRow(w int) []rune {
	r := make([]rune, w)
	for i := range r {
		r[i] = ' '
	}
	return r
}

// scroll moves the top row into scrollback, exactly as a real terminal does when
// the cursor is driven below the last line.
func (t *term) scroll() {
	t.scrollback = append(t.scrollback, strings.TrimRight(string(t.screen[0]), " "))
	copy(t.screen, t.screen[1:])
	t.screen[t.h-1] = blankRow(t.w)
}

func (t *term) clampAfterMove() {
	if t.cy < 0 {
		t.cy = 0
	}
	if t.cx < 0 {
		t.cx = 0
	}
	if t.cx >= t.w {
		t.cx = t.w - 1
	}
	for t.cy >= t.h {
		t.scroll()
		t.cy--
	}
}

func (t *term) put(r rune) {
	t.clampAfterMove()
	t.screen[t.cy][t.cx] = r
	t.cx++
	if t.cx >= t.w {
		// Line wrap: a real terminal moves to the next row.
		t.cx = 0
		t.cy++
		t.clampAfterMove()
	}
}

// Write replays a byte stream onto the emulated screen.
func (t *term) Write(s string) {
	for i := 0; i < len(s); {
		c := s[i]
		switch {
		case c == '\n':
			t.cy++
			t.clampAfterMove()
			i++
		case c == '\r':
			t.cx = 0
			i++
		case c == '\b':
			t.cx--
			t.clampAfterMove()
			i++
		case c == 0x1b:
			i += t.escape(s[i:])
		default:
			r, size := utf8.DecodeRuneInString(s[i:])
			if r == utf8.RuneError && size <= 1 {
				i++
				continue
			}
			t.put(r)
			i += size
		}
	}
}

// escape consumes one escape sequence and returns how many bytes it used.
func (t *term) escape(s string) int {
	if len(s) < 2 {
		return len(s)
	}
	switch s[1] {
	case '[':
		// CSI: parameters, then a final byte in @..~
		j := 2
		for j < len(s) && (s[j] >= '0' && s[j] <= '9' || s[j] == ';' || s[j] == '?') {
			j++
		}
		if j >= len(s) {
			return len(s)
		}
		params, final := s[2:j], s[j]
		t.csi(params, final)
		return j + 1
	case ']':
		// OSC: runs to BEL or ST. Nothing we model.
		if k := strings.IndexByte(s, 0x07); k >= 0 {
			return k + 1
		}
		return len(s)
	default:
		return 2
	}
}

func (t *term) csi(params string, final byte) {
	if strings.HasPrefix(params, "?") {
		return // DEC private mode: cursor visibility, mouse, bracketed paste
	}
	n := 1
	if params != "" {
		n = 0
		for _, c := range params {
			if c < '0' || c > '9' {
				break
			}
			n = n*10 + int(c-'0')
		}
		if n == 0 && params != "0" {
			n = 1
		}
	}
	switch final {
	case 'A':
		t.cy -= n
		t.clampAfterMove()
	case 'B':
		t.cy += n
		t.clampAfterMove()
	case 'C':
		t.cx += n
		t.clampAfterMove()
	case 'D':
		t.cx -= n
		t.clampAfterMove()
	case 'G':
		t.cx = n - 1
		t.clampAfterMove()
	case 'H', 'f':
		row, col := 1, 1
		if parts := strings.SplitN(params, ";", 2); params != "" {
			row = atoiDefault(parts[0], 1)
			if len(parts) > 1 {
				col = atoiDefault(parts[1], 1)
			}
		}
		t.cy, t.cx = row-1, col-1
		t.clampAfterMove()
	case 'K': // erase in line
		t.clampAfterMove()
		switch params {
		case "", "0":
			for x := t.cx; x < t.w; x++ {
				t.screen[t.cy][x] = ' '
			}
		case "1":
			for x := 0; x <= t.cx && x < t.w; x++ {
				t.screen[t.cy][x] = ' '
			}
		case "2":
			t.screen[t.cy] = blankRow(t.w)
		}
	case 'J': // erase in display
		t.clampAfterMove()
		switch params {
		case "", "0":
			for x := t.cx; x < t.w; x++ {
				t.screen[t.cy][x] = ' '
			}
			for y := t.cy + 1; y < t.h; y++ {
				t.screen[y] = blankRow(t.w)
			}
		case "2":
			for y := 0; y < t.h; y++ {
				t.screen[y] = blankRow(t.w)
			}
		}
	case 'S': // scroll up
		for i := 0; i < n; i++ {
			t.scroll()
		}
	case 'm', 'h', 'l', 'r', 's', 'u':
		// SGR and modes: no cell movement.
	}
}

func atoiDefault(s string, def int) int {
	if s == "" {
		return def
	}
	n := 0
	for _, c := range s {
		if c < '0' || c > '9' {
			return def
		}
		n = n*10 + int(c-'0')
	}
	if n == 0 {
		return def
	}
	return n
}

// Screen returns the visible rows, right-trimmed.
func (t *term) Screen() []string {
	out := make([]string, t.h)
	for i, row := range t.screen {
		out[i] = strings.TrimRight(string(row), " ")
	}
	return out
}

// All returns scrollback followed by the visible screen — everything the user
// could reach by scrolling up.
func (t *term) All() []string {
	return append(append([]string{}, t.scrollback...), t.Screen()...)
}

// CountOnScreen reports how many visible rows contain text.
func (t *term) CountOnScreen(text string) int {
	n := 0
	for _, row := range t.Screen() {
		if strings.Contains(row, text) {
			n++
		}
	}
	return n
}

// CountEverywhere includes scrollback.
func (t *term) CountEverywhere(text string) int {
	n := 0
	for _, row := range t.All() {
		if strings.Contains(row, text) {
			n++
		}
	}
	return n
}

// LastNonEmpty is the index of the lowest visible row with any text, or -1.
func (t *term) LastNonEmpty() int {
	rows := t.Screen()
	for i := len(rows) - 1; i >= 0; i-- {
		if strings.TrimSpace(rows[i]) != "" {
			return i
		}
	}
	return -1
}
