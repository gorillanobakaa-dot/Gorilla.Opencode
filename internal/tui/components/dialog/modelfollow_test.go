package dialog

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/opencode-ai/opencode/internal/config"
	"github.com/opencode-ai/opencode/internal/llm/models"
)

// The always-follow rule is only defensible because of this screen. It drags a
// user's deliberate choice, so it owes them three things, and each is a test:
//
//	1. tell them EXACTLY what moved, in words that mean something
//	2. tell them what it costs — money AND quota
//	3. put it back, exactly, on one key
//
// Without all three it is the silent re-pointing this whole thread was about,
// with extra steps.

func followFixture(t *testing.T) (cheap, dear models.ModelID) {
	t.Helper()
	if _, err := config.Load(t.TempDir(), false); err != nil {
		t.Fatalf("config.Load: %v", err)
	}
	cheap = models.ModelID("local.test/cheap-flash")
	dear = models.ModelID("local.test/dear-thinking")
	registerAt(t, cheap, "Cheap Flash", models.ProviderLocal, 0.10, 0.40)
	registerAt(t, dear, "Dear Thinking", models.ProviderLocal, 15, 75)
	return cheap, dear
}

func followDialog(t *testing.T, moves []config.AgentModelMove) ModelFollowDialogCmp {
	t.Helper()
	d, _ := NewModelFollowDialogCmp(moves).Update(tea.WindowSizeMsg{Width: 160, Height: 60})
	return d.(ModelFollowDialogCmp)
}

func followText(m ModelFollowDialogCmp) string {
	var b strings.Builder
	for _, l := range m.lines() {
		b.WriteString(l.text)
		b.WriteString("\n")
	}
	return b.String()
}

func TestTheScreenNamesEveryAgentThatMovedInPlainLanguage(t *testing.T) {
	cheap, dear := followFixture(t)
	moves := []config.AgentModelMove{
		{Agent: config.AgentTitle, From: cheap, To: dear},
		{Agent: config.AgentSummarizer, From: cheap, To: dear},
		{Agent: config.AgentTask, From: cheap, To: dear},
		{Agent: config.AgentResearch, From: cheap, To: dear},
	}
	text := followText(followDialog(t, moves))

	// Never the internal agent names on their own — they are our vocabulary.
	for _, plain := range []string{
		"names your chats",
		"squashes the conversation",
		"look things up",
		"deep digging",
	} {
		if !strings.Contains(text, plain) {
			t.Errorf("the screen never explains %q in plain language:\n%s", plain, text)
		}
	}
	if !strings.Contains(text, "Cheap Flash") || !strings.Contains(text, "Dear Thinking") {
		t.Errorf("the screen does not show both the old and new model:\n%s", text)
	}
}

func TestTheScreenStatesTheFinancialImplication(t *testing.T) {
	cheap, dear := followFixture(t)
	text := followText(followDialog(t, []config.AgentModelMove{
		{Agent: config.AgentResearch, From: cheap, To: dear},
	}))
	if !strings.Contains(text, "Per million words in") {
		t.Errorf("no before/after price:\n%s", text)
	}
	// 15 / 0.10 = 150x. The user must be told it went UP, and by how much.
	if !strings.Contains(text, "times") {
		t.Errorf("the screen shows two prices but never says the jobs got more expensive:\n%s", text)
	}
}

// Titles do not get better on a reasoning model. If we drag them there anyway,
// say so — otherwise the screen is a sales page for the change we just made.
func TestTheScreenAdmitsWhichMovesAreWaste(t *testing.T) {
	cheap, dear := followFixture(t)
	text := followText(followDialog(t, []config.AgentModelMove{
		{Agent: config.AgentTitle, From: cheap, To: dear},
		{Agent: config.AgentResearch, From: cheap, To: dear},
	}))
	if !strings.Contains(text, "PROBABLY NOT WORTH IT") {
		t.Errorf("dragging session-naming onto an expensive model is waste and the screen does not admit it:\n%s", text)
	}
}

// A free-tier user has no dollar figure, and telling them "no cost change" would
// be a lie — they are burning quota faster.
func TestFlatRateModelsAreReportedAsQuotaNotMoney(t *testing.T) {
	if _, err := config.Load(t.TempDir(), false); err != nil {
		t.Fatalf("config.Load: %v", err)
	}
	a := models.ModelID("local.test/free-a")
	b := models.ModelID("local.test/free-b")
	registerAt(t, a, "Free A", models.ProviderLocal, 0, 0)
	registerAt(t, b, "Free B", models.ProviderLocal, 0, 0)

	text := followText(followDialog(t, []config.AgentModelMove{
		{Agent: config.AgentResearch, From: a, To: b},
	}))
	if !strings.Contains(text, "QUOTA") {
		t.Errorf("both models are flat-rate; the screen must talk about quota, not money:\n%s", text)
	}
}

// THE LOAD-BEARING TEST. Dragging a deliberate choice is only acceptable
// because it is perfectly reversible. If r does not restore, the feature is the
// silent overwrite it was meant to replace.
func TestPressingRRestoresEveryAgentExactly(t *testing.T) {
	cheap, dear := followFixture(t)
	other := models.ModelID("local.test/third-model")
	registerAt(t, other, "Third Model", models.ProviderLocal, 1, 2)

	// Deliberately DIFFERENT starting models, so a lazy revert that puts
	// everything back to one value cannot pass.
	if err := config.UpdateAgentModel(config.AgentTitle, cheap); err != nil {
		t.Fatal(err)
	}
	if err := config.UpdateAgentModel(config.AgentTask, other); err != nil {
		t.Fatal(err)
	}
	moves := []config.AgentModelMove{
		{Agent: config.AgentTitle, From: cheap, To: dear},
		{Agent: config.AgentTask, From: other, To: dear},
	}
	if err := config.UpdateAgentModel(config.AgentTitle, dear); err != nil {
		t.Fatal(err)
	}
	if err := config.UpdateAgentModel(config.AgentTask, dear); err != nil {
		t.Fatal(err)
	}

	m := followDialog(t, moves)
	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'r'}})
	if cmd == nil {
		t.Fatal("pressing r produced no command; the dialog does not close")
	}
	_ = updated

	if got := configModelOf(t, config.AgentTitle); got != cheap {
		t.Errorf("title restored to %q, want %q", got, cheap)
	}
	if got := configModelOf(t, config.AgentTask); got != other {
		t.Errorf("task restored to %q, want %q — a revert that collapses two different "+
			"starting models into one is not a revert", got, other)
	}
}

// The escape hatch must be advertised. A key nobody is shown is not an option.
func TestRevertIsOfferedOnScreen(t *testing.T) {
	cheap, dear := followFixture(t)
	m := followDialog(t, []config.AgentModelMove{{Agent: config.AgentResearch, From: cheap, To: dear}})
	if !strings.Contains(followText(m), "put ALL of it back") {
		t.Error("the screen never tells the user the change can be undone")
	}
	view := m.View()
	if !strings.Contains(view, "put it back") {
		t.Errorf("no revert key in the hints:\n%s", view)
	}
	var hasR bool
	for _, b := range m.BindingKeys() {
		if b.Help().Key == "r" {
			hasR = true
		}
	}
	if !hasR {
		t.Error(`no "r" binding registered`)
	}
}

// configModelOf reads an agent's current model straight from config, so a test
// asserting "it was restored" reads the same place the app does.
func configModelOf(t *testing.T, name config.AgentName) models.ModelID {
	t.Helper()
	if got := config.AgentModel(name); got != "" {
		return got
	}
	t.Fatalf("agent %q has no configured model", name)
	return ""
}
