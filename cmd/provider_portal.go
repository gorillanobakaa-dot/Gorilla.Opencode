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
	"sort"
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

	// GORILLA OVERRIDE (2026-09-01): LM Studio had no row at all. It was
	// reachable only by typing a URL into the custom-endpoint form in /connect,
	// which is precisely the manual step the people this is built for cannot
	// take. It is one of the two most common ways to run a model locally and it
	// is free, private and needs no account — so it belongs in the picker beside
	// Ollama, not behind a form.
	lmStudioEndpointName = "LM Studio"
	lmStudioBaseURL      = "http://localhost:1234/v1"

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
	"deepseek":   models.ProviderDeepSeek,
}

// portalDefaults picks the model each provider starts on, for the providers
// whose lists are compiled in. Mirrors config.setDefaultModelForAgent.
//
// GORILLA OVERRIDE (2026-08-21): the fetched providers are deliberately absent.
// Their models are not known until their catalogue has been fetched, so their
// default is resolved AFTER the fetch, by models.PreferredCatalogueModel. A
// constant here would be exactly the stale-default bug this change removes —
// this map used to name Claude 3.7 Sonnet, GPT-4.1 and Grok-3-beta, all three of
// which were dead by the time anyone noticed.
var portalDefaults = map[string]struct{ coder, title models.ModelID }{
	"gemini-api": {models.GeminiFlashLatest, models.GeminiFlashLiteLatest},
	"openrouter": {models.OpenRouterNvidiaNemotron3Ultra550bA55bFree, models.OpenRouterOpenaiGptOss20bFree},
}

// fetchProviderCatalogue is a seam, like registerLocalEndpoint below: it talks to
// the network, and tests must not.
var fetchProviderCatalogue = models.FetchProviderCatalogue

// registerLocalEndpoint is a seam: RegisterLocalEndpoint fetches /v1/models
// over the network, and tests must not.
var registerLocalEndpoint = models.RegisterLocalEndpoint

// probeEndpoint is a seam for the same reason.
var probeEndpoint = models.ProbeEndpoint

