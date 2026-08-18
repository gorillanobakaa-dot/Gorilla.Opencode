package dialog

// GORILLA OVERRIDE (2026-08-18): behaviour tests for /sessions.
//
// The screen has one irreversible action on it, so the tests concentrate on the
// ways someone could reach it without meaning to — and on the ways the list
// could quietly lie about what a conversation costs, which is the number the
// erase decision is made from.

import (
	"fmt"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/opencode-ai/opencode/internal/session"
)

func rows(n int) SessionsStore {
	out := make([]SessionRow, 0, n)
	for i := range n {
		out = append(out, SessionRow{
			Session: session.Session{
				ID:        string(rune('a' + i)),
				Title:     []string{"kernel work", "New Session", "firefox fonts", "osint run"}[i%4],
				CreatedAt: time.Date(2026, 8, 17, 12, 0, 0, 0, time.UTC).Unix() - int64(i*60),
			},
			Bytes: int64(i+1) * 1000,
			Msgs:  int64(i + 1),
		})
	}
	return SessionsStore{Rows: out, Sessions: int64(n), Msg: int64(n)}
}

func pressKey(s string) tea.KeyMsg {
	switch s {
	case "delete":
		return tea.KeyMsg{Type: tea.KeyDelete}
	case "ctrl+e":
		return tea.KeyMsg{Type: tea.KeyCtrlE}
	case "tab":
		return tea.KeyMsg{Type: tea.KeyTab}
	case "enter":
		return tea.KeyMsg{Type: tea.KeyEnter}
	case "down":
		return tea.KeyMsg{Type: tea.KeyDown}
	case "esc":
		return tea.KeyMsg{Type: tea.KeyEsc}
	default:
		return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)}
	}
}

func send(m SessionsCmp, keys ...string) (SessionsCmp, tea.Msg) {
	var last tea.Msg
	for _, k := range keys {
		next, cmd := m.Update(pressKey(k))
		m = next.(SessionsCmp)
		if cmd != nil {
			last = cmd()
		}
	}
	return m, last
}

// The single most important property on this screen: erasing takes two
// deliberate keypresses, and anything that is not "y" cancels.
func TestErasingNeedsAConfirmationAndAnythingElseCancels(t *testing.T) {
	m := NewSessionsCmp()
	m.SetStore(rows(3), "")
	m.SetSize(100, 30)

	_, msg := send(m, "delete")
	if _, ok := msg.(SessionsDeleteMsg); ok {
		t.Fatal("ctrl+d deleted immediately — an irreversible action with no confirmation")
	}

	after, msg := send(m, "delete", "y")
	if _, ok := msg.(SessionsDeleteMsg); !ok {
		t.Errorf("ctrl+d then y did not delete; got %T", msg)
	}
	_ = after

	// Every other key must back out, including one someone might be holding.
	for _, k := range []string{"n", "down", "esc", "x"} {
		back, msg := send(m, "delete", k)
		if _, ok := msg.(SessionsDeleteMsg); ok {
			t.Errorf("%q after ctrl+d deleted a session", k)
		}
		if !strings.Contains(back.View(), "Not deleted") {
			t.Errorf("%q cancelled without saying so", k)
		}
	}
}

// Typing must never reach an action. Someone searching for the word "delete"
// spells d-e-l-e-t-e, and every one of those is a plain rune.
func TestTypingSearchesAndNeverActs(t *testing.T) {
	m := NewSessionsCmp()
	m.SetStore(rows(4), "")
	m.SetSize(100, 30)

	after, msg := send(m, "d", "e", "l", "e", "t", "e")
	if _, ok := msg.(SessionsDeleteMsg); ok {
		t.Fatal("typing the word 'delete' erased a session")
	}
	if after.Search() != "delete" {
		t.Errorf("search box holds %q, want %q", after.Search(), "delete")
	}
	// And a search that matches nothing shows nothing rather than everything.
	if got, ok := after.Current(); ok {
		t.Errorf("a search matching no title still had a selected row: %q", got.Session.Title)
	}
}

func TestSearchFiltersOnTitleAndOnContentMatchesFromTheDatabase(t *testing.T) {
	m := NewSessionsCmp()
	m.SetStore(rows(4), "")
	m.SetSize(100, 30)

	m, _ = send(m, "k", "e", "r", "n")
	if got, _ := m.Current(); !strings.Contains(got.Session.Title, "kernel") {
		t.Errorf("title search did not filter; selected %q", got.Session.Title)
	}

	// "New Session" is a real title this program writes, and it is unsearchable.
	// The database supplies content hits; the dialog must honour them.
	m2 := NewSessionsCmp()
	m2.SetStore(rows(4), "")
	m2.SetSize(100, 30)
	m2, _ = send(m2, "v", "a", "-", "a", "p", "i")
	if _, ok := m2.Current(); ok {
		t.Fatal("a needle matching no title matched something before the content hits arrived")
	}
	m2.SetMatches(map[string]bool{"b": true}) // "New Session"
	got, ok := m2.Current()
	if !ok || got.Session.ID != "b" {
		t.Errorf("a content-only match was not shown; a session titled 'New Session' stays unfindable")
	}
}

