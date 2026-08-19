// GORILLA OVERRIDE: this file did not exist upstream. It is the /add-dir dialog —
// add, remove and promote workspace roots.
//
// What adding a root does NOT do: grant access. There is no sandbox in this
// codebase; the file tools accept absolute paths anywhere and only consult the
// working directory to resolve RELATIVE paths and to choose a permission scope.
// The dialog says so on screen, because "add a directory" reads like an unlock
// and describing it that way would be a false claim.
//
// What it actually changes, per root:
//  1. that root's CLAUDE.md / opencode.md / .cursorrules load into the prompt
//  2. permissions scope to the root, so one "allow for session" covers it
//  3. the env block tells the model the root exists
//  4. LSP watches it, so edits there produce diagnostics
package dialog

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/opencode-ai/opencode/internal/config"
	"github.com/opencode-ai/opencode/internal/tui/layout"
	"github.com/opencode-ai/opencode/internal/tui/styles"
	"github.com/opencode-ai/opencode/internal/tui/theme"
	"github.com/opencode-ai/opencode/internal/tui/util"
)

// CloseAddDirDialogMsg closes the dialog.
type CloseAddDirDialogMsg struct{}

// RootsChangedMsg is emitted after a root is added, removed or promoted. The
// TUI turns it into a context-cache invalidation plus a provider rebuild, so the
// change reaches the model on the next turn rather than at next launch.
type RootsChangedMsg struct {
	Info string
	// PrimaryChanged is set when /cd repointed the primary root, which needs
	// more teardown than adding an extra: the persistent bash shell holds its
	// own cwd and must be respawned.
	PrimaryChanged bool
}

type AddDirDialog interface {
	tea.Model
	layout.Bindings
}

type addDirMode int

const (
	addDirModeList addDirMode = iota
	addDirModeForm
)

const addDirDialogWidth = 76

type addDirKeyMap struct {
	Up, Down, Add, Remove, Promote, Escape, Submit, Cancel, Narrow key.Binding
}

// Arrow keys only for navigation — `-`/`+`/`[`/`]` are awkward or hidden on
// non-US layouts (the JP-keyboard lesson from loadout.go).
var addDirKeys = addDirKeyMap{
	Up:      key.NewBinding(key.WithKeys("up", "k"), key.WithHelp("up", "up")),
	Down:    key.NewBinding(key.WithKeys("down", "j"), key.WithHelp("down", "down")),
	Add:     key.NewBinding(key.WithKeys("a"), key.WithHelp("a", "add a root")),
	Remove:  key.NewBinding(key.WithKeys("d"), key.WithHelp("d", "remove")),
	Promote: key.NewBinding(key.WithKeys("c"), key.WithHelp("c", "make primary (/cd)")),
	Escape:  key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "close")),
	Submit:  key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "confirm")),
	Cancel:  key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "back")),
	// GORILLA OVERRIDE: `p` in the add form makes the typed path the PRIMARY root
	// rather than an extra. This is the operation most users actually want and
	// the reason the command exists — pointing the agent at one project instead
	// of a home directory holding millions of files. It works even when the path
	// is inside the current root, which adding deliberately refuses.
	Narrow: key.NewBinding(key.WithKeys("p"), key.WithHelp("p", "make primary")),
}

type addDirDialogCmp struct {
	mode        addDirMode
	roots       []string
	selectedIdx int
	width       int
	height      int

	input   textinput.Model
	formErr string
}

func NewAddDirDialogCmp() AddDirDialog { return &addDirDialogCmp{} }

func (m *addDirDialogCmp) Init() tea.Cmd {
	m.mode = addDirModeList
	m.formErr = ""
	m.refresh()
	return nil
}

func (m *addDirDialogCmp) refresh() {
	m.roots = config.Roots()
	if m.selectedIdx >= len(m.roots) {
		m.selectedIdx = max(0, len(m.roots)-1)
	}
}

func (m *addDirDialogCmp) width_() int {
	// GORILLA OVERRIDE: full terminal width minus this dialog's own chrome, with
	// NO upper cap — matching /context (loadout.go). An earlier version capped the
	// width, which truncated the explanatory text with an ellipsis on a wide
	// terminal and left an unused black margin. The whole point of these rows is
	// that the description is readable.
	//
	// chrome is this dialog's border (1+1) plus padding (2+2). SUBTRACTED from the
	// terminal, never added on top — a wrapper is never free, and adding it made
	// the dialog 82 columns wide in an 80-column terminal.
	//
	// The floor is deliberately small: it only applies on a terminal too narrow to
	// hold the dialog at all, where cramped beats overflowing.
	const (
		chrome   = 6
		minWidth = 30
	)
	if m.width <= 0 {
		return addDirDialogWidth
	}
	if w := m.width - chrome; w > minWidth {
		return w
	}
	return minWidth
}

