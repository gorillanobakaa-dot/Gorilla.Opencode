package dialog

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/opencode-ai/opencode/internal/config"
	"github.com/opencode-ai/opencode/internal/llm/models"
)

// GORILLA OVERRIDE: tab opens a full page per model. One row cannot hold an
// informed decision; the page holds the full description, the exact prices,
// and — the fact no row could show — WHICH credential serves the request.

func TestTabOpensTheDetailPage(t *testing.T) {
	mockCatalogue(t)
	m := &modelDialogCmp{}
	m.SwitchToProvider(models.ProviderMock)
	m.width, m.height = 120, 40

	m.Update(tea.KeyMsg{Type: tea.KeyTab})
	if m.detail == nil {
		t.Fatal("tab must open the detail page for the highlighted model")
	}

	view := m.View()
	for _, want := range []string{
		"Alpha Coder",          // the model, by name
		"id: mock.alpha",       // the raw id, for config files and bug reports
		"served via",           // the connection — the fact the list cannot show
		"made by",              // vendor ≠ biller
		"mocklab",              // the vendor, from the api id
		"price",                // stated even when FREE
		"software engineering", // the Detail long text, no longer truncated
	} {
		if !strings.Contains(view, want) {
			t.Errorf("detail page must contain %q; view:\n%s", want, view)
		}
	}
}

// The detail page names the key by fingerprint and must NEVER contain the
// credential. A live key went into a transcript once in this project; picker
// frames are exactly what gets screenshotted into chats.
func TestDetailPageNeverShowsTheAPIKey(t *testing.T) {
	mockCatalogue(t)
	const rawKey = "mk-test-0123456789abcdef0123456789" // as set by mockCatalogue
	m := &modelDialogCmp{}
	m.SwitchToProvider(models.ProviderMock)
	m.width, m.height = 120, 40

	m.Update(tea.KeyMsg{Type: tea.KeyTab})
	view := m.View()

	if strings.Contains(view, rawKey) || strings.Contains(view, rawKey[8:20]) {
		t.Fatal("detail page leaked the API key")
	}
	if !strings.Contains(view, "chars") {
		t.Errorf("detail page should identify the key by fingerprint; view:\n%s", view)
	}
}

func TestDetailEnterSelectsAndEscReturns(t *testing.T) {
	mockCatalogue(t)
	m := &modelDialogCmp{}
	m.SwitchToProvider(models.ProviderMock)

	// esc closes the page, not the dialog
	m.Update(tea.KeyMsg{Type: tea.KeyTab})
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if m.detail != nil {
		t.Fatal("esc must close the detail page")
	}
	if cmd != nil {
		if _, closed := cmd().(CloseModelDialogMsg); closed {
			t.Fatal("esc on the detail page must not close the whole dialog")
		}
	}

	// enter uses the detailed model
	m.Update(tea.KeyMsg{Type: tea.KeyTab})
	_, cmd = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("enter on the detail page must select")
	}
	sel, ok := cmd().(ModelSelectedMsg)
	if !ok {
		t.Fatalf("expected ModelSelectedMsg, got %T", cmd())
	}
	if sel.Model.ID != "mock.alpha" {
		t.Fatalf("selected %q, want the detailed model", sel.Model.ID)
	}
	if m.detail != nil {
		t.Error("selection must close the detail page — the dialog object is reused")
	}
}

func TestDetailSpaceTogglesBookmark(t *testing.T) {
	mockCatalogue(t)
	m := &modelDialogCmp{}
	m.SwitchToProvider(models.ProviderMock)

	m.Update(tea.KeyMsg{Type: tea.KeyTab})
	m.Update(tea.KeyMsg{Type: tea.KeySpace})
	if !config.IsBookmarked("mock.alpha") {
		t.Fatal("space on the detail page must bookmark the model")
	}
	if m.detail == nil {
		t.Fatal("bookmarking must not close the detail page")
	}
	// And a second press must un-bookmark the SAME model, even though the
	// column rebuild underneath may have moved the list selection.
	m.Update(tea.KeyMsg{Type: tea.KeySpace})
	if config.IsBookmarked("mock.alpha") {
		t.Fatal("second space must remove the bookmark from the same model")
	}
}

// Search + detail from the search results: tab must show the row the cursor
// is on, and returning lands back in the search, query intact.
func TestDetailFromSearchReturnsToSearch(t *testing.T) {
	mockCatalogue(t)
	m := &modelDialogCmp{}
	m.SwitchToProvider(models.ProviderMock)
	typeRunes(m, "/")
	typeRunes(m, "reasoning")

	m.Update(tea.KeyMsg{Type: tea.KeyTab})
	if m.detail == nil || m.detail.ID != "mock.beta" {
		t.Fatalf("tab in search must detail the highlighted match, got %v", m.detail)
	}
	m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if m.detail != nil {
		t.Fatal("esc must close the detail page")
	}
	if !m.searchActive || m.query != "reasoning" {
		t.Fatalf("must return to the search as it was left: active=%v query=%q", m.searchActive, m.query)
	}
}

// NO LINE IN THE FRAME MAY BE WIDER THAN THE TERMINAL — the root cause of the
// marching-footer bug. Both new views hold the invariant at every size the
// screen tests use, including with a long Detail text forcing wraps.
func TestSearchAndDetailViewsNeverExceedTerminalWidth(t *testing.T) {
	mockCatalogue(t)
	long := models.SupportedModels["mock.alpha"]
	long.Detail = strings.Repeat("a very long vendor paragraph about capabilities and context windows ", 30)
	models.SupportedModels["mock.alpha"] = long

	for _, size := range []struct{ w, h int }{
		{60, 10}, {80, 24}, {100, 30}, {120, 40}, {160, 50}, {40, 8},
	} {
		m := &modelDialogCmp{}
		m.SwitchToProvider(models.ProviderMock)
		m.width, m.height = size.w, size.h

		typeRunes(m, "/")
		for i, line := range strings.Split(m.View(), "\n") {
			if got := lipgloss.Width(line); got > size.w {
				t.Errorf("%dx%d search view line %d is %d cells wide", size.w, size.h, i, got)
			}
		}
		m.Update(tea.KeyMsg{Type: tea.KeyTab})
		lines := strings.Split(m.View(), "\n")
		for i, line := range lines {
			if got := lipgloss.Width(line); got > size.w {
				t.Errorf("%dx%d detail view line %d is %d cells wide", size.w, size.h, i, got)
			}
		}
		// The house line (see screen_invariants_test.go): no dialog may
		// overflow a terminal 24 rows or taller. Below that the list view
		// itself already needs its 14-row floor.
		if size.h >= standardHeight && len(lines) > size.h {
			t.Errorf("%dx%d detail view is %d rows tall — taller than the window", size.w, size.h, len(lines))
		}
	}
}