// The conversation you are sitting in must not be erasable from under you.
func TestTheOpenConversationCannotBeErased(t *testing.T) {
	m := NewSessionsCmp()
	store := rows(3)
	m.SetStore(store, store.Rows[0].Session.ID)
	m.SetSize(100, 30)

	// rows() is newest-first by CreatedAt, and row 0 is the newest.
	after, msg := send(m, "delete", "y")
	if _, ok := msg.(SessionsDeleteMsg); ok {
		t.Fatal("erased the conversation currently open")
	}
	if !strings.Contains(after.View(), "leave it first") {
		t.Errorf("refused without explaining how to proceed")
	}
}

// Sorting by size is how someone on a 2 GB device finds what is worth deleting.
func TestSortBySizePutsTheBiggestFirst(t *testing.T) {
	m := NewSessionsCmp()
	m.SetStore(rows(5), "")
	m.SetSize(100, 30)

	if got, _ := m.Current(); got.Bytes != 1000 {
		t.Fatalf("default sort is not newest-first (top row is %d bytes)", got.Bytes)
	}
	m, _ = send(m, "tab")
	if got, _ := m.Current(); got.Bytes != 5000 {
		t.Errorf("after sorting by size the top row is %d bytes, want the largest (5000)", got.Bytes)
	}
	if !strings.Contains(m.View(), "sorted by size") {
		t.Errorf("the screen does not say which order it is in")
	}
}

func TestEnterRevivesAndCtrlEExports(t *testing.T) {
	m := NewSessionsCmp()
	m.SetStore(rows(3), "")
	m.SetSize(100, 30)

	if _, msg := send(m, "enter"); func() bool { _, ok := msg.(SessionsReviveMsg); return !ok }() {
		t.Errorf("enter did not revive the highlighted session; got %T", msg)
	}
	if _, msg := send(m, "ctrl+e"); func() bool { _, ok := msg.(SessionsExportMsg); return !ok }() {
		t.Errorf("ctrl+e did not export the highlighted session; got %T", msg)
	}
}

// The header carries the number someone compares against their free space. It
// must be the file size, not the sum of the message bodies — measured on the
// developer's machine those were 9.8 MB and 4.77 MB.
func TestHeaderReportsTheSizeOnDiskNotTheContentSize(t *testing.T) {
	m := NewSessionsCmp()
	m.SetStore(SessionsStore{
		Rows:       rows(2).Rows,
		TotalBytes: 4_774_160,
		FileBytes:  9_826_688,
		Sessions:   2, Helpers: 45, Msg: 578,
	}, "")
	m.SetSize(120, 30)

	view := m.View()
	if !strings.Contains(view, "9.4 MB") {
		t.Errorf("the header does not show what the database occupies on disk:\n%s", view)
	}
	if strings.Contains(view, "4.6 MB") {
		t.Errorf("the header shows the content size, which is not what the device sees")
	}
	if !strings.Contains(view, "45 helper sessions") {
		t.Errorf("helper sessions are invisible, and they hold most of a research run's bytes")
	}
}

// Session titles are generated from the first message, so they are long almost
// always. The first live run showed every row with its size truncated away —
// the one column the erase decision is made from.
func TestALongTitleNeverPushesTheSizeOffTheRow(t *testing.T) {
	long := strings.Repeat("a very long generated session title ", 5)
	m := NewSessionsCmp()
	m.SetStore(SessionsStore{Rows: []SessionRow{{
		Session: session.Session{ID: "x", Title: long, CreatedAt: 1_755_000_000},
		Bytes:   2_700_000, Msgs: 275,
	}}}, "")

	for _, w := range []int{60, 80, 100, 130} {
		m.SetSize(w, 30)
		view := m.View()
		if !strings.Contains(view, "2.6 MB") {
			t.Errorf("at width %d the size column was truncated away:\n%s", w, view)
		}
		if !strings.Contains(view, "275 msg") {
			t.Errorf("at width %d the message count was truncated away", w)
		}
	}
}

