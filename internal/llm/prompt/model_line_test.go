package prompt

// GORILLA FIX (2026-10-11): the system prompt names the model it is sent to.
// Measured: a Nemotron 3 Ultra session in which the model said "I'm not
// Nemotron. I'm a different model entirely." Nothing in its context said
// otherwise.

import (
	"strings"
	"testing"

	"github.com/opencode-ai/opencode/internal/config"
	"github.com/opencode-ai/opencode/internal/llm/models"
)

func TestTheEnvironmentBlockNamesTheModel(t *testing.T) {
	loadCfg(t)
	m := models.Model{ID: "local.nvidia/nemotron-3-ultra-550b-a55b", Name: "Nemotron 3 Ultra", Provider: models.ProviderLocal}
	got := getEnvironmentInfo(m)
	want := "Model: you are local.nvidia/nemotron-3-ultra-550b-a55b (Nemotron 3 Ultra)"
	if !strings.Contains(got, want) {
		t.Fatalf("environment block lacks %q:\n%s", want, got)
	}
	if !strings.Contains(got, "<env>") || !strings.Contains(got[strings.Index(got, want):], "</env>") {
		t.Fatalf("the model line is outside the <env> block:\n%s", got)
	}
}

func TestTheCoderPromptCarriesTheModelItIsBuiltFor(t *testing.T) {
	loadCfg(t)
	m := models.Model{ID: "local.nvidia/nemotron-3-super-120b-a12b", Provider: models.ProviderLocal}
	for name, got := range map[string]string{
		"coder":    GetAgentPromptFor(config.AgentCoder, m),
		"subcoder": GetAgentPromptFor(config.AgentSubCoder, m),
		"plan":     GetAgentPromptFor(config.AgentPlan, m),
		"task":     TaskPromptFor(m),
	} {
		if !strings.Contains(got, "Model: you are local.nvidia/nemotron-3-super-120b-a12b") {
			t.Errorf("%s prompt does not name its model", name)
		}
	}
}

func TestNoModelMeansNoModelLine(t *testing.T) {
	loadCfg(t)
	if got := getEnvironmentInfo(models.Model{}); strings.Contains(got, "Model:") {
		t.Fatalf("a zero model produced a model line:\n%s", got)
	}
}
