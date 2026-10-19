// GORILLA OVERRIDE: this file did not exist upstream. It is the OAuth + project
// layer for "Login with Google (Antigravity)", the sibling of gemini_oauth.go.
//
// Google Antigravity grants every personal Google account a free tier that
// includes Gemini, Claude (Sonnet/Opus), and GPT-OSS, served through
// daily-cloudcode-pa.googleapis.com. The Antigravity CLI is just the client that
// surfaces it; the entitlement is the user's own. This reaches the same tier by
// authenticating as the Antigravity CLI's installed-app OAuth client — exactly
// the pattern gemini_oauth.go already uses for the Gemini CLI's client id.
//
// Every constant below was MEASURED from a live agy 1.1.10 session on 2026-08-03
// (mitmproxy capture of a real token refresh + generation), not guessed:
//   - the client id/secret from the token-refresh POST body,
//   - the scopes from the refresh response's "scope" field,
//   - the generation User-Agent and the daily-cloudcode-pa endpoint from the
//     streamGenerateContent request headers.
package auth

import (
	"context"
	"crypto/rand"
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
	// Antigravity CLI installed-app OAuth credentials (public, embedded in agy).
	antigravityClientID     = "1071006060591-tmhssin2h21lcre235vtolojh4g403ep.apps.googleusercontent.com"
	antigravityClientSecret = "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" // public: shipped inside agy itself, see docs/CLIENT-SECRETS-EXPLAINED.md; privacy-scan: allow

	// AntigravityEndpoint is the backend serving the free Antigravity tier.
	AntigravityEndpoint = "https://daily-cloudcode-pa.googleapis.com"
	AntigravityVersion  = "v1internal"

	// AntigravityUserAgent identifies every Antigravity request (onboarding AND
	// generation use the same one — unlike the Gemini path).
	//
	// 2026-10-19, measured with the same login: Google's model list depends on
	// the client version in this header. As agy 1.1.10 (the value measured on
	// 2026-08-03) it listed 23 models; as the Antigravity app 2.24.0 it listed 26,
	// adding Gemini 3.8 Flash High/Medium/Low, and generateContent on
	// gemini-3.8-flash-low answered (200, "OK"). The installed app on that day
	// was 2.24.0. Gemini 4 Argon is NOT served to this sign-in under either
	// version (404); it is reachable with an AI Studio key (Gemini provider).
	AntigravityUserAgent = "antigravity/2.24.0 (aidev_client; os_type=windows; arch=amd64; auth_method=consumer)"

	// AntigravityRequestUA is the top-level "userAgent" field the envelope
	// carries (distinct from the HTTP header above). Measured value: "antigravity".
	AntigravityRequestUA = "antigravity"
)

// antigravityScopes are the scopes agy's token actually carries (measured from
// the refresh response). openid is included; drive.* scopes the IDE uses are not.
var antigravityScopes = []string{
	"https://www.googleapis.com/auth/cloud-platform",
	"https://www.googleapis.com/auth/userinfo.email",
	"https://www.googleapis.com/auth/userinfo.profile",
	"https://www.googleapis.com/auth/experimentsandconfigs",
	"https://www.googleapis.com/auth/cclog",
	"openid",
}

// AntigravityCreds is the on-disk credential + project state. Stored at
// ~/.config/gorilla-opencode/antigravity-oauth.json with 0600 perms — a separate
// file from gemini-oauth.json so the two logins never clobber each other.
type AntigravityCreds struct {
	AccessToken  string    `json:"access_token"`
	RefreshToken string    `json:"refresh_token"`
	TokenType    string    `json:"token_type"`
	Expiry       time.Time `json:"expiry"`
	Email        string    `json:"email,omitempty"`
	// ProjectID is the managed free-tier project (same one Google provisions for
	// the Gemini free tier), discovered via loadCodeAssist and sent on every
	// generation call.
	ProjectID string `json:"project_id,omitempty"`
	// Tier is loadCodeAssist's currentTier id ("free-tier" for every personal
	// account, paid or not). PlanID/PlanName are its paidTier, empty on a free
	// account. PlanChecked is when they were last read: a plan bought or
	// cancelled is picked up within a day, not only at the next sign-in.
	Tier        string    `json:"tier,omitempty"`
	PlanID      string    `json:"plan_id,omitempty"`
	PlanName    string    `json:"plan_name,omitempty"`
	PlanChecked time.Time `json:"plan_checked,omitempty"`
}

