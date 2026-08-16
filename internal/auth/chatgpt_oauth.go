// GORILLA OVERRIDE: this file did not exist upstream. It is the OAuth layer for
// "Sign in with ChatGPT", the sibling of gemini_oauth.go and antigravity_oauth.go.
//
// # WHY IT MATTERS FOR THIS PROJECT
//
// OpenAI's own help centre states: "Codex is included across ChatGPT plans,
// including Free and Go. Usage limits vary by plan." A free tier with no card is
// exactly the access this project exists to reach — the same reason the
// Antigravity and Code Assist free tiers are wired up above.
//
// # WHERE THE CONSTANTS COME FROM
//
// Read out of the Codex CLI's own Rust source, codex-rust-v0.147.0, on
// 2026-08-16 — not guessed, and not captured from traffic:
//   - client id           codex-rs/login/src/auth/manager.rs:1618
//   - issuer              codex-rs/login/src/server.rs:59
//   - scopes and the two
//     non-standard params codex-rs/login/src/server.rs:584-602
//   - PKCE S256 shape     codex-rs/login/src/pkce.rs
//   - callback path/port  codex-rs/login/src/server.rs:176
//
// The client id is a PUBLIC value shipped inside a downloadable binary. Per this
// estate's rule on secrets: a value that ships inside software a stranger can
// download was never confidential, and nothing here can be done with it alone.
// There is no client secret in this flow at all — that is what PKCE replaces.
//
// # THE ORIGINATOR QUESTION, STATED HONESTLY
//
// Codex sends `originator: codex_cli_rs` on its requests. We send
// `gorilla_opencode`. If OpenAI's backend accepts that, this works and the free
// tier is genuinely reachable by this client. If it refuses, we will have
// learned that by measurement in a day rather than by argument, and the decision
// about what to do next belongs to the project owner — not to this file, and not
// to a model. Do not change ChatGPTOriginator to impersonate another client
// without that decision being made explicitly and written down.
package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	// chatgptClientID is the Codex CLI's public installed-app client id.
	chatgptClientID = "app_EMoamEEZ73f0CkXaXp7hrann"

	// chatgptIssuer is the OAuth issuer; /oauth/authorize and /oauth/token hang
	// off it.
	chatgptIssuer = "https://auth.openai.com"

	// chatgptCallbackPort is FIXED at 1455 and cannot be randomised.
	//
	// The other two logins in this package bind port 0 and let the kernel choose,
	// because Google's installed-app clients accept any loopback port. OpenAI's
	// client registration does not: the redirect_uri must match what was
	// registered for this client id exactly, and that is
	// http://localhost:1455/auth/callback. A random port produces
	// "redirect_uri mismatch" at the authorize step, before the browser even
	// prompts. If 1455 is already in use we fail with that explanation rather
	// than silently trying another port that cannot work.
	chatgptCallbackPort = 1455
	chatgptCallbackPath = "/auth/callback"

	// ChatGPTBackend is where the token is actually spendable. A ChatGPT-plan
	// token is NOT an API key: it does not authenticate against api.openai.com at
	// all, only against this backend, which speaks the Responses API shape.
	ChatGPTBackend = "https://chatgpt.com/backend-api/codex"

	// ChatGPTOriginator identifies this client on every request. See the file
	// header before changing it.
	ChatGPTOriginator = "gorilla_opencode"

	// chatgptClientVersion is sent as the REQUIRED client_version query
	// parameter. It is not optional: omitting it returns
	// 400 invalid_request_error naming ('query', 'client_version').
	//
	// The value is the Codex release these constants were read from. It states
	// which version of the backend's contract this client was written against —
	// the backend uses it to decide whether the client is new enough (it returns
	// a "minimal_client_version" in its own payload, see
	// codex-api/src/endpoint/models.rs:207).
	chatgptClientVersion = "0.147.0"
)

// chatgptScopes are the scopes Codex requests, verbatim. offline_access is what
// yields a refresh token; without it the login would die after an hour.
var chatgptScopes = []string{
	"openid",
	"profile",
	"email",
	"offline_access",
}