func (m *addDirDialogCmp) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		return m, nil
	case tea.KeyMsg:
		if m.mode == addDirModeForm {
			return m.updateForm(msg)
		}
		return m.updateList(msg)
	}
	return m, nil
}

func (m *addDirDialogCmp) updateList(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, addDirKeys.Up):
		if m.selectedIdx > 0 {
			m.selectedIdx--
		} else {
			m.selectedIdx = len(m.roots) - 1
		}
	case key.Matches(msg, addDirKeys.Down):
		if m.selectedIdx < len(m.roots)-1 {
			m.selectedIdx++
		} else {
			m.selectedIdx = 0
		}
	case key.Matches(msg, addDirKeys.Escape):
		return m, util.CmdHandler(CloseAddDirDialogMsg{})

	case key.Matches(msg, addDirKeys.Add):
		m.openForm()

	case key.Matches(msg, addDirKeys.Remove):
		if len(m.roots) == 0 {
			return m, nil
		}
		target := m.roots[m.selectedIdx]
		// The primary root is changed with /cd, not removed — a workspace with
		// no primary has no way to resolve a relative path.
		if m.selectedIdx == 0 {
			return m, util.CmdHandler(RootsChangedMsg{
				Info: "that is the primary root — press c to change it, not d to remove it",
			})
		}
		if err := config.RemoveDir(target); err != nil {
			return m, util.CmdHandler(RootsChangedMsg{Info: err.Error()})
		}
		m.refresh()
		// Removal is trivially reversible with `a`, so no confirmation step.
		return m, util.CmdHandler(RootsChangedMsg{
			Info: fmt.Sprintf("removed root %s", filepath.Base(target)),
		})

	case key.Matches(msg, addDirKeys.Promote):
		if len(m.roots) == 0 {
			return m, nil
		}
		target := m.roots[m.selectedIdx]
		if m.selectedIdx == 0 {
			return m, util.CmdHandler(RootsChangedMsg{Info: "already the primary root"})
		}
		// keepOld=true: silently dropping the previous primary would remove its
		// context files and permission scope without the user asking.
		if _, err := config.SetWorkingDir(target, true); err != nil {
			return m, util.CmdHandler(RootsChangedMsg{Info: err.Error()})
		}
		m.refresh()
		m.selectedIdx = 0
		return m, util.CmdHandler(RootsChangedMsg{
			Info:           fmt.Sprintf("primary root is now %s", target),
			PrimaryChanged: true,
		})
	}
	return m, nil
}

func (m *addDirDialogCmp) openForm() {
	m.mode = addDirModeForm
	m.formErr = ""
	in := textinput.New()
	in.Placeholder = "/path/to/dir  (~ and relative paths work)"
	in.CharLimit = 500
	in.Width = m.width_() - 12
	// Bubbles inputs carry their own dark styles and render as a black box
	// without this — ROOT CAUSE #3 in the TUI lessons.
	applyInputTheme(&in)
	in.Focus()
	m.input = in
}

func (m *addDirDialogCmp) updateForm(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, addDirKeys.Cancel):
		m.mode = addDirModeList
		m.formErr = ""
		return m, nil

	// Narrow the workspace to the typed path. Deliberately checked BEFORE the
	// textinput gets the key, and deliberately allowed for a path inside the
	// current root — that is the case adding refuses and narrowing is for.
	case key.Matches(msg, addDirKeys.Narrow):
		dir := strings.TrimSpace(m.input.Value())
		if dir == "" {
			m.formErr = "enter a path first"
			return m, nil
		}
		return m, m.narrowTo(dir)

	case key.Matches(msg, addDirKeys.Submit):
		dir := strings.TrimSpace(m.input.Value())
		if dir == "" {
			m.formErr = "enter a path"
			return m, nil
		}
		added, err := config.AddDir(dir)
		if err != nil {
			// Stay in the form so the user can correct the path rather than
			// retyping it from scratch.
			m.formErr = err.Error()
			return m, nil
		}
		m.mode = addDirModeList
		m.refresh()
		return m, util.CmdHandler(RootsChangedMsg{
			Info: fmt.Sprintf("added root %s", added),
		})
	}
	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	return m, cmd
}

// narrowTo makes dir the primary workspace root. keepOld is false: the caller
// asked to work in dir, and config.SetWorkingDir additionally drops any root
// that CONTAINS dir, so a narrowing operation genuinely narrows rather than
// leaving the wide tree in scope while reporting success.
func (m *addDirDialogCmp) narrowTo(dir string) tea.Cmd {
	target, err := config.SetWorkingDir(dir, false)
	if err != nil {
		m.formErr = err.Error()
		return nil
	}
	m.mode = addDirModeList
	m.formErr = ""
	m.refresh()
	m.selectedIdx = 0
	return util.CmdHandler(RootsChangedMsg{
		Info:           fmt.Sprintf("workspace narrowed to %s", target),
		PrimaryChanged: true,
	})
}

