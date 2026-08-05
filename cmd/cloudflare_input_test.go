package cmd

import (
	"testing"

	"github.com/opencode-ai/opencode/internal/config"
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

func TestCloudflareInputAcceptsAWholePastedPage(t *testing.T) {
	pasted := `Test your API token with the following CURL command:

curl "https://api.cloudflare.com/client/v4/user/tokens/verify" \
  -H "Authorization: Bearer ` + cfTestToken + `"

Get Account ID
Use this account ID to make API calls to the Workers AI REST API.

` + cfTestAccount + `

curl https://api.cloudflare.com/client/v4/accounts/` + cfTestAccount + `/ai/run/@cf/openai/gpt-oss-120b`

	acc, tok, err := parseCloudflareInput(pasted)
	if err != nil {
		t.Fatalf("rejected a straight paste of Cloudflare's own page: %v", err)
	}
	if acc != cfTestAccount {
		t.Errorf("account = %q, want %q", acc, cfTestAccount)
	}
	if tok != cfTestToken {
		t.Errorf("token = %q, want %q", tok, cfTestToken)
	}
}

// Order must not matter: people paste them in whichever order they copied.
func TestCloudflareInputIsOrderIndependent(t *testing.T) {
	for _, in := range []string{
		cfTestAccount + " " + cfTestToken,
		cfTestToken + " " + cfTestAccount,
		cfTestAccount + ":" + cfTestToken,
		"account=" + cfTestAccount + "\ntoken=" + cfTestToken,
	} {
		acc, tok, err := parseCloudflareInput(in)
		if err != nil {
			t.Errorf("input %q rejected: %v", in, err)
			continue
		}
		if acc != cfTestAccount || tok != cfTestToken {
			t.Errorf("input %q gave account=%q token=%q", in, acc, tok)
		}
	}
}

// The account id must never be mistaken for the token. Both are hex-ish blobs,
// and returning the account id as the token would produce a 401 that looks like
// a bad key rather than a parsing mistake.
func TestAccountIDIsNeverReturnedAsTheToken(t *testing.T) {
	if _, tok, err := parseCloudflareInput(cfTestAccount); err == nil {
		t.Errorf("an account ID alone was accepted, with token=%q", tok)
	}
}

// Each missing half gets its own message, naming what is missing and where to
// find it — "invalid input" would leave the user guessing which of the two
// values was wrong.
func TestCloudflareInputSaysWhichHalfIsMissing(t *testing.T) {
	if _, _, err := parseCloudflareInput(cfTestToken); err == nil {
		t.Error("a token with no account ID was accepted")
	} else if !contains(err.Error(), "account ID") {
		t.Errorf("error does not name the missing account ID: %v", err)
	}

	if _, _, err := parseCloudflareInput(cfTestAccount + " and nothing else"); err == nil {
		t.Error("an account ID with no token was accepted")
	} else if !contains(err.Error(), "token") {
		t.Errorf("error does not name the missing token: %v", err)
	}

	if _, _, err := parseCloudflareInput("neither of them here"); err == nil {
		t.Error("junk input was accepted")
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
