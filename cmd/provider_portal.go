// GORILLA OVERRIDE: the config-aware half of the every-launch provider portal.
// The startup package renders; this file decides what the rows say, which ones
// count as configured, and how a selection becomes saved credentials plus agent
// models. Split this way because startup cannot import config.
package cmd

import (
	"context"
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/opencode-ai/opencode/internal/auth"
	"github.com/opencode-ai/opencode/internal/config"
	"github.com/opencode-ai/opencode/internal/llm/models"
	"github.com/opencode-ai/opencode/internal/tui/startup"
)

// Default endpoint names, used only when the user has no entry for that baseURL
// yet. An endpoint is identified by WHERE IT POINTS, not by what it is called:
// both the "is this configured?" check and the upsert resolve by baseURL and
// adopt the user's own name. Keying off the name instead is what let one config
// accumulate four entries for the same NVIDIA URL, and later made a keyed
// "Gorilla.FREE.NVIDIA.NIM" invisible to the portal.
const (
	nimEndpointName    = "NVIDIA NIM"
	nimBaseURL         = "https://integrate.api.nvidia.com/v1"
	ollamaEndpointName = "Ollama"
	ollamaBaseURL      = "http://localhost:11434/v1"

	// Cloudflare Workers AI. The base URL embeds the account id, so it is built
	// per-user from what they paste rather than being a constant.
	cloudflareEndpointName = "Cloudflare Workers AI"
	cloudflareBaseFmt      = "https://api.cloudflare.com/client/v4/accounts/%s/ai/v1"

	// oauthLoginPlaceholder is the sentinel API key that marks an OAuth-based
	// provider (Antigravity, Gemini Code Assist) as configured. The real auth is
	// the stored token; this only clears config's "apiKey == '' means disabled"
	// gate. Matches the value tui.go uses for the Gemini-OAuth provider.
	oauthLoginPlaceholder = "oauth-login"
)

// portalProvider maps row IDs to the typed provider constants that
// UpsertProviderKey requires. (The superseded spec passed raw strings here,
// which does not compile.)
var portalProvider = map[string]models.ModelProvider{
	"anthropic":  models.ProviderAnthropic,
	"openai":     models.ProviderOpenAI,
	"gemini-api": models.ProviderGemini,
	"groq":       models.ProviderGROQ,
	"cerebras":   models.ProviderCerebras,
	"openrouter": models.ProviderOpenRouter,
	"xai":        models.ProviderXAI,
}

// portalDefaults picks the model each provider starts on. Mirrors the choices
// config.setDefaultModelForAgent already makes for the same providers.
var portalDefaults = map[string]struct{ coder, title models.ModelID }{
	"anthropic":  {models.Claude37Sonnet, models.Claude37Sonnet},
	"openai":     {models.GPT41, models.GPT41Mini},
	"gemini-api": {models.GeminiFlashLatest, models.GeminiFlashLiteLatest},
	"groq":       {models.Llama3_3_70BVersatile, models.Llama3_3_70BVersatile},
	"cerebras":   {models.CerebrasGLM47, models.CerebrasGLM47},
	"openrouter": {models.OpenRouterClaude37Sonnet, models.OpenRouterClaude35Haiku},
	"xai":        {models.XAIGrok3Beta, models.XAIGrok3MiniBeta},
}

// registerLocalEndpoint is a seam: RegisterLocalEndpoint fetches /v1/models
// over the network, and tests must not.
var registerLocalEndpoint = models.RegisterLocalEndpoint

