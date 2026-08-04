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
