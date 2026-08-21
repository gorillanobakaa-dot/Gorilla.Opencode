// GORILLA OVERRIDE: this package did not exist upstream. It is the startup
// workspace picker — the first thing shown on an interactive launch, asking
// which folder this session should work in.
//
// Why it has to be here and not inside the TUI. The working directory is not a
// preference the program can adopt late: the context files it reads, the
// permission scope it grants, the roots it hands the language servers and the
// directories @-completion walks are all fixed from it during startup. So this
// runs as its own small Bubble Tea program BEFORE config.Load, and hands back a
// path — after which everything downstream initialises against the right root
// the first time. Choosing later would mean redoing all of it, and the LSP
// clients cannot currently be re-rooted at all.
//
// Why ask at all, rather than just remembering. The desktop entry is
// `Exec=gorilla-opencode launch` with no Path=, so an icon click — how nearly
// everyone opens the program — inherits $HOME. On a machine holding a kernel
// tree and a browser tree that puts over a million files inside the agent's
// reach before the user has typed anything. Asking once, at the point where the
// answer is cheap to give, is what Gemini CLI and Antigravity do, and it is the
// difference between a scoped session and a quota fire.
//
// This picker chooses the PRIMARY root only. Extra folders are /add-dir's job.
package startup

import (
	"fmt"
	"github.com/opencode-ai/opencode/internal/tui/styles"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/opencode-ai/opencode/internal/config"
	"github.com/opencode-ai/opencode/internal/llm/models"
)

// Choice is what the picker returns.
type Choice struct {
	// Dir is the absolute path the user settled on.
	Dir string
	// Quit is true if the user pressed ctrl+c / esc — the caller must abort the
	// launch rather than fall back to a directory the user did not pick.
	Quit bool
	// Remember is true if the user asked not to be prompted again.
	Remember bool
}

// candidate is one offered directory.
type candidate struct {
	dir   string
	label string
}

type model struct {
	input      textinput.Model
	candidates []candidate
	cursor     int // index into candidates; len(candidates) means "typing a path"
	err        string
	dontAsk    bool
	width      int
	height     int
	done       bool
	quit       bool
}

// Options configures the picker.
type Options struct {
	// LastUsed is the saved primary root, or "" if there is none.
	LastUsed string
	// Cwd is the directory the process actually started in.
	Cwd string
	// Recent are other saved roots.
	Recent []string
	// Home is the user's home directory, used to spot the $HOME case that makes
	// this prompt necessary in the first place.
	Home string
}

func newModel(o Options) *model {
	ti := textinput.New()
	ti.Placeholder = "type or paste a path, e.g. ~/Documents/my-project"
	ti.Prompt = "> "
	ti.CharLimit = 0

	m := &model{input: ti}

	seen := map[string]bool{}
	add := func(dir, label string) {
		if dir == "" || seen[dir] {
			return
		}
		seen[dir] = true
		m.candidates = append(m.candidates, candidate{dir: dir, label: label})
	}

	// Order is deliberate: the folder the user last worked in is the answer far
	// more often than the folder the launcher happened to start in.
	add(o.LastUsed, "last used")
	// The whole reason for this prompt is that an icon launch lands in $HOME.
	// Offering $HOME as a neat one-key choice would be offering the mistake, so
	// it is labelled for what it is and never preselected.
	if o.Cwd == o.Home && o.Home != "" {
		add(o.Cwd, "your home folder — everything in it comes into scope")
	} else {
		add(o.Cwd, "where this was started")
	}
	for _, d := range o.Recent {
		add(d, "also added")
	}

	// Preselect the first candidate that is not $HOME.
	for i, c := range m.candidates {
		if c.dir != o.Home {
			m.cursor = i
			break
		}
	}
	return m
}

func (m *model) Init() tea.Cmd { return textinput.Blink }

// typing reports whether the cursor is on the free-text row.
func (m *model) typing() bool { return m.cursor == len(m.candidates) }

