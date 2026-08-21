// Version: 1.0.0 · updated 26-08-21-13-05
package models

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// snapshotRegistry restores SupportedModels and LiveCatalogues after a test.
// Both are package globals; a leak between tests in this package is a known trap
// here (see the loadout note in CLAUDE.md).
func snapshotRegistry(t *testing.T) {
	t.Helper()
	savedModels := make(map[ModelID]Model, len(SupportedModels))
	for k, v := range SupportedModels {
		savedModels[k] = v
	}
	savedCats := make(map[ModelProvider]LiveCatalogue, len(LiveCatalogues))
	for k, v := range LiveCatalogues {
		savedCats[k] = v
	}
	t.Cleanup(func() {
		SupportedModels = savedModels
		LiveCatalogues = savedCats
	})
}

// stubCatalogue points a provider at a local test server returning ids.
func stubCatalogue(t *testing.T, p ModelProvider, cat LiveCatalogue, body string) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)
	cat.URL = srv.URL
	LiveCatalogues[p] = cat
	return srv
}

// The whole point of the change: a model the provider has RETIRED disappears
// from the registry. Before this, presence was decided by a Go file, so Groq's
// five decommissioned models sat in the picker looking selectable and returned
// HTTP 400 the moment anyone chose one.
func TestFetchDropsModelsTheProviderNoLongerServes(t *testing.T) {
	snapshotRegistry(t)
	stubCatalogue(t, ProviderGROQ, LiveCatalogues[ProviderGROQ],
		`{"data":[{"id":"openai/gpt-oss-120b"}]}`)

	// The dead one, as it would have been left by the old hardcoded list.
	dead := ModelID("groq.llama-3.3-70b-versatile")
	SupportedModels[dead] = Model{ID: dead, Provider: ProviderGROQ}

	res, err := FetchProviderCatalogue(ProviderGROQ, "test-key", t.TempDir())
	if err != nil {
		t.Fatalf("FetchProviderCatalogue: %v", err)
	}
	if _, still := SupportedModels[dead]; still {
		t.Error("a retired model survived the fetch; it will 400 the next time it is picked")
	}
	if _, ok := SupportedModels["groq.openai/gpt-oss-120b"]; !ok {
		t.Error("the model the provider actually serves was not registered")
	}
	if res.Usable != 1 {
		t.Errorf("Usable=%d, want 1", res.Usable)
	}
	if len(res.Removed) != 1 || res.Removed[0] != string(dead) {
		t.Errorf("Removed=%v — the retirement must be reportable, that is what /update tells the user", res.Removed)
	}
}

// Non-chat entries are filtered at the source. A safety classifier answers a
// chat request with a bare HTTP 400, which reads as a broken key rather than a
// wrong model — see the cannotChat note in local.go.
func TestFetchSkipsNonChatModels(t *testing.T) {
	snapshotRegistry(t)
	stubCatalogue(t, ProviderOpenAI, LiveCatalogues[ProviderOpenAI], `{"data":[
		{"id":"gpt-5.5"},
		{"id":"whisper-1"},
		{"id":"text-embedding-3-large"},
		{"id":"dall-e-3"},
		{"id":"omni-moderation-latest"}
	]}`)

	res, err := FetchProviderCatalogue(ProviderOpenAI, "sk-test", t.TempDir())
	if err != nil {
		t.Fatalf("FetchProviderCatalogue: %v", err)
	}
	if res.Usable != 1 {
		t.Errorf("Usable=%d, want only the chat model", res.Usable)
	}
	if res.Skipped != 4 {
		t.Errorf("Skipped=%d, want 4 — transcription, embedding, image and moderation models are not chat models", res.Skipped)
	}
	for id := range SupportedModels {
		if strings.Contains(string(id), "whisper") || strings.Contains(string(id), "embedding") {
			t.Errorf("%s reached the picker", id)
		}
	}
}

// An empty list must be an ERROR, not an empty registration. Registering it
// would silently blank the provider and look exactly like a successful refresh —
// the "silence and success must never look alike" rule.
func TestEmptyListingIsAFailureNotAnEmptyProvider(t *testing.T) {
	snapshotRegistry(t)
	stubCatalogue(t, ProviderCerebras, LiveCatalogues[ProviderCerebras], `{"data":[]}`)

	keep := ModelID("cerebras.zai-glm-4.7")
	SupportedModels[keep] = Model{ID: keep, Provider: ProviderCerebras}

	if _, err := FetchProviderCatalogue(ProviderCerebras, "csk-test", t.TempDir()); err == nil {
		t.Fatal("an empty listing was accepted as a successful refresh")
	}
	if _, ok := SupportedModels[keep]; !ok {
		t.Error("the previous list was wiped by a failed fetch, leaving the provider empty")
	}
}

