package config

import (
	"strings"
	"testing"

	"github.com/opencode-ai/opencode/internal/llm/models"
)

// doctorCfg loads an isolated config and returns it for the test to shape.
func doctorCfg(t *testing.T) *Config {
	t.Helper()
	t.Setenv("GEMINI_API_KEY", "test-key-for-a-reachable-default")
	if _, err := Load(t.TempDir(), false); err != nil {
		t.Fatalf("Load: %v", err)
	}
	c := Get()
	c.LocalEndpoints = nil
	for k := range discardedKeys {
		delete(discardedKeys, k)
	}
	return c
}

func texts(fs []Finding) string {
	var b strings.Builder
	for _, f := range fs {
		b.WriteString(f.String() + "\n")
	}
	return b.String()
}

// The state the owner was in on 2026-10-05: the coder set to a model Google no
// longer lists. The doctor must move it to the successor and say so; with no
// setter it must change nothing and still say so.
func TestDoctorMovesAnAgentOffAModelThatIsGone(t *testing.T) {
	c := doctorCfg(t)
	const gone, next = models.ModelID("antigravity.test-retired"), models.ModelID("antigravity.test-current")
	models.SupportedModels[next] = models.Model{ID: next, Name: "Current Model", Provider: models.ProviderAntigravity}
	models.LegacyModelIDs[gone] = next
	t.Cleanup(func() { delete(models.SupportedModels, next); delete(models.LegacyModelIDs, gone) })
	c.Agents[AgentCoder] = Agent{Model: gone}

	report := doctorAgents(nil)
	if len(report) != 1 || report[0].Fixed || !strings.Contains(report[0].Text, "Current Model") {
		t.Fatalf("report-only run: %s", texts(report))
	}
	if c.Agents[AgentCoder].Model != gone {
		t.Fatal("a report-only run changed the configuration")
	}

	var moved []string
	fixed := doctorAgents(func(name AgentName, to models.ModelID) error {
		moved = append(moved, string(name)+"->"+string(to))
		a := c.Agents[name]
		a.Model = to
		c.Agents[name] = a
		return nil
	})
	if len(fixed) != 1 || !fixed[0].Fixed || len(moved) != 1 || moved[0] != "coder->"+string(next) {
		t.Fatalf("fix run: %s moved=%v", texts(fixed), moved)
	}
	if again := doctorAgents(nil); len(again) != 0 {
		t.Errorf("after the fix the doctor still complains: %s", texts(again))
	}

	// No successor known: it must say so, not pick something.
	c.Agents[AgentCoder] = Agent{Model: "antigravity.nobody-knows"}
	if got := doctorAgents(func(AgentName, models.ModelID) error { t.Fatal("moved with no successor"); return nil }); len(got) != 1 || got[0].Fixed || !strings.Contains(got[0].Text, "/model") {
		t.Errorf("no-successor case: %s", texts(got))
	}
}

// Keys: nine NUL bytes (found in the owner's config, twice), a key with a space
// in it, one character, another vendor's key. A sound key raises nothing, and
// no finding may ever repeat a key.
func TestDoctorReportsKeysThatAreNotKeys(t *testing.T) {
	c := doctorCfg(t)
	good := "nvapi-" + strings.Repeat("Ab3_", 16)
	c.Providers = map[models.ModelProvider]Provider{
		models.ProviderGROQ:        {APIKey: "gsk_" + strings.Repeat("x", 40)},
		models.ProviderOpenRouter:  {APIKey: "gsk_" + strings.Repeat("y", 40)}, // a Groq key in the OpenRouter slot
		models.ProviderXAI:         {APIKey: "xai-abc def" + strings.Repeat("z", 30)},
		models.ProviderAntigravity: {APIKey: "oauth-login"},
	}
	c.LocalEndpoints = []LocalEndpoint{
		{Name: "NVIDIA NIM", BaseURL: "https://integrate.api.nvidia.com/v1", APIKey: good},
		{Name: "Short", BaseURL: "https://example.org/v1", APIKey: "v"},
		{Name: "LM Studio", BaseURL: "http://localhost:1234/v1", APIKey: "lm-studio"}, // a local placeholder is fine
	}
	discardedKeys["gemini"] = 9

	got := texts(doctorKeys())
	for _, want := range []string{
		"gemini is 9 invisible control character(s)",
		"openrouter does not start with sk-or-",
		"xai has a space inside it",
		"the endpoint Short is only 1 characters",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q in:\n%s", want, got)
		}
	}
	for _, clean := range []string{"groq", "NVIDIA NIM", "LM Studio", "antigravity"} {
		if strings.Contains(got, clean) {
			t.Errorf("%s has a sound key (or none is needed) and was reported:\n%s", clean, got)
		}
	}
	for _, secret := range []string{good, strings.Repeat("y", 40), strings.Repeat("z", 30)} {
		if strings.Contains(got, secret) {
			t.Fatal("a finding repeats a key")
		}
	}
}