func (m *addDirDialogCmp) View() string {
	if m.mode == addDirModeForm {
		return m.formView()
	}
	return m.listView()
}

func (m *addDirDialogCmp) listView() string {
	t := theme.CurrentTheme()
	base := styles.BaseStyle()
	w := m.width_()

	rows := []string{
		base.Foreground(t.Primary()).Bold(true).Width(w).
			Render("Workspace roots — where project context and permissions apply"),
		base.Foreground(t.TextMuted()).Width(w).
			Render("Adding a root does NOT grant new access — the agent can already open any"),
		base.Foreground(t.TextMuted()).Width(w).
			Render("absolute path. It loads that root's CLAUDE.md, scopes permissions to it,"),
		base.Foreground(t.TextMuted()).Width(w).
			Render("tells the model it exists, and watches it for diagnostics."),
		base.Width(w).Render(""),
	}

	for i, root := range m.roots {
		selected := i == m.selectedIdx
		label := "added"
		if i == 0 {
			label = "PRIMARY"
		}

		rowStyle := base.Width(w)
		if selected {
			rowStyle = rowStyle.Background(t.Primary()).Foreground(t.Background()).Bold(true)
		}
		rows = append(rows, rowStyle.Render(fmt.Sprintf("  %-9s %s", label, truncatePath(root, w-14))))

		detail := describeRoot(root)
		detailStyle := base.Width(w).Foreground(t.TextMuted())
		if selected {
			detailStyle = detailStyle.Background(t.Primary()).Foreground(t.Background())
		}
		rows = append(rows, detailStyle.Render("            "+detail))
	}

	rows = append(rows,
		base.Width(w).Render(""),
		base.Foreground(t.TextMuted()).Width(w).
			Render("a add / switch   d remove   c make primary   up/down move   esc close"),
	)

	return base.Padding(1, 2).Border(lipgloss.RoundedBorder()).
		BorderForeground(t.Primary()).Render(lipgloss.JoinVertical(lipgloss.Left, rows...))
}

func (m *addDirDialogCmp) formView() string {
	t := theme.CurrentTheme()
	base := styles.BaseStyle()
	w := m.width_()

	rows := []string{
		base.Foreground(t.Primary()).Bold(true).Width(w).Render("Add or switch to a workspace root"),
		base.Foreground(t.TextMuted()).Width(w).
			Render("enter adds it alongside the current root. p makes it THE root instead,"),
		base.Foreground(t.TextMuted()).Width(w).
			Render("which is what you want when the current root is too broad — a home"),
		base.Foreground(t.TextMuted()).Width(w).
			Render("directory holding a kernel tree and a browser tree is millions of files."),
		base.Width(w).Render(""),
		base.Width(w).Render(m.input.View()),
	}
	if m.formErr != "" {
		rows = append(rows, base.Width(w).Foreground(t.Error()).Render("  "+m.formErr))
	}
	rows = append(rows,
		base.Width(w).Render(""),
		base.Foreground(t.TextMuted()).Width(w).
			Render("enter: add alongside   p: make it the only root   esc: back"),
	)

	return base.Padding(1, 2).Border(lipgloss.RoundedBorder()).
		BorderForeground(t.Primary()).Render(lipgloss.JoinVertical(lipgloss.Left, rows...))
}

// describeRoot summarises what a root actually contributes, so the row is
// informative rather than just a path repeated back.
func describeRoot(root string) string {
	cfg := config.Get()
	if cfg == nil {
		return ""
	}
	n := 0
	for _, p := range cfg.ContextPaths {
		if strings.HasSuffix(p, "/") {
			continue
		}
		if _, err := statPath(filepath.Join(root, p)); err == nil {
			n++
		}
	}
	git := "not a git repo"
	if _, err := statPath(filepath.Join(root, ".git")); err == nil {
		git = "git repo"
	}
	plural := "s"
	if n == 1 {
		plural = ""
	}
	return fmt.Sprintf("%d context file%s | %s", n, plural, git)
}

func truncatePath(p string, maxLen int) string {
	if maxLen <= 3 || len(p) <= maxLen {
		return p
	}
	// Keep the tail — the distinctive part of a path is its end.
	return "..." + p[len(p)-maxLen+1:]
}

func (m *addDirDialogCmp) BindingKeys() []key.Binding {
	return layout.KeyMapToSlice(addDirKeys)
}

// statPath is a thin os.Stat wrapper kept local so this file's imports stay
// small; describeRoot only needs existence.
func statPath(p string) (os.FileInfo, error) { return os.Stat(p) }
