package startup

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

func tree(t *testing.T) (home, project, other string) {
	t.Helper()
	home = t.TempDir()
	project = filepath.Join(home, "Documents", "my-project")
	other = filepath.Join(home, "Documents", "other")
	for _, d := range []string{project, other} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	return home, project, other
}

func send(m *model, msgs ...tea.Msg) {
	for _, msg := range msgs {
		next, _ := m.Update(msg)
		*m = *next.(*model)
	}
}

func key(s string) tea.Msg {
	switch s {
	case "up":
		return tea.KeyMsg{Type: tea.KeyUp}
	case "down":
		return tea.KeyMsg{Type: tea.KeyDown}
	case "enter":
		return tea.KeyMsg{Type: tea.KeyEnter}
	case "esc":
		return tea.KeyMsg{Type: tea.KeyEsc}
	case "ctrl+r":
		return tea.KeyMsg{Type: tea.KeyCtrlR}
	}
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)}
}

// The failure this class of test exists to catch: a dialog wider than the
// terminal. v0.1.38 shipped an invisible input box because chrome was added to
// a content size instead of subtracted from the terminal size, and four
// dialogs shipped 82 columns wide in an 80-column terminal. Assert the real
// rendered geometry, per line, at sizes that actually hurt.
func TestPickerNeverExceedsTerminalWidth(t *testing.T) {
	home, project, other := tree(t)

	for _, width := range []int{40, 60, 80, 100, 120, 200} {
		m := newModel(Options{LastUsed: project, Cwd: home, Recent: []string{other}, Home: home})
		send(m, tea.WindowSizeMsg{Width: width, Height: 30})

		for i, l := range strings.Split(m.View(), "\n") {
			if got := lipgloss.Width(l); got > width {
				t.Errorf("width=%d: line %d renders %d columns wide — it would wrap and break the layout:\n%q",
					width, i, got, l)
			}
		}
	}
}

// A tiny terminal must still produce something legible rather than collapsing.
func TestPickerSurvivesTinyTerminal(t *testing.T) {
	home, project, _ := tree(t)
	m := newModel(Options{LastUsed: project, Cwd: home, Home: home})
	send(m, tea.WindowSizeMsg{Width: 24, Height: 8})

	view := m.View()
	if strings.TrimSpace(view) == "" {
		t.Fatal("rendered nothing at 24 columns")
	}
	// 24 is above the 16-column floor, so it must fit exactly, not merely
	// degrade: the content width is derived by subtracting chrome from the
	// terminal, so getting this wrong means chrome was added instead.
	for _, l := range strings.Split(view, "\n") {
		if got := lipgloss.Width(l); got > 24 {
			t.Errorf("line is %d columns in a 24-column terminal: %q", got, l)
		}
	}
}

// $HOME is the entire reason this prompt exists — on this machine it holds a
// kernel tree and a browser tree, over a million files. It must never be the
// preselected answer, or the icon-launch default is exactly the mistake again.
func TestHomeIsNeverPreselected(t *testing.T) {
	home, project, _ := tree(t)

	m := newModel(Options{LastUsed: project, Cwd: home, Home: home})
	if got := m.candidates[m.cursor].dir; got == home {
		t.Errorf("cursor starts on $HOME (%s); enter would scope the session to everything", got)
	}
	if got := m.candidates[m.cursor].dir; got != project {
		t.Errorf("cursor = %q, want the last-used folder %q", got, project)
	}

	// It is still OFFERED — a user with a small home directory may want it —
	// but labelled with the consequence.
	var homeLabel string
	for _, c := range m.candidates {
		if c.dir == home {
			homeLabel = c.label
		}
	}
	if homeLabel == "" {
		t.Fatal("$HOME was not offered at all; a deliberate choice must remain possible")
	}
	if !strings.Contains(homeLabel, "scope") {
		t.Errorf("$HOME is offered as %q without saying what it costs", homeLabel)
	}
}

// With nothing saved, the only candidate is the cwd, and it must be selected —
// a fresh install must not open with no answer available.
func TestFreshInstallSelectsCwd(t *testing.T) {
	_, project, _ := tree(t)
	m := newModel(Options{Cwd: project, Home: "/nonexistent-home"})
	if len(m.candidates) != 1 {
		t.Fatalf("candidates = %v, want just the cwd", m.candidates)
	}
	if m.candidates[m.cursor].dir != project {
		t.Errorf("cursor = %q, want %q", m.candidates[m.cursor].dir, project)
	}
}

func TestEnterAcceptsSelectedCandidate(t *testing.T) {
	home, project, _ := tree(t)
	m := newModel(Options{LastUsed: project, Cwd: home, Home: home})
	send(m, tea.WindowSizeMsg{Width: 80, Height: 30}, key("enter"))

	if !m.done || m.quit {
		t.Fatalf("enter did not accept: done=%v quit=%v", m.done, m.quit)
	}
	if m.input.Value() != project {
		t.Errorf("accepted %q, want %q", m.input.Value(), project)
	}
}

// Typing must reach the text field without a mode switch: the common case is a
// pasted path, and requiring tab first would swallow its first character.
func TestTypingDropsStraightIntoTheField(t *testing.T) {
	home, project, _ := tree(t)
	m := newModel(Options{LastUsed: project, Cwd: home, Home: home})
	send(m, tea.WindowSizeMsg{Width: 80, Height: 30}, key("/"), key("t"), key("m"), key("p"))

	if !m.typing() {
		t.Fatal("typing did not move focus to the path field")
	}
	if got := m.input.Value(); got != "/tmp" {
		t.Errorf("field holds %q, want %q — the first keystroke was swallowed", got, "/tmp")
	}
}

