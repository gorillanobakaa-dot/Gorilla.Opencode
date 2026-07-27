// GORILLA OVERRIDE: this file did not exist upstream. It is /settings — every
// tunable option, with a plain-language description, what it accepts, and its
// default, all on screen rather than in a help page.
//
// This is the first dialog whose list can exceed the terminal height, so it is
// the first that needs a viewport. Height is computed as terminal height MINUS
// this dialog's own chrome, never as a content height the chrome is added to —
// that arithmetic is what made the v0.1.38 input box invisible.
package dialog

import (
	"fmt"
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

// CloseSettingsDialogMsg closes the dialog.
type CloseSettingsDialogMsg struct{}

// SettingsChangedMsg is emitted after a setting changes, so the TUI can rebuild
// what needs rebuilding and report honestly if a turn is in flight.
type SettingsChangedMsg struct {
	Info string
	// InvalidateCtx is set for settings that change which files are read
	// (contextPaths), so the context cache must be dropped.
	InvalidateCtx bool
}

type SettingsDialog interface {
	tea.Model
	layout.Bindings
}

const (
	settingsDialogWidth = 108
	// Chrome this dialog spends on itself: border (1+1) + padding (2+2)
	// horizontally, border (1+1) + padding (1+1) vertically. Counted, not
	// guessed — a wrapper is never free.
	settingsHChrome = 6
	settingsVChrome = 4
	// Rows of fixed furniture: title, subtitle, blank, and the footer block.
	settingsFurniture = 8
)

type settingsMode int

const (
	settingsModeList settingsMode = iota
	settingsModeEdit
)

type settingsKeyMap struct {
	Up, Down, PageUp, PageDown, Left, Right, Toggle, Edit, Reset, Filter, Escape, Submit key.Binding
}

// Arrow keys for adjustment — `-`/`+`/`[`/`]` are awkward or hidden on non-US
// layouts (the JP-keyboard lesson).
var settingsKeys = settingsKeyMap{
	Up:       key.NewBinding(key.WithKeys("up", "k"), key.WithHelp("↑", "up")),
	Down:     key.NewBinding(key.WithKeys("down", "j"), key.WithHelp("↓", "down")),
	PageUp:   key.NewBinding(key.WithKeys("pgup"), key.WithHelp("pgup", "page up")),
	PageDown: key.NewBinding(key.WithKeys("pgdown"), key.WithHelp("pgdn", "page down")),
	Left:     key.NewBinding(key.WithKeys("left"), key.WithHelp("←", "lower")),
	Right:    key.NewBinding(key.WithKeys("right"), key.WithHelp("→", "higher")),
	Toggle:   key.NewBinding(key.WithKeys(" "), key.WithHelp("space", "toggle / edit")),
	Edit:     key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "edit")),
	Reset:    key.NewBinding(key.WithKeys("r"), key.WithHelp("r", "reset this one")),
	Filter:   key.NewBinding(key.WithKeys("/"), key.WithHelp("/", "filter")),
	Escape:   key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "close")),
	Submit:   key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "save")),
}

// settingsRow is either a group header or a setting. Flattened so one index
// walks the whole display, with headers skipped during navigation.
type settingsRow struct {
	header  string
	setting *config.Setting
	// pointer rows are read-only cross-references to another command's domain.
	pointer string
}

type settingsDialogCmp struct {
	mode        settingsMode
	rows        []settingsRow
	selectedIdx int
	scrollTop   int
	width       int
	height      int
	status      string

	filter      string
	filterInput textinput.Model
	filtering   bool

	editInput textinput.Model
	editErr   string
}

func NewSettingsDialogCmp() SettingsDialog { return &settingsDialogCmp{} }

func (m *settingsDialogCmp) Init() tea.Cmd {
	m.mode = settingsModeList
	m.status = ""
	m.filter = ""
	m.filtering = false
	m.rebuildRows()
	m.selectedIdx = m.firstSelectable()
	m.scrollTop = 0
	return nil
}

