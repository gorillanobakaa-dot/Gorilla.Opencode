package config

import (
	"errors"
	"maps"
	"strings"
	"testing"
	"time"

	"github.com/opencode-ai/opencode/internal/llm/models"
)

// The ids of 2026-10-25: the model config.json named (deleted from LM Studio),
// the model the old fallback chose (NVIDIA's cloud), and the owner's models.
const (
	deletedLocal = models.ModelID("local.deepseek/deepseek-r1-0528-qwen3-8b")
	nvidiaCloud  = models.ModelID("remote.nvidia/nemotron-3-ultra-550b-a55b")
	qwenLocal    = models.ModelID("local.qwen3.6-35b-a3b")
	gemmaLocal   = models.ModelID("local.gemma-4-26b-a4b-it")
	lmStudioURL  = "http://127.0.0.1:1234/v1"
	nvidiaURL    = "https://integrate.api.nvidia.com/v1"
)

// choiceFixture loads an isolated config with no cloud key, no hold, and the
// agents and preferred list put back afterwards.
func choiceFixture(t *testing.T) *Config {
	t.Helper()
	if _, err := Load(t.TempDir(), false); err != nil {
		t.Fatalf("Load: %v", err)
	}
	c := Get()
	for _, env := range []string{"GEMINI_API_KEY", "GROQ_API_KEY", "CEREBRAS_API_KEY", "OPENROUTER_API_KEY",
		"ANTHROPIC_API_KEY", "OPENAI_API_KEY", "XAI_API_KEY", "DEEPSEEK_API_KEY"} {
		t.Setenv(env, "")
	}
	prevAgents := maps.Clone(c.Agents)
	prevPref := c.PreferredLocalModels
	prevProviders := maps.Clone(c.Providers)
	resetPendingChoices()
	// Load found the developer's own LM Studio, if it is running. These tests
	// say exactly what this machine serves.
	restoreEndpoints := models.IsolateLocalEndpointsForTest()
	t.Cleanup(func() {
		restoreEndpoints()
		c.Agents = prevAgents
		c.PreferredLocalModels = prevPref
		c.Providers = prevProviders
		resetPendingChoices()
		for k := range legacyMoves {
			delete(legacyMoves, k)
		}
	})
	c.PreferredLocalModels = nil
	return c
}

// serve registers a model as an endpoint would after listing it.
func serve(t *testing.T, id models.ModelID, baseURL, endpoint string) {
	t.Helper()
	prev, had := models.SupportedModels[id]
	api := strings.TrimPrefix(strings.TrimPrefix(string(id), "local."), "remote.")
	models.SupportedModels[id] = models.Model{
		ID: id, Name: api, Provider: models.ProviderLocal, APIModel: api,
		ContextWindow: 32768, DefaultMaxTokens: 8192,
	}
	models.RegisterLocalRouteForTestNamed(id, baseURL, "", endpoint)
	t.Cleanup(func() {
		if had {
			models.SupportedModels[id] = prev
		} else {
			delete(models.SupportedModels, id)
		}
		models.ClearLocalRouteForTest(id)
	})
}

func holdCoder(t *testing.T, c *Config, model models.ModelID) {
	t.Helper()
	c.Agents[AgentCoder] = Agent{Model: model, MaxTokens: 5000}
	if err := validateAgent(c, AgentCoder, c.Agents[AgentCoder]); err != nil {
		t.Fatalf("validateAgent: %v", err)
	}
}

// The 2026-10-25 case: the configured model is gone from LM Studio and the
// only other endpoint is NVIDIA's cloud. The agent is held, every request is
// refused at the gate, and a run without a screen is refused, not moved to
// NVIDIA.
func TestUnusableLocalModelIsHeldAndNeverMovedToARemoteEndpoint(t *testing.T) {
	c := choiceFixture(t)
	serve(t, nvidiaCloud, nvidiaURL, "NVIDIA NIM")

	holdCoder(t, c, deletedLocal)

	if !ModelChoicePending() {
		t.Fatal("an unusable configured model was not held for a choice")
	}
	if err := CheckModelChosen(); !errors.Is(err, ErrModelChoicePending) {
		t.Fatalf("the send gate let a request through while a choice is pending: %v", err)
	}
	err := ChooseForRunWithoutScreen()
	if err == nil {
		t.Fatalf("a run without a screen was not refused; coder is now %s", c.Agents[AgentCoder].Model)
	}
	// With nothing on this machine the placeholder may be the remote model
	// (the TUI must still start, to show its picker). It is only a
	// placeholder: the hold stays and the gate below stays shut.
	if p, ok := pendingChoiceFor(AgentCoder); !ok || p.Configured != deletedLocal {
		t.Fatalf("the hold was lost by the refusal: %+v %v", p, ok)
	}
	if !strings.Contains(err.Error(), "Nothing was sent") {
		t.Errorf("the refusal does not say nothing was sent: %v", err)
	}
	if CheckModelChosen() == nil {
		t.Error("after a refusal the gate opened")
	}
}