func (m *model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.input.Width = max(20, m.contentWidth()-4)
		return m, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "esc":
			m.quit, m.done = true, true
			return m, tea.Quit

		case "up", "shift+tab":
			if m.cursor > 0 {
				m.cursor--
			}
			return m, m.syncFocus()

		case "down", "tab":
			if m.cursor < len(m.candidates) {
				m.cursor++
			}
			return m, m.syncFocus()

		case "ctrl+r":
			m.dontAsk = !m.dontAsk
			return m, nil

		case "enter":
			return m, m.accept()
		}

		// Digit shortcuts, but only while not typing — otherwise a path
		// containing a digit could not be typed.
		if !m.typing() && len(msg.String()) == 1 && msg.String()[0] >= '1' && msg.String()[0] <= '9' {
			if i := int(msg.String()[0] - '1'); i < len(m.candidates) {
				m.cursor = i
				return m, m.accept()
			}
		}

		// Any other printable key while a candidate is selected drops into the
		// text field and starts the path — so paste-and-go needs no ceremony.
		if !m.typing() && msg.Type == tea.KeyRunes {
			m.cursor = len(m.candidates)
			m.input.Focus()
			var cmd tea.Cmd
			m.input, cmd = m.input.Update(msg)
			return m, cmd
		}
	}

	if m.typing() {
		var cmd tea.Cmd
		m.input, cmd = m.input.Update(msg)
		m.err = ""
		return m, cmd
	}
	return m, nil
}

func (m *model) syncFocus() tea.Cmd {
	m.err = ""
	if m.typing() {
		return m.input.Focus()
	}
	m.input.Blur()
	return nil
}

// accept validates the current selection and quits on success.
func (m *model) accept() tea.Cmd {
	raw := ""
	if m.typing() {
		raw = strings.TrimSpace(m.input.Value())
		if raw == "" {
			m.err = "type a path, or press up to pick one of the folders above"
			return nil
		}
	} else {
		raw = m.candidates[m.cursor].dir
	}

	dir, err := ResolveDir(raw)
	if err != nil {
		m.err = err.Error()
		return nil
	}
	m.input.SetValue(dir)
	m.cursor = len(m.candidates)
	m.done = true
	return tea.Quit
}

// ResolveDir turns whatever the user typed into an absolute, existing
// directory. Exported because the caller validates a --cwd flag the same way.
func ResolveDir(raw string) (string, error) {
	dir := strings.TrimSpace(raw)
	dir = strings.Trim(dir, "\"'") // pasted paths often arrive quoted
	if dir == "" {
		return "", fmt.Errorf("no path given")
	}
	// Shell tilde expansion never happened: nothing here went through a shell.
	if dir == "~" || strings.HasPrefix(dir, "~"+string(filepath.Separator)) {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("cannot expand ~: %v", err)
		}
		dir = filepath.Join(home, strings.TrimPrefix(dir, "~"))
	}
	abs, err := filepath.Abs(dir)
	if err != nil {
		return "", fmt.Errorf("not a usable path: %v", err)
	}
	info, err := os.Stat(abs)
	if os.IsNotExist(err) {
		return "", fmt.Errorf("%s does not exist", abs)
	}
	if err != nil {
		return "", fmt.Errorf("cannot read %s: %v", abs, err)
	}
	if !info.IsDir() {
		return "", fmt.Errorf("%s is a file, not a folder", abs)
	}
	return abs, nil
}

// contentWidth is the usable width inside the border. Chrome is SUBTRACTED from
// the terminal width and never added to a content size — adding it is what made
// the input box render off-screen in v0.1.38.
func (m *model) contentWidth() int {
	const (
		// RoundedBorder is 1 column each side, Padding(1,2) is 2 each side.
		chrome   = 6
		fallback = 72
		// A floor low enough to still FIT a pathologically narrow terminal.
		// A larger minimum would overflow rather than degrade: there is no
		// content width that both reads well and fits in 20 columns, and of the
		// two failures, wrapped text beats a layout drawn off-screen.
		minimum = 16
	)
	if m.width <= 0 {
		return fallback
	}
	return max(minimum, min(fallback, m.width-chrome))
}

var (
	accent = lipgloss.AdaptiveColor{Light: "#0f766e", Dark: "#5eead4"}
	dim    = lipgloss.AdaptiveColor{Light: "#57534e", Dark: "#a8a29e"}
	warn   = lipgloss.AdaptiveColor{Light: "#b91c1c", Dark: "#fca5a5"}
)