// providerPortalRows builds the menu from the loaded config. Returns the rows
// and whether anything currently works (canKeep — what Esc means).
func providerPortalRows() ([]startup.ProviderRow, bool) {
	cfg := config.Get()

	envSet := map[models.ModelProvider]bool{}
	for _, p := range config.AvailableViaEnv() {
		envSet[p] = true
	}
	keyed := func(p models.ModelProvider) bool {
		if envSet[p] {
			return true
		}
		pr, ok := cfg.Providers[p]
		return ok && pr.APIKey != "" && !pr.Disabled
	}
	// GORILLA FIX: find a local endpoint by its baseURL, NOT by the name we
	// happen to give it.
	//
	// This used to match on e.Name == "NVIDIA NIM". An endpoint is identified by
	// where it points, and users name theirs whatever they like — one here is
	// called "Gorilla.FREE.NVIDIA.NIM". The portal therefore could not see a
	// perfectly good, keyed NIM endpoint and asked for the key again on every
	// launch, with the row showing as not configured.
	//
	// The same name assumption is what made the portal CREATE a second "NVIDIA
	// NIM" entry beside the user's own, leaving two endpoints on one baseURL —
	// the documented "last one wins" trap that takes the survivor's models down
	// with it. Matching on URL fixes both halves.
	//
	// A keyed entry wins over an unkeyed one when several point at the same URL,
	// so a stray blank duplicate cannot mask a working key.
	endpointFor := func(baseURL string) (config.LocalEndpoint, bool) {
		var found config.LocalEndpoint
		var ok bool
		for _, e := range cfg.LocalEndpoints {
			if e.BaseURL != baseURL || e.Disabled {
				continue
			}
			if !ok || (found.APIKey == "" && e.APIKey != "") {
				found, ok = e, true
			}
		}
		return found, ok
	}

	creds, _ := auth.LoadGeminiCreds()
	oauthReady := creds != nil && creds.AccessToken != ""
	agCreds, _ := auth.LoadAntigravityCreds()
	agReady := agCreds != nil && agCreds.AccessToken != ""
	// Cloudflare's baseURL contains the account id, so it cannot be matched by a
	// fixed constant the way NIM and Ollama are.
	var cfEp config.LocalEndpoint
	cfReady := false
	for _, e := range cfg.LocalEndpoints {
		if !e.Disabled && strings.Contains(e.BaseURL, "api.cloudflare.com") && e.APIKey != "" {
			cfEp, cfReady = e, true
			break
		}
	}
	_ = cfEp

	nimEp, nimReady := endpointFor(nimBaseURL)
	ollamaEp, _ := endpointFor(ollamaBaseURL)
	nimReady = nimReady && nimEp.APIKey != ""

	// The user's own name for each endpoint, so "Active" compares like with like.
	// Falls back to ours when they have no entry for that URL yet.
	nimName := nimEndpointName
	if nimEp.Name != "" {
		nimName = nimEp.Name
	}
	ollamaName := ollamaEndpointName
	if ollamaEp.Name != "" {
		ollamaName = ollamaEp.Name
	}

	// Which row is the session currently on?
	curModel := cfg.Agents[config.AgentCoder].Model
	curProv := models.SupportedModels[curModel].Provider
	curEndpoint := models.LocalEndpointFor(curModel) // "" unless a local model

	rows := []startup.ProviderRow{
		{
			ID:   "antigravity",
			Name: "Antigravity free tier - Claude + GPT-OSS + Gemini (Gmail sign-in)",
			What: "Signs in with your Google account and uses your free Google " +
				"Antigravity tier: Claude Sonnet/Opus 4.6, GPT-OSS 120B, and Gemini. " +
				"No API key, no cost - it is your account's own entitlement.",
			Warning: "Weekly quotas apply per model group (Gemini separate from " +
				"Claude/GPT). Unofficial: it speaks the Antigravity CLI's protocol, so " +
				"a Google-side change could break it without notice.",
			Configured: agReady,
			Active:     curProv == models.ProviderAntigravity,
		},
		{
			ID:   "google-oauth",
			Name: "Google - Code Assist free tier (Gemini only, Gmail sign-in, no key)",
			What: "Signs in with your Google account and uses the free Code Assist " +
				"tier. Gemini models only. No API key, no cost.",
			Warning: "Free-tier daily quotas are real and can be small; heavy use can " +
				"lock the account out for an extended period.",
			Configured: oauthReady,
			Active:     curProv == models.ProviderGeminiCA,
		},
		{
			ID:   "nvidia-nim",
			Name: "NVIDIA NIM (free API key)",
			What: "NVIDIA's hosted models via an nvapi-... key. Note: the key is only " +
				"proven at the first generation - NVIDIA lists models without " +
				"authentication, so setup succeeding is not the key working.",
			NeedsInput:  true,
			InputPrompt: "Paste your NVIDIA NIM key (nvapi-...). It is stored in config.json (mode 0600).",
			Secret:      true,
			Configured:  nimReady,
			Active:      curEndpoint == nimName,
		},
		{
			ID:   "ollama",
			Name: "Local Ollama (no key)",
			What: "Models running on this machine at localhost:11434. Free and private; " +
				"speed depends on this machine. Ollama must already be running.",
			Configured: true, // nothing to enter; reachability is checked on apply
			Active:     curEndpoint == ollamaName,
		},
		{
			ID:   "cloudflare",
			Name: "Cloudflare Workers AI - free tier, no card",
			What: "22 free models including a dedicated coder (Qwen2.5-Coder 32B), " +
				"GPT-OSS 120B/20B, Llama 3.3 70B and DeepSeek-R1. Needs a free " +
				"Cloudflare account - no payment details.",
			Warning: "A few models (Kimi, GLM-5.2) need a paid Workers plan and will " +
				"say so. The free daily allowance is shared across all of them.",
			NeedsInput: true,
			InputPrompt: "Paste your Cloudflare Account ID and API token. Order does not " +
				"matter and extra text is ignored - pasting the whole snippet from " +
				"Cloudflare's \"Use REST API\" page works.",
			Secret:     true,
			Configured: cfReady,
			Active:     curEndpoint == cloudflareEndpointName,
		},
		{
			ID:          "anthropic",
			Name:        "Anthropic (Claude) - API key",
			What:        "Paid API. Requires an ANTHROPIC_API_KEY (sk-ant-...).",
			NeedsInput:  true,
			InputPrompt: "Paste your Anthropic API key (sk-ant-...).",
			Secret:      true,
			Configured:  keyed(models.ProviderAnthropic),
			Active:      curProv == models.ProviderAnthropic,
		},
		{
			ID:          "openai",
			Name:        "OpenAI (GPT / o-series) - API key",
			What:        "Paid API. Requires an OPENAI_API_KEY (sk-...).",
			NeedsInput:  true,
			InputPrompt: "Paste your OpenAI API key (sk-...).",
			Secret:      true,
			Configured:  keyed(models.ProviderOpenAI),
			Active:      curProv == models.ProviderOpenAI,
		},
		{
			ID:   "gemini-api",
			Name: "Google Gemini - API key",
			What: "A Google AI Studio key (AIzaSy...). Different from the Gmail sign-in " +
				"above.",
			Warning: "Free-tier keys are heavily rate-limited and can return HTTP 429 " +
				"with a zero quota. The Gmail sign-in option above is usually the " +
				"better free route.",
			NeedsInput:  true,
			InputPrompt: "Paste your Gemini API key (AIzaSy...).",
			Secret:      true,
			Configured:  keyed(models.ProviderGemini),
			Active:      curProv == models.ProviderGemini,
		},
		{
			ID:   "groq",
			Name: "Groq - API key (free tier available)",
			What: "Very fast inference. Free tier caps around 12k tokens/minute - trim " +
				"the /context loadout if you hit it.",
			NeedsInput:  true,
			InputPrompt: "Paste your Groq API key (gsk_...).",
			Secret:      true,
			Configured:  keyed(models.ProviderGROQ),
			Active:      curProv == models.ProviderGROQ,
		},
		{
			ID:   "cerebras",
			Name: "Cerebras - API key",
			What: "Very fast inference. A free key lists models, but inference may " +
				"require credits.",
			NeedsInput:  true,
			InputPrompt: "Paste your Cerebras API key (csk-...).",
			Secret:      true,
			Configured:  keyed(models.ProviderCerebras),
			Active:      curProv == models.ProviderCerebras,
		},
		{
			ID:          "openrouter",
			Name:        "OpenRouter - API key (many models, one key)",
			What:        "A gateway to many providers' models under a single paid key.",
			NeedsInput:  true,
			InputPrompt: "Paste your OpenRouter API key (sk-or-...).",
			Secret:      true,
			Configured:  keyed(models.ProviderOpenRouter),
			Active:      curProv == models.ProviderOpenRouter,
		},
		{
			ID:          "xai",
			Name:        "xAI (Grok) - API key",
			What:        "Paid API. Requires an XAI_API_KEY (xai-...).",
			NeedsInput:  true,
			InputPrompt: "Paste your xAI API key (xai-...).",
			Secret:      true,
			Configured:  keyed(models.ProviderXAI),
			Active:      curProv == models.ProviderXAI,
		},
		{
			ID:   "gcp-custom",
			Name: "Google Cloud - your own project id (billing/quota)",
			What: "The Gmail sign-in, but billed against a specific Google Cloud " +
				"project you control.",
			NeedsInput:  true,
			InputPrompt: "Enter your Google Cloud project id.",
			Secret:      false, // a project id is not a credential
			Active:      false, // indistinguishable from google-oauth by model id
		},
	}

	_, canKeep := cfg.Agents[config.AgentCoder]
	return rows, canKeep
}

