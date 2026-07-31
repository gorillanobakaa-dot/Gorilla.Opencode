package inline

import (
	"bytes"
	"fmt"
	"io"
	"strings"
	"sync"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// THE SCROLL BOUNDARY.
//
// This is the test that decides whether bubbletea can do what gemini-cli and
// Claude Code do, and it had never been written. The existing tests in this
// package prove tea.Println reaches the stream once, and that the footer must
// stay smaller than the window. Neither exercises the moment that actually
// breaks: printing MORE output than the terminal is tall, so the terminal
// scrolls underneath a footer the renderer is still repainting in place.
//
// Everything reported by the user — the footer marching down the screen, then
// jumping back up, then oscillating — happens at or after that moment. Judging
// it from screenshots is what made every previous regression cost another
// session to re-find.

// scroller prints `total` lines in batches, redrawing a footer between batches,
// which is what a streaming reply does.
type scroller struct {
	footerRows int
	total      int
	batch      int
	sent       int
	tick       int
}

func (m scroller) Init() tea.Cmd {
	return tea.Tick(time.Millisecond, func(time.Time) tea.Msg { return tick{} })
}

type tick struct{}

func (m scroller) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if _, ok := msg.(tick); !ok {
		return m, nil
	}
	if m.sent >= m.total {
		// Deliberately does NOT quit: the test snapshots the live screen, and a
		// teardown sequence in the stream would erase the frame being measured.
		return m, tea.Tick(5*time.Millisecond, func(time.Time) tea.Msg { return tick{} })
	}
	cmds := make([]tea.Cmd, 0, m.batch+1)
	for i := 0; i < m.batch && m.sent < m.total; i++ {
		cmds = append(cmds, tea.Println(fmt.Sprintf("LINE-%03d", m.sent)))
		m.sent++
	}
	m.tick++
	cmds = append(cmds, tea.Tick(time.Millisecond, func(time.Time) tea.Msg { return tick{} }))
	return m, tea.Sequence(cmds...)
}

// A FIXED-height footer, which is the contract bubbletea's inline renderer
// depends on. The last row is uniquely identifiable so the test can find it.
func (m scroller) View() string {
	rows := make([]string, 0, m.footerRows)
	for i := 0; i < m.footerRows-1; i++ {
		rows = append(rows, fmt.Sprintf("FOOTER-ROW-%d", i))
	}
	rows = append(rows, "FOOTER-PROMPT>")
	return strings.Join(rows, "\n")
}

// safeBuf lets the test read what has been written so far while the program is
// still running.
type safeBuf struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (s *safeBuf) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.buf.Write(p)
}

func (s *safeBuf) String() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.buf.String()
}