func (m *model) View() string {
	if m.done {
		return ""
	}
	w := m.contentWidth()

	line := func(s string) string {
		return lipgloss.NewStyle().Width(w).MaxWidth(w).Render(s)
	}

	var b strings.Builder
	b.WriteString(line(lipgloss.NewStyle().Bold(true).Foreground(accent).Render("Which folder are you working in?")))
	b.WriteString("\n")
	b.WriteString(line(lipgloss.NewStyle().Foreground(dim).Render(
		"This is the only folder the AI reads and searches by default.")))
	b.WriteString("\n\n")

	for i, c := range m.candidates {
		marker, style := "  ", lipgloss.NewStyle()
		if i == m.cursor {
			marker, style = "> ", lipgloss.NewStyle().Foreground(accent).Bold(true)
		}
		row := fmt.Sprintf("%s%d. %s", marker, i+1, truncatePath(c.dir, w-6))
		b.WriteString(line(style.Render(row)))
		b.WriteString("\n")
		b.WriteString(line(lipgloss.NewStyle().Foreground(dim).Render("       " + c.label)))
		b.WriteString("\n")
	}

	b.WriteString("\n")
	marker := "  "
	if m.typing() {
		marker = "> "
	}
	b.WriteString(line(marker + "somewhere else:"))
	b.WriteString("\n")
	b.WriteString(line("    " + m.input.View()))
	b.WriteString("\n")

	if m.err != "" {
		b.WriteString(line(lipgloss.NewStyle().Foreground(warn).Render("    " + m.err)))
		b.WriteString("\n")
	}

	b.WriteString("\n")
	check := "[ ]"
	if m.dontAsk {
		check = "[x]"
	}
	b.WriteString(line(lipgloss.NewStyle().Foreground(dim).Render(
		fmt.Sprintf("ctrl+r %s don't ask again — remember this folder", check))))
	b.WriteString("\n")
	b.WriteString(line(lipgloss.NewStyle().Foreground(dim).Render(
		"enter choose  up/down move  esc quit    add more folders later with /add-dir")))

	// GORILLA OVERRIDE: tell people their model list has gone stale, but only
	// when it actually has.
	//
	// Model providers retire models constantly and a retired one does not fail
	// politely - it errors the moment you pick it. This shipped with a
	// hand-written OpenRouter list in which NINE of twenty-two models no longer
	// existed, including the default, so the provider simply did not work and
	// nothing said why.
	//
	// Shown only when the list has never been refreshed or is over a month old.
	// A banner on every launch is nagging, and nagging is noise - the thing this
	// project spends most of its effort removing. The cost is stated in seconds
	// on a slow line, not just kilobytes, because that is the unit someone
	// waiting actually feels.
	if note := staleModelsNotice(w); note != "" {
		b.WriteString("\n\n")
		// The notice is multiline and can be wider than a narrow terminal. Route
		// it through the same width-constrained renderer as every other row so
		// the border never expands beyond the terminal.
		b.WriteString(line(note))
	}

	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(accent).
		Padding(1, 2).
		Render(b.String())
}

// truncatePath shortens from the LEFT, because the tail of a path is the part
// that identifies it.
func truncatePath(p string, w int) string {
	if w < 12 || len(p) <= w {
		return p
	}
	return styles.Ellipsis + p[len(p)-(w-3):]
}

// Ask runs the picker. It returns Choice{Quit: true} if the user aborted.
func Ask(o Options) (Choice, error) {
	m := newModel(o)
	// GORILLA OVERRIDE: ask in the alternate screen, then print the answer.
	//
	// This used to draw inline, so that the answer stayed visible above the
	// session as a record of the scope that was chosen. The intent was right but
	// the mechanism leaked: bubbletea's inline renderer erases its previous frame
	// by walking the cursor up by the number of LOGICAL lines it last drew, and
	// nothing in bubbletea repaints on a resize. Narrow the window while the
	// question is on screen and those lines wrap into more physical rows than the
	// count knows about, so the cursor lands mid-frame and the next frame is
	// painted over part of the old one — one stale, half-drawn copy per resize
	// step, which is what the 2026-07-28 screencast recorded.
	//
	// Asking in the alternate screen makes resizing the terminal's problem, and
	// printing the answer afterwards keeps the record the original comment wanted
	// — a single durable line rather than a whole box.
	final, err := tea.NewProgram(m, tea.WithOutput(os.Stderr), tea.WithAltScreen()).Run()
	if err != nil {
		return Choice{}, err
	}
	fm := final.(*model)
	if fm.quit {
		return Choice{Quit: true}, nil
	}
	dir, err := ResolveDir(fm.input.Value())
	if err != nil {
		return Choice{}, err
	}
	fmt.Fprintln(os.Stderr, AnswerLine(dir, fm.dontAsk))
	return Choice{Dir: dir, Remember: fm.dontAsk}, nil
}

// AnswerLine is the durable record left above the session: which folder was
// chosen, and whether the question will be asked again. Exported so a test can
// assert the record survives, which is the whole reason the picker prints it.
func AnswerLine(dir string, remembered bool) string {
	if remembered {
		return "folder: " + dir + " (remembered — /add-dir to add more)"
	}
	return "folder: " + dir
}

// staleModelsHeadline is the shouted first line of the stale-model warning.
//
// Kept as a lone constant on purpose: it is the one string in this program that
// swears at the reader, and it should take exactly one edit to soften before the
// v1 release reaches people who did not choose the joke.
const staleModelsHeadline = "BITCH, REMEMBER TO REFRESH YOUR MODELS!"