// portalDoctor runs the model doctor after a provider has been set up and
// prints what it fixed or found. /provider and the launch portal both end here,
// so a switch can never leave an agent on a model that is gone.
func portalDoctor() {
	for _, line := range config.DoctorSummary(config.RunModelDoctor(config.CacheBase(), config.UpdateAgentModel)) {
		fmt.Println(line)
	}
}

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
			// GORILLA FIX (2026-10-05): compare where the address LEADS.
			// http://127.0.0.1:1234/v1 and http://localhost:1234/v1 are one
			// server. Compared as text they were two, so the owner's own
			// "lmstudio" entry was invisible here and choosing the LM Studio row
			// wrote a second entry beside it: two connections, one server.
			if models.CanonicalEndpointURL(e.BaseURL) != models.CanonicalEndpointURL(baseURL) || e.Disabled {
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
	cgCreds, _ := auth.LoadChatGPTCreds()
	cgReady := cgCreds != nil && cgCreds.AccessToken != ""
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
	lmStudioEp, _ := endpointFor(lmStudioBaseURL)
	nimReady = nimReady && nimEp.APIKey != ""

	// GORILLA OVERRIDE (2026-09-01): ask the two local runtimes what they are
	// serving, before drawing the menu.
	//
	// Both probes run concurrently against loopback with a 1.5s ceiling, and a
	// port with nothing behind it refuses instantly — so the common case costs
	// no measurable time. What it buys is a row that tells the truth: "running -
	// 3 models (qwen2.5-coder...)" instead of a readiness tick that means
	// nothing until you select it and it fails.
	localProbes := models.ProbeDefaultLocalRuntimes()

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
	lmStudioName := lmStudioEndpointName
	if lmStudioEp.Name != "" {
		lmStudioName = lmStudioEp.Name
	}

	// Which row is the session currently on?
	curModel := cfg.Agents[config.AgentCoder].Model
	curProv := models.SupportedModels[curModel].Provider
	curEndpoint := models.LocalEndpointFor(curModel) // "" unless a local model

	// GORILLA OVERRIDE (2026-08-21): the order is EASIEST ACCESS FIRST, and
	// vendor families are kept together.
	//
	// It used to be free-sign-ins, then local endpoints, then keys — a real rule
	// that was invisible on screen, because Google's three routes sat at
	// positions 1, 3 and 9 with unrelated providers between them. Reported by the
	// owner looking at his own picker: "if there is a logic in the way they are
	// displayed I can't see it."
	//
	// An order nobody can infer is not an order. So: Google first (two of its
	// three routes are a Gmail sign-in with no key and no card — the easiest way
	// in that exists), then the ChatGPT sign-in, then NVIDIA's free key, then
	// everything else free, then the ones that need a card. Within the Google
	// block the names now share a "Google" prefix so the grouping is legible
	// without counting rows.
	//
	// Pinned by TestPortalRowOrder in provider_portal_order_test.go, because an
	// ordering that lives only in the order of a literal is one careless insert
	// from being scrambled again.
	rows := []startup.ProviderRow{
		// Google, all three routes together. Two are a Gmail sign-in with no key
		// and no card, which is the easiest way in that exists, so the family
		// leads.
		{
			ID:   "antigravity",
			Free: true,
			Name: "Google Antigravity - Claude + GPT-OSS + Gemini (Gmail sign-in)",
			// GORILLA OVERRIDE (2026-10-05): the model names are READ from the
			// registry. This sentence used to say "Claude Sonnet/Opus 4.6" long
			// after Google had stopped listing either.
			What: "Signs in with your Google account and uses your free Google " +
				"Antigravity tier: Claude, GPT-OSS and Gemini models. " +
				portalModelSummary(models.ProviderAntigravity) +
				" No API key, no cost - it is your account's own entitlement.",
			Warning: "Weekly quotas apply per model group (Gemini separate from " +
				"Claude/GPT). Unofficial: it speaks the Antigravity CLI's protocol, so " +
				"a Google-side change could break it without notice.",
			Configured: agReady,
			Active:     curProv == models.ProviderAntigravity,
		},
		{
			ID:   "google-oauth",
			Free: true,
			Name: "Google Code Assist - Gemini only (Gmail sign-in, no key)",
			What: "Signs in with your Google account and uses the free Code Assist " +
				"tier. Gemini models only. No API key, no cost.",
			Warning: "Free-tier daily quotas are real and can be small; heavy use can " +
				"lock the account out for an extended period.",
			Configured: oauthReady,
			Active:     curProv == models.ProviderGeminiCA,
		},
		{
			ID:   "gemini-api",
			Name: "Google Gemini - API key",
			// GORILLA OVERRIDE (2026-08-21): say WHERE the key comes from and
			// that it costs nothing. The row named the key and assumed the
			// reader already had one — but somebody who does not have a key is
			// exactly who this row is for, and "go and find out how" is the
			// closed door PHILOSOPHY.md argues against. Confirmed on the owner's
			// own account the same day: Billing Tier reads "Free tier", with no
			// card attached and no billing set up.
			What: "A Google AI Studio key. Free, and it needs no card: make one at " +
				"aistudio.google.com/apikey with any Google account, and leave billing " +
				"switched off. It is separate from the Gmail sign-in rows above and has " +
				"its own allowance, so when one is used up the other still works.",
			Warning: "The free allowance is limited per minute and per day. A busy turn " +
				"can use it up, which arrives as HTTP 429 or a bare \"unknown error\" — " +
				"that is the limit, not a broken key. Wait, or switch to a Google " +
				"sign-in row above, which spends a different allowance.",
			NeedsInput: true,
			InputPrompt: "Paste your Gemini API key (it starts AIza... or AQ....). Free from " +
				"aistudio.google.com/apikey - no card needed.",
			Secret:     true,
			Configured: keyed(models.ProviderGemini),
			Active:     curProv == models.ProviderGemini,
		},
		// The other sign-in: a ChatGPT account, free plan included.
		{
			ID:   "chatgpt",
			Free: true,
			// GORILLA OVERRIDE (2026-10-05): no model is named in the label, and
			// the ones in the description are READ from the registry. The label
			// said "GPT-5.5", the description "GPT-5.5 and GPT-5.4 Mini", and the
			// warning that GPT-5.6 "is not offered here" - on the day the session
			// behind this very menu was running GPT-5.6-Terra, and OpenAI had
			// retired 5.4 Mini five weeks earlier. Three typed facts, three wrong.
			Name: "ChatGPT sign-in - OpenAI models (works on the FREE plan, no API key)",
			What: "Signs in with your ChatGPT account and uses OpenAI models through " +
				"the Codex backend. No API key and no credit card: a free ChatGPT " +
				"account is enough. " + portalModelSummary(models.ProviderChatGPT),
			Warning: "Usage counts against your ChatGPT plan's limits, so a free plan " +
				"will hit a cooldown rather than a bill.",
			Configured: cgReady,
			Active:     curProv == models.ProviderChatGPT,
		},
		// NVIDIA: a free key, pasted once, ~100 models.
		{
			ID:   "nvidia-nim",
			Free: true,
			Name: "NVIDIA NIM (free API key)",
			// GORILLA OVERRIDE (2026-10-05): this used to warn that "the key is
			// only proven at the first generation ... setup succeeding is not the
			// key working". True, and the wrong answer to it: the program now
			// asks a model one small question during setup (models/probe.go), so
			// the key IS proven before the menu closes.
			What: "NVIDIA's hosted models via a free nvapi-... key from build.nvidia.com. " +
				"When you save the key, the program asks a few models one small question " +
				"to prove the key works and to start you on a model that actually answers.",
			NeedsInput:  true,
			InputPrompt: "Paste your NVIDIA NIM key (nvapi-...). It is stored in config.json (mode 0600).",
			Secret:      true,
			// Refuse a value that cannot be an NVIDIA key BEFORE it is saved.
			// See checkNIMKey.
			Check: checkNIMKey,
			Configured:  nimReady,
			Active:      curEndpoint == nimName,
		},
		// Everything else that costs nothing: your own machine, then free keys.
		//
		// GORILLA OVERRIDE (2026-09-01): both local rows now report what is
		// ACTUALLY there. See localRuntimeRow.
		localRuntimeRow(localRuntimeSpec{
			id:      "ollama",
			label:   "Ollama",
			probe:   localProbes["ollama"],
			active:  curEndpoint == ollamaName,
			port:    "11434",
			getIt:   "Install it free from ollama.com, then run: ollama pull qwen2.5-coder",
			startIt: "Start Ollama (it runs in the background / system tray), then re-open this menu.",
		}),
		localRuntimeRow(localRuntimeSpec{
			id:      "lmstudio",
			label:   "LM Studio",
			probe:   localProbes["lmstudio"],
			active:  curEndpoint == lmStudioName,
			port:    "1234",
			getIt:   "Install it free from lmstudio.ai, download a model, then start its local server.",
			startIt: "Open LM Studio, load a model, and turn on the local server (the Developer tab), then re-open this menu.",
		}),
		{
			ID:   "cloudflare",
			Free: true,
			Name: "Cloudflare Workers AI - free tier, no card",
			What: "22 free models including a dedicated coder (Qwen2.5-Coder 32B), " +
				"GPT-OSS 120B/20B, Llama 3.3 70B and DeepSeek-R1. Needs a free " +
				"Cloudflare account - no payment details.",
			Warning: "A few models (Kimi, GLM-5.2) need a paid Workers plan and will " +
				"say so. The free daily allowance is shared across all of them.",
			NeedsInput: true,
			// Two fields, one value each. Cloudflare shows the account ID and the
			// token in separate boxes, and asking for both in one field meant a
			// paste containing a newline submitted early and silently lost the
			// rest.
			InputPrompt:  "Cloudflare Account ID (32 hex characters, shown under \"Account ID\")",
			InputPrompt2: "Cloudflare API token (starts with cfut_, shown once when you create it)",
			Secret:       false, // an account id is not a credential; showing it lets a typo be spotted
			Secret2:      true,
			Configured:   cfReady,
			Active:       curEndpoint == cloudflareEndpointName,
		},
		{
			ID:   "groq",
			Free: true,
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
			Free: true,
			Name: "Cerebras - API key",
			What: "Very fast inference. A free key lists models, but inference may " +
				"require credits.",
			NeedsInput:  true,
			InputPrompt: "Paste your Cerebras API key (csk-...).",
			Secret:      true,
			Configured:  keyed(models.ProviderCerebras),
			Active:      curProv == models.ProviderCerebras,
		},
		// Paid. Last, deliberately — see the ordering note above.
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
			// GORILLA OVERRIDE (2026-08-21): DeepSeek had a provider, a client
			// and a model list, but no row — so the only route to it was hand-
			// editing config.json, which the desktop-icon majority does not have
			// (see the "desktop entry passes NO arguments" trap in CLAUDE.md).
			ID:          "deepseek",
			Name:        "DeepSeek - API key",
			What:        "Paid API, priced well below the US providers. Requires a DEEPSEEK_API_KEY (sk-...).",
			NeedsInput:  true,
			InputPrompt: "Paste your DeepSeek API key (sk-...).",
			Secret:      true,
			Configured:  keyed(models.ProviderDeepSeek),
			Active:      curProv == models.ProviderDeepSeek,
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
		portalDoctor()
		return false, nil
	}
}