// Rune count is not display width. A real title in the developer's store holds
// U+FFFC, and titles from the people this is built for hold CJK and emoji —
// 中文 is two runes and FOUR columns, 😀 is one rune and two.
//
// The symptom is HEIGHT, not width: lipgloss WRAPS a line wider than its Width,
// so every resulting line measures under the frame and a width assertion sails
// past the bug while the row silently becomes two. Assert against an ASCII
// baseline instead — same number of rows, or the row wrapped.
func TestWideCharacterTitlesDoNotWrapARow(t *testing.T) {
	const width = 80
	render := func(title string) string {
		m := NewSessionsCmp()
		m.SetStore(SessionsStore{Rows: []SessionRow{{
			Session: session.Session{ID: "x", Title: title, CreatedAt: 1_755_000_000},
			Bytes:   2_700_000, Msgs: 275,
		}}}, "")
		m.SetSize(width, 30)
		return m.View()
	}

	baseline := render(strings.Repeat("ascii title ", 12))
	wide := render("￼DSML￼ " + strings.Repeat("中文 😀 ", 12))

	if got, want := lipgloss.Height(wide), lipgloss.Height(baseline); got != want {
		t.Errorf("a title with double-width characters made the screen %d rows instead of %d — "+
			"the row wrapped, and a wrapped row is a stranded row:\n%s", got, want, wide)
	}
	for i, line := range strings.Split(wide, "\n") {
		if got := lipgloss.Width(line); got > width {
			t.Errorf("line %d is %d columns in an %d-column terminal", i+1, got, width)
		}
	}
	if !strings.Contains(wide, "2.6 MB") {
		t.Errorf("the size was truncated away by a title measured in the wrong unit")
	}
}

// bubbletea coalesces fast input into a single KeyMsg carrying several runes,
// and a paste always arrives that way. Accepting only single-rune messages made
// the search box do nothing when typed into at speed — found by driving the
// real binary, not by any test that existed at the time.
func TestPastedAndFastTypedSearchTextIsNotDropped(t *testing.T) {
	m := NewSessionsCmp()
	m.SetStore(rows(4), "")
	m.SetSize(100, 30)

	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("kernel")})
	m = next.(SessionsCmp)

	if m.Search() != "kernel" {
		t.Fatalf("a six-rune key message left the search box holding %q", m.Search())
	}
	if got, ok := m.Current(); !ok || !strings.Contains(got.Session.Title, "kernel") {
		t.Errorf("the multi-rune search did not filter the list")
	}
}

// This repo already recorded which keys are unusable — provider_escape_test.go
// lists ctrl+d as EOF, ctrl+q/ctrl+s as flow control, ctrl+z as suspend — and
// the sessions manager was built with three of them anyway. Two were found only
// by driving the real binary and watching nothing happen.
//
// This pins the outcome: the manager's actions must not sit on a key the
// terminal or this app has already claimed.
func TestManagerActionsAvoidReservedKeys(t *testing.T) {
	reserved := map[tea.KeyType]string{
		tea.KeyCtrlD: "EOF",
		tea.KeyCtrlS: "flow control (XOFF), and this app's session switcher",
		tea.KeyCtrlQ: "flow control (XON)",
		tea.KeyCtrlZ: "suspend",
		tea.KeyCtrlC: "SIGINT",
	}

	for kt, why := range reserved {
		m := NewSessionsCmp()
		m.SetStore(rows(3), "")
		m.SetSize(100, 30)

		next, cmd := m.Update(tea.KeyMsg{Type: kt})
		after := next.(SessionsCmp)

		if cmd != nil {
			if msg := cmd(); msg != nil {
				switch msg.(type) {
				case SessionsDeleteMsg, SessionsExportMsg, SessionsReviveMsg:
					t.Errorf("an action is bound to a key that is %s — it will not arrive", why)
				}
			}
		}
		if strings.Contains(after.View(), "Erase \"") {
			t.Errorf("a key that is %s armed the erase confirmation", why)
		}
	}

	// And the keys that ARE used must still work.
	m := NewSessionsCmp()
	m.SetStore(rows(3), "")
	m.SetSize(100, 30)
	if _, msg := send(m, "delete", "y"); func() bool { _, ok := msg.(SessionsDeleteMsg); return !ok }() {
		t.Errorf("DEL then y did not erase; got %T", msg)
	}
}

// Every action must be reachable. This exists because a resume key was written
// into the help line and the changelog while the `case` that implements it was
// never added — a python edit targeted the wrong indentation and silently did
// nothing, and four rounds of live testing then "disproved" keys that had never
// been bound at all. A test asserting the message is emitted would have caught
// it in one second.
func TestEveryAdvertisedActionIsActuallyWired(t *testing.T) {
	for _, c := range []struct {
		key  string
		want any
	}{
		{"ctrl+r", SessionsResumeMsg{}},
		{"enter", SessionsReviveMsg{}},
		{"ctrl+e", SessionsExportMsg{}},
	} {
		m := NewSessionsCmp()
		m.SetStore(rows(3), "")
		m.SetSize(100, 30)

		_, msg := send(m, c.key)
		if msg == nil {
			t.Errorf("%s emitted nothing — the action is advertised but not wired", c.key)
			continue
		}
		if fmt.Sprintf("%T", msg) != fmt.Sprintf("%T", c.want) {
			t.Errorf("%s emitted %T, want %T", c.key, msg, c.want)
		}
	}

	// And the help line must name only keys that do something.
	m := NewSessionsCmp()
	m.SetStore(rows(3), "")
	m.SetSize(120, 30)
	view := m.View()
	for _, k := range []string{"enter", "ctrl+r", "ctrl+e", "DEL", "tab"} {
		if !strings.Contains(view, k) {
			t.Errorf("the help line does not mention %q", k)
		}
	}
}
