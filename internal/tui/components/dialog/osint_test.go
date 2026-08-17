package dialog

// GORILLA OVERRIDE: headless render + behaviour tests for the /osint gate and
// its capability page. The gate is the money conversation — the wording, the
// figures and the cancel path are load-bearing, so they are asserted, not
// eyeballed.

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/opencode-ai/opencode/internal/config"
	"github.com/opencode-ai/opencode/internal/llm/agent"
)

func TestOsintGateSaysWhatItCosts(t *testing.T) {
	if _, err := config.Load(t.TempDir(), false); err != nil {
		t.Fatalf("config.Load: %v", err)
	}
	d := NewOsintDialogCmp("test question")
	d.SetSize(130, 50)
	view := d.View()

	// The owner-specified voice and the honest content, all on one screen.
	for _, want := range []string{
		"GORILLA OSINT",
		"bitch",             // the fair warning opens as specified
		"wallet", "funeral", // and owns the consequence
		"walk away (costs nothing)",   // the free exit is stated
		"PARALLEL",                    // parallel is offered…
		"OUTSIDE your working folder", // …and the privacy promise is on screen
	} {
		if !strings.Contains(view, want) {
			t.Errorf("gate screen missing %q:\n%s", want, view)
		}
	}

	// A cost line must exist in SOME form — priced, quota, or an honest
	// UNPRICED. Silence about money on this screen is the one real failure.
	if !strings.Contains(view, "$") && !strings.Contains(view, "UNPRICED") && !strings.Contains(view, "QUOTA") {
		t.Errorf("gate screen says nothing about cost:\n%s", view)
	}
}

func TestOsintGateChoicesAndBounds(t *testing.T) {
	if _, err := config.Load(t.TempDir(), false); err != nil {
		t.Fatalf("config.Load: %v", err)
	}
	d := NewOsintDialogCmp("q")
	d.SetSize(130, 50)

	// Defaults: parallel, minimum helpers — the cheapest real run.
	if osintModes[d.selected].mode != "parallel" {
		t.Errorf("default mode = %q, want parallel", osintModes[d.selected].mode)
	}
	if d.agents != agent.ResearchMinAgents {
		t.Errorf("default agents = %d, want %d", d.agents, agent.ResearchMinAgents)
	}

	// Left at the floor stays at the floor; right walks to the ceiling and stops.
	m, _ := d.Update(tea.KeyMsg{Type: tea.KeyLeft})
	d = m.(OsintDialogCmp)
	if d.agents != agent.ResearchMinAgents {
		t.Errorf("left below the floor moved agents to %d", d.agents)
	}
	for i := 0; i < 20; i++ {
		m, _ = d.Update(tea.KeyMsg{Type: tea.KeyRight})
		d = m.(OsintDialogCmp)
	}
	if d.agents != agent.ResearchMaxAgents {
		t.Errorf("right past the ceiling reached %d, want cap %d", d.agents, agent.ResearchMaxAgents)
	}

	// Enter carries the exact choice; esc carries Chosen=false.
	m, cmd := d.Update(tea.KeyMsg{Type: tea.KeyEnter})
	d = m.(OsintDialogCmp)
	if cmd == nil {
		t.Fatal("enter produced no command")
	}
	if msg, ok := cmd().(CloseOsintDialogMsg); !ok || !msg.Chosen || msg.Agents != agent.ResearchMaxAgents || msg.Mode != "parallel" {
		t.Errorf("enter carried %+v; want Chosen=true agents=%d mode=parallel", cmd(), agent.ResearchMaxAgents)
	}
	m, cmd = d.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if cmd == nil {
		t.Fatal("esc produced no command")
	}
	if msg, ok := cmd().(CloseOsintDialogMsg); !ok || msg.Chosen {
		t.Errorf("esc must carry Chosen=false, got %+v — a cancel that runs would spend money", cmd())
	}
}

// Sequential's peak burn must be computed at one helper in flight, not the
// parallel fleet — quoting the wrong direction is still a wrong figure.
func TestOsintSequentialInFlightIsOne(t *testing.T) {
	if _, err := config.Load(t.TempDir(), false); err != nil {
		t.Fatalf("config.Load: %v", err)
	}
	d := NewOsintDialogCmp("q")
	for i := range osintModes {
		if osintModes[i].mode == "sequential" {
			d.selected = i
		}
	}
	if got := d.inFlight(); got != 1 {
		t.Errorf("sequential inFlight = %d, want 1", got)
	}
}

func TestOsintPageRendersAndScrolls(t *testing.T) {
	if _, err := config.Load(t.TempDir(), false); err != nil {
		t.Fatalf("config.Load: %v", err)
	}
	p := NewOsintPageCmp()
	p.SetSize(120, 30)
	view := p.View()

	// Content assertions run against the SOURCE, not one 30-row window — the
	// page scrolls, and "missing" must mean absent, not merely below the fold.
	var all strings.Builder
	for _, l := range osintContent() {
		all.WriteString(l.text + "\n")
	}
	for _, want := range []string{
		"GORILLA OSINT",
		"STATUS:",              // armed/off state is live, not prose
		"never cites a source", // the iron rule
		"git repositor",        // the privacy reason, stated
	} {
		if !strings.Contains(all.String(), want) {
			t.Errorf("page content missing %q", want)
		}
	}
	// And the first window opens with the title.
	if !strings.Contains(view, "GORILLA OSINT") {
		t.Errorf("first window does not open with the title")
	}
	// The page is longer than 30 rows, so the more-lines marker must show and
	// scrolling must change the window.
	if !strings.Contains(view, "more line") {
		t.Errorf("no scroll marker on a 30-row terminal")
	}
	m, _ := p.Update(tea.KeyMsg{Type: tea.KeyPgDown})
	p2 := m.(OsintPageCmp)
	if p2.View() == view {
		t.Errorf("PgDn did not change the visible window")
	}

	// esc closes.
	_, cmd := p2.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if cmd == nil {
		t.Fatal("esc produced no command")
	}
	if _, ok := cmd().(CloseOsintPageMsg); !ok {
		t.Errorf("esc did not close the page: %+v", cmd())
	}
}
