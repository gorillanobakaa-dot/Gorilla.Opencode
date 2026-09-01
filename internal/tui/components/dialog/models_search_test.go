package dialog

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/opencode-ai/opencode/internal/config"
	"github.com/opencode-ai/opencode/internal/llm/models"
)

// GORILLA OVERRIDE: "/" search across every provider at once, matching name
// AND description. The catalogue passed 270 models on one provider alone;
// finding "the free coding models" by scrolling is a reading assignment, and
// the descriptions were written to carry exactly the words someone would type.

// mockCatalogue registers three models under the test-only provider and
// enables ONLY that provider, so search results are deterministic. Restores
// both the model registry and the provider map — registry globals leaking
// between tests is a documented trap in this repo.
func mockCatalogue(t *testing.T) {
	t.Helper()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	if _, err := config.Load(t.TempDir(), false); err != nil {
		t.Fatalf("config.Load: %v", err)
	}
	for _, k := range []string{"ANTHROPIC_API_KEY", "OPENAI_API_KEY",
		"GEMINI_API_KEY", "GROQ_API_KEY", "CEREBRAS_API_KEY",
		"OPENROUTER_API_KEY", "XAI_API_KEY"} {
		t.Setenv(k, "")
	}

	cfg := config.Get()
	prevProviders := cfg.Providers
	t.Cleanup(func() { cfg.Providers = prevProviders })
	cfg.Providers = map[models.ModelProvider]config.Provider{
		models.ProviderMock: {APIKey: "mk-test-0123456789abcdef0123456789"},
	}

	// GORILLA OVERRIDE (2026-09-01): evict models discovered from THIS MACHINE.
	//
	// Clearing cfg.Providers is not enough. Local endpoints are a separate path
	// — they live in cfg.LocalEndpoints and register under ProviderLocal — so
	// config.Load above contacts whatever OpenAI-compatible server happens to be
	// running on the developer's laptop and registers its models into the global
	// SupportedModels. The picker then shows them alongside the mocks and the
	// count assertions fail.
	//
	// Found for real: with LM Studio running, these tests failed with "4 of 4
	// models match" against three mocks, the fourth being "[lmstudio] Qwen3
	// Coder". A test whose result depends on which applications are open is not
	// measuring what it claims to.
	prevEndpoints := cfg.LocalEndpoints
	cfg.LocalEndpoints = nil
	t.Cleanup(func() { cfg.LocalEndpoints = prevEndpoints })

	evicted := map[models.ModelID]models.Model{}
	for id, m := range models.SupportedModels {
		if m.Provider == models.ProviderLocal {
			evicted[id] = m
			delete(models.SupportedModels, id)
		}
	}
	t.Cleanup(func() {
		for id, m := range evicted {
			models.SupportedModels[id] = m
		}
	})

	mocks := []models.Model{
		{
			ID: "mock.alpha", Name: "Alpha Coder",
			Description: "FREE — CAN CODE — strong agentic coding model (262K ctx)",
			Detail:      "Vendor's own description: built for software engineering.",
			Provider:    models.ProviderMock, APIModel: "mocklab/alpha",
		},
		{
			ID: "mock.beta", Name: "Beta Thinker",
			Description: "$1.00/$2.00 per 1M — research/admin work — advanced reasoning model",
			Provider:    models.ProviderMock, APIModel: "mocklab/beta",
			CostPer1MIn: 1, CostPer1MOut: 2,
		},
		{
			ID: "mock.gamma", Name: "Gamma Painter",
			Description: "shit tier for code — vision/image model",
			Provider:    models.ProviderMock, APIModel: "mocklab/gamma",
		},
	}
	for _, mm := range mocks {
		id := mm.ID
		if _, exists := models.SupportedModels[id]; exists {
			t.Fatalf("model id %q already registered", id)
		}
		models.SupportedModels[id] = mm
		t.Cleanup(func() { delete(models.SupportedModels, id) })
	}
}

func typeRunes(m *modelDialogCmp, s string) {
	m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)})
}

func TestSlashOpensSearchAndFiltersOnDescription(t *testing.T) {
	mockCatalogue(t)
	m := &modelDialogCmp{}
	m.SwitchToProvider(models.ProviderMock)

	// Before "/", letters are list navigation, not text — pressing them must
	// not filter anything. This is the pre-search contract.
	typeRunes(m, "coding")
	if m.searchActive || len(m.models) != 3 {
		t.Fatalf("typing without / must not search: active=%v models=%d", m.searchActive, len(m.models))
	}

	typeRunes(m, "/")
	if !m.searchActive {
		t.Fatal("/ must open the search prompt")
	}
	if len(m.models) != 3 {
		t.Fatalf("empty query must show the whole domain, got %d", len(m.models))
	}

	// "coding" appears only in Alpha's DESCRIPTION — not in any name.
	typeRunes(m, "coding")
	if len(m.models) != 1 || m.models[0].ID != "mock.alpha" {
		t.Fatalf("query 'coding' should match Alpha by description, got %v", ids(m.models))
	}

	// Multi-word queries AND together: "reasoning research" is Beta only.
	m.query = ""
	typeRunes(m, "reasoning research")
	if len(m.models) != 1 || m.models[0].ID != "mock.beta" {
		t.Fatalf("query 'reasoning research' should match Beta, got %v", ids(m.models))
	}

	// A name term works too.
	m.query = ""
	typeRunes(m, "gamma")
	if len(m.models) != 1 || m.models[0].ID != "mock.gamma" {
		t.Fatalf("query 'gamma' should match by name, got %v", ids(m.models))
	}
}

