// GORILLA OVERRIDE: the config-aware half of the every-launch provider portal.
// The startup package renders; this file decides what the rows say, which ones
// count as configured, and how a selection becomes saved credentials plus agent
// models. Split this way because startup cannot import config.
package cmd

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/opencode-ai/opencode/internal/auth"
	"github.com/opencode-ai/opencode/internal/config"
	"github.com/opencode-ai/opencode/internal/llm/models"
	"github.com/opencode-ai/opencode/internal/tui/startup"
)

// Endpoint names are FIXED so a re-selection updates the same entry instead of
// accumulating duplicates — UpsertLocalEndpoint matches by Name, and one user
// config once held four entries for the same NVIDIA URL.
const (
	nimEndpointName    = "NVIDIA NIM"
	nimBaseURL         = "https://integrate.api.nvidia.com/v1"
	ollamaEndpointName = "Ollama"
	ollamaBaseURL      = "http://localhost:11434/v1"

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
	endpoint := func(name string) (config.LocalEndpoint, bool) {
		for _, e := range cfg.LocalEndpoints {
			if e.Name == name && !e.Disabled {
				return e, true
			}
		}
		return config.LocalEndpoint{}, false
	}

	creds, _ := auth.LoadGeminiCreds()
	oauthReady := creds != nil && creds.AccessToken != ""
	agCreds, _ := auth.LoadAntigravityCreds()
	agReady := agCreds != nil && agCreds.AccessToken != ""
	nimEp, nimReady := endpoint(nimEndpointName)
	_, ollamaReady := endpoint(ollamaEndpointName)
	nimReady = nimReady && nimEp.APIKey != ""
	_ = ollamaReady // reachability is decided on apply, not from stored state

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
			Active:      curEndpoint == nimEndpointName,
		},
		{
			ID:   "ollama",
			Name: "Local Ollama (no key)",
			What: "Models running on this machine at localhost:11434. Free and private; " +
				"speed depends on this machine. Ollama must already be running.",
			Configured: true, // nothing to enter; reachability is checked on apply
			Active:     curEndpoint == ollamaEndpointName,
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
	if key == "" {
		for _, e := range config.Get().LocalEndpoints {
			if e.Name == name {
				key = e.APIKey
			}
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
	return applyAgentModels(first, first)
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
