// GORILLA OVERRIDE (2026-08-18): /sessions — find, revive, export or erase any
// past conversation.
//
// WHY IT EXISTS, in the owner's words, because the reasoning is the design:
//
//	"the electricity isn't guaranteed either. 15 year old laptops have very poor
//	 battery life, measured in minutes maybe, and some of them seconds only, so
//	 they have to be plugged in all the time. If the electricity drops, the whole
//	 session goes as well. Sometimes it might not appear for days on end."
//
// Everything the program offered before this assumed you were still in the
// session you cared about. The switcher (ctrl+s) showed titles and nothing else
// — no date, no size, no search — and `/export` could only save the session you
// were currently inside. After a power cut, neither helps.
//
//	"these laptops will have very, very limited space... they'd be lucky to have
//	 1 to 2 GB of available storage. For them we need the ability to fully
//	 delete, erase and purge past sessions, as the current architecture doesn't
//	 seem to care about the kids — the data just gets accumulated and stored
//	 with no concern for storage management."
//
// So every row carries what it costs on disk, the header carries the total, and
// deleting is followed by a real reclaim — measured, and reported as bytes
// actually returned rather than as a cheerful "deleted". See
// internal/db/session_storage.go for why that distinction is the whole point.
package dialog

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/opencode-ai/opencode/internal/session"
	"github.com/opencode-ai/opencode/internal/tui/styles"
	"github.com/opencode-ai/opencode/internal/tui/theme"
	"github.com/opencode-ai/opencode/internal/tui/util"
)

// SessionsCloseMsg closes the manager.
type SessionsCloseMsg struct{}

// SessionsReviveMsg asks the app to reopen a past conversation.
type SessionsReviveMsg struct{ Session session.Session }

// SessionsExportMsg asks the app to write a past conversation to disk. The app
// owns the message store, so the dialog only names what it wants.
type SessionsExportMsg struct{ Session session.Session }

// SessionsDeleteMsg asks the app to erase a conversation, its helper sessions
// and everything they hold, then return the space to the disk.
type SessionsDeleteMsg struct{ Session session.Session }

// SessionRow is one conversation as the manager shows it.
type SessionRow struct {
	Session session.Session
	Bytes   int64
	Msgs    int64
}

// SessionsStore is everything the screen needs, gathered by the app in one go.
type SessionsStore struct {
	Rows []SessionRow
	// TotalBytes is live content across the whole store; FileBytes is what the
	// database actually occupies on disk, including the write-ahead log. They
	// differ, often by a lot, and only the second one is what the device sees.
	TotalBytes, FileBytes  int64
	Sessions, Helpers, Msg int64
}

type sessionsSort int

const (
	sortByDate sessionsSort = iota
	sortBySize
)

type SessionsCmp struct {
	store    SessionsStore
	filtered []SessionRow
	search   string
	// matched is the set of session ids whose MESSAGE CONTENT matches the
	// current search, supplied by the app. Titles are matched here; content
	// needs the database.
	matched     map[string]bool
	sortBy      sessionsSort
	selected    int
	offset      int
	confirming  bool // a delete is one keypress from happening
	width       int
	height      int
	currentID   string
	lastMessage string
}

func NewSessionsCmp() SessionsCmp { return SessionsCmp{} }

func (m *SessionsCmp) SetStore(s SessionsStore, currentID string) {
	m.store = s
	m.currentID = currentID
	m.applyFilter()
}

// SetMatches supplies the content-search hits for the current needle.
func (m *SessionsCmp) SetMatches(hits map[string]bool) {
	m.matched = hits
	m.applyFilter()
}

// SetNotice shows the outcome of the last action inside the dialog, where the
// person who pressed the key is still looking — a toast behind an open dialog
// is a message nobody reads.
func (m *SessionsCmp) SetNotice(s string) { m.lastMessage = s }

func (m *SessionsCmp) SetSize(w, h int) { m.width, m.height = w, h }

func (m SessionsCmp) Init() tea.Cmd { return nil }

// Current is the highlighted row, if there is one.
func (m SessionsCmp) Current() (SessionRow, bool) {
	if m.selected < 0 || m.selected >= len(m.filtered) {
		return SessionRow{}, false
	}
	return m.filtered[m.selected], true
}

// Search is the needle currently typed, so the app can run the content query.
func (m SessionsCmp) Search() string { return m.search }

func (m *SessionsCmp) applyFilter() {
	needle := strings.ToLower(strings.TrimSpace(m.search))
	m.filtered = m.filtered[:0]
	for _, r := range m.store.Rows {
		if needle == "" ||
			strings.Contains(strings.ToLower(r.Session.Title), needle) ||
			m.matched[r.Session.ID] {
			m.filtered = append(m.filtered, r)
		}
	}
	switch m.sortBy {
	case sortBySize:
		sort.SliceStable(m.filtered, func(i, j int) bool { return m.filtered[i].Bytes > m.filtered[j].Bytes })
	default:
		sort.SliceStable(m.filtered, func(i, j int) bool {
			return m.filtered[i].Session.CreatedAt > m.filtered[j].Session.CreatedAt
		})
	}
	if m.selected >= len(m.filtered) {
		m.selected = max(0, len(m.filtered)-1)
	}
	m.offset = 0
}