func (m *settingsDialogCmp) rebuildRows() {
	m.rows = nil
	needle := strings.ToLower(strings.TrimSpace(m.filter))

	match := func(s *config.Setting) bool {
		if needle == "" {
			return true
		}
		return strings.Contains(strings.ToLower(s.Name), needle) ||
			strings.Contains(strings.ToLower(s.ID), needle) ||
			strings.Contains(strings.ToLower(s.Layman), needle)
	}

	for _, g := range config.GroupOrder {
		var group []settingsRow
		for i := range config.Settings {
			s := &config.Settings[i]
			if s.Group == g && match(s) {
				group = append(group, settingsRow{setting: s})
			}
		}
		if len(group) == 0 {
			continue
		}
		m.rows = append(m.rows, settingsRow{header: string(g)})
		m.rows = append(m.rows, group...)
	}

	// Read-only pointers, so /settings is a complete inventory without becoming
	// a second source of truth for things another command owns.
	if needle == "" {
		m.rows = append(m.rows, settingsRow{header: "Owned by other commands"})
		m.rows = append(m.rows, settingsRow{
			pointer: fmt.Sprintf("AI model — %s   (press /model to change)", config.CurrentModelSummary()),
		})
		for _, e := range config.ModelOwnedElsewhere {
			if e.Name == "AI model" {
				continue
			}
			m.rows = append(m.rows, settingsRow{
				pointer: fmt.Sprintf("%s — %s   (press %s)", e.Name, e.Why, e.Owner),
			})
		}
	}
}

func (m *settingsDialogCmp) selectable(i int) bool {
	return i >= 0 && i < len(m.rows) && m.rows[i].setting != nil
}

func (m *settingsDialogCmp) firstSelectable() int {
	for i := range m.rows {
		if m.selectable(i) {
			return i
		}
	}
	return 0
}

// move steps to the next selectable row in the given direction, skipping headers
// and pointer rows so navigation never lands somewhere inert.
func (m *settingsDialogCmp) move(delta int) {
	if len(m.rows) == 0 {
		return
	}
	i := m.selectedIdx
	for n := 0; n < len(m.rows); n++ {
		i += delta
		if i < 0 {
			i = len(m.rows) - 1
		}
		if i >= len(m.rows) {
			i = 0
		}
		if m.selectable(i) {
			m.selectedIdx = i
			m.ensureVisible()
			return
		}
	}
}

func (m *settingsDialogCmp) contentWidth() int {
	if m.width > 0 {
		w := m.width - settingsHChrome
		if w < 60 {
			return 60
		}
		if w > settingsDialogWidth {
			return settingsDialogWidth
		}
		return w
	}
	return settingsDialogWidth
}

// visibleRows is how many list rows fit. Terminal height MINUS this dialog's own
// vertical chrome and fixed furniture — never a content height with chrome added
// on top. Each setting renders as two lines (value row + description row).
func (m *settingsDialogCmp) visibleRows() int {
	if m.height <= 0 {
		return 20
	}
	avail := m.height - settingsVChrome - m.furnitureLines()
	if avail < 2 {
		// Below this the dialog cannot show a single setting AND its chrome.
		// Returning 2 keeps one row visible and lets the height clamp below
		// drop optional furniture instead of overflowing the screen.
		avail = 2
	}
	return avail
}

// furnitureLines is how many rows the fixed header/footer occupy at the current
// height. On a very short terminal the subtitle and the blank spacers are shed
// rather than pushing the dialog past the bottom of the screen — an overflowing
// dialog scrolls the terminal and destroys the layout, which is strictly worse
// than a terser header.
func (m *settingsDialogCmp) furnitureLines() int {
	if m.height > 0 && m.height < 16 {
		return 3 // title + footer + one spacer
	}
	return settingsFurniture
}

// cramped reports whether to render the terse layout.
func (m *settingsDialogCmp) cramped() bool { return m.height > 0 && m.height < 16 }

func (m *settingsDialogCmp) ensureVisible() {
	// Rows render at two lines each, so budget in rendered lines.
	perRow := 2
	capacity := m.visibleRows() / perRow
	if capacity < 1 {
		capacity = 1
	}
	if m.selectedIdx < m.scrollTop {
		m.scrollTop = m.selectedIdx
	}
	if m.selectedIdx >= m.scrollTop+capacity {
		m.scrollTop = m.selectedIdx - capacity + 1
	}
	if m.scrollTop < 0 {
		m.scrollTop = 0
	}
}

func (m *settingsDialogCmp) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.ensureVisible()
		return m, nil
	case tea.KeyMsg:
		switch m.mode {
		case settingsModeEdit:
			return m.updateEdit(msg)
		default:
			if m.filtering {
				return m.updateFilter(msg)
			}
			return m.updateList(msg)
		}
	}
	return m, nil
}

func (m *settingsDialogCmp) updateFilter(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.filtering = false
		m.filter = ""
		m.rebuildRows()
		m.selectedIdx = m.firstSelectable()
		m.scrollTop = 0
		return m, nil
	case "enter":
		m.filtering = false
		return m, nil
	}
	var cmd tea.Cmd
	m.filterInput, cmd = m.filterInput.Update(msg)
	m.filter = m.filterInput.Value()
	m.rebuildRows()
	m.selectedIdx = m.firstSelectable()
	m.scrollTop = 0
	return m, cmd
}

