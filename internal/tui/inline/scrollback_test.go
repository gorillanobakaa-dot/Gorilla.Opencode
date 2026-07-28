package inline

import (
	"bytes"
	"fmt"
	"io"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// The whole inline/split-footer architecture rests on one claim: outside the
// alternate screen, tea.Println emits a line ONCE into the terminal's real
// output stream, while View() keeps repainting a small footer in place. If that
// claim is false, "history lives in the terminal's scrollback" is false too, and
// no amount of scrollbar or clipboard work would substitute for it.
//
// A common misreading is that dropping tea.WithAltScreen() alone turns the view
// into a growing log. It does not: the inline renderer repaints the same region
// (cursor up linesRendered-1, redraw). These tests pin down which half does what.

type probe struct {
	footer  string
	emit    []string
	emitted bool
}

func (m probe) Init() tea.Cmd {
	return func() tea.Msg { return time.Time{} }
}

func (m probe) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg.(type) {
	case time.Time:
		if m.emitted {
			return m, tea.Quit
		}
		m.emitted = true
		lines := make([]tea.Cmd, 0, len(m.emit)+1)
		for _, l := range m.emit {
			lines = append(lines, tea.Println(l))
		}
		lines = append(lines, func() tea.Msg { return time.Time{} })
		return m, tea.Sequence(lines...)
	}
	return m, nil
}

func (m probe) View() string { return m.footer }

// run drives a program to completion against an in-memory output, with no TTY,
// and returns every byte the program wrote.
func run(t *testing.T, opts ...tea.ProgramOption) string {
	t.Helper()
	history := []string{"HIST-alpha", "HIST-beta", "HIST-gamma"}
	var out bytes.Buffer
	in, _ := io.Pipe() // never written to: no input, so only our timer drives it

	base := []tea.ProgramOption{tea.WithInput(in), tea.WithOutput(&out)}
	p := tea.NewProgram(
		probe{footer: "FOOTER-LINE-1\nFOOTER-LINE-2", emit: history},
		append(base, opts...)...,
	)
	done := make(chan error, 1)
	go func() { _, err := p.Run(); done <- err }()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("program failed: %v", err)
		}
	case <-time.After(5 * time.Second):
		p.Kill()
		t.Fatal("program did not finish")
	}
	return out.String()
}

func TestPrintlnReachesTheOutputStreamInlineOnce(t *testing.T) {
	got := run(t)

	for _, want := range []string{"HIST-alpha", "HIST-beta", "HIST-gamma"} {
		if n := strings.Count(got, want); n != 1 {
			t.Errorf("history line %q appears %d times in the output; want exactly 1 "+
				"(more than once means it is part of the repainted frame, not scrollback)", want, n)
		}
	}
	// The footer is a frame: it is repainted, so it may appear more than once.
	// What matters is that it appears at all, i.e. the footer really renders.
	if !strings.Contains(got, "FOOTER-LINE-1") {
		t.Error("footer never rendered")
	}
}

// The counterpart measurement: under the alternate screen the same call is
// dropped on the floor. This is why the current interface cannot be scrolled or
// selected, and it is the fact that makes the architecture change necessary
// rather than merely tidier.
func TestPrintlnIsSwallowedUnderAltScreen(t *testing.T) {
	got := run(t, tea.WithAltScreen())

	// Non-vacuity first: absence of the history lines only means something if the
	// program actually ran and rendered. Without this the test would pass against
	// an empty buffer, which is the easiest way to fake this result.
	if !strings.Contains(got, "FOOTER-LINE-1") {
		t.Fatal("program produced no frame under the alternate screen; the absence " +
			"check below would be vacuous")
	}
	if !strings.Contains(got, "\x1b[?1049h") {
		t.Fatal("alternate screen was never entered; this test would be measuring " +
			"inline mode by accident")
	}

	for _, want := range []string{"HIST-alpha", "HIST-beta", "HIST-gamma"} {
		if strings.Contains(got, want) {
			t.Errorf("history line %q reached the output under the alternate screen; "+
				"the premise of this test file is that it cannot", want)
		}
	}
}

// A frame that grows without bound is the failure mode to avoid: the inline
// renderer erases its previous frame by walking the cursor up by the number of
// LOGICAL lines it last drew, so a frame taller than the window scrolls content
// away and the arithmetic stops matching what is on screen. Keeping the footer
// small is a correctness requirement, not a style preference.
func TestFooterMustStaySmallerThanTheWindow(t *testing.T) {
	const windowRows = 24
	tall := strings.Repeat("x\n", windowRows+5)

	var out bytes.Buffer
	in, _ := io.Pipe()
	p := tea.NewProgram(
		probe{footer: tall, emit: []string{"HIST-alpha"}},
		tea.WithInput(in), tea.WithOutput(&out),
	)
	done := make(chan error, 1)
	go func() { _, err := p.Run(); done <- err }()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		p.Kill()
		t.Fatal("program did not finish")
	}

	// Documented, not asserted as desirable: this is what the renderer does when
	// the frame is oversized. The test exists so the number is visible to whoever
	// designs the footer.
	fmt.Printf("  oversized footer (%d rows) wrote %d bytes of frame traffic\n",
		strings.Count(tall, "\n"), out.Len())
}