// runScroller returns the screen as it stands WHILE the program is running.
//
// Snapshotting mid-run rather than after exit is deliberate. On shutdown
// bubbletea tears the frame down — the first version of this test measured after
// Run() returned and reported the footer's last row missing, which was the
// teardown, not a defect. What matters is what the user is looking at during a
// streaming reply, so the snapshot is taken with the program still live.
func runScroller(t *testing.T, m scroller, cols, rows int) *term {
	t.Helper()
	out := &safeBuf{}
	in, _ := io.Pipe()
	p := tea.NewProgram(m,
		tea.WithInput(in),
		tea.WithOutput(out),
		tea.WithoutSignalHandler(),
	)
	go func() {
		time.Sleep(2 * time.Millisecond)
		p.Send(tea.WindowSizeMsg{Width: cols, Height: rows})
	}()

	done := make(chan error, 1)
	go func() { _, err := p.Run(); done <- err }()

	// Wait until every line has been printed and the renderer has settled.
	deadline := time.Now().Add(15 * time.Second)
	var snapshot string
	for {
		if time.Now().After(deadline) {
			p.Kill()
			t.Fatal("program did not print everything in time")
		}
		s := out.String()
		if strings.Contains(s, fmt.Sprintf("LINE-%03d", m.total-1)) {
			// Let the renderer draw at least one more frame after the last print.
			time.Sleep(60 * time.Millisecond)
			snapshot = out.String()
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	p.Kill()
	<-done

	tm := newTerm(cols, rows)
	tm.Write(snapshot)
	return tm
}

func dump(t *testing.T, tm *term) {
	t.Helper()
	t.Logf("--- visible screen (%d rows) ---", len(tm.Screen()))
	for i, r := range tm.Screen() {
		t.Logf("%2d |%s|", i, r)
	}
	t.Logf("--- scrollback: %d rows ---", len(tm.scrollback))
}

// The footer must appear EXACTLY ONCE on the visible screen after the terminal
// has scrolled. More than once means the renderer left a copy behind — that is
// the "it jumps up and there are two of them" symptom.
func TestFooterAppearsExactlyOnceAfterScrolling(t *testing.T) {
	const cols, rows = 80, 20
	tm := runScroller(t, scroller{footerRows: 3, total: 60, batch: 4}, cols, rows)

	got := tm.CountOnScreen("FOOTER-PROMPT>")
	if got != 1 {
		dump(t, tm)
		t.Fatalf("footer prompt appears %d times on a %d-row screen after printing "+
			"60 lines; want exactly 1", got, rows)
	}
}

// The footer must be the LAST thing on screen. If printed output appears below
// it, the renderer and the terminal disagree about where the frame is — which is
// what leaves a band of blank space and makes the footer appear to jump.
func TestNothingIsPrintedBelowTheFooter(t *testing.T) {
	const cols, rows = 80, 20
	tm := runScroller(t, scroller{footerRows: 3, total: 60, batch: 4}, cols, rows)

	screen := tm.Screen()
	footerRow := -1
	for i, r := range screen {
		if strings.Contains(r, "FOOTER-PROMPT>") {
			footerRow = i
		}
	}
	if footerRow < 0 {
		dump(t, tm)
		t.Fatal("footer is not on screen at all after scrolling")
	}
	for i := footerRow + 1; i < len(screen); i++ {
		if strings.Contains(screen[i], "LINE-") {
			dump(t, tm)
			t.Fatalf("printed output on row %d, BELOW the footer on row %d", i, footerRow)
		}
	}
}

// No footer fragment may be stranded in the printed output. This is the symptom
// the user sees as "it jumps up and leaves bits behind": debris the renderer's
// erase failed to reach.
func TestNoFooterDebrisIsStrandedInTheOutput(t *testing.T) {
	const cols, rows = 80, 20
	tm := runScroller(t, scroller{footerRows: 3, total: 60, batch: 4}, cols, rows)

	screen := tm.Screen()
	footerTop := -1
	for i, r := range screen {
		if strings.Contains(r, "FOOTER-PROMPT>") {
			footerTop = i
		}
	}
	// Everything ABOVE the contiguous footer block must be printed output only.
	for i := 0; i < footerTop-2; i++ {
		if strings.Contains(screen[i], "FOOTER-") {
			dump(t, tm)
			t.Fatalf("orphaned footer fragment on row %d, far above the footer at row %d",
				i, footerTop)
		}
	}
	for _, r := range tm.scrollback {
		if strings.Contains(r, "FOOTER-") {
			dump(t, tm)
			t.Fatalf("footer fragment leaked into SCROLLBACK: %q", r)
		}
	}
}

// Not one printed line may be lost or duplicated. This is the property that
// makes the terminal the transcript: if a line can vanish, scrollback is not a
// record of the conversation.
func TestEveryPrintedLineSurvivesExactlyOnce(t *testing.T) {
	const cols, rows = 80, 20
	const total = 60
	tm := runScroller(t, scroller{footerRows: 3, total: total, batch: 4}, cols, rows)

	var missing, duplicated []string
	for i := 0; i < total; i++ {
		want := fmt.Sprintf("LINE-%03d", i)
		switch n := tm.CountEverywhere(want); {
		case n == 0:
			missing = append(missing, want)
		case n > 1:
			duplicated = append(duplicated, fmt.Sprintf("%s x%d", want, n))
		}
	}
	if len(missing) > 0 || len(duplicated) > 0 {
		dump(t, tm)
		t.Fatalf("scrollback is not a faithful record.\n  missing: %v\n  duplicated: %v",
			missing, duplicated)
	}
}

// The same, at the size of a real window and a real reply.
func TestHoldsUpOverALongReplyOnASmallWindow(t *testing.T) {
	const cols, rows = 100, 12
	tm := runScroller(t, scroller{footerRows: 4, total: 200, batch: 7}, cols, rows)

	if got := tm.CountOnScreen("FOOTER-PROMPT>"); got != 1 {
		dump(t, tm)
		t.Fatalf("footer appears %d times after 200 lines on a %d-row window; want 1",
			got, rows)
	}
	for i := 0; i < 200; i++ {
		want := fmt.Sprintf("LINE-%03d", i)
		if n := tm.CountEverywhere(want); n != 1 {
			dump(t, tm)
			t.Fatalf("%s appears %d times across scrollback+screen; want exactly 1", want, n)
		}
	}
}

// THE ACTUAL ROOT CAUSE, pinned.
//
// A footer line WIDER than the terminal wraps to two physical rows, but
// bubbletea counts LOGICAL lines when deciding how far up to move the cursor to
// erase its last frame. It therefore under-reaches by one row per wrapped line,
// every cycle, stranding footer fragments in the output and letting the frame
// drift — which is what the user sees as the footer marching down the screen and
// jumping back up, leaving bits behind.
//
// This is the same shape as the lipgloss trap already in CLAUDE.md: Width(w)
// WRAPS rather than truncating, so an over-long string shows up as extra HEIGHT.
// Here that extra height is invisible to the renderer's arithmetic.
//
// The test asserts the FAILURE, deliberately. It documents a constraint we do
// not control, and it is what makes the debris check above non-vacuous: without
// it, nothing proves those assertions can fail at all.
type wideFooter struct{ scroller }

func (m wideFooter) Init() tea.Cmd { return m.scroller.Init() }
func (m wideFooter) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	inner, cmd := m.scroller.Update(msg)
	m.scroller = inner.(scroller)
	return m, cmd
}
func (m wideFooter) View() string {
	rows := []string{}
	for i := 0; i < m.footerRows-1; i++ {
		rows = append(rows, fmt.Sprintf("FOOTER-ROW-%d", i))
	}
	rows = append(rows, "FOOTER-PROMPT>"+strings.Repeat("x", 100))
	return strings.Join(rows, "\n")
}

func TestAnOverWideFooterLineStrandsDebris(t *testing.T) {
	const cols, rows = 80, 20
	const total = 60

	out := &safeBuf{}
	in, _ := io.Pipe()
	p := tea.NewProgram(wideFooter{scroller{footerRows: 4, total: total, batch: 4}},
		tea.WithInput(in), tea.WithOutput(out), tea.WithoutSignalHandler())
	go func() {
		time.Sleep(2 * time.Millisecond)
		p.Send(tea.WindowSizeMsg{Width: cols, Height: rows})
	}()
	done := make(chan error, 1)
	go func() { _, err := p.Run(); done <- err }()

	deadline := time.Now().Add(15 * time.Second)
	var snap string
	for time.Now().Before(deadline) {
		if strings.Contains(out.String(), fmt.Sprintf("LINE-%03d", total-1)) {
			time.Sleep(60 * time.Millisecond)
			snap = out.String()
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	p.Kill()
	<-done

	tm := newTerm(cols, rows)
	tm.Write(snap)

	screen := tm.Screen()
	footerTop := -1
	for i, r := range screen {
		if strings.Contains(r, "FOOTER-PROMPT>") {
			footerTop = i
		}
	}
	debris := 0
	for i := 0; i < footerTop-3 && i < len(screen); i++ {
		if strings.Contains(screen[i], "FOOTER-") {
			debris++
		}
	}
	if debris == 0 {
		dump(t, tm)
		t.Fatal("an over-wide footer line stranded NO debris — either bubbletea now " +
			"measures wrapped width, or the debris assertions above are vacuous. " +
			"Check before trusting them.")
	}
	t.Logf("confirmed: %d orphaned footer fragment(s) from one over-wide line", debris)
}

// THE BIG COLLAPSE.
//
// The earlier oscillation test moved the footer between 3 and 4 rows and found
// nothing wrong, which is what made me conclude height changes were harmless.
// That conclusion was drawn from too small a change.
//
// The real editor grows to maxEditorHeight (20 rows) while a long prompt is
// typed and collapses to 1 row the moment it is sent. That is a 19-row swing,
// and it happens exactly when a reply starts printing — i.e. at the scroll
// boundary. Reported 2026-07-30 after pasting a 30-line list.
type collapser struct {
	scroller
	tall bool
}

func (m collapser) Init() tea.Cmd { return m.scroller.Init() }

func (m collapser) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	inner, cmd := m.scroller.Update(msg)
	m.scroller = inner.(scroller)
	// Tall while nothing has been sent; collapses once output starts flowing,
	// which is the real sequence: type a long prompt, press enter, replies print.
	m.tall = m.sent < m.batch
	return m, cmd
}

func (m collapser) View() string {
	rows := []string{}
	if m.tall {
		for i := 0; i < 20; i++ {
			rows = append(rows, fmt.Sprintf("EDITOR-LINE-%02d", i))
		}
	}
	rows = append(rows, "FOOTER-PROMPT>")
	return strings.Join(rows, "\n")
}

func TestATallEditorCollapsingDoesNotCorruptTheScreen(t *testing.T) {
	const cols, rows = 80, 24
	const total = 80

	out := &safeBuf{}
	in, _ := io.Pipe()
	p := tea.NewProgram(collapser{scroller: scroller{footerRows: 1, total: total, batch: 6}},
		tea.WithInput(in), tea.WithOutput(out), tea.WithoutSignalHandler())
	go func() {
		time.Sleep(2 * time.Millisecond)
		p.Send(tea.WindowSizeMsg{Width: cols, Height: rows})
	}()
	done := make(chan error, 1)
	go func() { _, err := p.Run(); done <- err }()

	deadline := time.Now().Add(15 * time.Second)
	var snap string
	for time.Now().Before(deadline) {
		if strings.Contains(out.String(), fmt.Sprintf("LINE-%03d", total-1)) {
			time.Sleep(80 * time.Millisecond)
			snap = out.String()
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	p.Kill()
	<-done

	tm := newTerm(cols, rows)
	tm.Write(snap)

	// 1. Exactly one prompt on screen.
	if n := tm.CountOnScreen("FOOTER-PROMPT>"); n != 1 {
		dump(t, tm)
		t.Errorf("prompt appears %d times after a 20-row footer collapsed; want 1", n)
	}
	// 2. No editor rows stranded in the transcript.
	strays := 0
	for _, r := range tm.Screen() {
		if strings.Contains(r, "EDITOR-LINE-") {
			strays++
		}
	}
	if strays > 0 {
		dump(t, tm)
		t.Errorf("%d stranded EDITOR row(s) left behind by the collapse", strays)
	}
	// 3. Not one printed line lost or duplicated.
	for i := 0; i < total; i++ {
		want := fmt.Sprintf("LINE-%03d", i)
		if n := tm.CountEverywhere(want); n != 1 {
			dump(t, tm)
			t.Fatalf("%s appears %d times; want exactly 1", want, n)
		}
	}
}