// staleModelsNotice returns the refresh prompt, or "" when the list is current.
//
// w is the width the notice will be rendered into. It has to be passed in
// because the notice INDENTS its body, and a hand-indented line that is then
// wrapped by the caller loses the indent on every continuation — which is what
// the owner photographed on 2026-08-21: a paragraph that started six columns in
// and dropped its tail at column 0, looking like broken output. Wrapping here,
// with the indent applied after the wrap, keeps the left edge straight at any
// width. (CLAUDE.md: lipgloss fails by WRAPPING, silently, in a different place
// from the cause.)
func staleModelsNotice(w int) string {
	// GORILLA FIX (2026-08-21): ask about EVERY provider list, not OpenRouter's
	// alone. Seven other providers keep their own cache file now, so the old
	// check told a user who had just refreshed all of them that their models had
	// NEVER BEEN UPDATED.
	age, refreshed := models.NewestCatalogueAge(config.CacheBase())
	switch {
	case !refreshed:
		// Never refreshed: they are running whatever shipped with the build.
	case age < models.StaleAfter:
		return ""
	}

	// GORILLA OVERRIDE: the headline shouts. Bright red, bold, capitals, at the
	// one moment the user is definitely looking at the screen — the folder
	// picker, before any work starts. The old notice was polite grey-and-salmon
	// and got skimmed past, which meant people met a retired model mid-task
	// instead of reading a warning at startup. #FF0000 is the same alert red the
	// quota warnings use, so "something needs doing" looks the same everywhere.
	//
	// The wording lives in staleModelsHeadline, one line, deliberately easy to
	// change before this ships to strangers.
	return renderStaleNotice(w, age, refreshed)
}

// renderStaleNotice is the notice itself, separated from the question of whether
// to show it. Split so the layout can be tested at any width without depending
// on what happens to be in the cache directory of the machine running the test —
// the first version of this test skipped itself on a machine whose catalogue was
// current, which is a test that cannot fail.
func renderStaleNotice(w int, age time.Duration, refreshed bool) string {
	shout := lipgloss.NewStyle().Foreground(lipgloss.Color("#FF0000")).Bold(true)
	warn := lipgloss.NewStyle().Foreground(lipgloss.Color("203")).Bold(true)
	body := lipgloss.NewStyle().Foreground(lipgloss.Color("245"))

	const indent = "      "
	// Wrap to what is left after the indent, then indent every produced line —
	// including the ones the wrap created. Measured in DISPLAY COLUMNS, not
	// bytes: the notice carries a gorilla and an em dash, and len() would count
	// those as 4 and 3.
	para := func(style lipgloss.Style, pad, text string) string {
		wrapWidth := max(12, w-lipgloss.Width(pad))
		wrapped := lipgloss.NewStyle().Width(wrapWidth).Render(text)
		lines := strings.Split(wrapped, "\n")
		for i, l := range lines {
			lines[i] = pad + style.Render(strings.TrimRight(l, " "))
		}
		return strings.Join(lines, "\n")
	}

	when := "NEVER BEEN UPDATED"
	if refreshed {
		when = fmt.Sprintf("NOT BEEN UPDATED FOR %d DAYS", int(age.Hours()/24))
	}

	var b strings.Builder
	// The headline wraps too. It is 45 columns with its gorilla, and a 40-column
	// box made it spill — caught by the test, not by reading it.
	b.WriteString(para(shout, "  ", "🦍  "+staleModelsHeadline))
	b.WriteString("\n")
	b.WriteString(para(shout, indent, fmt.Sprintf("YOUR MODEL LIST HAS %s — IT GOES OFF WEEKLY.", when)))
	b.WriteString("\n")
	b.WriteString(para(body, indent, "Providers retire models all the time, and a retired one just errors "+
		"when you pick it. Refreshing takes about 650 KB — roughly 80 seconds even on a very "+
		"slow line, and it saves a lot of swearing later. Inside the program, type:"))
	b.WriteString("\n\n")
	// GORILLA FIX (2026-08-21): /update, not `gorilla-opencode models refresh`.
	// The CLI subcommand means quitting the session to run it, "so in practice
	// nobody did" — the exact reason /update was added on 2026-08-20. A notice
	// that asks for a quit is a notice that gets ignored.
	b.WriteString(indent + warn.Render("/update"))
	b.WriteString("\n")
	b.WriteString(para(body, indent, "No account, nothing sent about you. You're welcome.  🦍"))
	return b.String()
}
