// Version: 1.0.0 · updated 26-10-23-02-46
package inline

import (
	"io"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/opencode-ai/opencode/internal/app"
	"github.com/opencode-ai/opencode/internal/tui"
)

// THE MACBOOK AIR SIZES (2026-10-23, macOS 10.13 Terminal.app, gallery
// docs/screenshots/gallery/macos-air-2026-10/air-2026-10-23-*.png).
//
// Unlike the rest of this package, this drives the REAL app model, not a
// stand-in, through a real bubbletea program, and replays the bytes on the
// emulator. Before the fix, at both sizes, every keystroke left the previous
// input row on screen (" > h", " > he", " > hel"...), the status bar was
// pushed off the bottom, and the banner started at column 41 instead of 0:
// the owner's screenshots, reproduced. The cause was the frame being drawn up
// to and including the last column (see usableWidth in tui.go).

// airShell runs the app without its Init, which needs a loaded config and
// database. Everything under test (sizing, the footer, the banner, the editor)
// happens in Update and View.
type airShell struct{ m tea.Model }

func (s airShell) Init() tea.Cmd { return nil }
func (s airShell) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	s.m, cmd = s.m.Update(msg)
	return s, cmd
}
func (s airShell) View() string { return s.m.View() }

// typeIntoTheApp starts the app at cols x rows, types "hello" one key at a
// time with a redraw between keys, and returns the emulated screen.
func typeIntoTheApp(t *testing.T, cols, rows int) *term {
	t.Helper()
	out := &safeBuf{}
	in, _ := io.Pipe()
	p := tea.NewProgram(airShell{tui.New(&app.App{})},
		tea.WithInput(in), tea.WithOutput(out), tea.WithoutSignalHandler())
	done := make(chan error, 1)
	go func() { _, err := p.Run(); done <- err }()

	time.Sleep(20 * time.Millisecond)
	p.Send(tea.WindowSizeMsg{Width: cols, Height: rows})
	waitSettled(out, "Gorilla OpenCode", 60*time.Millisecond, 5*time.Second)
	for _, r := range "hello" {
		p.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
		// Slower than the renderer's ~16ms tick, so every key gets its own
		// frame and its own erase of the frame before it.
		time.Sleep(40 * time.Millisecond)
	}
	snapshot := waitSettled(out, "hello", 80*time.Millisecond, 5*time.Second)
	p.Kill()
	<-done

	tm := newTerm(cols, rows)
	tm.Write(snapshot)
	return tm
}

var airSizes = [][2]int{{167, 39}, {169, 33}}

func TestTheAirSizesNeverWriteTheLastColumn(t *testing.T) {
	for _, sz := range airSizes {
		tm := typeIntoTheApp(t, sz[0], sz[1])
		if tm.lastColumnWrites != 0 {
			dump(t, tm)
			t.Errorf("%dx%d: %d characters were written into the last column; on Terminal.app each one moves the next redraw",
				sz[0], sz[1], tm.lastColumnWrites)
		}
	}
}

// Defect 1 and 8: the input area is exactly one prompt row, with the text
// typed so far, and no earlier copy of it is left behind.
func TestTheAirSizesShowOnePromptRow(t *testing.T) {
	for _, sz := range airSizes {
		tm := typeIntoTheApp(t, sz[0], sz[1])
		prompts := 0
		for _, row := range tm.Screen() {
			if strings.HasPrefix(strings.TrimLeft(row, " "), ">") {
				prompts++
			}
		}
		if prompts != 1 || tm.CountOnScreen("> hello") != 1 {
			dump(t, tm)
			t.Errorf("%dx%d: %d prompt rows on screen (%d with the typed text), want exactly 1",
				sz[0], sz[1], prompts, tm.CountOnScreen("> hello"))
		}
	}
}

// Defect 10 and 7: the status bar is on screen, once, as the last row drawn.
func TestTheAirSizesKeepTheStatusBarUnderThePrompt(t *testing.T) {
	for _, sz := range airSizes {
		tm := typeIntoTheApp(t, sz[0], sz[1])
		rows := tm.Screen()
		last := tm.LastNonEmpty()
		if tm.CountOnScreen("? help") != 1 || last < 0 || !strings.Contains(rows[last], "? help") {
			dump(t, tm)
			t.Errorf("%dx%d: status bar on screen %d times; want once, as the last row",
				sz[0], sz[1], tm.CountOnScreen("? help"))
		}
	}
}

// Defect 3: the banner printed above the frame starts at column 0, so its
// first letter is not stranded at the end of the row above ("orilla OpenCode").
func TestTheAirSizesPrintTheBannerFromColumnZero(t *testing.T) {
	for _, sz := range airSizes {
		tm := typeIntoTheApp(t, sz[0], sz[1])
		found := false
		for _, row := range tm.All() {
			if strings.Contains(row, "Gorilla OpenCode") {
				found = true
				if !strings.HasPrefix(row, "Gorilla OpenCode") {
					dump(t, tm)
					t.Errorf("%dx%d: banner row does not start at column 0: %q", sz[0], sz[1], row)
				}
			}
		}
		if !found {
			dump(t, tm)
			t.Fatalf("%dx%d: the banner was never printed; nothing here tests it", sz[0], sz[1])
		}
	}
}