// AntigravityDataWarning is printed at every Antigravity sign-in, free or paid.
//
// Measured 2026-10-19: loadCodeAssist's privacyNotice (showNotice: true, on a
// Google AI Pro account too) says Google collects prompts, code, output and
// edits to improve its products and machine-learning models, that human
// reviewers may read them disconnected from the account, kept up to 18 months.
// The opt-out it mentions is NOT reachable from this sign-in:
// setCodeAssistGlobalUserSetting {freeTierDataCollectionOptin: false} answered
// 403 PERMISSION_DENIED on both backends. Owner, same day: accept it, but
// "put a nice warning there". Many users are children; it is written for them.
const AntigravityDataWarning = `HEADS UP: on this Google sign-in nothing is free. You are the product.
  Google keeps what you type here, your code and the AI's answers, and uses
  them to train its own AI (Gemini). People at Google may read them: Google
  says it removes your name first and keeps those copies for up to 18 months.
  This sign-in cannot switch that off (Google refuses the opt-out).
  So never type passwords, your real name, your address, your school, or
  anything you would not show a stranger.`

// planRecheck is how old a plan reading may be before SetupProject runs again.
const planRecheck = 24 * time.Hour

// NeedsSetup reports whether SetupProject should run before the next call: no
// project yet, or no plan reading (a login saved by a build that never read
// one), or a reading older than a day.
func (c *AntigravityCreds) NeedsSetup(now time.Time) bool {
	return c.ProjectID == "" || c.Tier == "" || now.Sub(c.PlanChecked) > planRecheck
}

// PlanLabel is what the user is told they are on: the paid plan's own name
// ("Google AI Pro") when Google reports one, else "free tier". Never a plan
// Google did not report.
func (c *AntigravityCreds) PlanLabel() string {
	if c.PlanName != "" {
		return c.PlanName
	}
	if c.PlanID != "" {
		return c.PlanID
	}
	return "free tier"
}

// applyPlan records the tier and plan from a loadCodeAssist answer.
func (c *AntigravityCreds) applyPlan(load loadCodeAssistResp, now time.Time) {
	c.Tier = "free-tier"
	if load.CurrentTier != nil && load.CurrentTier.ID != "" {
		c.Tier = load.CurrentTier.ID
	}
	c.PlanID, c.PlanName = "", ""
	if load.PaidTier != nil {
		c.PlanID, c.PlanName = load.PaidTier.ID, load.PaidTier.Name
	}
	c.PlanChecked = now
}

// AntigravityCredsPath returns ~/.config/gorilla-opencode/antigravity-oauth.json.
func AntigravityCredsPath() string {
	base := os.Getenv("XDG_CONFIG_HOME")
	if base == "" {
		home, _ := os.UserHomeDir()
		base = filepath.Join(home, ".config")
	}
	return filepath.Join(base, "gorilla-opencode", "antigravity-oauth.json")
}

// LoadAntigravityCreds reads stored credentials, or returns (nil, nil) if none.
func LoadAntigravityCreds() (*AntigravityCreds, error) {
	data, err := os.ReadFile(AntigravityCredsPath())
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var c AntigravityCreds
	if err := json.Unmarshal(data, &c); err != nil {
		return nil, err
	}
	return &c, nil
}