func (m *settingsDialogCmp) updateList(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, settingsKeys.Escape):
		return m, util.CmdHandler(CloseSettingsDialogMsg{})

	case key.Matches(msg, settingsKeys.Filter):
		in := textinput.New()
		in.Placeholder = "filter settings"
		in.Width = 30
		applyInputTheme(&in)
		in.Focus()
		m.filterInput = in
		m.filtering = true
		return m, nil

	case key.Matches(msg, settingsKeys.Up):
		m.move(-1)
	case key.Matches(msg, settingsKeys.Down):
		m.move(1)
	case key.Matches(msg, settingsKeys.PageUp):
		for i := 0; i < 5; i++ {
			m.move(-1)
		}
	case key.Matches(msg, settingsKeys.PageDown):
		for i := 0; i < 5; i++ {
			m.move(1)
		}

	case key.Matches(msg, settingsKeys.Left):
		return m, m.nudge(-1)
	case key.Matches(msg, settingsKeys.Right):
		return m, m.nudge(1)

	case key.Matches(msg, settingsKeys.Toggle), key.Matches(msg, settingsKeys.Edit):
		return m, m.activate()

	case key.Matches(msg, settingsKeys.Reset):
		return m, m.resetOne()
	}
	return m, nil
}

func (m *settingsDialogCmp) current() *config.Setting {
	if !m.selectable(m.selectedIdx) {
		return nil
	}
	return m.rows[m.selectedIdx].setting
}

// refuseReadOnly reports the reason rather than silently doing nothing, so a
// row that cannot be changed explains itself.
func (m *settingsDialogCmp) refuseReadOnly(s *config.Setting) tea.Cmd {
	m.status = s.Name + ": " + s.ReadOnlyWhy
	return nil
}

// nudge handles ←/→: step a number, walk a preset ladder, cycle an enum, flip a
// bool. Text and list settings open the editor instead.
func (m *settingsDialogCmp) nudge(dir int) tea.Cmd {
	s := m.current()
	if s == nil {
		return nil
	}
	if s.ReadOnly {
		return m.refuseReadOnly(s)
	}

	switch s.Kind {
	case config.KindBool:
		cur, _ := s.Get().(bool)
		return m.applySet(s, !cur)

	case config.KindInt:
		cur, _ := s.Get().(int)
		step := s.Step
		if step == 0 {
			step = 1
		}
		next := cur + dir*step
		if next < s.Min {
			next = s.Min
		}
		if next > s.Max {
			next = s.Max
		}
		return m.applySet(s, next)

	case config.KindLadder:
		cur, _ := s.Get().(int)
		idx := nearestPresetIndex(s.Presets, cur)
		idx += dir
		if idx < 0 {
			idx = 0
		}
		if idx >= len(s.Presets) {
			idx = len(s.Presets) - 1
		}
		return m.applySet(s, s.Presets[idx])

	case config.KindEnum:
		if len(s.Options) == 0 {
			m.status = s.Name + ": no options available"
			return nil
		}
		cur, _ := s.Get().(string)
		idx := 0
		for i, o := range s.Options {
			if o == cur {
				idx = i
				break
			}
		}
		idx = (idx + dir + len(s.Options)) % len(s.Options)
		return m.applySet(s, s.Options[idx])

	default:
		return m.activate()
	}
}

func nearestPresetIndex(presets []int, cur int) int {
	best, bestDist := 0, 1<<30
	for i, p := range presets {
		d := p - cur
		if d < 0 {
			d = -d
		}
		if d < bestDist {
			best, bestDist = i, d
		}
	}
	return best
}

// activate flips a bool or opens the text editor for anything free-form.
func (m *settingsDialogCmp) activate() tea.Cmd {
	s := m.current()
	if s == nil {
		return nil
	}
	if s.ReadOnly {
		return m.refuseReadOnly(s)
	}
	if s.Kind == config.KindBool {
		cur, _ := s.Get().(bool)
		return m.applySet(s, !cur)
	}

	in := textinput.New()
	in.SetValue(config.FormatSettingValue(s.Get()))
	in.CharLimit = 800
	in.Width = m.contentWidth() - 14
	applyInputTheme(&in)
	in.Focus()
	m.editInput = in
	m.editErr = ""
	m.mode = settingsModeEdit
	return nil
}