// reopenProviderPortal is the mid-session escape hatch, wired into the TUI as
// tui.ReopenProviderPortal.
//
// GORILLA OVERRIDE: same picker, same rows, same readiness markers as launch.
// It differs from the startup path in two ways that matter:
//
//   - a failure RETURNS instead of looping back to the menu. At startup the
//     TUI has not begun and looping is free; here the terminal is on loan from
//     bubbletea via tea.Exec, and staying inside it means the caller cannot
//     report anything. The error surfaces in the status bar instead.
//   - Quit does not quit the program. The user asked to change provider, not to
//     end the session; treating ctrl+c as "abandon the switch" is the reading
//     that cannot lose their work.
func reopenProviderPortal() error {
	rows, canKeep := providerPortalRows()
	choice, err := startup.AskProviders(rows, true) // something already works: we are mid-session
	_ = canKeep
	if err != nil {
		return fmt.Errorf("could not show the provider picker: %w", err)
	}
	if choice.Keep || choice.Quit || choice.ID == "" {
		return nil
	}
	if err := applyPortalChoice(context.Background(), choice); err != nil {
		return fmt.Errorf("could not set up %s: %w", choice.ID, err)
	}
	portalDoctor()
	return nil
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
		// GORILLA OVERRIDE (2026-10-05): ask the backend what it serves, THEN
		// choose. This used to name two constants, AGClaudeSonnet46 and
		// AGGemini36Flash. Google stopped listing claude-sonnet-4-6 and every
		// sign-in went on putting the coder on it. Same fault, same fix as the
		// ChatGPT branch below (2026-08-23), which should have been applied to
		// both at the time.
		//
		// A failed listing is not a failed sign-in: whatever is registered (the
		// last cached list, or the built-in one) is used.
		refreshAntigravityCatalogue(ctx)
		// Coder on the best Claude Sonnet; the background agents
		// (summarizer/task/title) on a Gemini Flash, which draws the SEPARATE
		// Gemini weekly pool and so leaves the Claude/GPT quota for the work the
		// user actually watches.
		coder, background := models.PreferredAntigravityModels()
		if coder == "" {
			return fmt.Errorf("signed in, but no Antigravity models are registered")
		}
		// GORILLA FIX: re-selecting the row must not undo a /model choice. The
		// portal runs on every launch; if the coder is already on a model this
		// provider still offers, it stands (same rule as applyLocalEndpoint).
		if cur := config.Get().Agents[config.AgentCoder].Model; cur != "" {
			if m, known := models.SupportedModels[cur]; known && m.Provider == models.ProviderAntigravity {
				fmt.Printf("\nKeeping your model: %s. /model to switch.\n", m.Name)
				return nil
			}
		}
		if err := config.UpdateAgentModel(config.AgentCoder, coder); err != nil {
			return err
		}
		for _, a := range []config.AgentName{config.AgentSummarizer, config.AgentTask, config.AgentTitle} {
			if err := config.UpdateAgentModel(a, background); err != nil {
				return err
			}
		}
		fmt.Printf("\nStart chatting - %s is selected; /model to switch.\n", models.SupportedModels[coder].Name)
		return nil

	case "chatgpt":
		if err := runChatGPTPortalLogin(ctx); err != nil {
			return err
		}
		// Same in-session registration as antigravity above: config.Load read
		// the creds file before this login existed, so without this the provider
		// is disabled for the rest of the session and every agent silently
		// reverts to Gemini.
		if err := config.UpsertProviderKey(models.ProviderChatGPT, oauthLoginPlaceholder); err != nil {
			return err
		}
		// GORILLA OVERRIDE (2026-08-23): ask the backend what it serves before
		// choosing, rather than naming two models here.
		//
		// This line used to read applyAgentModels(models.ChatGPT55,
		// models.ChatGPT54Mini). Two constants, decided once at sign-in, that
		// then silently governed the whole session. By 2026-08-23 both were
		// wrong: the backend had been serving GPT-5.6 Terra and Luna above 5.5
		// in its own ordering, and 5.4-Mini is retired on 31 Aug 2026.
		//
		// A failed refresh is not a failed sign-in. The built-in list in
		// chatgpt.go is still registered, so falling through to it leaves the
		// user working rather than stranded at a portal step.
		refreshChatGPTCatalogue(ctx)
		best, cheap := models.PreferredChatGPTModels()
		if best == "" {
			return fmt.Errorf("signed in, but no ChatGPT models are registered")
		}
		// The strong model codes; the cheapest one does titles and summaries.
		// On a free plan the COOLDOWN is the scarce resource rather than money,
		// so the good model must not be spent generating conversation titles.
		if err := applyAgentModels(best, cheap); err != nil {
			return err
		}
		// Printed AFTER the choice, from the choice. It used to be printed before
		// the list was fetched and said "GPT-5.5 is selected" whatever was.
		fmt.Printf("\nStart chatting - %s is selected; /model to switch.\n", models.SupportedModels[best].Name)
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

	case "nvidia-nim":
		return applyLocalEndpoint(nimEndpointName, nimBaseURL, c.Input)

	case "ollama":
		return applyLocalEndpoint(ollamaEndpointName, ollamaBaseURL, "")
	case "lmstudio":
		return applyLocalEndpoint(lmStudioEndpointName, lmStudioBaseURL, "")
	case "cloudflare":
		return applyCloudflare(c.Input, c.Input2)

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
		// GORILLA OVERRIDE (2026-08-21): fetch the provider's list the moment a
		// key is accepted. This is the point where a key first exists, so it is
		// the earliest the provider can be asked what it serves — and asking now
		// means the picker is populated before the user reaches it, rather than
		// showing an empty provider until the next /update.
		if _, live := models.LiveCatalogues[prov]; live {
			key := strings.TrimSpace(c.Input)
			if key == "" {
				key = config.ProviderAPIKey(prov)
			}
			res, err := fetchProviderCatalogue(prov, key, config.CacheBase())
			if err != nil {
				// The key is saved and the endpoint may simply be unreachable
				// right now. Say what happened rather than failing the whole
				// selection — /update retries, and a cached list may already be
				// in place from a previous run.
				return fmt.Errorf("saved the key, but could not list %s models: %w", prov, err)
			}
			model := models.PreferredCatalogueModel(prov)
			if model == "" {
				return fmt.Errorf("%s listed %d models, none usable for chat", res.Label, res.Usable)
			}
			return applyAgentModels(model, model)
		}
		// Gemini's list is fetched the moment a key exists, like the others.
		// Best-effort: the built-in rolling aliases keep working without it.
		if prov == models.ProviderGemini {
			if raw, err := models.FetchGeminiList(config.ProviderAPIKey(prov)); err == nil {
				if res, err := models.RefreshGemini(config.CacheBase(), raw); err == nil {
					fmt.Printf("Model list refreshed from Google: %d available.\n", res.Usable)
				}
			}
		}
		d := portalDefaults[c.ID]
		return applyAgentModels(d.coder, d.title)
	}
}

