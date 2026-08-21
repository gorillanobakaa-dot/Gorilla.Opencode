package cmd

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/opencode-ai/opencode/internal/config"
	"github.com/opencode-ai/opencode/internal/config/configtest"
	"github.com/opencode-ai/opencode/internal/llm/models"
	"github.com/opencode-ai/opencode/internal/tui/startup"
)

// TestMain isolates config: applyPortalChoice writes real credentials through
// updateCfgFile, which without this lands in the developer's live config.json
// (that has happened three times — see internal/config/configtest).
func TestMain(m *testing.M) { os.Exit(configtest.Isolate(m)) }

// loadCfg loads the isolated config once. config.Load caches globally, so the
// first call in the process wins and later calls return the same object; every
// test in this file therefore shares one isolated config, and assertions read
// the file back rather than trusting in-memory state.
func loadCfg(t *testing.T) {
	t.Helper()
	if config.Get() != nil {
		return
	}
	cwd, _ := os.Getwd()
	if _, err := config.Load(cwd, false); err != nil {
		t.Fatalf("config.Load: %v", err)
	}
}

func readCfgFile(t *testing.T) config.Config {
	t.Helper()
	b, err := os.ReadFile(config.GorillaConfigFile())
	if err != nil {
		t.Fatalf("reading config file: %v", err)
	}
	var c config.Config
	if err := json.Unmarshal(b, &c); err != nil {
		t.Fatalf("unmarshalling config file: %v", err)
	}
	return c
}

// An API-key selection saves the key and moves every agent onto that provider,
// persisted to disk — not just held in memory.
//
// GORILLA OVERRIDE (2026-08-21): Anthropic's model list is now FETCHED, so this
// test stubs the fetch. The stub is the point as much as the assertion: a test
// that reached the real api.anthropic.com would be a network test wearing a unit
// test's clothes, and would fail on the metered connections this is built for.
func TestApplyAPIKeyChoicePersists(t *testing.T) {
	loadCfg(t)

	const listed = "claude-sonnet-5"
	want := models.ModelID(string(models.ProviderAnthropic) + "." + listed)

	orig := fetchProviderCatalogue
	fetchProviderCatalogue = func(p models.ModelProvider, apiKey, dir string) (models.CatalogueResult, error) {
		// A fetch registers the models; the portal then asks the registry which
		// one to start on. Mirror both halves.
		models.SupportedModels[want] = models.Model{
			ID: want, Name: "Claude Sonnet 5", Provider: p, APIModel: listed,
			ContextWindow: 1_000_000, DefaultMaxTokens: 8192,
		}
		return models.CatalogueResult{Provider: p, Label: "Anthropic", Usable: 1}, nil
	}
	t.Cleanup(func() {
		fetchProviderCatalogue = orig
		delete(models.SupportedModels, want)
	})

	const key = "sk-ant-testonly-000"
	err := applyPortalChoice(context.Background(), startup.ProviderChoice{ID: "anthropic", Input: key})
	if err != nil {
		t.Fatalf("applyPortalChoice: %v", err)
	}
	c := readCfgFile(t)
	if got := c.Providers[models.ProviderAnthropic].APIKey; got != key {
		t.Fatalf("anthropic key not persisted: got %q", got)
	}
	if got := c.Agents[config.AgentCoder].Model; got != want {
		t.Fatalf("coder not moved to the fetched anthropic model: got %q, want %q", got, want)
	}
	if got := c.Agents[config.AgentTitle].Model; got != want {
		t.Fatalf("title not moved: got %q", got)
	}
}

// A provider whose listing fails must report it rather than silently leaving the
// agents pointed somewhere else. The key is still saved — it may be perfectly
// good and the network simply down — so /update can retry it later.
func TestApplyAPIKeyChoiceReportsAFailedListing(t *testing.T) {
	loadCfg(t)

	orig := fetchProviderCatalogue
	fetchProviderCatalogue = func(models.ModelProvider, string, string) (models.CatalogueResult, error) {
		return models.CatalogueResult{}, errors.New("connection refused")
	}
	t.Cleanup(func() { fetchProviderCatalogue = orig })

	err := applyPortalChoice(context.Background(), startup.ProviderChoice{ID: "groq", Input: "gsk-testonly-000"})
	if err == nil {
		t.Fatal("a failed listing was reported as success; the picker would show no Groq models and say nothing")
	}
	if !strings.Contains(err.Error(), "saved the key") {
		t.Errorf("error does not say the key was kept: %q", err)
	}
	if got := readCfgFile(t).Providers[models.ProviderGROQ].APIKey; got == "" {
		t.Error("the key was discarded because the listing failed; /update could never retry it")
	}
}

