package tui

// GORILLA (2026-10-10): typing /editor, /helpers or /hooks (or an alias) in the
// full interface opens that command's page. Driven through Update with the
// same message the editor sends, so the dispatch switch itself is exercised,
// not only the helper it calls.

import (
	"strings"
	"testing"

	"github.com/opencode-ai/opencode/internal/tui/components/chat"
	"github.com/opencode-ai/opencode/internal/tui/components/dialog"
)

func TestInfoPageCommandsOpenTheirPage(t *testing.T) {
	// /helpers reads the helper limit from the settings folder; point it at an
	// empty one rather than the developer's.
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	for typed, want := range map[string]string{
		"editor": "editor", "acp": "editor",
		"helpers": "helpers", "roles": "helpers",
		"hooks": "hooks",
	} {
		a := appModel{width: 100, height: 30}
		next, _ := a.Update(chat.SlashCommandMsg{Name: typed})
		got := next.(appModel)
		if !got.showInfoPage {
			t.Errorf("/%s did not open a page", typed)
			continue
		}
		if name := got.infoPage.Page().Name; name != want {
			t.Errorf("/%s opened the %q page, want %q", typed, name, want)
		}
		if !got.anyOverlayOpen() {
			t.Errorf("/%s: the page is open but not counted as an overlay", typed)
		}
		if v := got.infoPage.View(); !strings.Contains(v, got.infoPage.Page().Title) {
			t.Errorf("/%s: the page does not show its title", typed)
		}

		// esc closes it again.
		closed, _ := got.Update(dialog.CloseInfoPageMsg{})
		if closed.(appModel).showInfoPage {
			t.Errorf("/%s: CloseInfoPageMsg left the page open", typed)
		}
	}
}