// The owner's config held "lmstudio" on 127.0.0.1 and "LM Studio" on localhost:
// one server, every model listed twice.
func TestDoctorReportsTwoConnectionsToOneServer(t *testing.T) {
	c := doctorCfg(t)
	c.LocalEndpoints = []LocalEndpoint{
		{Name: "lmstudio", BaseURL: "http://127.0.0.1:1234/v1"},
		{Name: "LM Studio", BaseURL: "http://localhost:1234/v1"},
		{Name: "Ollama", BaseURL: "http://localhost:11434/v1"},
		{Name: "Off", BaseURL: "http://localhost:11434/v1", Disabled: true},
	}
	got := doctorEndpoints()
	if len(got) != 1 || !strings.Contains(got[0].Text, "lmstudio, LM Studio") {
		t.Errorf("findings: %s", texts(got))
	}
}

func TestDoctorReportsRowsThatReadTheSameAndRanksThatAreUpsideDown(t *testing.T) {
	doctorCfg(t)
	saved := map[models.ModelID]models.Model{}
	for id, m := range models.SupportedModels {
		if m.Provider == models.ProviderChatGPT {
			saved[id] = m
			delete(models.SupportedModels, id)
		}
	}
	t.Cleanup(func() {
		for id, m := range models.SupportedModels {
			if m.Provider == models.ProviderChatGPT {
				delete(models.SupportedModels, id)
			}
		}
		for id, m := range saved {
			models.SupportedModels[id] = m
		}
	})
	// The list as it was before the fix: counting down from 9, two alike.
	for id, m := range map[models.ModelID]models.Model{
		"chatgpt.a": {Name: "Same Name", Rank: 9},
		"chatgpt.b": {Name: "Same Name", Rank: 8},
		"chatgpt.c": {Name: "Other", Rank: 8},
	} {
		m.ID, m.Provider = id, models.ProviderChatGPT
		models.SupportedModels[id] = m
	}
	got := texts(append(doctorNames(), doctorRanks()...))
	for _, want := range []string{`under the one name "Same Name"`, "starts at rank 8, not 1", "share rank 8"} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q in:\n%s", want, got)
		}
	}

	// Put right: nothing to say.
	for id, m := range map[models.ModelID]models.Model{
		"chatgpt.a": {Name: "A", Rank: 1}, "chatgpt.b": {Name: "B", Rank: 2}, "chatgpt.c": {Name: "C", Rank: 3},
	} {
		m.ID, m.Provider = id, models.ProviderChatGPT
		models.SupportedModels[id] = m
	}
	for _, f := range append(doctorNames(), doctorRanks()...) {
		if strings.Contains(f.Text, "chatgpt") {
			t.Errorf("a sound list was reported: %s", f.Text)
		}
	}
}

// Deterministic: the same state gives the same findings in the same order.
func TestDoctorSaysTheSameThingTwice(t *testing.T) {
	c := doctorCfg(t)
	c.LocalEndpoints = []LocalEndpoint{
		{Name: "a", BaseURL: "http://localhost:1/v1"}, {Name: "b", BaseURL: "http://127.0.0.1:1/v1"},
		{Name: "c", BaseURL: "http://localhost:2/v1"}, {Name: "d", BaseURL: "http://127.0.0.1:2/v1"},
	}
	discardedKeys["x"], discardedKeys["y"] = 3, 4
	dir := t.TempDir()
	first := texts(RunModelDoctor(dir, nil))
	for i := 0; i < 20; i++ {
		if again := texts(RunModelDoctor(dir, nil)); again != first {
			t.Fatalf("run %d differs:\n%s\n---\n%s", i, first, again)
		}
	}
	lines := DoctorSummary([]Finding{{Text: "p"}, {Fixed: true, Text: "f"}})
	if len(lines) != 2 || !strings.HasPrefix(lines[0], "fixed:") || !strings.HasPrefix(lines[1], "PROBLEM:") {
		t.Errorf("summary order: %v", lines)
	}
}
