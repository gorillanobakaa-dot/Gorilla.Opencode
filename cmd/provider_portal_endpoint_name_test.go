package cmd

import (
	"testing"

	"github.com/opencode-ai/opencode/internal/config"
	"github.com/opencode-ai/opencode/internal/llm/models"
)

// nimRow returns the NVIDIA NIM row from the portal menu.
func nimRow(t *testing.T) (configured bool) {
	t.Helper()
	rows, _ := providerPortalRows()
	for _, r := range rows {
		if r.ID == "nvidia-nim" {
			return r.Configured
		}
	}
	t.Fatal("the portal has no nvidia-nim row")
	return false
}

// THE BUG: the portal decided whether NIM was set up by looking for an endpoint
// NAMED "NVIDIA NIM". An endpoint is identified by where it points, and people
// name theirs whatever they like — this user's is "Gorilla.FREE.NVIDIA.NIM".
// A perfectly good, keyed endpoint was therefore invisible: the row showed as
// unconfigured and the portal demanded the key again on every single launch.
func TestAUserNamedNimEndpointCountsAsConfigured(t *testing.T) {
	loadCfg(t)
	const userName = "Gorilla.FREE.NVIDIA.NIM"

	if err := config.UpsertLocalEndpoint(config.LocalEndpoint{
		Name: userName, BaseURL: nimBaseURL, APIKey: "nvapi-seeded-for-test",
	}); err != nil {
		t.Fatalf("seeding endpoint: %v", err)
	}
	defer config.RemoveLocalEndpoint(userName)

	if !nimRow(t) {
		t.Error("a keyed NIM endpoint under the user's own name is not recognised, " +
			"so the portal asks for the key again on every launch")
	}
}

// The negative case, so the assertion above cannot pass vacuously: with no
// endpoint at that URL at all, the row must NOT claim to be configured.
func TestNoNimEndpointMeansNotConfigured(t *testing.T) {
	loadCfg(t)
	for _, e := range config.Get().LocalEndpoints {
		if e.BaseURL == nimBaseURL {
			t.Skipf("an endpoint at %s is already present; cannot test the empty case", nimBaseURL)
		}
	}
	if nimRow(t) {
		t.Error("the NIM row claims to be configured with no endpoint at that URL")
	}
}

// A keyless entry at the same URL must not be mistaken for a working one.
func TestNimEndpointWithoutAKeyIsNotConfigured(t *testing.T) {
	loadCfg(t)
	const name = "keyless-nim-test"
	if err := config.UpsertLocalEndpoint(config.LocalEndpoint{
		Name: name, BaseURL: nimBaseURL, APIKey: "",
	}); err != nil {
		t.Fatalf("seeding endpoint: %v", err)
	}
	defer config.RemoveLocalEndpoint(name)

	if nimRow(t) {
		t.Error("an endpoint with no API key is being reported as configured")
	}
}

// THE OTHER HALF OF THE SAME BUG: applying a local-endpoint choice wrote our
// fixed name, creating a SECOND endpoint beside the user's own on the same
// baseURL. Two endpoints on one URL steal each other's model routes (last one
// wins), which is how both ended up with zero registered models.
func TestApplyingReusesTheUsersEndpointInsteadOfCreatingATwin(t *testing.T) {
	loadCfg(t)
	const userName = "Gorilla.FREE.NVIDIA.NIM"

	if err := config.UpsertLocalEndpoint(config.LocalEndpoint{
		Name: userName, BaseURL: nimBaseURL, APIKey: "nvapi-original",
	}); err != nil {
		t.Fatalf("seeding endpoint: %v", err)
	}
	defer config.RemoveLocalEndpoint(userName)
	defer config.RemoveLocalEndpoint(nimEndpointName) // in case the bug returns

	fakeID := models.ModelID("local.faketest-nim")
	models.SupportedModels[fakeID] = models.Model{
		ID: fakeID, Provider: models.ProviderLocal, ContextWindow: 8192, DefaultMaxTokens: 4096,
	}
	// The route must exist under the USER's endpoint name, or setting the agent
	// model fails validation before the assertions below are reached.
	models.RegisterLocalRouteForTestNamed(fakeID, nimBaseURL, "nvapi-original", userName)
	defer func() {
		delete(models.SupportedModels, fakeID)
		models.ClearLocalRouteForTest(fakeID)
	}()

	var gotName, gotKey string
	orig := registerLocalEndpoint
	defer func() { registerLocalEndpoint = orig }()
	registerLocalEndpoint = func(name, base, key string) (int, models.ModelID) {
		gotName, gotKey = name, key
		return 1, fakeID
	}

	// The portal passes ITS default name; the user's must win.
	if err := applyLocalEndpoint(nimEndpointName, nimBaseURL, ""); err != nil {
		t.Fatalf("applyLocalEndpoint: %v", err)
	}

	if gotName != userName {
		t.Errorf("registered under %q instead of the user's %q — a second endpoint "+
			"on the same URL steals the first's model routes", gotName, userName)
	}
	if gotKey != "nvapi-original" {
		t.Errorf("the stored key was not carried over (got %q); pressing Enter on the "+
			"row would blank a working credential", gotKey)
	}

	// And on disk: exactly one endpoint for that URL.
	var atURL []string
	for _, e := range readCfgFile(t).LocalEndpoints {
		if e.BaseURL == nimBaseURL {
			atURL = append(atURL, e.Name)
		}
	}
	if len(atURL) != 1 || atURL[0] != userName {
		t.Errorf("expected exactly one endpoint at %s named %q, found %v",
			nimBaseURL, userName, atURL)
	}
}