// ChatGPTCreds is the on-disk credential state, stored at
// ~/.config/gorilla-opencode/chatgpt-oauth.json with 0600 perms — a separate
// file from the Google logins so none of them can clobber another.
type ChatGPTCreds struct {
	AccessToken  string    `json:"access_token"`
	RefreshToken string    `json:"refresh_token"`
	TokenType    string    `json:"token_type"`
	Expiry       time.Time `json:"expiry"`
	Email        string    `json:"email,omitempty"`
	// AccountID is the chatgpt_account_id claim from the id_token. It travels as
	// the ChatGPT-Account-ID header on every backend call; requests are rejected
	// without it on accounts that belong to a workspace.
	AccountID string `json:"chatgpt_account_id,omitempty"`
	// PlanType is recorded only so the UI can tell someone which plan is paying
	// for this. It is never used to gate anything locally.
	PlanType string `json:"plan_type,omitempty"`
}

// ChatGPTCredsPath returns ~/.config/gorilla-opencode/chatgpt-oauth.json.
func ChatGPTCredsPath() string {
	base := os.Getenv("XDG_CONFIG_HOME")
	if base == "" {
		home, _ := os.UserHomeDir()
		base = filepath.Join(home, ".config")
	}
	return filepath.Join(base, "gorilla-opencode", "chatgpt-oauth.json")
}

// LoadChatGPTCreds reads stored credentials, or returns (nil, nil) if none.
func LoadChatGPTCreds() (*ChatGPTCreds, error) {
	data, err := os.ReadFile(ChatGPTCredsPath())
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var c ChatGPTCreds
	if err := json.Unmarshal(data, &c); err != nil {
		return nil, err
	}
	return &c, nil
}

// Save writes the credentials atomically with 0600 perms.
func (c *ChatGPTCreds) Save() error {
	path := ChatGPTCredsPath()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// LogoutChatGPT removes the stored ChatGPT credentials.
func LogoutChatGPT() error {
	if err := os.Remove(ChatGPTCredsPath()); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

// pkceCodes is a PKCE verifier/challenge pair (RFC 7636, S256).
type pkceCodes struct {
	verifier  string
	challenge string
}

// generatePKCE produces the pair. PKCE is what makes a public client safe
// without a secret: the challenge goes out with the authorize request, the
// verifier is held locally, and only the holder of the verifier can redeem the
// code — so intercepting the redirect is not enough to steal the token.
func generatePKCE() (pkceCodes, error) {
	buf := make([]byte, 64)
	if _, err := rand.Read(buf); err != nil {
		return pkceCodes{}, err
	}
	verifier := base64.RawURLEncoding.EncodeToString(buf)
	sum := sha256.Sum256([]byte(verifier))
	return pkceCodes{
		verifier:  verifier,
		challenge: base64.RawURLEncoding.EncodeToString(sum[:]),
	}, nil
}

// ChatGPTLogin runs the OAuth loopback flow with PKCE against OpenAI's issuer.
//
// It deliberately mirrors AntigravityLogin rather than refactoring it: the two
// flows differ in ways that would make a shared abstraction lie (PKCE vs client
// secret, fixed vs kernel-assigned port, different callback path), and the
// working Google flows are not worth destabilising for the sake of one.
func ChatGPTLogin(ctx context.Context) (*ChatGPTCreds, error) {
	addr := fmt.Sprintf("127.0.0.1:%d", chatgptCallbackPort)
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, fmt.Errorf(
			"could not open the sign-in listener on port %d: %w\n"+
				"That exact port is required — OpenAI registered this client for "+
				"http://localhost:%d%s and will reject any other address.\n"+
				"Something else is using it; the usual culprit is a Codex CLI "+
				"sign-in still running. Close it and try again.",
			chatgptCallbackPort, err, chatgptCallbackPort, chatgptCallbackPath)
	}
	defer ln.Close()

	// Registered as "localhost", not "127.0.0.1". They resolve to the same
	// socket but are compared as strings by the authorization server.
	redirectURI := fmt.Sprintf("http://localhost:%d%s", chatgptCallbackPort, chatgptCallbackPath)

	pkce, err := generatePKCE()
	if err != nil {
		return nil, err
	}

	stateBytes := make([]byte, 32)
	if _, err := rand.Read(stateBytes); err != nil {
		return nil, err
	}
	state := hex.EncodeToString(stateBytes)

	authParams := url.Values{}
	authParams.Set("response_type", "code")
	authParams.Set("client_id", chatgptClientID)
	authParams.Set("redirect_uri", redirectURI)
	authParams.Set("scope", strings.Join(chatgptScopes, " "))
	authParams.Set("code_challenge", pkce.challenge)
	authParams.Set("code_challenge_method", "S256")
	// Both non-standard params are sent by Codex. Without
	// id_token_add_organizations the id_token carries no chatgpt_account_id, and
	// without that header workspace accounts are rejected by the backend.
	authParams.Set("id_token_add_organizations", "true")
	authParams.Set("codex_cli_simplified_flow", "true")
	authParams.Set("state", state)
	authParams.Set("originator", ChatGPTOriginator)
	authURL := chatgptIssuer + "/oauth/authorize?" + authParams.Encode()

	type result struct {
		code string
		err  error
	}
	resCh := make(chan result, 1)

	mux := http.NewServeMux()
	mux.HandleFunc(chatgptCallbackPath, func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		if e := q.Get("error"); e != "" {
			if d := q.Get("error_description"); d != "" {
				e = e + ": " + d
			}
			writeCallbackPage(w, false, e)
			resCh <- result{err: fmt.Errorf("authorization denied: %s", e)}
			return
		}
		if q.Get("state") != state {
			writeCallbackPage(w, false, "state mismatch")
			resCh <- result{err: fmt.Errorf("state mismatch (possible CSRF); aborted")}
			return
		}
		code := q.Get("code")
		if code == "" {
			writeCallbackPage(w, false, "no authorization code")
			resCh <- result{err: fmt.Errorf("no authorization code in callback")}
			return
		}
		writeCallbackPage(w, true, "")
		resCh <- result{code: code}
	})
	srv := &http.Server{Handler: mux}
	go srv.Serve(ln)
	defer srv.Close()

	report := AuthPromptFrom(ctx)
	report(authURL)
	_ = openBrowser(authURL)

	var code string
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-time.After(5 * time.Minute):
		return nil, fmt.Errorf("timed out waiting for ChatGPT sign-in")
	case r := <-resCh:
		if r.err != nil {
			return nil, r.err
		}
		code = r.code
	}

	form := url.Values{}
	form.Set("grant_type", "authorization_code")
	form.Set("code", code)
	form.Set("redirect_uri", redirectURI)
	form.Set("client_id", chatgptClientID)
	form.Set("code_verifier", pkce.verifier)

	tok, err := postChatGPTToken(ctx, form)
	if err != nil {
		return nil, err
	}
	creds := &ChatGPTCreds{
		AccessToken:  tok.AccessToken,
		RefreshToken: tok.RefreshToken,
		TokenType:    tok.TokenType,
		Expiry:       time.Now().Add(time.Duration(tok.ExpiresIn) * time.Second),
		Email:        emailFromIDToken(tok.IDToken),
	}
	creds.AccountID, creds.PlanType = chatgptClaimsFromIDToken(tok.IDToken)
	return creds, nil
}