// Without a screen, the first model of the preferred list that this machine
// serves now is taken, and the hold ends.
func TestRunWithoutScreenTakesTheFirstPreferredModelOnThisMachine(t *testing.T) {
	c := choiceFixture(t)
	serve(t, nvidiaCloud, nvidiaURL, "NVIDIA NIM")
	serve(t, gemmaLocal, lmStudioURL, "lmstudio")
	serve(t, qwenLocal, lmStudioURL, "lmstudio")

	holdCoder(t, c, deletedLocal)
	if err := ChooseForRunWithoutScreen(); err != nil {
		t.Fatalf("ChooseForRunWithoutScreen: %v", err)
	}
	if got := c.Agents[AgentCoder].Model; got != qwenLocal {
		t.Fatalf("coder is on %s, want %s (first of the preferred list)", got, qwenLocal)
	}
	if err := CheckModelChosen(); err != nil {
		t.Fatalf("the gate is still closed after a choice: %v", err)
	}
}

// The order of the list is the owner's order: with the first one not served,
// the second is taken.
func TestPreferredListOrderIsKept(t *testing.T) {
	c := choiceFixture(t)
	serve(t, gemmaLocal, lmStudioURL, "lmstudio")

	holdCoder(t, c, deletedLocal)
	if err := ChooseForRunWithoutScreen(); err != nil {
		t.Fatalf("ChooseForRunWithoutScreen: %v", err)
	}
	if got := c.Agents[AgentCoder].Model; got != gemmaLocal {
		t.Fatalf("coder is on %s, want %s", got, gemmaLocal)
	}
}

// Where a model runs is decided by the endpoint's host, not by its id: a
// "local." id served from NVIDIA's host is not on this machine.
func TestTheHostDecidesNotTheName(t *testing.T) {
	c := choiceFixture(t)
	serve(t, qwenLocal, nvidiaURL, "NVIDIA NIM")

	holdCoder(t, c, deletedLocal)
	if err := ChooseForRunWithoutScreen(); err == nil {
		t.Fatalf("a model called local. but served by %s was taken: coder on %s", nvidiaURL, c.Agents[AgentCoder].Model)
	}
}

// A remote model is taken only when the owner wrote it in preferredLocalModels.
func TestARemoteModelOnlyWhenTheOwnerListedIt(t *testing.T) {
	c := choiceFixture(t)
	serve(t, nvidiaCloud, nvidiaURL, "NVIDIA NIM")

	c.PreferredLocalModels = []string{string(nvidiaCloud)}
	holdCoder(t, c, deletedLocal)
	if err := ChooseForRunWithoutScreen(); err != nil {
		t.Fatalf("a remote model the owner listed was refused: %v", err)
	}
	if got := c.Agents[AgentCoder].Model; got != nvidiaCloud {
		t.Fatalf("coder is on %s, want the listed %s", got, nvidiaCloud)
	}
}

// A model the last probe found gone is not "available now".
func TestAPreferredModelFoundGoneIsSkipped(t *testing.T) {
	c := choiceFixture(t)
	serve(t, qwenLocal, lmStudioURL, "lmstudio")
	serve(t, gemmaLocal, lmStudioURL, "lmstudio")
	models.SetProbeVerdictForTest(qwenLocal, models.ProbeGone, time.Now())
	t.Cleanup(func() { models.ClearProbeVerdictForTest(qwenLocal) })

	holdCoder(t, c, deletedLocal)
	if err := ChooseForRunWithoutScreen(); err != nil {
		t.Fatalf("ChooseForRunWithoutScreen: %v", err)
	}
	if got := c.Agents[AgentCoder].Model; got != gemmaLocal {
		t.Fatalf("coder is on %s, want %s (qwen was found gone)", got, gemmaLocal)
	}
}