func ids(ms []models.Model) []string {
	var out []string
	for _, m := range ms {
		out = append(out, string(m.ID))
	}
	return out
}

func TestSearchSpaceIsTextNotBookmark(t *testing.T) {
	mockCatalogue(t)
	m := &modelDialogCmp{}
	m.SwitchToProvider(models.ProviderMock)
	typeRunes(m, "/")
	typeRunes(m, "free")
	m.Update(tea.KeyMsg{Type: tea.KeySpace})
	typeRunes(m, "code")

	if m.query != "free code" {
		t.Fatalf("space must type into the query, got %q", m.query)
	}
	if got := config.BookmarkedModels(); len(got) != 0 {
		t.Fatalf("space while searching must not bookmark, got %v", got)
	}
}

func TestSearchEnterSelectsTheMatch(t *testing.T) {
	mockCatalogue(t)
	m := &modelDialogCmp{}
	m.SwitchToProvider(models.ProviderMock)
	typeRunes(m, "/")
	typeRunes(m, "coding")

	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("enter on a match must produce a command")
	}
	msg, ok := cmd().(ModelSelectedMsg)
	if !ok {
		t.Fatalf("expected ModelSelectedMsg, got %T", cmd())
	}
	if msg.Model.ID != "mock.alpha" {
		t.Fatalf("selected %q, want mock.alpha", msg.Model.ID)
	}
	// Selection closes the episode: the next open must not resurrect a stale
	// search prompt — the dialog object is reused between opens.
	if m.searchActive {
		t.Error("search must be closed after a selection")
	}
}

func TestSearchEscRestoresTheColumnAndSelection(t *testing.T) {
	mockCatalogue(t)
	m := &modelDialogCmp{}
	m.SwitchToProvider(models.ProviderMock)
	m.selectedIdx = 2

	typeRunes(m, "/")
	typeRunes(m, "zzz-no-such-model")
	if len(m.models) != 0 {
		t.Fatalf("nonsense query should match nothing, got %v", ids(m.models))
	}
	m.Update(tea.KeyMsg{Type: tea.KeyEsc})

	if m.searchActive {
		t.Fatal("esc must close the search")
	}
	if m.provider != models.ProviderMock {
		t.Fatalf("esc must land back on the column it left, got %q", m.provider)
	}
	if len(m.models) != 3 || m.selectedIdx != 2 {
		t.Fatalf("column state must be restored: models=%d selectedIdx=%d", len(m.models), m.selectedIdx)
	}
}

func TestSearchBackspaceWidensTheFilter(t *testing.T) {
	mockCatalogue(t)
	m := &modelDialogCmp{}
	m.SwitchToProvider(models.ProviderMock)
	typeRunes(m, "/")
	typeRunes(m, "codingx") // no match
	if len(m.models) != 0 {
		t.Fatalf("expected no matches for 'codingx', got %v", ids(m.models))
	}
	m.Update(tea.KeyMsg{Type: tea.KeyBackspace})
	if m.query != "coding" || len(m.models) != 1 {
		t.Fatalf("backspace must re-widen: query=%q matches=%d", m.query, len(m.models))
	}
}

func TestSearchViewTagsEveryRowWithItsConnection(t *testing.T) {
	mockCatalogue(t)
	m := &modelDialogCmp{}
	m.SwitchToProvider(models.ProviderMock)
	m.width, m.height = 120, 40
	typeRunes(m, "/")

	view := m.View()
	if !strings.Contains(view, "[__mock]") {
		t.Fatalf("search rows must name the connection that serves them; view:\n%s", view)
	}
	if !strings.Contains(view, "3 of 3 models match") {
		t.Fatalf("the query line must count matches; view:\n%s", view)
	}
}

// The J/K/H/L letters navigate the list normally — they must still do that
// when search is NOT active, and become plain text when it is.
func TestSearchDoesNotBreakLetterNavigation(t *testing.T) {
	mockCatalogue(t)
	m := &modelDialogCmp{}
	m.SwitchToProvider(models.ProviderMock)

	typeRunes(m, "j")
	if m.selectedIdx != 1 {
		t.Fatalf("j must still move the selection outside search, idx=%d", m.selectedIdx)
	}

	typeRunes(m, "/")
	typeRunes(m, "j") // now it is text
	if m.query != "j" {
		t.Fatalf("in search, j is text; query=%q", m.query)
	}
	if m.selectedIdx != 0 {
		t.Fatalf("in search, j must not move the selection, idx=%d", m.selectedIdx)
	}
}