func (m SessionsCmp) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		return m, nil

	case tea.KeyMsg:
		// A pending delete owns the keyboard. Every other key cancels it, so
		// the dangerous path needs a deliberate "y" and nothing else can wander
		// into it — including the arrow keys someone is already holding down.
		if m.confirming {
			switch msg.String() {
			case "y", "Y":
				m.confirming = false
				if row, ok := m.Current(); ok {
					return m, util.CmdHandler(SessionsDeleteMsg{Session: row.Session})
				}
			default:
				m.confirming = false
				m.lastMessage = "Not deleted."
			}
			return m, nil
		}

		switch msg.String() {
		case "esc":
			return m, util.CmdHandler(SessionsCloseMsg{})
		case "up", "ctrl+p":
			if m.selected > 0 {
				m.selected--
			}
		case "down", "ctrl+n":
			if m.selected < len(m.filtered)-1 {
				m.selected++
			}
		case "enter":
			if row, ok := m.Current(); ok {
				return m, util.CmdHandler(SessionsReviveMsg{Session: row.Session})
			}
		case "ctrl+e":
			if row, ok := m.Current(); ok {
				return m, util.CmdHandler(SessionsExportMsg{Session: row.Session})
			}
		// DELETE, not ctrl+d.
		//
		// GORILLA FIX (2026-08-18): ctrl+d was the first choice and never
		// arrived. This repo already recorded why, in
		// internal/tui/provider_escape_test.go: ctrl+d is EOF, ctrl+q and
		// ctrl+s are flow control, ctrl+z suspends, and of the rest this app
		// already binds ctrl+a c d e f h k l n o r s t u x y. There was no free
		// control key, and the rule against taking a reserved one was written
		// down before this screen existed.
		//
		// The Delete key is unbound, unreserved, and says what it does — which
		// matters more here than anywhere else in the program, because this is
		// the one action that cannot be undone.
		case "delete":
			if row, ok := m.Current(); ok {
				if row.Session.ID == m.currentID {
					m.lastMessage = "That is the conversation you are in — leave it first (/clear), then delete it."
					return m, nil
				}
				m.confirming = true
			}
		// TAB, not ctrl+s.
		//
		// GORILLA FIX (2026-08-18): ctrl+s was the first choice and it never
		// arrived. Two independent reasons, either of which is fatal: this
		// program already binds ctrl+s globally to the old session switcher and
		// handles it before any dialog sees it, and ctrl+s is XOFF — on a
		// terminal that has not disabled software flow control it freezes the
		// display rather than reaching the application. Found by driving the
		// real binary; no headless test would have caught either.
		case "tab":
			if m.sortBy == sortByDate {
				m.sortBy = sortBySize
			} else {
				m.sortBy = sortByDate
			}
			m.applyFilter()
		case "backspace":
			if r := []rune(m.search); len(r) > 0 {
				m.search = string(r[:len(r)-1])
				m.applyFilter()
				return m, util.CmdHandler(SessionsSearchMsg{Needle: m.search})
			}
		default:
			// Plain characters type into the search box. This is why the
			// actions are on ctrl: someone searching for "delete" must not
			// erase a session by spelling it.
			//
			// GORILLA FIX (2026-08-18): every rune in the message, not just a
			// single one. bubbletea coalesces fast input into ONE KeyMsg
			// carrying several runes, so typing at speed — and pasting, always
			// — arrived as a multi-rune message and was dropped on the floor.
			// The search box did nothing at all when driven live; it looked
			// like the keys were not reaching the dialog.
			if msg.Type == tea.KeyRunes && len(msg.Runes) > 0 {
				m.search += string(msg.Runes)
				m.applyFilter()
				return m, util.CmdHandler(SessionsSearchMsg{Needle: m.search})
			}
		}

		if m.selected < m.offset {
			m.offset = m.selected
		}
		if rows := m.visibleRows(); m.selected >= m.offset+rows {
			m.offset = m.selected - rows + 1
		}
	}
	return m, nil
}

// SessionsSearchMsg asks the app to run a content search for the needle. Titles
// are filtered locally; message bodies need the database.
type SessionsSearchMsg struct{ Needle string }

func (m SessionsCmp) visibleRows() int {
	if m.height <= 0 {
		return 8
	}
	// Chrome, counted rather than guessed — the first attempt said 13 and drew
	// 25 rows in a 24-row window, caught by
	// TestDialogFramesNeverExceedTheTerminal before it reached a screen:
	// border and padding (4); title, totals, blank, search, sort-count, blank
	// (6); blank, notice, blank, keys (4). Fourteen.
	rows := m.height - 14
	if rows > 12 {
		rows = 12
	}
	if rows < 1 {
		rows = 1
	}
	return rows
}