// Ensure returns a valid access token, refreshing it when expired or near
// expiry, and persisting the result.
func (c *ChatGPTCreds) Ensure(ctx context.Context) (string, error) {
	if c.AccessToken != "" && time.Until(c.Expiry) > 60*time.Second {
		return c.AccessToken, nil
	}
	if c.RefreshToken == "" {
		return "", fmt.Errorf("ChatGPT sign-in has expired and there is no refresh token; run: gorilla-opencode login --chatgpt")
	}

	form := url.Values{}
	form.Set("grant_type", "refresh_token")
	form.Set("refresh_token", c.RefreshToken)
	form.Set("client_id", chatgptClientID)
	// No client_secret: this is a public client and PKCE covered the initial
	// exchange. Sending one would be rejected.

	tok, err := postChatGPTToken(ctx, form)
	if err != nil {
		return "", err
	}
	c.AccessToken = tok.AccessToken
	c.TokenType = tok.TokenType
	c.Expiry = time.Now().Add(time.Duration(tok.ExpiresIn) * time.Second)
	// A refresh response may omit refresh_token, which means "keep using the one
	// you have". Overwriting it with "" would silently end the session at the
	// next expiry.
	if tok.RefreshToken != "" {
		c.RefreshToken = tok.RefreshToken
	}
	if tok.IDToken != "" {
		if id, plan := chatgptClaimsFromIDToken(tok.IDToken); id != "" {
			c.AccountID, c.PlanType = id, plan
		}
	}
	if err := c.Save(); err != nil {
		return "", err
	}
	return c.AccessToken, nil
}

