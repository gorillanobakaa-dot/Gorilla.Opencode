package dialog

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/opencode-ai/opencode/internal/config"
	"github.com/opencode-ai/opencode/internal/llm/agent"
	"github.com/opencode-ai/opencode/internal/llm/models"
)

// GORILLA (2026-10-25): the /research chooser fits every supported terminal
// WITH this machine's measured history present, which is the state of every
// user after their first run, and the state the old fit test never saw on CI.
//
// On the owner's laptop (20 helper timings, 3 recorded runs) the compact form
// was 32 rows on 90x30: the timing row was MEASURED while the run size was not,
// so the dialog printed both the MEASURED timing row and the ASSUMED row, and
// nothing past compact gave way. The sizes are the owner's: 80x24 is the
// smallest supported terminal, 90x30 the one that failed, the MacBook Air's
// 138x37, 144x33 and 169x43, and the wide 176x48.
var researchFitSizes = []struct{ w, h int }{
	{80, 24}, {90, 30}, {100, 32}, {120, 40}, {138, 37}, {144, 33}, {169, 43}, {176, 48},
}

// chatModelOtherThanTheHelper pins the chat model to one that is not the
// helper model, which adds the HELPERS RUN ON block: the tallest shape. The
// name is as long as the owner's LM Studio label, so the row wraps where it
// wrapped on the laptop. This used to come from whatever LM Studio served on
// the test machine, so CI never rendered it.
func chatModelOtherThanTheHelper(t *testing.T, provider models.ModelProvider) {
	t.Helper()
	id := models.ModelID("local.test/fit-chat-" + string(provider))
	models.SupportedModels[id] = models.Model{
		ID: id, Name: "Gemma 4 [google/gemma-4-12b-qat]", Provider: provider,
		ContextWindow: 128000, DefaultMaxTokens: 4096,
	}
	if provider == models.ProviderLocal {
		models.RegisterLocalRouteForTestNamed(id, "http://127.0.0.1:1/v1", "k", "test-chat-endpoint")
	}
	t.Cleanup(func() {
		delete(models.SupportedModels, id)
		models.ClearLocalRouteForTest(id)
	})
	if err := config.UpdateAgentModel(config.AgentCoder, id); err != nil {
		t.Fatalf("pin the chat model: %v", err)
	}
	if h, c, ok := config.ResearchModelChoice(); !ok || h.ID == c.ID {
		t.Fatalf("setup: helper and chat model are not distinct (ok=%v helper=%s chat=%s)", ok, h.ID, c.ID)
	}
}

// withHistory sets this test's measured history: timed adds helper durations
// (in memory, as a finished run leaves them), runs records finished runs in
// the test's own cache directory.
func withHistory(t *testing.T, timed, runs bool) {
	t.Helper()
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	config.ResetHelperTimingForTest()
	t.Cleanup(config.ResetHelperTimingForTest)
	if timed {
		for i := 0; i < 20; i++ {
			config.RecordHelperDuration(time.Duration(100+i*2) * time.Second)
		}
		if _, n, ok := config.MeasuredSecondsPerHelper(); !ok || n != 20 {
			t.Fatalf("setup: helper timing not measured (n=%d ok=%v)", n, ok)
		}
	}
	if runs {
		for _, in := range []int64{60000, 90000, 120000} {
			config.RecordResearchRun(4, in, 3000, 20, 90*time.Second)
		}
		if _, _, n, ok := config.MeasuredRunSize(); !ok || n != 3 {
			t.Fatalf("setup: run size not measured (n=%d ok=%v)", n, ok)
		}
	}
}

// plainRows is the view without colour codes, one entry per screen row.
func plainRows(v string) []string { return strings.Split(ansi.Strip(v), "\n") }

// oneRowWith reports whether a single screen row carries every part. A key
// hint split across two rows reads as two half-instructions.
func oneRowWith(v string, parts ...string) bool {
	for _, r := range plainRows(v) {
		all := true
		for _, p := range parts {
			all = all && strings.Contains(r, p)
		}
		if all {
			return true
		}
	}
	return false
}

