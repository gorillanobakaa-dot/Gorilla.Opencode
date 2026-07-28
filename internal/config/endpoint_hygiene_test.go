package config

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/opencode-ai/opencode/internal/llm/models"
)

// One user config held four entries aimed at the same NVIDIA URL: the same key
// four times, twice with its "nvapi-" prefix and twice without. /connect could
// add and disable endpoints but never remove one, so clearing them needed a
// hand-edit of config.json.
func TestRemoveLocalEndpointDeletesItFromTheFile(t *testing.T) {
	if _, err := Load(t.TempDir(), false); err != nil {
		t.Fatalf("Load: %v", err)
	}
	prev := cfg.LocalEndpoints
	t.Cleanup(func() { cfg.LocalEndpoints = prev })

	cfg.LocalEndpoints = []LocalEndpoint{
		{Name: "keeper", BaseURL: "http://127.0.0.1:1/v1", APIKey: "nvapi-good"},
		{Name: "typo", BaseURL: "http://127.0.0.1:1/v1", APIKey: "nvapi-good"},
	}
	if err := UpsertLocalEndpoint(cfg.LocalEndpoints[0]); err != nil {
		t.Fatalf("seed: %v", err)
	}
	if err := UpsertLocalEndpoint(cfg.LocalEndpoints[1]); err != nil {
		t.Fatalf("seed: %v", err)
	}

	if err := RemoveLocalEndpoint("typo"); err != nil {
		t.Fatalf("RemoveLocalEndpoint: %v", err)
	}

	// In memory.
	for _, ep := range cfg.LocalEndpoints {
		if ep.Name == "typo" {
			t.Error("the endpoint is still in the live config")
		}
	}
	// And on disk — a removal that does not persist reappears at next launch,
	// which is exactly the complaint. Read the file rather than trusting the
	// globals.
	names := endpointNamesOnDisk(t)
	for _, n := range names {
		if n == "typo" {
			t.Errorf("the endpoint is still in config.json: %v", names)
		}
	}
	if len(names) != 1 || names[0] != "keeper" {
		t.Errorf("expected only \"keeper\" left on disk, got %v", names)
	}
}

// Removing an unknown name must report it rather than silently succeeding.
func TestRemoveUnknownLocalEndpointErrors(t *testing.T) {
	if _, err := Load(t.TempDir(), false); err != nil {
		t.Fatalf("Load: %v", err)
	}
	if err := RemoveLocalEndpoint("no-such-endpoint"); err == nil {
		t.Error("removing a name that is not configured reported success")
	}
}

// The trap that made this worth a test: the duplicates being cleaned up SHARE a
// baseURL with the endpoint that is staying, and only one of them owns the
// registered models. Unregistering by URL would take the survivor's models down
// with the entry being removed — leaving the user with a tidy config and an
// empty model picker.
func TestRemovingADuplicateKeepsTheSurvivorsModels(t *testing.T) {
	if _, err := Load(t.TempDir(), false); err != nil {
		t.Fatalf("Load: %v", err)
	}
	prev := cfg.LocalEndpoints
	t.Cleanup(func() { cfg.LocalEndpoints = prev })

	const sharedURL = "http://127.0.0.1:1/v1"
	cfg.LocalEndpoints = []LocalEndpoint{
		{Name: "keeper", BaseURL: sharedURL, APIKey: "nvapi-good"},
		{Name: "duplicate", BaseURL: sharedURL, APIKey: "nvapi-good"},
	}
	if err := UpsertLocalEndpoint(cfg.LocalEndpoints[0]); err != nil {
		t.Fatalf("seed: %v", err)
	}

	// Stand in for what a live /v1/models fetch would have registered: models
	// owned by "keeper", at the URL "duplicate" also points to.
	const id models.ModelID = "local.keeper/glm-test"
	models.SupportedModels[id] = models.Model{
		ID: id, Name: "test", Provider: models.ProviderLocal,
		ContextWindow: 8192, DefaultMaxTokens: 2048,
	}
	models.RegisterLocalRouteForTestNamed(id, sharedURL, "nvapi-good", "keeper")
	t.Cleanup(func() {
		delete(models.SupportedModels, id)
		models.ClearLocalRouteForTest(id)
	})

	if err := RemoveLocalEndpoint("duplicate"); err != nil {
		t.Fatalf("RemoveLocalEndpoint: %v", err)
	}

	if _, _, ok := models.LocalRouteFor(id); !ok {
		t.Error("removing \"duplicate\" unregistered the route owned by \"keeper\" — they share a baseURL, so removal must match on endpoint name, not URL")
	}
	if _, ok := models.SupportedModels[id]; !ok {
		t.Error("keeper's model vanished from the picker when a different endpoint was removed")
	}
}

