package auth

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"strings"
	"testing"
)

// TestPKCEChallengeIsTheS256OfTheVerifier checks the property that makes PKCE
// work, rather than just checking the code ran. If the challenge were ever
// derived from something other than the verifier, or the encoding gained
// padding, the token exchange would fail server-side with an opaque
// "invalid_grant" and no local test would have objected.
func TestPKCEChallengeIsTheS256OfTheVerifier(t *testing.T) {
	p, err := generatePKCE()
	if err != nil {
		t.Fatalf("generatePKCE: %v", err)
	}

	sum := sha256.Sum256([]byte(p.verifier))
	want := base64.RawURLEncoding.EncodeToString(sum[:])
	if p.challenge != want {
		t.Errorf("challenge is not S256(verifier)\n got %q\nwant %q", p.challenge, want)
	}

	// RFC 7636 §4.1: the verifier is 43..128 characters.
	if len(p.verifier) < 43 || len(p.verifier) > 128 {
		t.Errorf("verifier length %d is outside the RFC 7636 range 43..128", len(p.verifier))
	}

	// Padding is invalid in the challenge; a "=" would be rejected upstream.
	for _, s := range []string{p.verifier, p.challenge} {
		if strings.ContainsAny(s, "=+/") {
			t.Errorf("%q contains characters that are not URL-safe-base64-without-padding", s)
		}
	}
}

// TestPKCEVerifiersAreNotReused would catch a generator accidentally seeded
// once, which would make every login share a verifier.
func TestPKCEVerifiersAreNotReused(t *testing.T) {
	seen := map[string]bool{}
	for i := 0; i < 50; i++ {
		p, err := generatePKCE()
		if err != nil {
			t.Fatalf("generatePKCE: %v", err)
		}
		if seen[p.verifier] {
			t.Fatalf("verifier repeated after %d generations", i)
		}
		seen[p.verifier] = true
	}
}

// idTokenWith builds an unsigned JWT-shaped string carrying the given claims
// payload. Only the middle segment is ever read.
func idTokenWith(t *testing.T, payload any) string {
	t.Helper()
	b, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	return "header." + base64.RawURLEncoding.EncodeToString(b) + ".signature"
}

// TestClaimsComeFromTheNamespacedObject pins the shape that actually matters:
// chatgpt_account_id lives under "https://api.openai.com/auth", not at the top
// level. Reading it from the top level returns empty, the ChatGPT-Account-ID
// header is then omitted, and workspace accounts are rejected by the backend
// with an error that says nothing about claims.
func TestClaimsComeFromTheNamespacedObject(t *testing.T) {
	tok := idTokenWith(t, map[string]any{
		"email": "someone@example.com",
		"https://api.openai.com/auth": map[string]string{
			"chatgpt_account_id": "acct-123",
			"chatgpt_plan_type":  "free",
		},
	})

	id, plan := chatgptClaimsFromIDToken(tok)
	if id != "acct-123" {
		t.Errorf("account id: got %q, want %q", id, "acct-123")
	}
	if plan != "free" {
		t.Errorf("plan type: got %q, want %q", plan, "free")
	}

	// The shared email helper must still work on the same token.
	if got := emailFromIDToken(tok); got != "someone@example.com" {
		t.Errorf("email: got %q", got)
	}
}

// TestClaimsAtTopLevelAreIgnored is the negative half: a token with the fields
// in the wrong place must yield empty, not a false positive.
func TestClaimsAtTopLevelAreIgnored(t *testing.T) {
	tok := idTokenWith(t, map[string]string{
		"chatgpt_account_id": "acct-wrong-place",
	})
	if id, _ := chatgptClaimsFromIDToken(tok); id != "" {
		t.Errorf("read a top-level claim that the real issuer never sends: %q", id)
	}
}

// TestMalformedIDTokensDoNotPanic — the id_token is remote input, and a login
// must fail as a login, not as a crash.
func TestMalformedIDTokensDoNotPanic(t *testing.T) {
	for _, tok := range []string{
		"", "a", "a.b", "a.b.c.d",
		"header.!!!not-base64!!!.sig",
		"header." + base64.RawURLEncoding.EncodeToString([]byte("not json")) + ".sig",
	} {
		if id, plan := chatgptClaimsFromIDToken(tok); id != "" || plan != "" {
			t.Errorf("malformed token %q produced claims %q/%q", tok, id, plan)
		}
	}
}

// TestAuthHeadersCarryEverythingTheBackendNeeds guards the header set. Omitting
// the account id for a workspace account, or sending the wrong originator, both
// fail at the server with messages that do not name the cause.
func TestAuthHeadersCarryEverythingTheBackendNeeds(t *testing.T) {
	c := &ChatGPTCreds{AccountID: "acct-123"}
	h := c.AuthHeaders("tok-abc")

	if h["Authorization"] != "Bearer tok-abc" {
		t.Errorf("Authorization: got %q", h["Authorization"])
	}
	if h["ChatGPT-Account-ID"] != "acct-123" {
		t.Errorf("ChatGPT-Account-ID: got %q", h["ChatGPT-Account-ID"])
	}
	if h["originator"] != "gorilla_opencode" {
		t.Errorf("originator: got %q, want gorilla_opencode.\n"+
			"This client identifies itself honestly; changing it to impersonate "+
			"another client is an explicit owner decision, not a code tweak.",
			h["originator"])
	}

	// With no account id the header must be absent, not present-and-empty: an
	// empty header value is not the same request as no header.
	if _, present := (&ChatGPTCreds{}).AuthHeaders("t")["ChatGPT-Account-ID"]; present {
		t.Error("empty account id produced a ChatGPT-Account-ID header")
	}
}