// applyLocalEndpoint saves the endpoint, registers its models, and points the
// agents at the first one. An empty key keeps whatever key is already stored,
// which is what Enter-on-a-ready-row means.
// localRuntimeSpec describes one on-this-machine model server for the picker.
type localRuntimeSpec struct {
	id      string
	label   string
	probe   models.LocalProbe
	active  bool
	port    string
	getIt   string // what to do when it is not installed
	startIt string // what to do when it is installed but not running
}

// localRuntimeRow builds a picker row that states what is actually on this
// machine right now.
//
// GORILLA OVERRIDE (2026-09-01): the Ollama row used to be a flat
// `Configured: true` with the comment "reachability is checked on apply". That
// gave a local server that was not running the SAME readiness marker as a
// working cloud key. A user picked it, the apply failed, and nothing in the
// interface had given them a reason to expect that — so the honest reading was
// "this program is broken", not "I need to start Ollama".
//
// PHILOSOPHY.md's whole argument is that a closed door must say how to open it.
// So the row now distinguishes the three states that need three different
// actions, and says which one you are in before you commit:
//
//	running        -> how many models, and the first couple by name
//	not running    -> the exact thing to click, and where the setting is
//	not installed  -> where to get it, free, and the one command after that
//
// Marked Configured only when it is genuinely usable, so the readiness column
// means one thing everywhere in this menu.
func localRuntimeRow(s localRuntimeSpec) startup.ProviderRow {
	name := fmt.Sprintf("%s on this machine - %s", s.label, s.probe.Summary())

	what := fmt.Sprintf(
		"Models running on your own computer at localhost:%s. Free, private, and it "+
			"works with no internet: nothing you type leaves the machine. Speed depends "+
			"on your computer rather than on a subscription.", s.port)

	var warning string
	if s.probe.Running && s.probe.Count > 0 {
		what += fmt.Sprintf("\n\nReady now: %d model(s) available - %s.",
			s.probe.Count, strings.Join(s.probe.Names, ", "))
	} else {
		// Not reachable. Say both things, because from here we cannot tell
		// "installed but switched off" from "never installed", and guessing
		// wrong sends the reader to the wrong fix.
		warning = s.label + " is not answering on port " + s.port + " right now.\n" +
			"  If it IS installed: " + s.startIt + "\n" +
			"  If it is NOT installed: " + s.getIt
	}

	return startup.ProviderRow{
		ID:         s.id,
		Free:       true,
		Name:       name,
		What:       what,
		Warning:    warning,
		Configured: s.probe.Running && s.probe.Count > 0,
		Active:     s.active,
	}
}

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
		// The same server under another spelling of its address is the same
		// connection: adopt it, and keep the address the user wrote.
		if models.CanonicalEndpointURL(e.BaseURL) != models.CanonicalEndpointURL(baseURL) {
			continue
		}
		name = e.Name
		baseURL = e.BaseURL
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

	// GORILLA OVERRIDE (2026-10-05): ASK before trusting the list.
	//
	// NVIDIA lists its models to anyone, key or no key, and lists models that
	// answer 500. So "n > 0" proved nothing about the key, and "first" was the
	// first id in the list. A few small requests settle both: is the key
	// accepted, and which model answers with a tool call. A model server on
	// this machine is never asked (the probe refuses loopback itself).
	rep := probeEndpoint(name, config.CacheBase())
	if rep.Skipped == "" {
		fmt.Println("\n" + rep.Note())
		for _, v := range rep.Failed {
			if v.Outcome == models.ProbeRefused {
				return fmt.Errorf("the key was saved, but %s refused it (HTTP %d). Check that the whole key was copied, then press r on the row to enter it again", name, v.Status)
			}
		}
		if id := models.PreferredOnEndpoint(name); id != "" {
			first = id
		}
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
			// ... unless that model has since been found retired: keeping it
			// would be keeping an error.
			if v, seen := models.ProbeVerdictFor(cur); !seen || !v.Dead() {
				return nil
			}
		}
	}
	if err := applyAgentModels(first, first); err != nil {
		return err
	}
	fmt.Printf("Start chatting - %s is selected; /model to switch.\n", models.SupportedModels[first].Name)
	return nil
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