// flattened collapses all whitespace in a rendered view so an assertion about a
// PHRASE cannot be broken by where the renderer happened to wrap.
//
// GORILLA FIX (2026-08-05): TestBadPathIsRefusedWithAReason failed
// intermittently with "the error is not rendered". It was not a timing flake and
// not an app bug. The error line is 84 columns against a content width of 72,
// and lipgloss Width() WRAPS rather than truncates (a documented trap in
// CLAUDE.md), so the message is shown in full across two rows. Whether the
// literal substring "does not exist" survives depends on which space the wrap
// falls on — and the path contains t.TempDir()'s random digits, so its length,
// and therefore the wrap point, changes between runs.
func flattened(view string) string {
	// Strip the panel border before collapsing whitespace: the wrapped remainder
	// of a long line begins on the next row, and the border glyph would otherwise
	// sit between the two halves of the phrase ("... does not │ │ exist ...").
	cleaned := strings.Map(func(r rune) rune {
		switch r {
		case '│', '─', '╭', '╮', '╰', '╯':
			return ' '
		}
		return r
	}, view)
	return strings.Join(strings.Fields(cleaned), " ")
}

// A path that does not exist must be refused in place, with the reason on
// screen. Accepting it would put the session in a directory that is not there.
func TestBadPathIsRefusedWithAReason(t *testing.T) {
	home, project, _ := tree(t)
	m := newModel(Options{LastUsed: project, Cwd: home, Home: home})
	send(m, tea.WindowSizeMsg{Width: 80, Height: 30}, key("down"), key("down"))
	m.cursor = len(m.candidates)
	m.input.SetValue(filepath.Join(home, "no-such-folder"))
	send(m, key("enter"))

	if m.done {
		t.Fatal("accepted a directory that does not exist")
	}
	if !strings.Contains(m.err, "does not exist") {
		t.Errorf("error = %q, want it to say the path is missing", m.err)
	}
	if !strings.Contains(flattened(m.View()), "does not exist") {
		t.Errorf("the error is not rendered, so the user sees a dead enter key:\n%s", m.View())
	}
}

func TestEscQuitsWithoutChoosing(t *testing.T) {
	home, project, _ := tree(t)
	m := newModel(Options{LastUsed: project, Cwd: home, Home: home})
	send(m, key("esc"))
	if !m.quit {
		t.Error("esc did not signal quit; the caller would launch in an unpicked folder")
	}
}

func TestResolveDir(t *testing.T) {
	home, project, _ := tree(t)
	// GORILLA OVERRIDE (2026-09-01): os.UserHomeDir reads USERPROFILE on Windows
	// and HOME everywhere else. Setting only HOME meant the tilde expanded to the
	// real profile on Windows, so this test could not have passed there whatever
	// the code did.
	setHome(t, home)

	file := filepath.Join(project, "a.txt")
	if err := os.WriteFile(file, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	if got, err := ResolveDir("  \"" + project + "\"  "); err != nil || got != project {
		t.Errorf("quoted+padded paste: got %q, %v — pasted paths arrive like this", got, err)
	}
	if got, err := ResolveDir("~/Documents/my-project"); err != nil || got != project {
		t.Errorf("tilde: got %q, %v — nothing expanded it, no shell was involved", got, err)
	}
	if _, err := ResolveDir(file); err == nil {
		t.Error("accepted a file as a working folder")
	}
	if _, err := ResolveDir(""); err == nil {
		t.Error("accepted an empty path")
	}
}

// The opt-out must be reachable, since the user's standing requirement is that
// anything added can be turned off.
func TestDontAskAgainToggles(t *testing.T) {
	home, project, _ := tree(t)
	m := newModel(Options{LastUsed: project, Cwd: home, Home: home})
	send(m, tea.WindowSizeMsg{Width: 80, Height: 30})

	if !strings.Contains(m.View(), "don't ask again") {
		t.Error("the opt-out is not shown")
	}
	send(m, key("ctrl+r"))
	if !m.dontAsk {
		t.Fatal("ctrl+r did not set the flag")
	}
	if !strings.Contains(m.View(), "[x]") {
		t.Error("the checkbox does not reflect the toggle")
	}
}

// setHome redirects os.UserHomeDir for the duration of a test, on any platform.
func setHome(t *testing.T, dir string) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Setenv("USERPROFILE", dir)
		return
	}
	t.Setenv("HOME", dir)
}

// GORILLA OVERRIDE (2026-09-01): `~/` and `~\` must both expand.
//
// The check used filepath.Separator, which is `\` on Windows, so only `~\`
// matched there. `~/` is what people actually type - it is the universal
// convention and what every piece of documentation shows - and a Windows user
// typing `~/Documents/my-project` got "does not exist" naming a path with a
// literal tilde in it, with no hint that the tilde was the problem.
func TestTildeExpandsWithEitherSeparator(t *testing.T) {
	home, project, _ := tree(t)
	setHome(t, home)

	for _, form := range []string{"~/Documents/my-project", `~\Documents\my-project`} {
		got, err := ResolveDir(form)
		if err != nil {
			t.Errorf("ResolveDir(%q) failed: %v", form, err)
			continue
		}
		if got != project {
			t.Errorf("ResolveDir(%q) = %q, want %q", form, got, project)
		}
	}
}

// A bare "~" is the home directory itself.
func TestBareTildeIsHome(t *testing.T) {
	home, _, _ := tree(t)
	setHome(t, home)
	got, err := ResolveDir("~")
	if err != nil {
		t.Fatalf("ResolveDir(\"~\") failed: %v", err)
	}
	if got != home {
		t.Errorf("ResolveDir(\"~\") = %q, want %q", got, home)
	}
}
