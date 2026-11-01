package dialog

import (
	"fmt"
	"os"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/opencode-ai/opencode/internal/arsenal"
	"github.com/opencode-ai/opencode/internal/toolbox"
)

func builtModel(platform string) ArsenalCmp {
	m := NewArsenalCmp()
	m.platform = platform
	m.built = toolbox.Pinned(platform)
	m.SetSize(120, 60)
	return m
}

func texts(ls []arsLine) string {
	var b strings.Builder
	for _, l := range ls {
		b.WriteString(l.text + "\n")
	}
	return b.String()
}

// Every helper pinned for the system is a line: name, version, what it is for.
func TestBuiltInPageListsEveryHelperOneLineEach(t *testing.T) {
	for _, plat := range toolbox.Platforms() {
		m := builtModel(plat)
		lines := m.builtInLines()
		if !strings.Contains(texts(lines), "BUILT IN (ready now)") {
			t.Fatalf("%s: no Built in heading", plat)
		}
		for _, h := range toolbox.Pinned(plat) {
			found := false
			for _, l := range lines {
				if strings.Contains(l.text, h.Name+" ") && strings.Contains(l.text, h.VersionOn(plat)) && strings.Contains(l.text, h.Purpose) {
					found = true
				}
			}
			if !found {
				t.Errorf("%s: no single line carries %s, its version %s and its purpose", plat, h.Name, h.VersionOn(plat))
			}
		}
	}
}

func TestBuiltInPageSaysPlainlyWhenThisBuildCarriesNone(t *testing.T) {
	m := builtModel("windows-amd64")
	m.built = nil
	if out := texts(m.builtInLines()); !strings.Contains(out, "carries no built-in helpers") {
		t.Fatalf("a build without helpers must say so:\n%s", out)
	}
}

func TestBuiltInPageMacNamesTheHelpersNeedingMacOS11(t *testing.T) {
	out := texts(builtModel("darwin-amd64").builtInLines())
	if !strings.Contains(out, "ffmpeg needs macOS 11.0") || !strings.Contains(out, "ffprobe needs macOS 11.0") {
		t.Fatalf("macOS page must name ffmpeg and ffprobe as needing macOS 11:\n%s", out)
	}
	if strings.Contains(texts(builtModel("linux-amd64").builtInLines()), "needs macOS") {
		t.Fatal("the macOS note must not appear on Linux")
	}
}

func TestBuiltInPageShowsAddableWithOCRFallback(t *testing.T) {
	m := builtModel("linux-amd64")
	m.status["tesseract"] = arsenal.Status{}
	out := texts(m.builtInLines())
	for _, want := range []string{"tesseract - not installed", "imagemagick - ", "falls back to a model that can see images"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in:\n%s", want, out)
		}
	}
}

func TestArsenalOpensOnBuiltInAndEnterGoesToTheList(t *testing.T) {
	m := builtModel("windows-amd64").OpenOnBuiltIn()
	if m.view != viewBuiltIn || !strings.Contains(m.View(), "BUILT IN (ready now)") {
		t.Fatal("must open on the Built in page")
	}
	var model tea.Model = m
	model, _ = model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if model.(ArsenalCmp).view != viewSeries {
		t.Fatal("enter must go on to the series list")
	}
	model, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("b")})
	if model.(ArsenalCmp).view != viewBuiltIn {
		t.Fatal("b must come back to the Built in page")
	}
	_, cmd := model.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if cmd == nil {
		t.Fatal("esc on the Built in page must close")
	}
}

func TestBuiltInPageFitsTheTerminal(t *testing.T) {
	for _, sz := range [][2]int{{60, 20}, {80, 24}, {100, 30}} {
		m := builtModel("windows-amd64").OpenOnBuiltIn()
		m.SetSize(sz[0], sz[1])
		v := m.View()
		if w := lipgloss.Width(v); w > sz[0] {
			t.Errorf("%dx%d: frame %d wide", sz[0], sz[1], w)
		}
		if h := lipgloss.Height(v); h > sz[1] {
			t.Errorf("%dx%d: frame %d tall", sz[0], sz[1], h)
		}
	}
}

func TestContextLineSaysBuiltInAndInstallable(t *testing.T) {
	if got := ArsenalCountsLine(13, 5); got != "13 built-in helpers, 5 installable (type /arsenal to see them)" {
		t.Fatal(got)
	}
	d := NewLoadoutDialogCmp().(*loadoutDialogCmp)
	d.arsenalText = ArsenalCountsLine(13, 5)
	d.Update(tea.WindowSizeMsg{Width: 130, Height: 60})
	if !strings.Contains(d.View(), "13 built-in helpers, 5 installable") {
		t.Fatal("/context does not show the count")
	}
}

func TestHelpNamesArsenal(t *testing.T) {
	h := NewCommandHelpCmp().(*commandHelpCmp)
	h.Init()
	h.SetSize(140, 50)
	if !strings.Contains(h.View(), "/arsenal: tools") {
		t.Fatal("/help must name /arsenal as the place to see tools")
	}
}

// The one test of the definition: every helper pinned in SHA256SUMS, on every system, is on the arsenal page,
// counted by /context, described for the release page (helpers.json) and named by the everyday card; /help names
// /arsenal. Remove any of them and this fails.
func TestEveryPinnedHelperIsVisible(t *testing.T) {
	card, err := os.ReadFile("../../../howto/cards/everyday.md")
	if err != nil {
		t.Fatal(err)
	}
	cat, err := toolbox.Catalog()
	if err != nil {
		t.Fatal(err)
	}
	described := map[string]bool{}
	for _, h := range cat {
		described[h.Name] = h.Purpose != "" && h.Version != ""
	}
	for _, plat := range toolbox.Platforms() {
		pinned := toolbox.Pinned(plat)
		if len(pinned) == 0 {
			t.Fatalf("%s: nothing pinned", plat)
		}
		page := texts(builtModel(plat).builtInLines())
		if !strings.Contains(ArsenalCountsLine(len(pinned), 0), fmt.Sprintf("%d built-in helpers", len(pinned))) {
			t.Errorf("%s: /context count line", plat)
		}
		for _, h := range pinned {
			if !strings.Contains(page, h.Name+" ") {
				t.Errorf("%s: %s missing from /arsenal", plat, h.Name)
			}
			if !described[h.Name] {
				t.Errorf("%s: %s not described in helpers.json (release page)", plat, h.Name)
			}
			if !strings.Contains(string(card), h.Name) {
				t.Errorf("%s: %s not named by the everyday card (/help)", plat, h.Name)
			}
		}
	}
	TestHelpNamesArsenal(t)
}
