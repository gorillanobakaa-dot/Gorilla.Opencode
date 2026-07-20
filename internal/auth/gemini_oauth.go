// GORILLA OVERRIDE: this package did not exist upstream. It implements
// "Login with Google" for Gemini — the same OAuth + Code Assist flow that
// the Gemini CLI and Antigravity use — so a user with a normal Gmail
// account can reach Google's free tier (cloudcode-pa.googleapis.com)
// instead of the API-key path, which returns HTTP 429 "limit: 0" on many
// free keys.
//
// Why this exists, plainly: the value here is quota control for people who
// can't pay. A lean client (this fork makes few, small calls) plus the
// free login tier is what used to let a learner code for hours a day at
// zero cost. This restores that.
//
// Transparency, per this project's philosophy: this reuses the Gemini CLI's
// PUBLIC OAuth client id/secret (an installed-app "secret" is not truly
// secret; it ships in an Apache-2.0 open-source binary) and Google's
// private Code Assist backend. That is a Google-ToS gray area, stated here
// rather than hidden. The credentials below are copied verbatim from the
// open-source Gemini CLI so the flow matches what Google already serves.
package auth

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

const (
	// Public installed-app credentials from the open-source Gemini CLI.
	geminiOAuthClientID     = "681255809395-oo8ft2oprdrnp9e3aqf6av3hmdib135j.apps.googleusercontent.com"
	geminiOAuthClientSecret = "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"

	googleAuthURL  = "https://accounts.google.com/o/oauth2/v2/auth"
	googleTokenURL = "https://oauth2.googleapis.com/token"

	// Google Code Assist backend (the "free tier via login" endpoint).
	CodeAssistEndpoint = "https://cloudcode-pa.googleapis.com"
	CodeAssistVersion  = "v1internal"
)

// geminiOAuthScopes are copied verbatim from the Gemini CLI.
var geminiOAuthScopes = []string{
	"https://www.googleapis.com/auth/cloud-platform",
	"https://www.googleapis.com/auth/userinfo.email",
	"https://www.googleapis.com/auth/userinfo.profile",
}

// GeminiCreds is the on-disk credential + onboarding state. Stored at
// ~/.config/gorilla-opencode/gemini-oauth.json with 0600 perms.
type GeminiCreds struct {
	AccessToken  string    `json:"access_token"`
	RefreshToken string    `json:"refresh_token"`
	TokenType    string    `json:"token_type"`
	Expiry       time.Time `json:"expiry"`
	Email        string    `json:"email,omitempty"`
	// ProjectID is the Code Assist project (auto-provisioned for the free
	// tier, or a user-supplied GCP project) used in generateContent calls.
	ProjectID string `json:"project_id,omitempty"`
	Tier      string `json:"tier,omitempty"`
}

// CredsPath returns ~/.config/gorilla-opencode/gemini-oauth.json.
func CredsPath() string {
	base := os.Getenv("XDG_CONFIG_HOME")
	if base == "" {
		home, _ := os.UserHomeDir()
		base = filepath.Join(home, ".config")
	}
	return filepath.Join(base, "gorilla-opencode", "gemini-oauth.json")
}

// LoadGeminiCreds reads stored credentials, or returns (nil, nil) if none.
func LoadGeminiCreds() (*GeminiCreds, error) {
	data, err := os.ReadFile(CredsPath())
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var c GeminiCreds
	if err := json.Unmarshal(data, &c); err != nil {
		return nil, err
	}
	return &c, nil
}