// applyCloudflare configures Workers AI from the account id and API token.
//
// Both are still run through the shape-matchers rather than trusted verbatim:
// people paste surrounding whitespace, quotes, "Bearer " prefixes and the odd
// stray line, and rejecting that would be pedantry. The fields only decide
// WHICH value goes where; the matchers decide what each one actually is.
func applyCloudflare(account, token string) error {
	// GORILLA FIX: selecting an ALREADY-CONFIGURED row supplies no values.
	//
	// The portal only asks for input when a row is not yet configured, so
	// pressing Enter on a "(ready)" Cloudflare row arrives here with both
	// arguments empty and this function rejected it — "that does not look like a
	// Cloudflare account ID" — for credentials that were saved and working.
	// Reported 2026-08-05, three times in a row, with the cursor bouncing back
	// to the previous provider each time.
	//
	// NVIDIA never hit this because its base URL is a constant and
	// applyLocalEndpoint already falls back to the stored key. Cloudflare's base
	// URL embeds the account id, so the whole stored endpoint has to be reused.
	// `r` on the row is still how you REPLACE the credentials deliberately.
	if strings.TrimSpace(account) == "" && strings.TrimSpace(token) == "" {
		for _, e := range config.Get().LocalEndpoints {
			if !e.Disabled && strings.Contains(e.BaseURL, "api.cloudflare.com") && e.APIKey != "" {
				return applyLocalEndpoint(e.Name, e.BaseURL, e.APIKey)
			}
		}
		return fmt.Errorf("no saved Cloudflare credentials to use - press r on this row to enter them")
	}

	acc := cfAccountRe.FindString(account)
	if acc == "" {
		return fmt.Errorf("that does not look like a Cloudflare account ID - it is 32 " +
			"hex characters, shown on the Workers AI page under \"Account ID\"")
	}
	tok := cfTokenRe.FindString(token)
	if tok == "" {
		return fmt.Errorf("that does not look like a Cloudflare API token - it usually " +
			"starts with cfut_ and is only shown once, when you create it")
	}
	return applyLocalEndpoint(cloudflareEndpointName, fmt.Sprintf(cloudflareBaseFmt, acc), tok)
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
	return nil
}