func (m SessionsCmp) View() string {
	t := theme.CurrentTheme()
	width := dialogWidth(m.width, 96, 6)
	base := styles.BaseStyle().Background(styles.PanelBackground())
	mute := base.Foreground(t.TextMuted()).Width(width)
	text := base.Foreground(t.Text()).Width(width)

	var lines []string
	lines = append(lines,
		base.Foreground(t.Primary()).Bold(true).Width(width).
			Render(fitLine("Sessions — find, revive, export, erase", width)))

	// The totals line is the reason this screen exists. On a device with 1 GB
	// free, "what is this costing me" is the first question, not a footnote.
	lines = append(lines, mute.Render(fitLine(fmt.Sprintf(
		"%d conversations · %d helper sessions · %d messages · %s on disk",
		m.store.Sessions, m.store.Helpers, m.store.Msg, humanBytes(m.store.FileBytes)), width)))

	search := m.search
	if search == "" {
		search = "(type to search titles and message text)"
	}
	lines = append(lines, "", text.Render(fitLine("search: "+search, width)))

	sortLabel := "date"
	if m.sortBy == sortBySize {
		sortLabel = "size"
	}
	lines = append(lines, mute.Render(fitLine(fmt.Sprintf(
		"%d shown · sorted by %s", len(m.filtered), sortLabel), width)), "")

	if len(m.filtered) == 0 {
		lines = append(lines, text.Render(fitLine("Nothing matches.", width)))
	}

	end := m.offset + m.visibleRows()
	if end > len(m.filtered) {
		end = len(m.filtered)
	}
	for i := m.offset; i < end; i++ {
		row := m.filtered[i]
		when := time.Unix(row.Session.CreatedAt, 0).Format("Jan 02 15:04")
		title := strings.TrimSpace(row.Session.Title)
		if title == "" {
			title = "(untitled)"
		}
		marker := " "
		if row.Session.ID == m.currentID {
			marker = "•" // the conversation you are in
		}
		// Size right-aligned so the expensive rows are findable by eye, which
		// is the whole point of being able to sort by it.
		right := fmt.Sprintf("%5d msg  %9s", row.Msgs, humanBytes(row.Bytes))
		// The TITLE is truncated, never the size.
		//
		// GORILLA FIX (2026-08-18): the first version padded the left side and
		// let the whole line be truncated at the end, so a long title pushed the
		// size column off the screen — and session titles are auto-generated
		// from the first message, so they are long almost always. Seen on the
		// first live run: every row on screen had lost its size, which is the
		// one column the erase decision is made from.
		// -3, not -2: the row is leftWidth + two spaces + right, and fitLine
		// truncates at width-1, so a row sized to exactly `width` loses its last
		// two characters — which is the tail of the size, the very thing being
		// protected here.
		leftWidth := width - lipgloss.Width(right) - 3
		left := fitLine(fmt.Sprintf("%s %s  %s", marker, when, title), leftWidth)
		line := fitLine(pad(left, leftWidth)+"  "+right, width)

		if i == m.selected {
			lines = append(lines, base.Foreground(t.Background()).Background(t.Primary()).
				Bold(true).Width(width).Render(line))
		} else {
			lines = append(lines, text.Render(line))
		}
	}

	lines = append(lines, "")
	switch {
	case m.confirming:
		row, _ := m.Current()
		lines = append(lines, base.Foreground(lipgloss.Color("#FF0000")).Bold(true).Width(width).
			Render(fitLine(fmt.Sprintf(
				"Erase \"%s\" and everything in it? %s comes back. y / any other key",
				strings.TrimSpace(row.Session.Title), humanBytes(row.Bytes)), width)))
	case m.lastMessage != "":
		lines = append(lines, base.Foreground(t.Success()).Width(width).Render(fitLine(m.lastMessage, width)))
	default:
		lines = append(lines, mute.Render(fitLine(
			"Erasing removes the conversation, its helper sessions, and returns the space to the disk.", width)))
	}

	lines = append(lines, "", mute.Render(fitLine(
		"↑↓ move   enter revive   ctrl+e export   DEL erase   tab sort   esc close", width)))

	return base.Padding(1, 2).Border(lipgloss.RoundedBorder()).
		BorderBackground(styles.PanelBackground()).BorderForeground(t.TextMuted()).
		Width(width + 4).
		Render(strings.Join(lines, "\n"))
}

func (m SessionsCmp) Bindings() []key.Binding { return nil }

// pad right-pads to at least n columns, and never truncates — fitLine owns
// truncation, and doing it in two places is how a line ends up one column too
// wide on exactly one screen size.
func pad(s string, n int) string {
	if w := lipgloss.Width(s); w < n {
		return s + strings.Repeat(" ", n-w)
	}
	return s
}

// humanBytes is deliberately short: the column is narrow and the reader wants a
// magnitude, not a precise byte count. KB/MB in the sense a file manager means
// them, because that is what the person comparing this against their free space
// is reading elsewhere.
func humanBytes(n int64) string {
	switch {
	case n >= 1<<20:
		return fmt.Sprintf("%.1f MB", float64(n)/(1<<20))
	case n >= 1<<10:
		return fmt.Sprintf("%.0f KB", float64(n)/(1<<10))
	default:
		return fmt.Sprintf("%d B", n)
	}
}