func (m *settingsDialogCmp) updateEdit(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.mode = settingsModeList
		m.editErr = ""
		return m, nil
	case "enter":
		s := m.current()
		if s == nil {
			m.mode = settingsModeList
			return m, nil
		}
		raw := strings.TrimSpace(m.editInput.Value())
		var val any = raw
		if s.Kind == config.KindStringList {
			list, err := splitList(raw)
			if err != nil {
				m.editErr = err.Error()
				return m, nil
			}
			val = list
		}
		if err := s.Set(val); err != nil {
			// Stay in the editor so the value can be corrected rather than
			// retyped from scratch.
			m.editErr = err.Error()
			return m, nil
		}
		m.mode = settingsModeList
		return m, m.changedCmd(s)
	}
	var cmd tea.Cmd
	m.editInput, cmd = m.editInput.Update(msg)
	return m, cmd
}

func splitList(raw string) ([]string, error) {
	var out []string
	for _, p := range strings.Split(raw, ",") {
		if v := strings.TrimSpace(p); v != "" {
			out = append(out, v)
		}
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("enter at least one value, comma-separated")
	}
	return out, nil
}

func (m *settingsDialogCmp) applySet(s *config.Setting, v any) tea.Cmd {
	if err := s.Set(v); err != nil {
		m.status = s.Name + ": " + err.Error()
		return nil
	}
	m.status = ""
	return m.changedCmd(s)
}

func (m *settingsDialogCmp) resetOne() tea.Cmd {
	s := m.current()
	if s == nil {
		return nil
	}
	if s.ReadOnly || s.Set == nil {
		return m.refuseReadOnly(s)
	}
	old := config.FormatSettingValue(s.Get())
	if old == config.FormatSettingValue(s.Default) {
		m.status = s.Name + " is already the default"
		return nil
	}
	if err := s.Set(s.Default); err != nil {
		m.status = s.Name + ": " + err.Error()
		return nil
	}
	// Name the old value so the user knows what was discarded.
	m.status = fmt.Sprintf("%s reset to %s (was %s)",
		s.Name, config.FormatSettingValue(s.Default), old)
	return m.changedCmd(s)
}

func (m *settingsDialogCmp) changedCmd(s *config.Setting) tea.Cmd {
	info := fmt.Sprintf("%s: %s", s.Name, config.FormatSettingValue(s.Get()))
	if s.Restart {
		info += " — takes effect next time you start gorilla-opencode"
	}
	return util.CmdHandler(SettingsChangedMsg{
		Info:          info,
		InvalidateCtx: s.ID == "contextPaths",
	})
}

func (m *settingsDialogCmp) View() string {
	t := theme.CurrentTheme()
	base := styles.BaseStyle()
	w := m.contentWidth()

	if m.mode == settingsModeEdit {
		return m.editView(w)
	}

	changed := config.SettingsChangedFromDefault()
	head := []string{
		base.Foreground(t.Primary()).Bold(true).Width(w).
			Render("Settings — every option, what it accepts, and its default"),
	}
	if !m.cramped() {
		head = append(head, base.Foreground(t.TextMuted()).Width(w).
			Render(fmt.Sprintf("%d of %d differ from the shipped defaults · r resets one · /reset does whole scopes",
				changed, len(config.Settings))))
	}
	if m.filtering || m.filter != "" {
		head = append(head, base.Width(w).Render("  filter: "+m.filterInput.View()))
	}
	if !m.cramped() {
		head = append(head, base.Width(w).Render(""))
	}

	// Render the visible window, two lines per setting.
	perRow := 2
	capacity := m.visibleRows() / perRow
	if capacity < 1 {
		capacity = 1
	}
	end := m.scrollTop + capacity
	if end > len(m.rows) {
		end = len(m.rows)
	}

	var body []string
	for i := m.scrollTop; i < end; i++ {
		r := m.rows[i]
		switch {
		case r.header != "":
			body = append(body, base.Width(w).Foreground(t.Primary()).Bold(true).Render("  "+r.header))
		case r.pointer != "":
			body = append(body, base.Width(w).Foreground(t.TextMuted()).Render("    "+clip(r.pointer, w-6)))
		default:
			body = append(body, m.settingRows(r.setting, i == m.selectedIdx, w)...)
		}
	}

	var foot []string
	if !m.cramped() {
		foot = append(foot, base.Width(w).Render(""))
	}
	if m.status != "" {
		foot = append(foot, base.Width(w).Foreground(t.Warning()).Render("  "+clip(m.status, w-4)))
	}
	scrollNote := ""
	if len(m.rows) > capacity {
		scrollNote = fmt.Sprintf("  (%d-%d of %d)", m.scrollTop+1, end, len(m.rows))
	}
	foot = append(foot,
		base.Foreground(t.TextMuted()).Width(w).
			Render("↑↓ move  ←→ change  space toggle/edit  r reset one  / filter  esc close"+scrollNote),
	)

	all := append(append(head, body...), foot...)

	// Last-resort clamp. Everything above budgets carefully, but a dialog that
	// renders even one line past the bottom of the terminal scrolls the screen
	// and wrecks the layout — so trim the body rather than trust the arithmetic.
	// settingsVChrome accounts for the border and padding this Render adds.
	if m.height > 0 {
		maxLines := m.height - settingsVChrome
		if maxLines < 1 {
			maxLines = 1
		}
		if len(all) > maxLines {
			keepFoot := len(foot)
			if keepFoot > maxLines-1 {
				keepFoot = 0
			}
			bodyRoom := maxLines - len(head) - keepFoot
			if bodyRoom < 0 {
				bodyRoom = 0
			}
			if bodyRoom > len(body) {
				bodyRoom = len(body)
			}
			all = append(append(append([]string{}, head...), body[:bodyRoom]...), foot[:keepFoot]...)
			if len(all) > maxLines {
				all = all[:maxLines]
			}
		}
	}

	return base.Padding(1, 2).Border(lipgloss.RoundedBorder()).
		BorderForeground(t.Primary()).Render(lipgloss.JoinVertical(lipgloss.Left, all...))
}