// runProviderPortal shows the portal in a loop until a choice applies cleanly,
// the user keeps the current setup, or the user quits. Returns quit=true when
// the launch should abort silently (mirrors the workspace picker contract).
func runProviderPortal(ctx context.Context) (quit bool, err error) {
	for {
		rows, canKeep := providerPortalRows()
		choice, err := startup.AskProviders(rows, canKeep)
		if err != nil {
			// A portal that cannot run must not block the program - same rule
			// as the workspace picker (root.go).
			fmt.Fprintf(os.Stderr, "could not show the provider portal (%v); continuing with current settings\n", err)
			return false, nil
		}
		if choice.Quit {
			return true, nil
		}
		if choice.Keep {
			return false, nil
		}
		if err := applyPortalChoice(ctx, choice); err != nil {
			// The portal has exited, the TUI has not started: plain printing
			// is legal here and nowhere later.
			fmt.Fprintf(os.Stderr, "\nCould not set up %s: %v\n\n", choice.ID, err)
			continue // back to the menu so another provider can be picked
		}
		return false, nil
	}
}

// applyPortalChoice turns a selection into saved credentials and agent models.
func applyPortalChoice(ctx context.Context, c startup.ProviderChoice) error {
	switch c.ID {
	case "antigravity":
		if err := runAntigravityLogin(ctx); err != nil {
			return err
		}
		// GORILLA OVERRIDE: register the provider in-memory BEFORE setting agent
		// models. config.Load enables it from the creds file, but that ran before
		// this login, so within THIS session cfg.Providers has no antigravity
		// entry — and validateAgent then silently reverts every agent to Gemini
		// (revertAgentToDefault returns nil, so the reverts are invisible to
		// UpdateAgentModel). The "oauth-login" placeholder clears the "apiKey==''
		// means disabled" gate; the real auth is the stored token.
		if err := config.UpsertProviderKey(models.ProviderAntigravity, oauthLoginPlaceholder); err != nil {
			return err
		}
		// Coder on Claude Sonnet; the background agents (summarizer/task/title)
		// on Gemini Flash, which draws the SEPARATE Gemini weekly pool and so
		// leaves the Claude/GPT quota for the work the user actually watches.
		if err := config.UpdateAgentModel(config.AgentCoder, models.AGClaudeSonnet46); err != nil {
			return err
		}
		for _, a := range []config.AgentName{config.AgentSummarizer, config.AgentTask, config.AgentTitle} {
			if err := config.UpdateAgentModel(a, models.AGGemini36Flash); err != nil {
				return err
			}
		}
		return nil

	case "google-oauth":
		if err := runGoogleLogin(ctx, ""); err != nil {
			return err
		}
		// Same in-session registration as antigravity above.
		if err := config.UpsertProviderKey(models.ProviderGeminiCA, oauthLoginPlaceholder); err != nil {
			return err
		}
		return applyAgentModels(models.GeminiCAFlash, models.GeminiCA31FlashLite)

	case "gcp-custom":
		if err := runGoogleLogin(ctx, c.Input); err != nil {
			return err
		}
		if err := config.UpsertProviderKey(models.ProviderGeminiCA, oauthLoginPlaceholder); err != nil {
			return err
		}
		return applyAgentModels(models.GeminiCAFlash, models.GeminiCA31FlashLite)

	case "nvidia-nim":
		return applyLocalEndpoint(nimEndpointName, nimBaseURL, c.Input)

	case "ollama":
		return applyLocalEndpoint(ollamaEndpointName, ollamaBaseURL, "")
	case "cloudflare":
		return applyCloudflare(c.Input)

	default:
		prov, ok := portalProvider[c.ID]
		if !ok {
			return fmt.Errorf("unknown provider row %q", c.ID)
		}
		if c.Input != "" {
			if err := config.UpsertProviderKey(prov, c.Input); err != nil {
				return err
			}
		}
		d := portalDefaults[c.ID]
		return applyAgentModels(d.coder, d.title)
	}
}

