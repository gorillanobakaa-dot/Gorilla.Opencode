package cmd

import (
	"testing"

	"github.com/opencode-ai/opencode/internal/config"
	"github.com/opencode-ai/opencode/internal/llm/models"
)

// Cloudflare needs TWO values, and presents them in separate boxes on a page
// full of prose and a sample curl command. The most error-prone step of the
// setup must not also be a typing exercise, so the whole page can be pasted.
//
// The fixtures below are shaped like what Cloudflare actually shows.
const (
	cfTestAccount = "1a2c0afcb37af9546ec7d80e7b8921c2"
	cfTestToken   = "cfut_3xampleTokenValueThatIsLongEnough123456"
)

// Each field takes ONE value, but the value is still matched by shape rather
// than trusted verbatim — people paste quotes, "Bearer " prefixes, trailing
// whitespace and the odd stray line, and rejecting that would be pedantry.
func TestCloudflareFieldsToleratePastedNoise(t *testing.T) {
	for _, c := range []struct{ acc, tok string }{
		{cfTestAccount, cfTestToken},
		{"  " + cfTestAccount + "  ", "Bearer " + cfTestToken},
		{"Account ID\n" + cfTestAccount, `"` + cfTestToken + `"`},
	} {
		if err := applyCloudflareCheck(c.acc, c.tok); err != nil {
			t.Errorf("account=%q token=%q rejected: %v", c.acc, c.tok, err)
		}
	}
}

// A value in the WRONG field must be refused with a message naming that field,
// not a generic failure — swapping the two is the obvious mistake to make.
func TestCloudflareFieldsRejectTheWrongValue(t *testing.T) {
	if err := applyCloudflareCheck(cfTestToken, cfTestToken); err == nil {
		t.Error("a token in the account-ID field was accepted")
	} else if !contains(err.Error(), "account ID") {
		t.Errorf("error does not name the account ID field: %v", err)
	}

	if err := applyCloudflareCheck(cfTestAccount, cfTestAccount); err == nil {
		t.Error("an account ID in the token field was accepted")
	} else if !contains(err.Error(), "API token") {
		t.Errorf("error does not name the token field: %v", err)
	}
}

// applyCloudflareCheck exercises only the validation half of applyCloudflare,
// so the test does not reach the network.
func applyCloudflareCheck(account, token string) error {
	if cfAccountRe.FindString(account) == "" {
		return errAccount
	}
	if cfTokenRe.FindString(token) == "" {
		return errToken
	}
	return nil
}

var (
	errAccount = errTest("that does not look like a Cloudflare account ID")
	errToken   = errTest("that does not look like a Cloudflare API token")
)

type errTest string

func (e errTest) Error() string { return string(e) }

// The row must report itself configured once a keyed Cloudflare endpoint
// exists — otherwise the portal asks for the credentials again on every launch,
// which is exactly the bug fixed for NVIDIA in v0.1.69.
func TestCloudflareRowGoesReadyOnceConfigured(t *testing.T) {
	loadCfg(t)
	cfRow := func() bool {
		rows, _ := providerPortalRows()
		for _, r := range rows {
			if r.ID == "cloudflare" {
				return r.Configured
			}
		}
		t.Fatal("the portal has no cloudflare row")
		return false
	}

	if cfRow() {
		t.Skip("a Cloudflare endpoint is already configured; cannot test the empty case")
	}

	const name = "cf-test-endpoint"
	if err := config.UpsertLocalEndpoint(config.LocalEndpoint{
		Name:    name,
		BaseURL: "https://api.cloudflare.com/client/v4/accounts/" + cfTestAccount + "/ai/v1",
		APIKey:  cfTestToken,
	}); err != nil {
		t.Fatalf("seeding endpoint: %v", err)
	}
	defer config.RemoveLocalEndpoint(name)

	if !cfRow() {
		t.Error("a keyed Cloudflare endpoint is not recognised, so the portal would " +
			"demand the account ID and token again on every launch")
	}
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

// THE BUG: selecting an already-configured Cloudflare row supplies NO values,
// because the portal only asks for input when a row is not yet configured.
// applyCloudflare rejected that as a malformed account ID — for credentials
// that were saved and working. Reported 2026-08-05: Enter did nothing three
// times and the cursor bounced back to the previous provider.
func TestSelectingAConfiguredCloudflareRowReusesTheSavedEndpoint(t *testing.T) {
	loadCfg(t)
	const name = "cf-reuse-test"
	base := "https://api.cloudflare.com/client/v4/accounts/" + cfTestAccount + "/ai/v1"
	if err := config.UpsertLocalEndpoint(config.LocalEndpoint{
		Name: name, BaseURL: base, APIKey: cfTestToken,
	}); err != nil {
		t.Fatalf("seeding endpoint: %v", err)
	}
	defer config.RemoveLocalEndpoint(name)

	var gotName, gotBase, gotKey string
	orig := registerLocalEndpoint
	defer func() { registerLocalEndpoint = orig }()
	fake := models.ModelID("local.cf/fake")
	models.SupportedModels[fake] = models.Model{
		ID: fake, Provider: models.ProviderLocal, ContextWindow: 8192, DefaultMaxTokens: 4096,
	}
	models.RegisterLocalRouteForTestNamed(fake, base, cfTestToken, name)
	defer func() {
		delete(models.SupportedModels, fake)
		models.ClearLocalRouteForTest(fake)
	}()
	registerLocalEndpoint = func(n, b, k string) (int, models.ModelID) {
		gotName, gotBase, gotKey = n, b, k
		return 1, fake
	}

	// Exactly what the portal sends for a configured row: nothing.
	if err := applyCloudflare("", ""); err != nil {
		t.Fatalf("selecting a configured row failed: %v", err)
	}
	if gotName != name || gotBase != base || gotKey != cfTestToken {
		t.Errorf("did not reuse the saved endpoint: name=%q base=%q keyLen=%d",
			gotName, gotBase, len(gotKey))
	}
}

// With nothing saved, the empty case must say what to do rather than complain
// about a value the user was never asked for.
func TestNoSavedCloudflareCredentialsSaysPressR(t *testing.T) {
	loadCfg(t)
	for _, e := range config.Get().LocalEndpoints {
		if contains(e.BaseURL, "api.cloudflare.com") {
			t.Skip("a Cloudflare endpoint is configured; cannot test the empty case")
		}
	}
	err := applyCloudflare("", "")
	if err == nil {
		t.Fatal("empty input with nothing saved was accepted")
	}
	if !contains(err.Error(), "press r") {
		t.Errorf("error does not tell the user how to enter credentials: %v", err)
	}
}