// A refused key says so. "Provider is down" and "your key is wrong" need
// different actions from the user, and only one of them is worth retrying.
func TestRefusedKeyIsReportedAsSuch(t *testing.T) {
	snapshotRegistry(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	t.Cleanup(srv.Close)
	cat := LiveCatalogues[ProviderXAI]
	cat.URL = srv.URL
	LiveCatalogues[ProviderXAI] = cat

	_, err := FetchProviderCatalogue(ProviderXAI, "xai-wrong", t.TempDir())
	if err == nil {
		t.Fatal("a 401 was treated as success")
	}
	if !strings.Contains(err.Error(), "refused the key") {
		t.Errorf("error does not name the cause: %q", err)
	}
}

// Anthropic authenticates with x-api-key, not a bearer token, and publishes real
// context limits. Both are load-bearing: the wrong header is a 401 on a valid
// key, and a missing context window would silently become the 32K floor on a 1M
// model.
func TestAnthropicAuthHeaderAndPublishedLimits(t *testing.T) {
	snapshotRegistry(t)
	var gotKey, gotBearer, gotVersion string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotKey = r.Header.Get("x-api-key")
		gotBearer = r.Header.Get("Authorization")
		gotVersion = r.Header.Get("anthropic-version")
		_, _ = w.Write([]byte(`{"data":[{"id":"claude-sonnet-5","display_name":"Claude Sonnet 5","max_input_tokens":1000000,"max_tokens":128000}]}`))
	}))
	t.Cleanup(srv.Close)
	cat := LiveCatalogues[ProviderAnthropic]
	cat.URL = srv.URL
	LiveCatalogues[ProviderAnthropic] = cat

	if _, err := FetchProviderCatalogue(ProviderAnthropic, "sk-ant-test", t.TempDir()); err != nil {
		t.Fatalf("FetchProviderCatalogue: %v", err)
	}
	if gotKey != "sk-ant-test" {
		t.Errorf("x-api-key not sent (got %q); Anthropic answers a bearer token with 401", gotKey)
	}
	if gotBearer != "" {
		t.Errorf("sent an Authorization header too: %q", gotBearer)
	}
	if gotVersion == "" {
		t.Error("anthropic-version header missing; the API requires it")
	}
	m, ok := SupportedModels["anthropic.claude-sonnet-5"]
	if !ok {
		t.Fatal("model not registered")
	}
	if m.ContextWindow != 1_000_000 {
		t.Errorf("ContextWindow=%d — the provider's own figure was ignored and the 32K floor applied", m.ContextWindow)
	}
	if m.DefaultMaxTokens != 128_000 {
		t.Errorf("DefaultMaxTokens=%d, want the published 128000", m.DefaultMaxTokens)
	}
	if !strings.Contains(m.Name, "Claude Sonnet 5") {
		t.Errorf("display name lost: %q", m.Name)
	}
}

// The default pick follows the curated order, never list order. NVIDIA returns
// its catalogue in id order, so "first" once meant 01-ai/yi-large — chosen for
// starting with a digit, and not even entitled on that account.
func TestPreferredModelFollowsTheCuratedOrderNotListOrder(t *testing.T) {
	snapshotRegistry(t)
	stubCatalogue(t, ProviderGROQ, LiveCatalogues[ProviderGROQ], `{"data":[
		{"id":"allam-2-7b"},
		{"id":"qwen/qwen3.6-27b"},
		{"id":"openai/gpt-oss-120b"}
	]}`)

	if _, err := FetchProviderCatalogue(ProviderGROQ, "test", t.TempDir()); err != nil {
		t.Fatalf("FetchProviderCatalogue: %v", err)
	}
	got := PreferredCatalogueModel(ProviderGROQ)
	if got != "groq.openai/gpt-oss-120b" {
		t.Errorf("PreferredCatalogueModel=%q, want the first entry of Prefer that is actually served", got)
	}
}