// runChatGPTPortalLogin runs the ChatGPT OAuth flow from the provider portal.
// Prints plainly because the portal runs before the TUI owns the screen (same
// as runGoogleLogin and runAntigravityLogin).
//
// Separate from cmd/login.go's runChatGPTLogin, which is the standalone
// `login --chatgpt` command: that one reuses existing credentials and prints the
// model list, which is the right behaviour at a prompt and the wrong behaviour
// mid-portal, where the user has just chosen to set this provider up.
func runChatGPTPortalLogin(ctx context.Context) error {
	fmt.Println("\nStarting ChatGPT sign-in...")
	creds, err := auth.ChatGPTLogin(ctx)
	if err != nil {
		return fmt.Errorf("sign-in failed: %w", err)
	}
	if err := creds.Save(); err != nil {
		return fmt.Errorf("could not save credentials: %w", err)
	}
	who := creds.Email
	if who == "" {
		who = "your ChatGPT account"
	}
	plan := creds.PlanType
	if plan == "" {
		plan = "unknown"
	}
	fmt.Printf("\nSigned in as %s (plan: %s).\n", who, plan)
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

// refreshChatGPTCatalogue asks the backend what it currently serves and
// registers it, best-effort.
//
// Best-effort deliberately: this runs immediately after a successful sign-in,
// and a listing that fails must not turn a working sign-in into an error. The
// built-in list in internal/llm/models/chatgpt.go stays registered either way,
// so the fallback is a slightly stale picker rather than an empty one.
//
// It costs no extra round trip beyond the one GET the portal would make anyway
// to confirm the token works.
func refreshChatGPTCatalogue(ctx context.Context) {
	creds, err := auth.LoadChatGPTCreds()
	if err != nil || creds == nil {
		return
	}
	status, body, err := creds.ProbeBackend(ctx)
	if err != nil || status != 200 {
		return
	}
	res, err := models.RefreshChatGPT(config.CacheBase(), []byte(body))
	if err != nil || res == nil {
		return
	}
	fmt.Printf("Model list refreshed from OpenAI: %d available.\n", res.Usable)
}

// refreshAntigravityCatalogue asks the Antigravity backend what it currently
// serves and registers it, best-effort. The pair to refreshChatGPTCatalogue, and
// best-effort for the same reason: it runs straight after a sign-in, and a
// listing that fails must not turn a working sign-in into an error.
func refreshAntigravityCatalogue(ctx context.Context) {
	creds, err := auth.LoadAntigravityCreds()
	if err != nil || creds == nil {
		return
	}
	fetched, err := creds.FetchAvailableModels(ctx)
	if err != nil {
		return
	}
	rows := make([]models.AntigravityRow, 0, len(fetched))
	for id, m := range fetched {
		rows = append(rows, models.AntigravityRow{
			ID: id, DisplayName: m.DisplayName, APIProvider: m.APIProvider,
			MaxTokens: m.MaxTokens, MaxOutputTokens: m.MaxOutputTokens,
			SupportsImages: m.SupportsImages, SupportsThinking: m.SupportsThinking,
			IsInternal: m.IsInternal,
		})
	}
	res, err := models.RefreshAntigravity(config.CacheBase(), rows)
	if err != nil || res == nil {
		return
	}
	fmt.Printf("Model list refreshed from Google: %d available.\n", res.Usable)
}

// portalModelSummary says what a sign-in provider offers RIGHT NOW, from the
// registry: how many models, and the first few by rank. It exists so no portal
// row has a model name typed into it; every one that did went stale.
func portalModelSummary(p models.ModelProvider) string {
	type row struct {
		name string
		rank int
	}
	var rows []row
	for _, m := range models.SupportedModels {
		if m.Provider != p {
			continue
		}
		name := m.Name
		for _, suffix := range []string{" (Antigravity free)", " (ChatGPT sign-in)"} {
			name = strings.TrimSuffix(name, suffix)
		}
		rank := m.Rank
		if rank <= 0 {
			rank = 1 << 20
		}
		rows = append(rows, row{name, rank})
	}
	if len(rows) == 0 {
		return "The model list is fetched when you sign in."
	}
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].rank != rows[j].rank {
			return rows[i].rank < rows[j].rank
		}
		return rows[i].name < rows[j].name
	})
	const show = 3
	names := make([]string, 0, show)
	for _, r := range rows {
		if len(names) == show {
			break
		}
		names = append(names, r.name)
	}
	if len(rows) > show {
		return fmt.Sprintf("%d models at the last check, among them %s.", len(rows), strings.Join(names, ", "))
	}
	return fmt.Sprintf("At the last check: %s.", strings.Join(names, ", "))
}

// checkNIMKey refuses a value that cannot be an NVIDIA key, before it is saved.
//
// GORILLA OVERRIDE (2026-10-05): the field accepted anything, one character
// included, and NVIDIA lists its models WITHOUT authentication - so a wrong
// value was saved, the model list came back, setup reported success, and the
// first message failed with an error that reads like a broken service. The
// owner's screenshot shows the field holding "(1 chars)" after a paste. A
// check that costs nothing and says exactly what is wrong belongs here, at
// the one moment the user is looking at the field.
//
// Returns "" when the value is acceptable, otherwise the sentence to show.
func checkNIMKey(v string) string {
	v = strings.TrimSpace(v)
	switch {
	case !strings.HasPrefix(v, "nvapi-"):
		return fmt.Sprintf("That is %d character(s) and does not start with nvapi- so it is not an "+
			"NVIDIA key, and it was NOT saved. Copy the whole key from build.nvidia.com, then press "+
			"Ctrl+V here.", len([]rune(v)))
	case len(v) < 40:
		return fmt.Sprintf("That starts with nvapi- but is only %d characters. A real key is about 70, "+
			"so part of it is missing, and it was NOT saved. Copy the whole key and press Ctrl+V again.", len(v))
	}
	return ""
}