// applyLocalEndpoint saves the endpoint, registers its models, and points the
// agents at the first one. An empty key keeps whatever key is already stored,
// which is what Enter-on-a-ready-row means.
func applyLocalEndpoint(name, baseURL, key string) error {
	key = strings.TrimSpace(key)

	// GORILLA FIX: adopt whatever the user already calls this endpoint.
	//
	// UpsertLocalEndpoint matches by Name, so writing our fixed name beside a
	// user-named entry on the SAME baseURL created a second endpoint — and two
	// endpoints on one URL steal each other's model routes, last one wins. A
	// config here held "Gorilla.FREE.NVIDIA.NIM" and "NVIDIA NIM" side by side,
	// both with zero registered models.
	//
	// Reusing their name means the upsert updates in place. Their existing key is
	// also carried over when none was typed, so simply pressing Enter on the row
	// never blanks a working credential.
	for _, e := range config.Get().LocalEndpoints {
		if e.BaseURL != baseURL {
			continue
		}
		name = e.Name
		if key == "" {
			key = e.APIKey
		}
		if e.APIKey != "" {
			break // a keyed entry wins over a blank duplicate
		}
	}
	if err := config.UpsertLocalEndpoint(config.LocalEndpoint{
		Name: name, BaseURL: baseURL, APIKey: key,
	}); err != nil {
		return err
	}
	n, first := registerLocalEndpoint(name, baseURL, key)
	if n == 0 {
		return fmt.Errorf("no models found at %s - is it running, and is the key valid?", baseURL)
	}

	// GORILLA FIX: re-selecting an endpoint must not overwrite a model the user
	// already chose on it.
	//
	// The portal runs on EVERY launch, so choosing NVIDIA NIM again re-applied
	// the default to all four agents — silently undoing a /models choice made
	// minutes earlier. Combined with the default being the provider's first
	// listed id ("01-ai/yi-large", which this account cannot even run), the
	// effect was landing on the same unusable model after every single login,
	// no matter what had been picked in between.
	//
	// If the coder is already on a model served by THIS endpoint, and that model
	// is still registered, the choice stands. Same principle as adopting the
	// user's endpoint name above: confirm what is there rather than replace it.
	if cur := config.Get().Agents[config.AgentCoder].Model; cur != "" {
		if _, known := models.SupportedModels[cur]; known && models.LocalEndpointFor(cur) == name {
			return nil
		}
	}
	return applyAgentModels(first, first)
}