func TestTheResearchDialogFitsEverySupportedScreenWithMeasuredHistory(t *testing.T) {
	// "" leaves the chat model unpinned (no HELPERS RUN ON block); local is the
	// owner's shape; gemini adds the DIFFERENT PROVIDER row as well.
	for _, chat := range []models.ModelProvider{"", models.ProviderLocal, models.ProviderGemini} {
		for _, h := range []struct{ timed, runs bool }{{false, false}, {true, false}, {false, true}, {true, true}} {
			pricedConfig(t)
			if chat != "" {
				chatModelOtherThanTheHelper(t, chat)
			}
			withHistory(t, h.timed, h.runs)
			for _, mode := range []string{"sequential", "parallel", "supervised"} {
				for _, agents := range []int{agent.ResearchMinAgents, agent.ResearchMaxAgents} {
					for _, sz := range researchFitSizes {
						name := fmt.Sprintf("%dx%d %s x%d chat=%q timed=%v runs=%v", sz.w, sz.h, mode, agents, chat, h.timed, h.runs)
						m := dialogAt(mode, agents)
						m.SetSize(sz.w, sz.h)
						v := m.View()
						if got := lipgloss.Height(v); got > sz.h {
							t.Errorf("%s: dialog is %d rows, taller than the screen\n%s", name, got, ansi.Strip(v))
							continue
						}
						if got := lipgloss.Width(v); got > sz.w {
							t.Errorf("%s: dialog is %d columns, wider than the screen", name, got)
						}
						// What is never cut: the way out, the way in, and the choice.
						if !strings.Contains(ansi.Strip(v), "enter: go") || !strings.Contains(ansi.Strip(v), "esc: cancel") {
							t.Errorf("%s: key hints missing\n%s", name, ansi.Strip(v))
						}
						if !oneRowWith(v, "esc: cancel") {
							t.Errorf("%s: \"esc: cancel\" is split across rows", name)
						}
						if !oneRowWith(v, "Helpers: <-") || !oneRowWith(v, "▸") {
							t.Errorf("%s: the helper selector or the selected mode is missing", name)
						}
					}
				}
			}
		}
	}
}

// The tallest shape on the smallest screen: the money and the model it is
// charged on survive the shedding, and nothing is clipped while shedding was
// enough. A dialog that fits by hiding the price fits nothing worth showing.
func TestShortScreensKeepTheMoneyAndTheHelperModel(t *testing.T) {
	pricedConfig(t)
	chatModelOtherThanTheHelper(t, models.ProviderLocal)
	withHistory(t, true, false) // the owner's shape on 2026-10-25
	for _, sz := range []struct{ w, h int }{{80, 24}, {90, 30}} {
		m := dialogAt("supervised", agent.ResearchMaxAgents)
		m.SetSize(sz.w, sz.h)
		v := ansi.Strip(m.View())
		for _, want := range []string{"PER MINUTE", "THIS RUN", "HELPERS RUN ON", "NOTHING WORKS", "ASSUMED"} {
			if !strings.Contains(v, want) {
				t.Errorf("%dx%d: %q was shed; it must never be\n%s", sz.w, sz.h, want, v)
			}
		}
		if strings.Contains(v, "rows hidden") {
			t.Errorf("%dx%d: the cost block was clipped although shedding could fit it\n%s", sz.w, sz.h, v)
		}
	}
}

// Shedding happens only where it is needed: on a screen the dialog fits, every
// row is there.
func TestTallScreensShedNothing(t *testing.T) {
	pricedConfig(t)
	chatModelOtherThanTheHelper(t, models.ProviderLocal)
	withHistory(t, true, false)
	m := dialogAt("supervised", agent.ResearchMaxAgents)
	m.SetSize(176, 48)
	v := ansi.Strip(m.View())
	for _, want := range []string{"WHAT THIS COSTS", "QUOTA", "Only 8 of 10 lanes are audited", "s per helper, median of your last 20", "audits each blind lane"} {
		if !strings.Contains(v, want) {
			t.Errorf("176x48: %q missing on a screen the dialog fits\n%s", want, v)
		}
	}
}

// Below the smallest supported screen the dialog still keeps its key line, and
// says that rows are hidden rather than dropping them in silence.
func TestBelowTheSmallestScreenTheDialogClipsAndSaysSo(t *testing.T) {
	pricedConfig(t)
	chatModelOtherThanTheHelper(t, models.ProviderLocal)
	withHistory(t, true, false)
	m := dialogAt("supervised", agent.ResearchMaxAgents)
	m.SetSize(80, 16)
	v := m.View()
	if got := lipgloss.Height(v); got > 16 {
		t.Fatalf("80x16: dialog is %d rows\n%s", got, ansi.Strip(v))
	}
	plain := ansi.Strip(v)
	if !strings.Contains(plain, "esc: cancel") || !strings.Contains(plain, "rows hidden") {
		t.Errorf("80x16: want the key line and the hidden-rows note\n%s", plain)
	}
}

// The last shed step gives back the blank rows inside the border, which are
// two rows of nothing; before it they stay, because they frame the dialog.
func TestTheLastShedStepGivesBackTheOuterBlankRows(t *testing.T) {
	pricedConfig(t)
	withHistory(t, false, false)
	m := dialogAt("supervised", agent.ResearchMaxAgents)
	m.SetSize(80, 24)
	m.compact = true
	for shed, wantBlank := range map[int]bool{0: true, researchMaxShed - 1: true, researchMaxShed: false} {
		m.shed = shed
		rows := plainRows(m.render())
		inner := strings.Trim(rows[1], "│ ")
		if blank := inner == ""; blank != wantBlank {
			t.Errorf("shed=%d: row under the top border is %q, want blank=%v", shed, rows[1], wantBlank)
		}
	}
}