// The placeholder that lets the program start is a model on this machine when
// there is one, never the remote endpoint.
func TestThePlaceholderIsOnThisMachineWhenItCanBe(t *testing.T) {
	c := choiceFixture(t)
	serve(t, nvidiaCloud, nvidiaURL, "NVIDIA NIM")
	serve(t, gemmaLocal, lmStudioURL, "lmstudio")

	holdCoder(t, c, deletedLocal)
	if got := c.Agents[AgentCoder].Model; !models.IsOnMachine(got) {
		t.Fatalf("the placeholder is %s, not a model on this machine", got)
	}
	p, ok := pendingChoiceFor(AgentCoder)
	if !ok || p.Configured != deletedLocal {
		t.Fatalf("hold = %+v, %v; want the configured %s", p, ok, deletedLocal)
	}
	if !strings.Contains(p.Reason, "not in lmstudio any more") {
		t.Errorf("reason %q does not say the model left LM Studio", p.Reason)
	}
}

// A config written before remote endpoints got "remote." ids names an NVIDIA
// model "local.nvidia/x". It keeps working: renamed, not held.
func TestAnOldLocalIDOfARemoteModelIsRenamedNotHeld(t *testing.T) {
	c := choiceFixture(t)
	serve(t, nvidiaCloud, nvidiaURL, "NVIDIA NIM")
	old := models.ModelID("local.nvidia/nemotron-3-ultra-550b-a55b")

	holdCoder(t, c, old)
	if ModelChoicePending() {
		t.Fatal("a renamed remote model was held as unusable")
	}
	if got := c.Agents[AgentCoder].Model; got != nvidiaCloud {
		t.Fatalf("coder is on %s, want %s", got, nvidiaCloud)
	}
	if mv := legacyMoves[AgentCoder]; mv != [2]models.ModelID{old, nvidiaCloud} {
		t.Errorf("the rename was not recorded for the model doctor: %v", mv)
	}
}

// Picking a model ends the hold; picking an unusable one is refused and the
// hold stays.
func TestAPickEndsTheHoldAndAnUnusablePickIsRefused(t *testing.T) {
	c := choiceFixture(t)
	serve(t, qwenLocal, lmStudioURL, "lmstudio")
	holdCoder(t, c, deletedLocal)

	// A cloud model whose provider has no key: validateAgent would hold it.
	const cloud = models.ModelID("xai.test-no-key")
	models.SupportedModels[cloud] = models.Model{ID: cloud, Name: "no key", Provider: models.ProviderXAI, ContextWindow: 32768}
	t.Cleanup(func() { delete(models.SupportedModels, cloud) })
	delete(c.Providers, models.ProviderXAI)
	if err := UpdateAgentModel(AgentCoder, cloud); err == nil {
		t.Fatalf("an unusable pick (%s, no key) was accepted", cloud)
	}
	if p, ok := pendingChoiceFor(AgentCoder); !ok || p.Configured != deletedLocal {
		t.Fatalf("a refused pick changed the hold: %+v %v", p, ok)
	}

	if err := UpdateAgentModel(AgentCoder, qwenLocal); err != nil {
		t.Fatalf("UpdateAgentModel(%s): %v", qwenLocal, err)
	}
	if _, ok := pendingChoiceFor(AgentCoder); ok {
		t.Fatal("picking a usable model did not end the hold")
	}
}

// A run without a screen does not ask a remote endpoint for its list unless a
// configured model is on one.
func TestRemoteEndpointsAreNeededOnlyForARemoteModel(t *testing.T) {
	c := choiceFixture(t)
	c.Agents = map[AgentName]Agent{AgentCoder: {Model: deletedLocal}}
	if remoteEndpointsNeeded() {
		t.Fatal("a local. model made a remote endpoint needed")
	}
	c.Agents[AgentTitle] = Agent{Model: nvidiaCloud}
	if !remoteEndpointsNeeded() {
		t.Fatal("a remote. agent model did not make remote endpoints needed")
	}
	delete(c.Agents, AgentTitle)
	c.PreferredLocalModels = []string{string(nvidiaCloud)}
	if !remoteEndpointsNeeded() {
		t.Fatal("a remote. id in preferredLocalModels did not make remote endpoints needed")
	}
}