// cfAccountRe matches a Cloudflare account id: 32 lowercase hex characters.
var cfAccountRe = regexp.MustCompile(`\b[0-9a-f]{32}\b`)

// cfTokenRe matches a Cloudflare API token. The current template issues
// "cfut_"-prefixed tokens; older ones are a bare 40-char blob.
var cfTokenRe = regexp.MustCompile(`\b(?:cfut_[A-Za-z0-9_\-]{20,}|[A-Za-z0-9_\-]{40})\b`)

// parseCloudflareInput pulls an account id and API token out of whatever the
// user pasted.
//
// GORILLA OVERRIDE: deliberately forgiving. Cloudflare needs TWO values, and its
// "Use REST API" page presents them in separate boxes surrounded by prose and a
// sample curl command. Demanding a precise format would make the most
// error-prone step of the whole setup a typing exercise — so the whole page can
// be pasted and the two values are found by shape. Order does not matter.
//
// The account id is unambiguous (32 hex). The token is matched second and must
// not be the account id itself, which is what the exclusion below prevents.
func parseCloudflareInput(in string) (account, token string, err error) {
	account = cfAccountRe.FindString(in)
	for _, m := range cfTokenRe.FindAllString(in, -1) {
		if m != account {
			token = m
			break
		}
	}
	switch {
	case account == "" && token == "":
		return "", "", fmt.Errorf("could not find a Cloudflare account ID or API token in that - " +
			"the account ID is 32 hex characters and the token usually starts with cfut_")
	case account == "":
		return "", "", fmt.Errorf("found an API token but no account ID - it is 32 hex " +
			"characters, shown on the same page under \"Account ID\"")
	case token == "":
		return "", "", fmt.Errorf("found an account ID but no API token - it usually starts " +
			"with cfut_ and is only shown once, when you create it")
	}
	return account, token, nil
}