// THE BUG: the portal runs on EVERY launch, and re-selecting an endpoint
// re-applied its default to all four agents — silently undoing a /models choice
// made minutes earlier. Combined with that default being the provider's first
// listed id, every login landed back on the same unusable model.
//
// Observed 2026-08-05: the user switched away from 01-ai/yi-large repeatedly and
// was returned to it after each re-login.
func TestReselectingAnEndpointKeepsTheModelAlreadyChosen(t *testing.T) {
	loadCfg(t)
	const userName = "Gorilla.FREE.NVIDIA.NIM"
	if err := config.UpsertLocalEndpoint(config.LocalEndpoint{
		Name: userName, BaseURL: nimBaseURL, APIKey: "nvapi-x",
	}); err != nil {
		t.Fatalf("seeding endpoint: %v", err)
	}
	defer config.RemoveLocalEndpoint(userName)

	chosen := models.ModelID("local.deliberate/choice")
	models.SupportedModels[chosen] = models.Model{
		ID: chosen, Provider: models.ProviderLocal, ContextWindow: 8192, DefaultMaxTokens: 4096,
	}
	models.RegisterLocalRouteForTestNamed(chosen, nimBaseURL, "nvapi-x", userName)
	defer func() {
		delete(models.SupportedModels, chosen)
		models.ClearLocalRouteForTest(chosen)
	}()
	if err := config.UpdateAgentModel(config.AgentCoder, chosen); err != nil {
		t.Fatalf("setting the deliberate choice: %v", err)
	}

	// Registration returns a DIFFERENT default, as it would on a real re-login.
	other := models.ModelID("local.some/default")
	models.SupportedModels[other] = models.Model{
		ID: other, Provider: models.ProviderLocal, ContextWindow: 8192, DefaultMaxTokens: 4096,
	}
	models.RegisterLocalRouteForTestNamed(other, nimBaseURL, "nvapi-x", userName)
	defer func() {
		delete(models.SupportedModels, other)
		models.ClearLocalRouteForTest(other)
	}()

	orig := registerLocalEndpoint
	defer func() { registerLocalEndpoint = orig }()
	registerLocalEndpoint = func(name, base, key string) (int, models.ModelID) { return 2, other }

	if err := applyLocalEndpoint(nimEndpointName, nimBaseURL, ""); err != nil {
		t.Fatalf("applyLocalEndpoint: %v", err)
	}
	if got := config.Get().Agents[config.AgentCoder].Model; got != chosen {
		t.Errorf("coder became %q; re-selecting the endpoint discarded the model the "+
			"user had deliberately chosen on it (wanted %q)", got, chosen)
	}
}

// But an agent pointing at a model that is NOT from this endpoint must still be
// moved onto it — otherwise choosing a provider would do nothing at all.
func TestSelectingAnEndpointStillAppliesWhenTheModelIsElsewhere(t *testing.T) {
	loadCfg(t)
	const userName = "Gorilla.FREE.NVIDIA.NIM"
	if err := config.UpsertLocalEndpoint(config.LocalEndpoint{
		Name: userName, BaseURL: nimBaseURL, APIKey: "nvapi-x",
	}); err != nil {
		t.Fatalf("seeding endpoint: %v", err)
	}
	defer config.RemoveLocalEndpoint(userName)

	// A model on a DIFFERENT endpoint — still local, so it can be set without a
	// configured cloud provider, but not served by the endpoint being selected.
	// (The first attempt used a Claude model and simply SKIPPED, which verifies
	// nothing.)
	elsewhere := models.ModelID("local.other-endpoint/model")
	models.SupportedModels[elsewhere] = models.Model{
		ID: elsewhere, Provider: models.ProviderLocal, ContextWindow: 8192, DefaultMaxTokens: 4096,
	}
	models.RegisterLocalRouteForTestNamed(elsewhere, ollamaBaseURL, "", "some-other-endpoint")
	defer func() {
		delete(models.SupportedModels, elsewhere)
		models.ClearLocalRouteForTest(elsewhere)
	}()
	if err := config.UpdateAgentModel(config.AgentCoder, elsewhere); err != nil {
		t.Fatalf("setting a model on another endpoint: %v", err)
	}

	fresh := models.ModelID("local.fresh/model")
	models.SupportedModels[fresh] = models.Model{
		ID: fresh, Provider: models.ProviderLocal, ContextWindow: 8192, DefaultMaxTokens: 4096,
	}
	models.RegisterLocalRouteForTestNamed(fresh, nimBaseURL, "nvapi-x", userName)
	defer func() {
		delete(models.SupportedModels, fresh)
		models.ClearLocalRouteForTest(fresh)
	}()

	orig := registerLocalEndpoint
	defer func() { registerLocalEndpoint = orig }()
	registerLocalEndpoint = func(name, base, key string) (int, models.ModelID) { return 1, fresh }

	if err := applyLocalEndpoint(nimEndpointName, nimBaseURL, ""); err != nil {
		t.Fatalf("applyLocalEndpoint: %v", err)
	}
	if got := config.Get().Agents[config.AgentCoder].Model; got != fresh {
		t.Errorf("coder is %q; picking an endpoint whose models were not in use must "+
			"actually switch to it (wanted %q)", got, fresh)
	}
}