// Save writes the credentials atomically with 0600 perms.
func (c *AntigravityCreds) Save() error {
	path := AntigravityCredsPath()
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

// LogoutAntigravity removes the stored Antigravity credentials.
func LogoutAntigravity() error {
	if err := os.Remove(AntigravityCredsPath()); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

// AntigravityLogin runs the OAuth loopback flow against Antigravity's client id.
// It mirrors Login (gemini_oauth.go) and deliberately reuses that file's shared
// helpers — postToken, writeCallbackPage, openBrowser, emailFromIDToken,
// AuthPromptFrom — rather than the working Gemini flow being refactored under it.
func AntigravityLogin(ctx context.Context) (*AntigravityCreds, error) {
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
	authParams.Set("client_id", antigravityClientID)
	authParams.Set("redirect_uri", redirectURI)
	authParams.Set("response_type", "code")
	authParams.Set("scope", strings.Join(antigravityScopes, " "))
	authParams.Set("state", state)
	authParams.Set("access_type", "offline")
	authParams.Set("prompt", "consent")
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

	report := AuthPromptFrom(ctx)
	report(authURL)
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

	form := url.Values{}
	form.Set("code", code)
	form.Set("client_id", antigravityClientID)
	form.Set("client_secret", antigravityClientSecret)
	form.Set("redirect_uri", redirectURI)
	form.Set("grant_type", "authorization_code")
	tok, err := postToken(ctx, form)
	if err != nil {
		return nil, err
	}
	return &AntigravityCreds{
		AccessToken:  tok.AccessToken,
		RefreshToken: tok.RefreshToken,
		TokenType:    tok.TokenType,
		Expiry:       time.Now().Add(time.Duration(tok.ExpiresIn) * time.Second),
		Email:        emailFromIDToken(tok.IDToken),
	}, nil
}

// Ensure returns a valid access token, refreshing via Antigravity's client id
// if it is expired or near expiry, and persisting the refreshed token.
func (c *AntigravityCreds) Ensure(ctx context.Context) (string, error) {
	if c.AccessToken != "" && time.Until(c.Expiry) > 60*time.Second {
		return c.AccessToken, nil
	}
	if c.RefreshToken == "" {
		return "", fmt.Errorf("Antigravity session expired and no refresh token; sign in again")
	}
	form := url.Values{}
	form.Set("client_id", antigravityClientID)
	form.Set("client_secret", antigravityClientSecret)
	form.Set("refresh_token", c.RefreshToken)
	form.Set("grant_type", "refresh_token")
	tok, err := postToken(ctx, form)
	if err != nil {
		return "", err
	}
	c.AccessToken = tok.AccessToken
	c.TokenType = tok.TokenType
	c.Expiry = time.Now().Add(time.Duration(tok.ExpiresIn) * time.Second)
	if tok.RefreshToken != "" {
		c.RefreshToken = tok.RefreshToken
	}
	_ = c.Save()
	return c.AccessToken, nil
}

// SetupProject discovers the managed project AND the account's plan via
// loadCodeAssist on daily-cloudcode-pa and stores both. Reuses the
// caTier/loadCodeAssistResp types declared in gemini_oauth.go (same package).
// A free account and a paid one get the same project and the same calls; only
// the recorded plan differs.
func (c *AntigravityCreds) SetupProject(ctx context.Context) error {
	token, err := c.Ensure(ctx)
	if err != nil {
		return err
	}
	loadBody := map[string]any{"metadata": caMetadata()}
	var load loadCodeAssistResp
	if err := c.callAntigravity(ctx, token, "loadCodeAssist", loadBody, &load); err != nil {
		return fmt.Errorf("loadCodeAssist: %w", err)
	}
	if load.CloudaicompanionProject != "" {
		c.ProjectID = load.CloudaicompanionProject
	}
	c.applyPlan(load, time.Now())
	if c.ProjectID == "" {
		return fmt.Errorf("no project returned; account may not be provisioned for the free tier")
	}
	return c.Save()
}

// ---- quota (the /usage view) ----------------------------------------------

// QuotaBucket is one rolling limit within a model group (measured shape).
type QuotaBucket struct {
	DisplayName       string  `json:"displayName"`
	Window            string  `json:"window"`
	ResetTime         string  `json:"resetTime"`
	Description       string  `json:"description"`
	RemainingFraction float64 `json:"remainingFraction"`
}

// QuotaGroup is a family of models sharing a weekly limit (Gemini; Claude+GPT).
type QuotaGroup struct {
	DisplayName string        `json:"displayName"`
	Description string        `json:"description"`
	Buckets     []QuotaBucket `json:"buckets"`
}

// QuotaSummary is the retrieveUserQuotaSummary response.
type QuotaSummary struct {
	Groups      []QuotaGroup `json:"groups"`
	Description string       `json:"description"`
}

// RetrieveQuota fetches the weekly-limit summary (agy's /usage screen data).
func (c *AntigravityCreds) RetrieveQuota(ctx context.Context) (*QuotaSummary, error) {
	if c.NeedsSetup(time.Now()) {
		// A failed plan re-check is not a failed quota read when a project is
		// already known: the old plan reading stands until the next try.
		if err := c.SetupProject(ctx); err != nil && c.ProjectID == "" {
			return nil, err
		}
	}
	token, err := c.Ensure(ctx)
	if err != nil {
		return nil, err
	}
	var q QuotaSummary
	if err := c.callAntigravity(ctx, token, "retrieveUserQuotaSummary",
		map[string]any{"project": c.ProjectID}, &q); err != nil {
		return nil, err
	}
	return &q, nil
}

// QuotaSummaryLine formats the quota as a single compact line for the status
// bar (the status area is one line with a TTL, not a scrollable panel). Example:
// "Antigravity weekly quota — Gemini Models: 31% (resets in 6d) · Claude and GPT models: 100%".
func (c *AntigravityCreds) QuotaSummaryLine(ctx context.Context) (string, error) {
	q, err := c.RetrieveQuota(ctx)
	if err != nil {
		return "", err
	}
	return FormatQuotaLine(q, time.Now()), nil
}

// FormatQuotaLine is the pure formatter (no network, no wall clock) so the wire
// shape and the wording can be tested against a captured response. `now` is
// passed in for a deterministic "resets in Nd". Exported so the TUI can format
// the footer line from a QuotaSummary it already fetched for the full /usage
// panel — one request, two views.
func FormatQuotaLine(q *QuotaSummary, now time.Time) string {
	return FormatQuotaLineFor(q, now, "")
}

// FormatQuotaLineFor is FormatQuotaLine naming the plan the quota belongs to
// ("Antigravity (Google AI Pro) weekly quota — ..."); plan "" names none.
func FormatQuotaLineFor(q *QuotaSummary, now time.Time, plan string) string {
	head := "Antigravity weekly quota — "
	if plan != "" {
		head = "Antigravity (" + plan + ") weekly quota — "
	}
	var parts []string
	for _, g := range q.Groups {
		if len(g.Buckets) == 0 {
			continue
		}
		b := g.Buckets[0]
		seg := fmt.Sprintf("%s: %d%%", g.DisplayName, int(b.RemainingFraction*100+0.5))
		if b.ResetTime != "" {
			if t, perr := time.Parse(time.RFC3339, b.ResetTime); perr == nil {
				if d := int(t.Sub(now).Hours() / 24); d >= 0 {
					seg += fmt.Sprintf(" (resets in %dd)", d)
				}
			}
		}
		parts = append(parts, seg)
	}
	if len(parts) == 0 {
		return "Antigravity: no quota groups reported"
	}
	return head + strings.Join(parts, " · ")
}

// callAntigravity POSTs {endpoint}/{version}:{method} with the Antigravity UA.
func (c *AntigravityCreds) callAntigravity(ctx context.Context, token, method string, body, out any) error {
	data, err := json.Marshal(body)
	if err != nil {
		return err
	}
	u := fmt.Sprintf("%s/%s:%s", AntigravityEndpoint, AntigravityVersion, method)
	req, err := http.NewRequestWithContext(ctx, "POST", u, strings.NewReader(string(data)))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", AntigravityUserAgent)
	resp, err := authHTTP().Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return fmt.Errorf("HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(b)))
	}
	return json.NewDecoder(resp.Body).Decode(out)
}