// applyCloudflare configures Workers AI from a pasted account id + token.
func applyCloudflare(input string) error {
	account, token, err := parseCloudflareInput(input)
	if err != nil {
		return err
	}
	return applyLocalEndpoint(cloudflareEndpointName, fmt.Sprintf(cloudflareBaseFmt, account), token)
}

// runAntigravityLogin runs the Antigravity OAuth flow and provisions the
// managed free-tier project. Prints plainly because the portal runs before the
// TUI owns the screen (same as runGoogleLogin).
func runAntigravityLogin(ctx context.Context) error {
	fmt.Println("\nStarting Google sign-in (Antigravity free tier)...")
	creds, err := auth.AntigravityLogin(ctx)
	if err != nil {
		return fmt.Errorf("sign-in failed: %w", err)
	}
	if err := creds.Save(); err != nil {
		return fmt.Errorf("could not save credentials: %w", err)
	}
	who := creds.Email
	if who == "" {
		who = "your Google account"
	}
	fmt.Printf("\nSigned in as %s.\n", who)
	fmt.Println("Setting up your Antigravity free tier...")
	if err := creds.SetupProject(ctx); err != nil {
		// Sign-in landed; only project discovery hiccuped. It retries on first
		// use, so do not fail the whole login.
		fmt.Fprintf(os.Stderr, "\nSigned in, but project setup failed: %v\n", err)
		fmt.Fprintln(os.Stderr, "Your token is saved; it will retry on first use.")
		return nil
	}
	fmt.Printf("Ready. Project: %s\n", creds.ProjectID)
	fmt.Println("\nStart chatting — pick a Claude / GPT-OSS / Gemini model from /model.")
	return nil
}

// applyAgentModels re-points ALL agents, not just the coder. Leaving the
// background agents (title/summarizer/task) on the previous provider is the
// documented "title generation failed on Groq" trap.
func applyAgentModels(coder, title models.ModelID) error {
	for _, a := range []config.AgentName{config.AgentCoder, config.AgentSummarizer, config.AgentTask} {
		if err := config.UpdateAgentModel(a, coder); err != nil {
			return err
		}
	}
	return config.UpdateAgentModel(config.AgentTitle, title)
}