// A key pasted without its "nvapi-" prefix is the specific mistake that hides
// itself: NVIDIA serves /v1/models unauthenticated, so the endpoint lists all
// its models and looks connected, then 401s on every real request.
func TestNormaliseLocalAPIKeyRestoresTheNvidiaPrefix(t *testing.T) {
	const nvidia = "https://integrate.api.nvidia.com/v1"
	const body = "PKZM6Tabcdef"

	got, note := NormaliseLocalAPIKey(nvidia, body)
	if got != "nvapi-"+body {
		t.Errorf("key = %q, want the nvapi- prefix restored", got)
	}
	if note == "" {
		t.Error("the repair was silent; the user will paste it the same way next time")
	}

	// Already correct: left exactly alone, and no note to explain a non-change.
	full := "nvapi-" + body
	if got, note := NormaliseLocalAPIKey(nvidia, full); got != full || note != "" {
		t.Errorf("a correct key was altered: %q (note %q)", got, note)
	}

	// Idempotent — normalising twice must not stack prefixes.
	once, _ := NormaliseLocalAPIKey(nvidia, body)
	twice, _ := NormaliseLocalAPIKey(nvidia, once)
	if twice != once {
		t.Errorf("not idempotent: %q then %q", once, twice)
	}

	// Whitespace from a paste is trimmed, and reported.
	if got, note := NormaliseLocalAPIKey(nvidia, "  "+full+"\n"); got != full || note == "" {
		t.Errorf("whitespace not handled: %q (note %q)", got, note)
	}

	// Empty stays empty — a local Ollama needs no key at all.
	if got, _ := NormaliseLocalAPIKey("http://localhost:11434/v1", ""); got != "" {
		t.Errorf("an empty key became %q", got)
	}
}

// And it must not guess at other providers' key formats. Ollama keys, LM Studio
// keys and custom-gateway tokens have no shape we know; prefixing them would
// break a working setup to fix an imagined one.
func TestNormaliseLocalAPIKeyLeavesNonNvidiaKeysAlone(t *testing.T) {
	for _, url := range []string{
		"http://localhost:11434/v1",
		"http://localhost:1234/v1",
		"https://my-gateway.example.com/v1",
	} {
		const key = "PKZM6Tabcdef"
		got, note := NormaliseLocalAPIKey(url, key)
		if got != key {
			t.Errorf("%s: key rewritten to %q — we do not know this provider's key format", url, got)
		}
		if note != "" {
			t.Errorf("%s: reported a change it did not make: %q", url, note)
		}
	}
}

func endpointNamesOnDisk(t *testing.T) []string {
	t.Helper()
	raw, err := os.ReadFile(GorillaConfigFile())
	if err != nil {
		t.Fatalf("read config: %v", err)
	}
	var onDisk Config
	if err := json.Unmarshal(raw, &onDisk); err != nil {
		t.Fatalf("parse config: %v", err)
	}
	names := make([]string, 0, len(onDisk.LocalEndpoints))
	for _, ep := range onDisk.LocalEndpoints {
		names = append(names, ep.Name)
	}
	return names
}

// The bug this test exists for was found by accident, while asserting a removal
// persisted: nothing was on disk to remove from, because the write before it had
// been discarded.
//
// updateCfgFile keyed "is there a config file?" off viper.ConfigFileUsed(), which
// stays empty for the WHOLE PROCESS when no config.json existed at startup —
// nothing re-runs ReadInConfig. So every write re-based from a literal `{}` and
// threw away the previous one. On a fresh install: paste an API key in /connect,
// then add a local endpoint, and the key is gone. Two successive writes are all
// it takes.
func TestSuccessiveConfigWritesDoNotDiscardEachOther(t *testing.T) {
	if _, err := Load(t.TempDir(), false); err != nil {
		t.Fatalf("Load: %v", err)
	}
	// Start from the fresh-install condition: no file, and viper never found one.
	os.Remove(GorillaConfigFile())

	prevEndpoints := cfg.LocalEndpoints
	prevProviders := cfg.Providers
	t.Cleanup(func() { cfg.LocalEndpoints = prevEndpoints; cfg.Providers = prevProviders })
	cfg.LocalEndpoints = nil

	// Write one. Then write something unrelated — the second must not erase it.
	if err := UpsertProviderKey(models.ProviderGROQ, "gsk-test-key"); err != nil {
		t.Fatalf("first write: %v", err)
	}
	if err := UpsertLocalEndpoint(LocalEndpoint{Name: "second-write", BaseURL: "http://127.0.0.1:1/v1"}); err != nil {
		t.Fatalf("second write: %v", err)
	}

	raw, err := os.ReadFile(GorillaConfigFile())
	if err != nil {
		t.Fatalf("read config: %v", err)
	}
	var onDisk Config
	if err := json.Unmarshal(raw, &onDisk); err != nil {
		t.Fatalf("parse config: %v", err)
	}

	if got := onDisk.Providers[models.ProviderGROQ].APIKey; got != "gsk-test-key" {
		t.Errorf("the API key written first was discarded by the next write (got %q) — on a fresh install this silently loses whatever the user configured a moment ago", got)
	}
	if len(onDisk.LocalEndpoints) != 1 || onDisk.LocalEndpoints[0].Name != "second-write" {
		t.Errorf("the second write did not land: %+v", onDisk.LocalEndpoints)
	}
}