// Save writes the credentials atomically with 0600 perms.
func (c *GeminiCreds) Save() error {
	path := CredsPath()
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

// tokenResponse is the shape of Google's token endpoint reply.
type tokenResponse struct {
	AccessToken  string `json:"access_token"`
	ExpiresIn    int    `json:"expires_in"`
	RefreshToken string `json:"refresh_token"`
	TokenType    string `json:"token_type"`
	IDToken      string `json:"id_token"`
	Scope        string `json:"scope"`
}

// Login runs the OAuth loopback flow: it starts a localhost server, opens
// the browser to Google's consent screen, and exchanges the returned code
// for tokens. The port may be pinned with OAUTH_CALLBACK_PORT.
func Login(ctx context.Context) (*GeminiCreds, error) {
	port := os.Getenv("OAUTH_CALLBACK_PORT")
	ln, err := net.Listen("tcp", "127.0.0.1:"+port)
	if err != nil {
		return nil, fmt.Errorf("could not open loopback listener: %w", err)
	}
	defer ln.Close()
	actualPort := ln.Addr().(*net.TCPAddr).Port
	redirectURI := fmt.Sprintf("http://127.0.0.1:%d/oauth2callback", actualPort)

	stateBytes := make([]byte, 32)
	if _, err := rand.Read(stateBytes); err != nil {
		return nil, err
	}
	state := hex.EncodeToString(stateBytes)

	authParams := url.Values{}
	authParams.Set("client_id", geminiOAuthClientID)
	authParams.Set("redirect_uri", redirectURI)
	authParams.Set("response_type", "code")
	authParams.Set("scope", strings.Join(geminiOAuthScopes, " "))
	authParams.Set("state", state)
	authParams.Set("access_type", "offline") // ask for a refresh token
	authParams.Set("prompt", "consent")      // force refresh token every time
	authURL := googleAuthURL + "?" + authParams.Encode()

	type result struct {
		code string
		err  error
	}
	resCh := make(chan result, 1)

	mux := http.NewServeMux()
	mux.HandleFunc("/oauth2callback", func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		if e := q.Get("error"); e != "" {
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

	fmt.Println("Opening your browser to sign in with Google.")
	fmt.Println("If it does not open, paste this URL into your browser:")
	fmt.Println()
	fmt.Println("  " + authURL)
	fmt.Println()
	_ = openBrowser(authURL)

	var code string
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-time.After(5 * time.Minute):
		return nil, fmt.Errorf("timed out waiting for Google sign-in")
	case r := <-resCh:
		if r.err != nil {
			return nil, r.err
		}
		code = r.code
	}

	tok, err := exchangeCode(ctx, code, redirectURI)
	if err != nil {
		return nil, err
	}
	creds := &GeminiCreds{
		AccessToken:  tok.AccessToken,
		RefreshToken: tok.RefreshToken,
		TokenType:    tok.TokenType,
		Expiry:       time.Now().Add(time.Duration(tok.ExpiresIn) * time.Second),
		Email:        emailFromIDToken(tok.IDToken),
	}
	return creds, nil
}

// exchangeCode swaps an authorization code for tokens.
func exchangeCode(ctx context.Context, code, redirectURI string) (*tokenResponse, error) {
	form := url.Values{}
	form.Set("code", code)
	form.Set("client_id", geminiOAuthClientID)
	form.Set("client_secret", geminiOAuthClientSecret)
	form.Set("redirect_uri", redirectURI)
	form.Set("grant_type", "authorization_code")
	return postToken(ctx, form)
}

// Ensure returns a valid access token, refreshing it if it is expired or
// about to expire. It persists a refreshed token back to disk.
func (c *GeminiCreds) Ensure(ctx context.Context) (string, error) {
	if c.AccessToken != "" && time.Until(c.Expiry) > 60*time.Second {
		return c.AccessToken, nil
	}
	if c.RefreshToken == "" {
		return "", fmt.Errorf("session expired and no refresh token; run 'gorilla-opencode login' again")
	}
	form := url.Values{}
	form.Set("client_id", geminiOAuthClientID)
	form.Set("client_secret", geminiOAuthClientSecret)
	form.Set("refresh_token", c.RefreshToken)
	form.Set("grant_type", "refresh_token")
	tok, err := postToken(ctx, form)
	if err != nil {
		return "", err
	}
	c.AccessToken = tok.AccessToken
	c.TokenType = tok.TokenType
	c.Expiry = time.Now().Add(time.Duration(tok.ExpiresIn) * time.Second)
	if tok.RefreshToken != "" { // Google may rotate it
		c.RefreshToken = tok.RefreshToken
	}
	_ = c.Save()
	return c.AccessToken, nil
}

func postToken(ctx context.Context, form url.Values) (*tokenResponse, error) {
	req, err := http.NewRequestWithContext(ctx, "POST", googleTokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var tok tokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&tok); err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK || tok.AccessToken == "" {
		return nil, fmt.Errorf("token endpoint returned HTTP %d", resp.StatusCode)
	}
	return &tok, nil
}

// ---- Code Assist onboarding ------------------------------------------------

type caTier struct {
	ID        string `json:"id"`
	IsDefault bool   `json:"isDefault"`
}

type loadCodeAssistResp struct {
	CurrentTier             *caTier  `json:"currentTier"`
	AllowedTiers            []caTier `json:"allowedTiers"`
	CloudaicompanionProject string   `json:"cloudaicompanionProject"`
}

type onboardUserResp struct {
	Name     string `json:"name"`
	Done     bool   `json:"done"`
	Response struct {
		CloudaicompanionProject struct {
			ID string `json:"id"`
		} `json:"cloudaicompanionProject"`
	} `json:"response"`
}

func caMetadata() map[string]string {
	return map[string]string{
		"ideType":    "IDE_UNSPECIFIED",
		"platform":   "PLATFORM_UNSPECIFIED",
		"pluginType": "GEMINI",
	}
}

// SetupCodeAssist runs loadCodeAssist then onboardUser (polling the LRO)
// to determine the tier and the project id used for generateContent.
// projectID may be "" for the auto-provisioned free tier, or a user's GCP
// project id for the "Use a Google Cloud project" method.
func (c *GeminiCreds) SetupCodeAssist(ctx context.Context, projectID string) error {
	token, err := c.Ensure(ctx)
	if err != nil {
		return err
	}

	// 1. loadCodeAssist: discover tier + any existing project.
	loadBody := map[string]any{"metadata": caMetadata()}
	if projectID != "" {
		loadBody["cloudaicompanionProject"] = projectID
	}
	var load loadCodeAssistResp
	if err := c.callCodeAssist(ctx, token, "loadCodeAssist", loadBody, &load); err != nil {
		return fmt.Errorf("loadCodeAssist: %w", err)
	}
	if projectID == "" && load.CloudaicompanionProject != "" {
		projectID = load.CloudaicompanionProject
	}
	tierID := "free-tier"
	if load.CurrentTier != nil && load.CurrentTier.ID != "" {
		tierID = load.CurrentTier.ID
	} else {
		for _, t := range load.AllowedTiers {
			if t.IsDefault {
				tierID = t.ID
				break
			}
		}
	}

	// 2. onboardUser: long-running op; poll until done.
	onboardBody := map[string]any{"tierId": tierID, "metadata": caMetadata()}
	if projectID != "" {
		onboardBody["cloudaicompanionProject"] = projectID
	}
	deadline := time.Now().Add(2 * time.Minute)
	for {
		var ob onboardUserResp
		if err := c.callCodeAssist(ctx, token, "onboardUser", onboardBody, &ob); err != nil {
			return fmt.Errorf("onboardUser: %w", err)
		}
		if ob.Done {
			if ob.Response.CloudaicompanionProject.ID != "" {
				projectID = ob.Response.CloudaicompanionProject.ID
			}
			break
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("onboarding did not complete in time")
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(3 * time.Second):
		}
	}

	c.ProjectID = projectID
	c.Tier = tierID
	return c.Save()
}

// callCodeAssist POSTs {endpoint}/{version}:{method} with a bearer token.
func (c *GeminiCreds) callCodeAssist(ctx context.Context, token, method string, body any, out any) error {
	data, err := json.Marshal(body)
	if err != nil {
		return err
	}
	u := fmt.Sprintf("%s/%s:%s", CodeAssistEndpoint, CodeAssistVersion, method)
	req, err := http.NewRequestWithContext(ctx, "POST", u, strings.NewReader(string(data)))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		snippet := make([]byte, 512)
		n, _ := resp.Body.Read(snippet)
		return fmt.Errorf("HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(snippet[:n])))
	}
	if out == nil {
		return nil
	}
	return json.NewDecoder(resp.Body).Decode(out)
}

// ---- helpers ---------------------------------------------------------------

// emailFromIDToken best-effort decodes the "email" claim from a JWT id_token.
func emailFromIDToken(idToken string) string {
	parts := strings.Split(idToken, ".")
	if len(parts) != 3 {
		return ""
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return ""
	}
	var claims struct {
		Email string `json:"email"`
	}
	if json.Unmarshal(payload, &claims) != nil {
		return ""
	}
	return claims.Email
}

// openBrowser tries the common Linux/macOS openers; failure is non-fatal
// because the URL is also printed for manual paste.
func openBrowser(u string) error {
	for _, opener := range []string{"xdg-open", "open", "sensible-browser"} {
		if path, err := exec.LookPath(opener); err == nil {
			return exec.Command(path, u).Start()
		}
	}
	return fmt.Errorf("no browser opener found")
}

func writeCallbackPage(w http.ResponseWriter, ok bool, msg string) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if ok {
		fmt.Fprint(w, `<!doctype html><meta charset=utf-8><title>Signed in</title>
<body style="font-family:system-ui;background:#0b0b0b;color:#e6e6e6;text-align:center;padding-top:12vh">
<h2 style="color:#22d3ee">You are signed in.</h2>
<p>You can close this tab and return to the terminal.</p></body>`)
		return
	}
	fmt.Fprintf(w, `<!doctype html><meta charset=utf-8><title>Sign-in failed</title>
<body style="font-family:system-ui;background:#0b0b0b;color:#e6e6e6;text-align:center;padding-top:12vh">
<h2 style="color:#f472b6">Sign-in failed</h2><p>%s</p>
<p>Return to the terminal and try again.</p></body>`, msg)
}