// The cache is what makes the next launch offline-capable: fetch once, read from
// disk forever after. A cold start that needed the network would put the picker
// behind a metered connection.
func TestCacheRoundTrip(t *testing.T) {
	snapshotRegistry(t)
	dir := t.TempDir()
	stubCatalogue(t, ProviderDeepSeek, LiveCatalogues[ProviderDeepSeek],
		`{"data":[{"id":"deepseek-chat"},{"id":"deepseek-reasoner"}]}`)

	if _, err := FetchProviderCatalogue(ProviderDeepSeek, "sk-test", dir); err != nil {
		t.Fatalf("FetchProviderCatalogue: %v", err)
	}
	path := filepath.Join(dir, "deepseek-models.json")
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("no cache written: %v", err)
	}

	// Wipe the registry as a cold start would, then load from disk only.
	for id, m := range SupportedModels {
		if m.Provider == ProviderDeepSeek {
			delete(SupportedModels, id)
		}
	}
	if n := LoadCachedCatalogues(dir); n < 2 {
		t.Errorf("LoadCachedCatalogues applied %d models, want 2", n)
	}
	if _, ok := SupportedModels["deepseek.deepseek-chat"]; !ok {
		t.Error("the cached model did not come back; every launch would need the network")
	}
	if CatalogueAgeFor(dir, ProviderDeepSeek).IsZero() {
		t.Error("no fetch time recorded — a list with no date is not a measurement")
	}
}

// A corrupt cache must be skipped, not fatal. A bad file leaving someone with no
// models at all is worse than an out-of-date list.
func TestCorruptCacheIsSkipped(t *testing.T) {
	snapshotRegistry(t)
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "groq-models.json"), []byte("{not json"), 0o600); err != nil {
		t.Fatal(err)
	}
	if n := LoadCachedCatalogues(dir); n != 0 {
		t.Errorf("applied %d models from a corrupt cache", n)
	}
}

// Two providers serving the same model must not collide in the flat registry —
// Groq and Cerebras both serve gpt-oss-120b.
func TestSameModelFromTwoProvidersCoexists(t *testing.T) {
	snapshotRegistry(t)
	dir := t.TempDir()
	stubCatalogue(t, ProviderGROQ, LiveCatalogues[ProviderGROQ], `{"data":[{"id":"openai/gpt-oss-120b"}]}`)
	stubCatalogue(t, ProviderCerebras, LiveCatalogues[ProviderCerebras], `{"data":[{"id":"gpt-oss-120b"}]}`)

	if _, err := FetchProviderCatalogue(ProviderGROQ, "k", dir); err != nil {
		t.Fatal(err)
	}
	if _, err := FetchProviderCatalogue(ProviderCerebras, "k", dir); err != nil {
		t.Fatal(err)
	}
	groq, okG := SupportedModels["groq.openai/gpt-oss-120b"]
	cer, okC := SupportedModels["cerebras.gpt-oss-120b"]
	if !okG || !okC {
		t.Fatal("one provider's entry overwrote the other's")
	}
	if !strings.Contains(groq.Name, "Groq") || !strings.Contains(cer.Name, "Cerebras") {
		t.Errorf("rows do not say who serves them: %q / %q", groq.Name, cer.Name)
	}
}

// verify.go must not grow a second copy of the endpoint table. This is the
// launcher-in-three-places trap in table form.
func TestVerifyTableIsDerivedFromLiveCatalogues(t *testing.T) {
	if len(CatalogueEndpoints) != len(LiveCatalogues) {
		t.Fatalf("CatalogueEndpoints has %d entries, LiveCatalogues %d — the two tables have drifted",
			len(CatalogueEndpoints), len(LiveCatalogues))
	}
	for p, cat := range LiveCatalogues {
		ep, ok := CatalogueEndpoints[p]
		if !ok {
			t.Errorf("%s is fetched but cannot be verified", p)
			continue
		}
		if ep.URL != cat.URL {
			t.Errorf("%s: verify checks %q, the fetcher reads %q", p, ep.URL, cat.URL)
		}
	}
}

// The cache file must never contain a credential — it sits beside files that do.
func TestCacheHoldsNoCredential(t *testing.T) {
	snapshotRegistry(t)
	dir := t.TempDir()
	const key = "gsk-secret-value-000"
	stubCatalogue(t, ProviderGROQ, LiveCatalogues[ProviderGROQ], `{"data":[{"id":"openai/gpt-oss-120b"}]}`)
	if _, err := FetchProviderCatalogue(ProviderGROQ, key, dir); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(filepath.Join(dir, "groq-models.json"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(b), key) {
		t.Error("the API key was written into the model cache")
	}
	var probe map[string]any
	if err := json.Unmarshal(b, &probe); err != nil {
		t.Errorf("cache is not valid JSON: %v", err)
	}
}