// A local endpoint that lists no models is a failure, surfaced as an error so
// the portal loop re-opens rather than silently leaving a dead endpoint.
func TestApplyLocalEndpointNoModelsErrors(t *testing.T) {
	loadCfg(t)
	orig := registerLocalEndpoint
	defer func() { registerLocalEndpoint = orig }()
	registerLocalEndpoint = func(name, base, key string) (int, models.ModelID) { return 0, "" }

	err := applyLocalEndpoint(nimEndpointName, nimBaseURL, "nvapi-xxx")
	if err == nil {
		t.Fatal("expected an error when no models are found")
	}
}

// A successful local endpoint saves the endpoint (with its key) and points the
// agents at the first discovered model.
func TestApplyLocalEndpointSuccess(t *testing.T) {
	loadCfg(t)

	fakeID := models.ModelID("local.faketest-model")
	models.SupportedModels[fakeID] = models.Model{
		ID: fakeID, Provider: models.ProviderLocal, ContextWindow: 8192, DefaultMaxTokens: 4096,
	}
	models.RegisterLocalRouteForTestNamed(fakeID, nimBaseURL, "nvapi-xxx", nimEndpointName)
	defer func() {
		delete(models.SupportedModels, fakeID)
		models.ClearLocalRouteForTest(fakeID)
	}()

	orig := registerLocalEndpoint
	defer func() { registerLocalEndpoint = orig }()
	registerLocalEndpoint = func(name, base, key string) (int, models.ModelID) { return 1, fakeID }

	if err := applyLocalEndpoint(nimEndpointName, nimBaseURL, "nvapi-xxx"); err != nil {
		t.Fatalf("applyLocalEndpoint: %v", err)
	}
	c := readCfgFile(t)
	var found *config.LocalEndpoint
	for i := range c.LocalEndpoints {
		if c.LocalEndpoints[i].Name == nimEndpointName {
			found = &c.LocalEndpoints[i]
		}
	}
	if found == nil {
		t.Fatal("NIM endpoint not persisted")
	}
	if found.APIKey != "nvapi-xxx" {
		t.Fatalf("endpoint key not persisted: got %q", found.APIKey)
	}
	if got := c.Agents[config.AgentCoder].Model; got != fakeID {
		t.Fatalf("coder not pointed at discovered model: got %q", got)
	}
}

// An empty key on a re-selection must reuse the stored key, not blank it — this
// is what Enter-on-a-ready-row means.
func TestApplyLocalEndpointEmptyKeyReusesStored(t *testing.T) {
	loadCfg(t)
	// Seed a stored key.
	if err := config.UpsertLocalEndpoint(config.LocalEndpoint{
		Name: ollamaEndpointName, BaseURL: ollamaBaseURL, APIKey: "seeded-key",
	}); err != nil {
		t.Fatalf("seeding endpoint: %v", err)
	}

	fakeID := models.ModelID("local.faketest-ollama")
	models.SupportedModels[fakeID] = models.Model{
		ID: fakeID, Provider: models.ProviderLocal, ContextWindow: 8192, DefaultMaxTokens: 4096,
	}
	models.RegisterLocalRouteForTestNamed(fakeID, ollamaBaseURL, "seeded-key", ollamaEndpointName)
	defer func() {
		delete(models.SupportedModels, fakeID)
		models.ClearLocalRouteForTest(fakeID)
	}()

	var gotKey string
	orig := registerLocalEndpoint
	defer func() { registerLocalEndpoint = orig }()
	registerLocalEndpoint = func(name, base, key string) (int, models.ModelID) {
		gotKey = key
		return 1, fakeID
	}

	if err := applyLocalEndpoint(ollamaEndpointName, ollamaBaseURL, ""); err != nil {
		t.Fatalf("applyLocalEndpoint: %v", err)
	}
	if gotKey != "seeded-key" {
		t.Fatalf("empty key should have reused the stored key, got %q", gotKey)
	}
}

// Every row the portal presents must be handled by applyPortalChoice's switch —
// the guard that stops a future new row from silently doing nothing. Checked
// structurally so no OAuth/network side effect runs.
func TestEveryPortalRowIsHandled(t *testing.T) {
	loadCfg(t)
	handled := map[string]bool{
		"antigravity":  true,
		"chatgpt":      true,
		"google-oauth": true,
		"gcp-custom":   true,
		"nvidia-nim":   true,
		"ollama":       true,
		"cloudflare":   true,
	}
	for id := range portalProvider {
		handled[id] = true
	}
	rows, _ := providerPortalRows()
	if len(rows) == 0 {
		t.Fatal("no rows produced")
	}
	for _, r := range rows {
		if !handled[r.ID] {
			t.Fatalf("row %q is presented but not handled by applyPortalChoice", r.ID)
		}
	}
}