// AuthHeaders returns the headers every ChatGPT-backend request must carry.
// Kept in one place so the token, the account id and the originator can never
// drift apart across call sites.
func (c *ChatGPTCreds) AuthHeaders(token string) map[string]string {
	h := map[string]string{
		"Authorization": "Bearer " + token,
		"originator":    ChatGPTOriginator,
	}
	if c.AccountID != "" {
		h["ChatGPT-Account-ID"] = c.AccountID
	}
	return h
}

// ProbeBackend asks the Codex backend whether it will talk to this client.
//
// It is a read-only GET of the model list — the cheapest request that still
// proves the whole chain (token valid, account id accepted, originator not
// rejected) without generating a single token of billable output.
//
// This exists because one question could not be answered by reading anything:
// OpenAI documents signing in to Codex with a ChatGPT plan, and documents
// nothing at all about clients other than their own. Rather than argue about it,
// this measures it. The returned status and body are reported verbatim to the
// user so the answer is theirs to read, not a model's to summarise.
func (c *ChatGPTCreds) ProbeBackend(ctx context.Context) (status int, body string, err error) {
	token, err := c.Ensure(ctx)
	if err != nil {
		return 0, "", err
	}
	// client_version is a REQUIRED query parameter, not a header. Omitting it
	// gets a 400 "Field required" naming ('query', 'client_version') — which is
	// itself informative: that check runs after authentication, so reaching it
	// proves the token and the client identifier were both accepted.
	// Codex appends it the same way (codex-api/src/endpoint/models.rs:35-42).
	u := fmt.Sprintf("%s/models?client_version=%s", ChatGPTBackend, url.QueryEscape(chatgptClientVersion))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return 0, "", err
	}
	for k, v := range c.AuthHeaders(token) {
		req.Header.Set(k, v)
	}
	req.Header.Set("Accept", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return 0, "", fmt.Errorf("could not reach %s: %w", ChatGPTBackend, err)
	}
	defer resp.Body.Close()

	// 1 MB, not 64 KB: the model list carries a large per-model feature block and
	// overran the smaller cap, which truncated the JSON mid-object. A truncated
	// body still parses as "not the expected shape", so the failure looked like a
	// protocol mismatch rather than a read limit — the cap must exceed the real
	// payload or it silently changes what the caller concludes.
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return resp.StatusCode, "", err
	}
	return resp.StatusCode, strings.TrimSpace(string(raw)), nil
}

// postChatGPTToken posts to OpenAI's token endpoint.
//
// Deliberately separate from postToken in gemini_oauth.go, which is hardcoded to
// Google's endpoint. On failure it includes the response body: OAuth errors are
// nearly always explained there ("invalid_grant", "redirect_uri mismatch"), and
// swallowing it turns a five-second fix into an afternoon.
func postChatGPTToken(ctx context.Context, form url.Values) (*tokenResponse, error) {
	endpoint := chatgptIssuer + "/oauth/token"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint,
		strings.NewReader(form.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("originator", ChatGPTOriginator)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("token request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("token endpoint returned %d: %s",
			resp.StatusCode, strings.TrimSpace(string(body)))
	}
	var tok tokenResponse
	if err := json.Unmarshal(body, &tok); err != nil {
		return nil, fmt.Errorf("could not parse token response: %w", err)
	}
	if tok.AccessToken == "" {
		return nil, fmt.Errorf("token endpoint returned no access token")
	}
	return &tok, nil
}

// chatgptClaimsFromIDToken pulls chatgpt_account_id and the plan type out of the
// id_token. The claims are nested under an "https://api.openai.com/auth"
// namespace, which is why the standard email helper cannot reach them.
//
// The id_token is NOT verified here, and that is deliberate: it arrived over TLS
// directly from the token endpoint in response to our own PKCE exchange, so it
// is not attacker-supplied. These two values are used for a request header and a
// UI label, never for an authorisation decision on this machine.
func chatgptClaimsFromIDToken(idToken string) (accountID, planType string) {
	parts := strings.Split(idToken, ".")
	if len(parts) != 3 {
		return "", ""
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return "", ""
	}
	var claims struct {
		Auth struct {
			ChatGPTAccountID string `json:"chatgpt_account_id"`
			ChatGPTPlanType  string `json:"chatgpt_plan_type"`
		} `json:"https://api.openai.com/auth"`
	}
	if json.Unmarshal(payload, &claims) != nil {
		return "", ""
	}
	return claims.Auth.ChatGPTAccountID, claims.Auth.ChatGPTPlanType
}