// settingRows renders one setting as a value line and a description line. Value,
// range and default are all on screen — that is the requirement, and it belongs
// here rather than in a help page.
func (m *settingsDialogCmp) settingRows(s *config.Setting, selected bool, w int) []string {
	t := theme.CurrentTheme()
	base := styles.BaseStyle()

	value := config.FormatSettingValue(s.Get())
	if s.Unit != "" && s.Kind != config.KindBool {
		value += " " + s.Unit
	}
	tail := fmt.Sprintf("(%s, default %s)", config.SettingRange(s), config.FormatSettingValue(s.Default))
	if s.ReadOnly {
		tail = "FIXED"
	}
	if s.Restart {
		tail += " (next launch)"
	}

	main := fmt.Sprintf("   %-30s %-22s %s", clip(s.Name, 30), clip(value, 22), tail)

	// Bool rows explain BOTH states, so the off case is never left to inference.
	desc := s.Layman
	if s.Kind == config.KindBool {
		cur, _ := s.Get().(bool)
		if cur {
			desc = "ON: " + s.WhenOn + "   ·   OFF: " + s.WhenOff
		} else {
			desc = "OFF: " + s.WhenOff + "   ·   ON: " + s.WhenOn
		}
	}
	if s.ReadOnly {
		desc = s.ReadOnlyWhy
	}

	mainStyle := base.Width(w)
	descStyle := base.Width(w).Foreground(t.TextMuted())
	switch {
	case selected:
		mainStyle = mainStyle.Background(t.Primary()).Foreground(t.Background()).Bold(true)
		descStyle = descStyle.Background(t.Primary()).Foreground(t.Background())
	case s.ReadOnly:
		mainStyle = mainStyle.Foreground(t.TextMuted())
	}

	return []string{
		mainStyle.Render(clip(main, w)),
		descStyle.Render(clip("       "+desc, w)),
	}
}

func (m *settingsDialogCmp) editView(w int) string {
	t := theme.CurrentTheme()
	base := styles.BaseStyle()
	s := m.current()

	rows := []string{
		base.Foreground(t.Primary()).Bold(true).Width(w).Render("Edit: " + s.Name),
		base.Width(w).Render(""),
		base.Foreground(t.TextMuted()).Width(w).Render("  " + clip(s.Layman, w-4)),
		base.Foreground(t.TextMuted()).Width(w).Render(fmt.Sprintf("  accepts: %s   ·   default: %s",
			config.SettingRange(s), config.FormatSettingValue(s.Default))),
		base.Width(w).Render(""),
		base.Width(w).Render("  " + m.editInput.View()),
	}
	if m.editErr != "" {
		rows = append(rows, base.Width(w).Foreground(t.Error()).Render("  "+clip(m.editErr, w-4)))
	}
	rows = append(rows,
		base.Width(w).Render(""),
		base.Foreground(t.TextMuted()).Width(w).Render("enter save   esc cancel"),
	)

	return base.Padding(1, 2).Border(lipgloss.RoundedBorder()).
		BorderForeground(t.Primary()).Render(lipgloss.JoinVertical(lipgloss.Left, rows...))
}

func clip(s string, max int) string {
	if max <= 1 || len(s) <= max {
		return s
	}
	if max <= 4 {
		return s[:max]
	}
	return s[:max-1] + "…"
}

func (m *settingsDialogCmp) BindingKeys() []key.Binding {
	return layout.KeyMapToSlice(settingsKeys)
}
