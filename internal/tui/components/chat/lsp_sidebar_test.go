package chat

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/opencode-ai/opencode/internal/config"
	"github.com/opencode-ai/opencode/internal/llm/models"
)

func loadWithLSPs(t *testing.T, names ...string) {
	t.Helper()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	if _, err := config.Load(t.TempDir(), false); err != nil {
		t.Fatalf("config.Load: %v", err)
	}
	cfg := config.Get()

	prevLSP := cfg.LSP
	prevComponents := config.LoadoutComponents
	t.Cleanup(func() {
		cfg.LSP = prevLSP
		config.LoadoutComponents = prevComponents
	})

	cfg.LSP = map[string]config.LSPConfig{}
	reg := map[string]bool{}
	for _, n := range names {
		cfg.LSP[n] = config.LSPConfig{Command: n + "-server"}
		reg[n] = false
	}
	config.RegisterLSPComponents(reg)
	_ = models.ProviderLocal // keep the import honest across refactors
}

// The reported bug: "although I have disabled all LSPs from /context those
// servers are still being displayed, which leads me to believe they don't get
// disabled." They DO get disabled — with all nine off, zero language-server
// processes spawn — but this panel iterated cfg.LSP raw and listed them exactly
// as before, which is indistinguishable from a switch that does nothing.
func TestSidebarHidesDisabledLanguageServers(t *testing.T) {
	loadWithLSPs(t, "c", "cpp", "go")

	base := lipgloss.NewStyle()
	before := lspsConfigured(60, base)
	for _, n := range []string{"c", "cpp", "go"} {
		if !strings.Contains(before, n) {
			t.Fatalf("setup: %q missing while all servers are enabled:\n%s", n, before)
		}
	}

	if n := config.SetAllLSPs(false); n == 0 {
		t.Fatal("setup: nothing was disabled")
	}
	after := lspsConfigured(60, base)

	// The command names are what identify a listed server; the language names
	// legitimately reappear in the "N off" note.
	for _, n := range []string{"c-server", "cpp-server", "go-server"} {
		if strings.Contains(after, n) {
			t.Errorf("%q is still listed after being disabled:\n%s", n, after)
		}
	}
	// But it must say they exist and are off, or a fully disabled setup looks
	// unconfigured instead of deliberately quiet.
	if !strings.Contains(after, "off") {
		t.Errorf("the panel does not say anything is switched off:\n%s", after)
	}
	if !strings.Contains(after, "/context") {
		t.Errorf("the panel does not say how to change it:\n%s", after)
	}
}

// A mixed state must show the running ones and count the rest.
func TestSidebarShowsEnabledAndCountsDisabled(t *testing.T) {
	loadWithLSPs(t, "c", "cpp", "go")
	config.SetAllLSPs(true)
	config.ToggleLoadout(config.LSPComponentID("cpp")) // exactly one off

	view := lspsConfigured(60, lipgloss.NewStyle())

	if !strings.Contains(view, "c-server") || !strings.Contains(view, "go-server") {
		t.Errorf("an enabled server is missing:\n%s", view)
	}
	if strings.Contains(view, "cpp-server") {
		t.Errorf("the disabled server is still listed:\n%s", view)
	}
	if !strings.Contains(view, "1 off") {
		t.Errorf("want a \"1 off\" note, got:\n%s", view)
	}
}

// Width discipline, as for every other panel: chrome is subtracted from the
// available width, never added to the content.
func TestSidebarLSPPanelRespectsWidth(t *testing.T) {
	loadWithLSPs(t, "c", "cpp", "go", "rust", "typescript")
	config.SetAllLSPs(true)

	for _, w := range []int{20, 30, 40, 60, 80} {
		view := lspsConfigured(w, lipgloss.NewStyle())
		for i, l := range strings.Split(view, "\n") {
			if got := lipgloss.Width(l); got > w {
				t.Errorf("width=%d: line %d is %d columns wide: %q", w, i, got, l)
			}
		}
	}
}
