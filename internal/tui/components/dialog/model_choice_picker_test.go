package dialog

// GORILLA OVERRIDE (2026-10-25): when the configured model is unusable the
// picker opens on what this machine serves now, says why, and offers nothing
// else while this machine serves something. See config/model_choice.go.

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/opencode-ai/opencode/internal/config"
	"github.com/opencode-ai/opencode/internal/llm/models"
)

func servedForTest(t *testing.T, id models.ModelID, baseURL string) {
	t.Helper()
	api := strings.TrimPrefix(strings.TrimPrefix(string(id), "local."), "remote.")
	models.SupportedModels[id] = models.Model{ID: id, Name: api, Provider: models.ProviderLocal, APIModel: api, ContextWindow: 32768}
	models.RegisterLocalRouteForTestNamed(id, baseURL, "", "lmstudio")
	t.Cleanup(func() { delete(models.SupportedModels, id); models.ClearLocalRouteForTest(id) })
}

func TestPickerOpensOnModelsAvailableNowWhenAChoiceIsPending(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	if _, err := config.Load(t.TempDir(), false); err != nil {
		t.Fatalf("config.Load: %v", err)
	}
	restore := models.IsolateLocalEndpointsForTest()
	t.Cleanup(restore)
	servedForTest(t, "local.qwen3.6-35b-a3b", "http://127.0.0.1:1234/v1")
	servedForTest(t, "remote.nvidia/nemotron-3-ultra-550b-a55b", "https://integrate.api.nvidia.com/v1")

	release := config.HoldModelChoiceForTest(config.AgentCoder, "local.deepseek/deepseek-r1-0528-qwen3-8b", "not in lmstudio any more")
	t.Cleanup(release)

	m := &modelDialogCmp{}
	m.Init()
	m.Update(tea.WindowSizeMsg{Width: 140, Height: 40})
	if len(m.availableProviders) != 1 || m.availableProviders[0] != ProviderAvailableNow {
		t.Fatalf("columns = %v, want only %q", m.availableProviders, ProviderAvailableNow)
	}
	if len(m.models) != 1 || m.models[0].ID != "local.qwen3.6-35b-a3b" {
		var ids []models.ModelID
		for _, mm := range m.models {
			ids = append(ids, mm.ID)
		}
		t.Fatalf("offered %v, want only the model on this machine", ids)
	}
	view := m.View()
	if !strings.Contains(view, "is not in lmstudio any more") {
		t.Errorf("the picker does not say why it opened:\n%s", view)
	}

	// Once chosen, the picker is the ordinary one again.
	release()
	m.Init()
	if len(m.availableProviders) == 1 && m.availableProviders[0] == ProviderAvailableNow {
		t.Error("with no choice pending the picker still shows only the choice column")
	}
}

// When this machine serves nothing, the other columns follow, so a model can
// still be chosen on purpose.
func TestPickerKeepsOtherColumnsWhenThisMachineServesNothing(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	if _, err := config.Load(t.TempDir(), false); err != nil {
		t.Fatalf("config.Load: %v", err)
	}
	restore := models.IsolateLocalEndpointsForTest()
	t.Cleanup(restore)
	servedForTest(t, "remote.nvidia/nemotron-3-ultra-550b-a55b", "https://integrate.api.nvidia.com/v1")
	t.Cleanup(config.HoldModelChoiceForTest(config.AgentCoder, "local.x", "no model server on this machine answered"))

	got := getEnabledProviders(config.Get())
	if len(got) < 2 || got[0] != ProviderAvailableNow {
		t.Fatalf("columns = %v, want %q first and the others after it", got, ProviderAvailableNow)
	}
	if len(availableNowModels()) != 0 {
		t.Error("a remote model is in the available-now column")
	}
}
