// Package config manages application configuration from various sources.
package config

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"

	"github.com/opencode-ai/opencode/internal/auth"
	"github.com/opencode-ai/opencode/internal/fileutil"
	"github.com/opencode-ai/opencode/internal/llm/models"
	"github.com/opencode-ai/opencode/internal/logging"
	"github.com/spf13/viper"
)

// MCPType defines the type of MCP (Model Control Protocol) server.
type MCPType string

// Supported MCP types
const (
	MCPStdio MCPType = "stdio"
	MCPSse   MCPType = "sse"
)

// MCPServer defines the configuration for a Model Control Protocol server.
type MCPServer struct {
	Command string            `json:"command"`
	Env     []string          `json:"env"`
	Args    []string          `json:"args"`
	Type    MCPType           `json:"type"`
	URL     string            `json:"url"`
	Headers map[string]string `json:"headers"`
}

type AgentName string

const (
	AgentCoder      AgentName = "coder"
	AgentSummarizer AgentName = "summarizer"
	AgentTask       AgentName = "task"
	AgentTitle      AgentName = "title"
	// GORILLA OVERRIDE: research helpers get their own agent entry so they can
	// be pointed at a DIFFERENT model from the coder. Research is read-heavy and
	// wide, not clever: a cheap long-context model is usually the right helper,
	// and paying coder-model rates for six of them at once is how a research run
	// stops being affordable on this audience's budget.
	AgentResearch AgentName = "research"
)

// Agent defines configuration for different LLM models and their token limits.
type Agent struct {
	Model           models.ModelID `json:"model"`
	MaxTokens       int64          `json:"maxTokens"`
	ReasoningEffort string         `json:"reasoningEffort"` // For openai models low,medium,heigh
}

// Provider defines configuration for an LLM provider.
type Provider struct {
	APIKey   string `json:"apiKey"`
	Disabled bool   `json:"disabled"`
}

// LocalEndpoint defines one OpenAI-compatible endpoint (NVIDIA NIM, Ollama,
// Kilo, LM Studio, or any custom /v1 server). GORILLA OVERRIDE: multiple may
// be configured and coexist — each contributes its /v1/models to the picker
// and routes with its own BaseURL + APIKey, instead of a single shared slot.
type LocalEndpoint struct {
	Name     string `json:"name"`
	BaseURL  string `json:"baseURL"`
	APIKey   string `json:"apiKey,omitempty"`
	Disabled bool   `json:"disabled,omitempty"`
}

// Data defines storage configuration.
type Data struct {
	Directory string `json:"directory,omitempty"`
}

// LSPConfig defines configuration for Language Server Protocol integration.
type LSPConfig struct {
	Disabled bool     `json:"enabled"`
	Command  string   `json:"command"`
	Args     []string `json:"args"`
	Options  any      `json:"options"`
}

// TUIConfig defines the configuration for the Terminal User Interface.
type TUIConfig struct {
	Theme string `json:"theme,omitempty"`
}

// ShellConfig defines the configuration for the shell used by the bash tool.
type ShellConfig struct {
	Path string   `json:"path,omitempty"`
	Args []string `json:"args,omitempty"`
}

// Config is the main configuration structure for the application.
type Config struct {
	Data       Data   `json:"data"`
	WorkingDir string `json:"wd,omitempty"`
	// GORILLA OVERRIDE: extra workspace roots added with /add-dir. WorkingDir
	// stays the PRIMARY root — relative paths still resolve against it, so no
	// tool call site changes. These extend context-file loading, permission
	// scoping, the env block and LSP watching. See internal/config/roots.go.
	AdditionalDirs []string `json:"additionalDirs,omitempty"`
	// GORILLA OVERRIDE: suppresses the startup workspace picker. Stored as the
	// negative because omitempty drops a false, so an "ask" bool could never be
	// persisted as off. See PeekStartupWorkspace.
	SkipWorkspacePrompt bool `json:"skipWorkspacePrompt,omitempty"`
	// GORILLA OVERRIDE: per-feature switches for the optional behaviours that
	// show the agent's working — see extras.go. A map rather than named bools so
	// an explicit false survives: omitempty drops a false field, but a false
	// VALUE inside a present map is preserved, which is what lets "I turned this
	// off deliberately" be distinguished from "never chose".
	Extras map[string]bool `json:"extras,omitempty"`
	// ExtrasChoiceMade records that the cost explanation has been shown and
	// answered, so it is asked exactly once rather than every launch.
	ExtrasChoiceMade bool `json:"extrasChoiceMade,omitempty"`
	// GORILLA OVERRIDE: report mouse events to the program. OFF by default because
	// enabling it takes drag-to-select away from the terminal. See extras.go.
	// Only consulted when AlternateScreen is on: without the alternate screen the
	// terminal handles the wheel itself, so asking for mouse events would take a
	// working scroll away and give nothing back.
	MouseWheel bool `json:"mouseWheel,omitempty"`
	// GORILLA OVERRIDE: draw the full interface on the terminal's alternate screen.
	// OFF by default, which is what makes the conversation land in your terminal's
	// own scrollback where the wheel, Select-All and Ctrl+Shift+C all work. See
	// AlternateScreenEnabled in extras.go for the measurements behind that default.
	//
	// omitempty is safe here precisely because the default is false: an absent key
	// and an explicit false mean the same thing, so nothing is lost by dropping it.
	AlternateScreen bool `json:"alternateScreen,omitempty"`
	// GORILLA OVERRIDE: which interface to start. "full" (default) or "plain".
	//
	// This is a persisted SETTING and not only a flag because the desktop entry
	// runs `gorilla-opencode launch` with no arguments — clicking the icon is how
	// nearly everyone starts this program, and a capability reachable only by
	// typing --plain is a capability most users do not have. Same lesson as the
	// working directory defaulting to $HOME on an icon launch.
	Interface string `json:"interface,omitempty"`
	// GORILLA OVERRIDE: base URL of a self-hosted SearXNG, e.g.
	// "http://localhost:8888". Empty means general web search is OFF and
	// web_search says so rather than guessing — see internal/llm/tools/websearch.go.
	//
	// Self-hosting is not a preference here, it is the only remaining option:
	// as of 2026-08 Google's Custom Search JSON API is closed to new customers
	// (sunset 2027-01-01), Microsoft retired the Bing Search API, Brave requires
	// a card even on its free credit, and Mojeek has no self-serve tier. SearXNG
	// is what is left that needs no key and no account.
	//
	// The explicit mapstructure tag is not decoration: viper reads THESE, not
	// json tags, and a field it cannot match becomes write-only. That is exactly
	// how WorkingDir's `json:"wd"` silently never loaded back.
	SearxNGURL string `json:"searxngURL,omitempty" mapstructure:"searxngURL"`
	// GORILLA OVERRIDE: the user's personal model shortlist, shown above every
	// provider in the picker. Model IDs in the order they were added — see
	// bookmarks.go for why IDs rather than names or list positions.
	BookmarkedModels []string                          `json:"bookmarkedModels,omitempty" mapstructure:"bookmarkedModels"`
	MCPServers       map[string]MCPServer              `json:"mcpServers,omitempty"`
	Providers        map[models.ModelProvider]Provider `json:"providers,omitempty"`
	LocalEndpoints   []LocalEndpoint                   `json:"localEndpoints,omitempty"`
	LSP              map[string]LSPConfig              `json:"lsp,omitempty"`
	Agents           map[AgentName]Agent               `json:"agents,omitempty"`
	Debug            bool                              `json:"debug,omitempty"`
	DebugLSP         bool                              `json:"debugLSP,omitempty"`
	ContextPaths     []string                          `json:"contextPaths,omitempty"`
	TUI              TUIConfig                         `json:"tui"`
	Shell            ShellConfig                       `json:"shell,omitempty"`
	AutoCompact      bool                              `json:"autoCompact,omitempty"`
}

// Application constants
const (
	defaultLogLevel = "info"
	appName         = "gorilla-opencode"

	MaxTokensFallbackDefault = 4096
)

var defaultContextPaths = []string{
	// GORILLA OVERRIDE (2026-08-19): AGENTS.md was missing entirely, and it is
	// ordered FIRST — ahead of .cursorrules — so the open standard wins when a
	// repository carries both. It is gated: see AutoLoadProjectInstructions,
	// which refuses it for repositories that are not the user's, and
	// prompt.processContextPaths, which never reads it from an /add-dir root.
	AgentsFile,
	".github/copilot-instructions.md",
	".cursorrules",
	".cursor/rules/",
	"CLAUDE.md",
	"CLAUDE.local.md",
	"opencode.md",
	"opencode.local.md",
	"OpenCode.md",
	"OpenCode.local.md",
	"OPENCODE.md",
	"OPENCODE.local.md",
}

// Global configuration instance
var cfg *Config

// Load initializes the configuration from environment variables and config files.
// If debug is true, debug mode is enabled and log level is set to debug.
// It returns an error if configuration loading fails.
func Load(workingDir string, debug bool) (*Config, error) {
	if cfg != nil {
		return cfg, nil
	}

	cfg = &Config{
		WorkingDir: workingDir,
		MCPServers: make(map[string]MCPServer),
		Providers:  make(map[models.ModelProvider]Provider),
		LSP:        make(map[string]LSPConfig),
	}

	// GORILLA OVERRIDE: load ~/.config/gorilla-opencode/env before anything
	// reads the environment. It used to be parsed only by the hidden `launch`
	// subcommand the desktop entry runs, so keys in that file were invisible
	// to a terminal user and providers were silently disabled. Doing it here
	// makes the desktop icon and the shell behave identically. Variables
	// already exported win — see ParseEnvFile.
	applyEnvFile()

	configureViper()
	setDefaults(debug)

	// Read global config
	if err := readConfig(viper.ReadInConfig()); err != nil {
		return cfg, err
	}

	// Load and merge local config
	mergeLocalConfig(workingDir)

	// GORILLA OVERRIDE (2026-08-21): apply the cached provider catalogues —
	// Groq, Cerebras, Anthropic, OpenAI, xAI, DeepSeek — before the default-model
	// ladder runs, because that ladder now asks the registry which models a
	// provider actually has rather than naming a constant that can go stale.
	//
	// Disk only, never the network: a slow link must not delay launch. The
	// network fetch happens on connect and on /update.
	//
	// Deliberately silent. configureLogging() has not run yet at this point, so
	// slog's built-in default handler is still in force and it writes to STDERR
	// — which the TUI then paints over with no way to clear it. Same trap as the
	// duplicate-endpoint warnings; see the comment on configureLogging below.
	models.LoadCachedCatalogues(CacheBase())

	setProviderDefaults()

	// Apply configuration to the struct
	if err := viper.Unmarshal(cfg); err != nil {
		return cfg, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	// GORILLA OVERRIDE: an empty apiKey in config.json must not shadow a key
	// present in the environment. The /connect dialog can persist empty-key
	// provider entries when toggling enable/disable; without this, that empty
	// value would disable a provider whose real key lives in the env file.
	backfillProviderKeysFromEnv()

	// GORILLA OVERRIDE (2026-09-01): drop credentials that cannot legally be
	// sent, BEFORE anything decides whether a provider is configured.
	//
	// Readiness is tested all over the codebase as `p.APIKey != ""`, so a key
	// made of NUL bytes counted as "ready": the provider showed a green marker
	// in the portal, was offered in the picker, was selected — and then every
	// single request failed inside net/http with `invalid header field value`.
	// Sanitising here means one answer to "is this provider usable", given
	// before anyone asks.
	sanitiseProviderKeys()

	applyDefaultValues()

	// GORILLA OVERRIDE: point the logger at its file BEFORE any of the steps
	// below can log. This used to sit ~50 lines further down, after
	// registerLocalEndpoints, so every warning those steps emitted went to
	// slog's built-in default handler — which writes to STDERR. The duplicate
	// local-endpoint warnings therefore printed straight onto the user's
	// terminal on every launch, and because the TUI takes the screen a moment
	// later, that text is painted over the frame with no record in the
	// renderer: no redraw can ever clear it. Same class of bug as the /login
	// URL, different mechanism — a log line, not a fmt.Print.
	if err := configureLogging(); err != nil {
		return cfg, err
	}

	// GORILLA OVERRIDE: register every configured OpenAI-compatible local
	// endpoint (NIM, Ollama, ...) so their models coexist in the picker.
	registerLocalEndpoints()

	// GORILLA OVERRIDE: one switchable /context row per configured language
	// server, so clangd/gopls/rust-analyzer can be turned off individually
	// rather than only in bulk. Must run after the LSP map is unmarshalled.
	registerLSPLoadoutRows()

	// GORILLA OVERRIDE: let fileutil's ripgrep search every workspace root so
	// @-file completion spans /add-dir roots. Inverted dependency — fileutil is
	// the lower layer and must not import config.
	fileutil.SetWorkspaceRootsFn(Roots)

	// Validate configuration
	if err := Validate(); err != nil {
		return cfg, fmt.Errorf("config validation failed: %w", err)
	}

	if cfg.Agents == nil {
		cfg.Agents = make(map[AgentName]Agent)
	}

	// Override the max tokens for title agent
	cfg.Agents[AgentTitle] = Agent{
		Model:     cfg.Agents[AgentTitle].Model,
		MaxTokens: 80,
	}
	return cfg, nil
}

// GorillaConfigFile is the single, clearly-labelled home for all of this
// app's config: ~/.config/gorilla-opencode/config.json (or under
// $XDG_CONFIG_HOME). GORILLA OVERRIDE: everything lives in ONE folder
// named "gorilla-opencode" — keys (env), token loadout (loadout.json),
// and the main config (config.json) — instead of scattered dotfiles
// sharing the generic "opencode" name with other tools.
func GorillaConfigFile() string {
	base := gorillaConfigBase()
	return filepath.Join(base, "config.json")
}

// gorillaConfigBase is retained as the in-package name for ConfigBase(); the
// body moved to store.go, which is now the single owner of this directory. It
// used to be duplicated byte-for-byte as loadoutConfigBase() in loadout.go.
func gorillaConfigBase() string { return ConfigBase() }

// configureViper sets up viper's configuration paths and environment variables.
func configureViper() {
	// Clean-break build: the only global config is the branded XDG path.
	viper.SetConfigFile(GorillaConfigFile())
	viper.SetConfigType("json")
	viper.SetEnvPrefix("GORILLA_OPENCODE")
	viper.AutomaticEnv()
}

// setDefaults configures default values for configuration options.
func setDefaults(debug bool) {
	viper.SetDefault("data.directory", DataBase())
	viper.SetDefault("contextPaths", defaultContextPaths)
	viper.SetDefault("tui.theme", "opencode")
	viper.SetDefault("autoCompact", true)

	// GORILLA OVERRIDE (2026-09-01): do NOT default shell.path here.
	//
	// This block used to set shell.path to $SHELL (or /bin/bash) and shell.args
	// to {"-l"} on every platform, unconditionally. Because viper defaults are
	// indistinguishable from a real setting, cfg.Shell.Path was NEVER empty —
	// which meant the shell package's `if shellPath == "" { detect() }` branch
	// was dead code in the running application. It only ever executed in unit
	// tests, where config is nil.
	//
	// On Windows the consequence was total: the bash tool tried to start
	// "/bin/bash" with "-l", that path does not exist, newPersistentShell
	// returned nil, and EVERY bash call in the session failed with "Shell could
	// not be started" — 10 of 10 recorded calls in the local session database,
	// plus three nil-dereference panics. The carefully written PowerShell
	// detection and wrapper downstream of it were never reached once.
	//
	// Leaving these unset lets shell.resolveShellPath() do the platform-aware
	// detection it was written for, while an explicit "shell.path" in config.json
	// still wins — which is the only behaviour anyone actually relied on.
	if os.Getenv("SHELL") != "" && runtime.GOOS != "windows" {
		viper.SetDefault("shell.path", os.Getenv("SHELL"))
		viper.SetDefault("shell.args", []string{"-l"})
	}

	if debug {
		viper.SetDefault("debug", true)
		viper.Set("log.level", "debug")
	} else {
		viper.SetDefault("debug", false)
		viper.SetDefault("log.level", defaultLogLevel)
	}
}

// setProviderDefaults configures LLM provider defaults based on provider provided by
// environment variables and configuration file.
func setProviderDefaults() {
	// Set all API keys we can find in the environment
	// Note: Viper does not default if the json apiKey is ""
	if apiKey := os.Getenv("ANTHROPIC_API_KEY"); apiKey != "" {
		viper.SetDefault("providers.anthropic.apiKey", apiKey)
	}
	if apiKey := os.Getenv("OPENAI_API_KEY"); apiKey != "" {
		viper.SetDefault("providers.openai.apiKey", apiKey)
	}
	if apiKey := os.Getenv("GEMINI_API_KEY"); apiKey != "" {
		viper.SetDefault("providers.gemini.apiKey", apiKey)
	}
	if apiKey := os.Getenv("GROQ_API_KEY"); apiKey != "" {
		viper.SetDefault("providers.groq.apiKey", apiKey)
	}
	if apiKey := os.Getenv("CEREBRAS_API_KEY"); apiKey != "" {
		viper.SetDefault("providers.cerebras.apiKey", apiKey)
	}
	// GORILLA OVERRIDE: enable the Gemini "Login with Google" provider when
	// OAuth credentials exist on disk (from `gorilla-opencode login`). It
	// has no API key — the placeholder just clears the "apiKey == '' means
	// disabled" gate in Validate(); the real auth is the stored token.
	if creds, _ := auth.LoadGeminiCreds(); creds != nil && creds.AccessToken != "" {
		viper.SetDefault("providers.gemini-oauth.apiKey", "oauth-login")
	}
	// GORILLA OVERRIDE: same for the Antigravity free tier (Claude/GPT-OSS/
	// Gemini) — enable the provider when its OAuth credentials exist on disk.
	// The placeholder just clears the "apiKey == '' means disabled" gate; the
	// real auth is the stored token. See internal/auth/antigravity_oauth.go.
	if creds, _ := auth.LoadAntigravityCreds(); creds != nil && creds.AccessToken != "" {
		viper.SetDefault("providers.antigravity.apiKey", "oauth-login")
	}
	// GORILLA OVERRIDE: same again for a ChatGPT sign-in. Without this the
	// provider is disabled on every start AFTER the login, so the models would
	// appear in the picker once and then silently revert on the next launch.
	// See internal/auth/chatgpt_oauth.go.
	if creds, _ := auth.LoadChatGPTCreds(); creds != nil && creds.AccessToken != "" {
		viper.SetDefault("providers.chatgpt.apiKey", "oauth-login")
	}
	if apiKey := os.Getenv("OPENROUTER_API_KEY"); apiKey != "" {
		viper.SetDefault("providers.openrouter.apiKey", apiKey)
	}
	if apiKey := os.Getenv("XAI_API_KEY"); apiKey != "" {
		viper.SetDefault("providers.xai.apiKey", apiKey)
	}

	// Default models, in the order someone is most likely to be able to USE
	// them: free-with-a-sign-in first, free-with-a-key next, paid last
	// (directive §8 — this audience mostly has no card).
	//
	// GORILLA OVERRIDE (2026-08-21): the fetched providers no longer name a
	// model constant here. They cannot: their lists come from the provider at
	// runtime, so there is no compile-time id to point at — which is the whole
	// reason a hardcoded default like Llama3_3_70BVersatile could sit in this
	// file for months after Groq shut the model down. Ask the registry instead;
	// if a provider has no cached list yet, it simply yields nothing and the
	// next candidate is tried.
	for _, cand := range []struct {
		key   string
		model models.ModelID
	}{
		{"providers.gemini.apiKey", models.Gemini36Flash},
		{"providers.groq.apiKey", models.PreferredCatalogueModel(models.ProviderGROQ)},
		{"providers.cerebras.apiKey", models.PreferredCatalogueModel(models.ProviderCerebras)},
		{"providers.openrouter.apiKey", models.OpenRouterNvidiaNemotron3Ultra550bA55bFree},
		{"providers.anthropic.apiKey", models.PreferredCatalogueModel(models.ProviderAnthropic)},
		{"providers.openai.apiKey", models.PreferredCatalogueModel(models.ProviderOpenAI)},
		{"providers.xai.apiKey", models.PreferredCatalogueModel(models.ProviderXAI)},
		{"providers.deepseek.apiKey", models.PreferredCatalogueModel(models.ProviderDeepSeek)},
	} {
		if strings.TrimSpace(viper.GetString(cand.key)) == "" || cand.model == "" {
			continue
		}
		setAgentDefaults(cand.model, cand.model)
		return
	}
}

// setAgentDefaults points the four agents at a model. Helpers get the cheaper
// one where a provider has a curated split; passing the same id twice is the
// normal case for a fetched provider, where no such split is known.
func setAgentDefaults(main, helper models.ModelID) {
	viper.SetDefault("agents.coder.model", main)
	viper.SetDefault("agents.summarizer.model", main)
	viper.SetDefault("agents.task.model", helper)
	viper.SetDefault("agents.title.model", helper)
}

// readConfig handles the result of reading a configuration file.
func readConfig(err error) error {
	if err == nil {
		return nil
	}

	// It's okay if the config file doesn't exist
	if _, ok := err.(viper.ConfigFileNotFoundError); ok || os.IsNotExist(err) {
		return nil
	}

	return fmt.Errorf("failed to read config: %w", err)
}

// mergeLocalConfig loads and merges configuration from the local directory.
func mergeLocalConfig(workingDir string) {
	local := viper.New()
	local.SetConfigName(fmt.Sprintf(".%s", appName))
	local.SetConfigType("json")
	local.AddConfigPath(workingDir)

	// Merge local config if it exists
	if err := local.ReadInConfig(); err == nil {
		viper.MergeConfigMap(local.AllSettings())
	}
}

// applyDefaultValues sets default values for configuration fields that need processing.
func applyDefaultValues() {
	// Set default MCP type if not specified
	for k, v := range cfg.MCPServers {
		if v.Type == "" {
			v.Type = MCPStdio
			cfg.MCPServers[k] = v
		}
	}
}

// revertAgentToDefault swaps an agent onto a default model because its
// configured one is unusable, and says so on stderr.
//
// GORILLA OVERRIDE 2026-07-27: this used to be a bare logging.Warn at each
// call site. Nothing renders those on the non-interactive (-p) path, so a
// configured model could be silently replaced and the user would only see the
// downstream API error from a model they never chose — with no hint the swap
// had happened. Substituting someone's model is worth a line of output.
func revertAgentToDefault(name AgentName, agent Agent, reason string) error {
	logging.Warn("reverting agent to default model",
		"agent", name, "configured_model", agent.Model, "reason", reason)

	if !setDefaultModelForAgent(name) {
		return fmt.Errorf("no valid provider available for agent %s", name)
	}
	fmt.Fprintf(os.Stderr, "note: agent %q model %q is unusable (%s) — falling back to %q\n",
		name, agent.Model, reason, cfg.Agents[name].Model)
	logging.Info("set default model for agent", "agent", name, "model", cfg.Agents[name].Model)
	return nil
}

// validateAgent validates an agent's model ID and provider, ensuring they are
// supported, and clamps its max-tokens to the model's limits.
//
// Every path that swaps the model RETURNS immediately: setDefaultModelForAgent
// has already written a coherent Model+MaxTokens pair, and the checks below are
// written against `model` — the model that was just REJECTED. Falling through
// would clamp the fallback's max-tokens using the rejected model's context
// window. With today's numbers that is inert (both fallbacks and the common
// rejects have 1M windows, so the clamp never fires), but the arithmetic is
// wrong and becomes visible as soon as the two windows differ — e.g. a rejected
// openai model with a 1047576 window would clamp to 523788 on a 1M fallback.
func validateAgent(cfg *Config, name AgentName, agent Agent) error {
	// Check if model exists
	// TODO:	If a copilot model is specified, but model is not found,
	// 		 	it might be new model. The https://api.githubcopilot.com/models
	// 		 	endpoint should be queried to validate if the model is supported.
	// GORILLA OVERRIDE: translate ids retired by a rename before deciding the
	// model is unknown, so an older config.json keeps the model it asked for
	// instead of being silently dropped onto an unrelated default.
	if current, isLegacy := models.LegacyModelIDs[agent.Model]; isLegacy {
		logging.Info("migrating legacy model id",
			"agent", name, "from", agent.Model, "to", current)
		agent.Model = current
		updated := cfg.Agents[name]
		updated.Model = current
		cfg.Agents[name] = updated
	}

	model, modelExists := models.SupportedModels[agent.Model]
	if !modelExists {
		return revertAgentToDefault(name, agent, "unknown model")
	}

	// GORILLA OVERRIDE: a model served by a configured local endpoint (Ollama,
	// NIM, LM Studio) is reachable by definition — models.RegisterLocalEndpoint
	// only registers a model after successfully listing it from that endpoint,
	// and LocalRouteFor carries its baseURL and key. But ProviderLocal has no
	// entry in cfg.Providers and no *_API_KEY, so the checks below judged every
	// local model "not configured" and silently swapped the agent onto a cloud
	// model. With Ollama running and gemma3:270m registered, three agents were
	// reverted on every single startup and the user saw three "is unusable"
	// notes for a setup that was working.
	if model.Provider == models.ProviderLocal {
		if _, _, routed := models.LocalRouteFor(agent.Model); routed {
			return validateAgentMaxTokens(cfg, name, agent, model)
		}
	}

	// Check if provider for the model is configured
	provider := model.Provider
	providerCfg, providerExists := cfg.Providers[provider]

	if !providerExists {
		// Provider not configured, check if we have environment variables
		apiKey := getProviderAPIKey(provider)
		if apiKey == "" {
			return revertAgentToDefault(name, agent,
				fmt.Sprintf("provider %s is not configured", provider))
		} else {
			// Add provider with API key from environment
			cfg.Providers[provider] = Provider{
				APIKey: apiKey,
			}
			logging.Info("added provider from environment", "provider", provider)
		}
	} else if providerCfg.Disabled || providerCfg.APIKey == "" {
		// GORILLA FIX: an entry in cfg.Providers must not hide an env key.
		//
		// The !providerExists branch above already rescues a provider whose key
		// lives in the environment. This branch did not, so a provider that had
		// EVER been written to config — typically with disabled:true from the
		// "no API key" loop below — was reverted even when its *_API_KEY was
		// present and working. Having an entry was what broke it; a provider the
		// user had never touched worked fine.
		//
		// Observed 2026-08-05: GROQ_API_KEY, CEREBRAS_API_KEY and XAI_API_KEY all
		// set, all three shown as "(ready)" in the startup portal (it consults
		// AvailableViaEnv), and all four agents reverted to Gemini on selection
		// with "provider cerebras is disabled". The portal and the validator had
		// two different definitions of "configured".
		if apiKey := getProviderAPIKey(provider); apiKey != "" {
			providerCfg.APIKey = apiKey
			providerCfg.Disabled = false
			cfg.Providers[provider] = providerCfg
			logging.Info("provider key found in environment; clearing stale disabled flag",
				"provider", provider)
			return validateAgentMaxTokens(cfg, name, agent, model)
		}
		reason := fmt.Sprintf("provider %s is disabled", provider)
		if !providerCfg.Disabled {
			reason = fmt.Sprintf("provider %s has no API key", provider)
		}
		return revertAgentToDefault(name, agent, reason)
	}

	return validateAgentMaxTokens(cfg, name, agent, model)
}

// validateAgentMaxTokens clamps an agent's max-tokens and reasoning effort to
// what its model supports. Split out of validateAgent so the local-endpoint
// path can run the same checks without going through the provider-key logic
// that does not apply to a locally-served model. GORILLA OVERRIDE.
func validateAgentMaxTokens(cfg *Config, name AgentName, agent Agent, model models.Model) error {
	// Validate max tokens
	if agent.MaxTokens <= 0 {
		logging.Warn("invalid max tokens, setting to default",
			"agent", name,
			"model", agent.Model,
			"max_tokens", agent.MaxTokens)

		// Update the agent with default max tokens
		updatedAgent := cfg.Agents[name]
		if model.DefaultMaxTokens > 0 {
			updatedAgent.MaxTokens = model.DefaultMaxTokens
		} else {
			updatedAgent.MaxTokens = MaxTokensFallbackDefault
		}
		cfg.Agents[name] = updatedAgent
	} else if model.ContextWindow > 0 && agent.MaxTokens > model.ContextWindow/2 {
		// Ensure max tokens doesn't exceed half the context window (reasonable limit)
		logging.Warn("max tokens exceeds half the context window, adjusting",
			"agent", name,
			"model", agent.Model,
			"max_tokens", agent.MaxTokens,
			"context_window", model.ContextWindow)

		// Update the agent with adjusted max tokens
		updatedAgent := cfg.Agents[name]
		updatedAgent.MaxTokens = model.ContextWindow / 2
		cfg.Agents[name] = updatedAgent
	}

	// Validate reasoning effort for models that support reasoning
	// GORILLA OVERRIDE: operator-precedence bug — the original condition
	// parsed as (CanReason && OpenAI) || Local, forcing reasoning effort
	// onto every local-provider model whether it can reason or not.
	if model.CanReason && (model.Provider == models.ProviderOpenAI || model.Provider == models.ProviderLocal) {
		if agent.ReasoningEffort == "" {
			// Set default reasoning effort for models that support it
			logging.Info("setting default reasoning effort for model that supports reasoning",
				"agent", name,
				"model", agent.Model)

			// Update the agent with default reasoning effort
			updatedAgent := cfg.Agents[name]
			updatedAgent.ReasoningEffort = "medium"
			cfg.Agents[name] = updatedAgent
		} else {
			// Check if reasoning effort is valid (low, medium, high)
			effort := strings.ToLower(agent.ReasoningEffort)
			if effort != "low" && effort != "medium" && effort != "high" {
				logging.Warn("invalid reasoning effort, setting to medium",
					"agent", name,
					"model", agent.Model,
					"reasoning_effort", agent.ReasoningEffort)

				// Update the agent with valid reasoning effort
				updatedAgent := cfg.Agents[name]
				updatedAgent.ReasoningEffort = "medium"
				cfg.Agents[name] = updatedAgent
			}
		}
	} else if !model.CanReason && agent.ReasoningEffort != "" {
		// Model doesn't support reasoning but reasoning effort is set
		logging.Warn("model doesn't support reasoning but reasoning effort is set, ignoring",
			"agent", name,
			"model", agent.Model,
			"reasoning_effort", agent.ReasoningEffort)

		// Update the agent to remove reasoning effort
		updatedAgent := cfg.Agents[name]
		updatedAgent.ReasoningEffort = ""
		cfg.Agents[name] = updatedAgent
	}

	return nil
}

// Validate checks if the configuration is valid and applies defaults where needed.
// backfillProviderKeysFromEnv fills any provider whose config apiKey is empty
// with the matching environment variable, so an explicit "" in config.json
// does not shadow an env-provided key (which would wrongly disable it).
func backfillProviderKeysFromEnv() {
	if cfg.Providers == nil {
		return
	}
	envFor := map[models.ModelProvider]string{
		models.ProviderAnthropic:  "ANTHROPIC_API_KEY",
		models.ProviderOpenAI:     "OPENAI_API_KEY",
		models.ProviderGemini:     "GEMINI_API_KEY",
		models.ProviderGROQ:       "GROQ_API_KEY",
		models.ProviderCerebras:   "CEREBRAS_API_KEY",
		models.ProviderOpenRouter: "OPENROUTER_API_KEY",
		models.ProviderXAI:        "XAI_API_KEY",
		models.ProviderDeepSeek:   "DEEPSEEK_API_KEY",
	}
	for p, env := range envFor {
		pc, ok := cfg.Providers[p]
		if !ok || pc.APIKey != "" {
			continue
		}
		if v := os.Getenv(env); v != "" {
			pc.APIKey = v
			cfg.Providers[p] = pc
		}
	}
}

func Validate() error {
	if cfg == nil {
		return fmt.Errorf("config not loaded")
	}

	// Validate agent models
	for name, agent := range cfg.Agents {
		if err := validateAgent(cfg, name, agent); err != nil {
			return err
		}
	}

	// Validate providers
	for provider, providerCfg := range cfg.Providers {
		// GORILLA FIX: the key may be in the environment rather than in config.
		// Disabling on an empty config field alone is what wrote the stale
		// disabled:true that validateAgent then read as truth on the next launch.
		// Adopt the env key instead, and clear any flag left by an earlier run.
		if apiKey := getProviderAPIKey(provider); apiKey != "" {
			if providerCfg.APIKey == "" || providerCfg.Disabled {
				providerCfg.APIKey = apiKey
				providerCfg.Disabled = false
				cfg.Providers[provider] = providerCfg
			}
			continue
		}
		if providerCfg.APIKey == "" && !providerCfg.Disabled {
			fmt.Fprintf(os.Stderr, "note: provider %s has no API key, marking as disabled\n", provider)
			logging.Warn("provider has no API key, marking as disabled", "provider", provider)
			providerCfg.Disabled = true
			cfg.Providers[provider] = providerCfg
		}
	}

	// Validate LSP configurations
	for language, lspConfig := range cfg.LSP {
		if lspConfig.Command == "" && !lspConfig.Disabled {
			logging.Warn("LSP configuration has no command, marking as disabled", "language", language)
			lspConfig.Disabled = true
			cfg.LSP[language] = lspConfig
		}
	}

	return nil
}

// ProviderAPIKey returns the usable key for a provider: the one saved in
// config.json, falling back to the environment. Exported because listing a
// provider's models needs the same credential the chat requests use, and a
// caller re-deriving "which key applies" is how two answers to one question
// start to disagree.
//
// It returns a CREDENTIAL. Never log or print the result; the fingerprint
// helper below exists for anything user-facing.
func ProviderAPIKey(provider models.ModelProvider) string {
	if cfg != nil {
		if p, ok := cfg.Providers[provider]; ok {
			if k := SanitiseAPIKey(p.APIKey); k != "" {
				return k
			}
		}
	}
	return SanitiseAPIKey(getProviderAPIKey(provider))
}

// sanitiseProviderKeys rewrites every stored credential through SanitiseAPIKey,
// so an unusable one reads as absent everywhere rather than as configured-but-
// broken. Local endpoint keys go through it too: the same paste can corrupt them.
func sanitiseProviderKeys() {
	if cfg == nil {
		return
	}
	for name, p := range cfg.Providers {
		clean := SanitiseAPIKey(p.APIKey)
		if clean == p.APIKey {
			continue
		}
		if clean == "" && p.APIKey != "" {
			logging.Warn("ignoring unusable API key (contains control characters)", "provider", name)
		}
		p.APIKey = clean
		cfg.Providers[name] = p
	}
	for i, e := range cfg.LocalEndpoints {
		cfg.LocalEndpoints[i].APIKey = SanitiseAPIKey(e.APIKey)
	}
}

// SanitiseAPIKey strips whitespace and refuses a key containing control
// characters.
//
// GORILLA OVERRIDE (2026-09-01): found in a real config.json on Windows, where
// the gemini provider's key was stored as nine NUL bytes — JSON "" nine
// times over. A key like that is not merely wrong, it is
// unusable in a way that produces a baffling error a long way from the cause:
// Go's net/http refuses to send a header containing control bytes, so every
// request died with
//
//	net/http: invalid header field value for "X-Goog-Api-Key"
//
// which reads like a bug in the HTTP layer rather than "your saved key is
// garbage". A key of only control characters is treated as absent, so the
// provider falls back to the environment and, failing that, reports itself
// unconfigured — which is the truth and is actionable.
//
// Trailing whitespace is stripped rather than rejected: a key pasted from a
// browser or read from a file almost always carries a newline, that is the
// user's most likely single mistake, and silently working is the right
// behaviour there.
func SanitiseAPIKey(key string) string {
	key = strings.TrimSpace(key)
	if key == "" {
		return ""
	}
	for _, r := range key {
		// Anything below space (NUL, embedded newlines, tabs mid-key) or DEL
		// cannot legally travel in an HTTP header value.
		if r < 0x20 || r == 0x7F {
			return ""
		}
	}
	return key
}

// getProviderAPIKey gets the API key for a provider from environment variables
func getProviderAPIKey(provider models.ModelProvider) string {
	switch provider {
	case models.ProviderAnthropic:
		return os.Getenv("ANTHROPIC_API_KEY")
	case models.ProviderOpenAI:
		return os.Getenv("OPENAI_API_KEY")
	case models.ProviderGemini:
		return os.Getenv("GEMINI_API_KEY")
	case models.ProviderGROQ:
		return os.Getenv("GROQ_API_KEY")
	case models.ProviderCerebras:
		return os.Getenv("CEREBRAS_API_KEY")
	case models.ProviderOpenRouter:
		return os.Getenv("OPENROUTER_API_KEY")
	case models.ProviderDeepSeek:
		return os.Getenv("DEEPSEEK_API_KEY")
	}
	return ""
}

// ProviderKeyFingerprint says WHICH credential currently backs a provider
// without revealing it. People on free tiers rotate several keys to spread
// quota across them, and the model picker has to answer "which of my keys is
// this column billing right now" — but a credential must never appear in a
// frame, because picker frames get screenshotted into chats (directive:
// a length and a short prefix, never the value).
//
// Format: "sk-or-...#a3f2 (73 chars)". The prefix is the first six characters —
// vendor boilerplate shared by every key from that vendor. The hash is the
// first two bytes of SHA-256, which is what actually distinguishes two rotated
// keys, and is useless for recovering one. Empty string = no key-based
// credential (OAuth providers, nothing configured).
func ProviderKeyFingerprint(provider models.ModelProvider) string {
	key := ""
	if cfg != nil {
		if p, ok := cfg.Providers[provider]; ok {
			key = p.APIKey
		}
	}
	fromEnv := false
	if key == "" {
		key = getProviderAPIKey(provider)
		fromEnv = key != ""
	}
	switch key {
	case "":
		return ""
	case "aws-credentials-available", "vertex-ai-credentials-available":
		// Sentinels for ambient cloud auth, not credentials someone rotates.
		return "ambient cloud credentials"
	}
	sum := sha256.Sum256([]byte(key))
	prefix := ""
	if r := []rune(key); len(r) >= 20 {
		prefix = string(r[:6])
	}
	fp := fmt.Sprintf("%s...#%02x%02x (%d chars", prefix, sum[0], sum[1], len(key))
	if fromEnv {
		fp += ", from env"
	}
	return fp + ")"
}

// registerLSPLoadoutRows creates a /context row per configured language server.
// GORILLA OVERRIDE.
func registerLSPLoadoutRows() {
	if len(cfg.LSP) == 0 {
		return
	}
	disabled := make(map[string]bool, len(cfg.LSP))
	for name, l := range cfg.LSP {
		disabled[name] = l.Disabled
	}
	RegisterLSPComponents(disabled)
}

// LSPEnabled reports whether a configured language server should run. It ANDs
// the config's own Disabled flag with the /context loadout toggle, so either
// switch can turn a server off and neither silently overrides the other.
//
// GORILLA OVERRIDE: the gate consulted by internal/app/lsp.go before starting
// a client. Turning a row off in /context therefore prevents the process from
// starting at all on the next launch — the memory and CPU saving, not just a
// token one.
func LSPEnabled(name string) bool {
	if cfg == nil {
		return true
	}
	if l, ok := cfg.LSP[name]; ok && l.Disabled {
		return false
	}
	return LoadoutEnabled(LSPComponentID(name))
}

// EnabledLSPNames returns the sorted names of language servers that are both
// configured and enabled. Used by the prompt's LSP block so it can name the
// servers actually watching the user's edits.
func EnabledLSPNames() []string {
	if cfg == nil {
		return nil
	}
	var out []string
	for name := range cfg.LSP {
		if LSPEnabled(name) {
			out = append(out, name)
		}
	}
	sort.Strings(out)
	return out
}

// AvailableViaEnv returns providers whose API-key environment variable is
// currently set, regardless of whether they appear in cfg.Providers.
//
// GORILLA OVERRIDE: exists so the /model picker can show a provider the
// instant its env var is exported — no /connect → save round-trip required.
// getEnabledProviders unions this with cfg.Providers to build the tab list.
// Copilot / AWS Bedrock / VertexAI are intentionally omitted: they authenticate
// via richer credentials than a single env-var key, and getProviderAPIKey
// above already surfaces them through their own paths.
func AvailableViaEnv() []models.ModelProvider {
	candidates := []struct {
		provider models.ModelProvider
		envKey   string
	}{
		{models.ProviderAnthropic, "ANTHROPIC_API_KEY"},
		{models.ProviderOpenAI, "OPENAI_API_KEY"},
		{models.ProviderGemini, "GEMINI_API_KEY"},
		{models.ProviderGROQ, "GROQ_API_KEY"},
		{models.ProviderCerebras, "CEREBRAS_API_KEY"},
		{models.ProviderOpenRouter, "OPENROUTER_API_KEY"},
		{models.ProviderXAI, "XAI_API_KEY"},
	}
	var out []models.ModelProvider
	for _, c := range candidates {
		if os.Getenv(c.envKey) != "" {
			out = append(out, c.provider)
		}
	}
	return out
}

// setDefaultModelForAgent points ONE agent at a model, used when an agent has no
// entry of its own. Order is the same as setProviderDefaults: free sign-ins,
// then free keys, then paid.
//
// GORILLA OVERRIDE (2026-08-21): this used to be ~180 lines of near-identical
// blocks, one per provider, each naming a hardcoded model id. That shape is what
// let Groq's default sit at llama-3.3-70b-versatile long after Groq retired it —
// the id is only visible at the bottom of a block nobody reads. Fetched providers
// now resolve through the registry, and no candidate can name a model that is not
// actually registered.
func setDefaultModelForAgent(agent AgentName) bool {
	maxTokens := int64(5000)
	if agent == AgentTitle {
		maxTokens = 80
	}
	helper := agent == AgentTitle || agent == AgentTask || agent == AgentResearch

	// pick returns the model for this agent from a (main, helper) pair.
	pick := func(main, light models.ModelID) models.ModelID {
		if helper && light != "" {
			return light
		}
		return main
	}

	type candidate struct {
		available   func() bool
		main, light models.ModelID
	}
	envSet := func(name string) func() bool {
		return func() bool { return os.Getenv(name) != "" }
	}
	fetched := func(p models.ModelProvider, env string) candidate {
		return candidate{
			available: envSet(env),
			main:      models.PreferredCatalogueModel(p),
		}
	}

	for _, c := range []candidate{
		// Free, no card, signed in with a Google account.
		{
			available: func() bool {
				creds, _ := auth.LoadGeminiCreds()
				return creds != nil && creds.AccessToken != ""
			},
			// GORILLA OVERRIDE 2026-07-27: Flash, not Pro. Pro models resolve on
			// the free tier and then answer "you have exhausted your capacity",
			// so a Pro fallback fails on first use.
			main: models.GeminiCAFlash, light: models.GeminiCA31FlashLite,
		},
		// Free key, no card.
		{
			available: envSet("GEMINI_API_KEY"),
			// GORILLA OVERRIDE 2026-07-27: Flash, not Pro. Reaching for the
			// $1.25/$10-per-1M Pro alias to name a session spends real money on
			// work Flash handles.
			main: models.GeminiFlashLatest, light: models.GeminiFlashLiteLatest,
		},
		fetched(models.ProviderGROQ, "GROQ_API_KEY"),
		fetched(models.ProviderCerebras, "CEREBRAS_API_KEY"),
		{
			available: envSet("OPENROUTER_API_KEY"),
			main:      models.OpenRouterNvidiaNemotron3Ultra550bA55bFree,
			light:     models.OpenRouterOpenaiGptOss20bFree,
		},
		// Paid keys last.
		fetched(models.ProviderAnthropic, "ANTHROPIC_API_KEY"),
		fetched(models.ProviderOpenAI, "OPENAI_API_KEY"),
		fetched(models.ProviderXAI, "XAI_API_KEY"),
		fetched(models.ProviderDeepSeek, "DEEPSEEK_API_KEY"),
		// GORILLA FIX (2026-09-01): a locally served model, LAST.
		//
		// Every candidate above is gated on a cloud API key. A user whose only
		// models are local therefore had NO fallback at all, so when their
		// configured model went away validation could substitute nothing and the
		// program refused to start: "no valid provider available for agent
		// title". The owner hit precisely that by deleting a local model that
		// happened to be the configured one -- on a machine with a working model
		// server listening on loopback the whole time.
		//
		// Local models are registered by registerLocalEndpoints(), which Load()
		// calls before Validate(), so by the time this runs they are known.
		//
		// Placed last deliberately. Anyone holding a cloud key keeps exactly the
		// behaviour they had; this only rescues the case that used to be fatal.
		{
			available: models.HasLocalModels,
			main:      models.PreferredLocalModel(),
		},
	} {
		if !c.available() {
			continue
		}
		model := pick(c.main, c.light)
		if model == "" {
			// A fetched provider with no cached list yet. Not an error: the list
			// arrives on the next connect or /update. Try the next candidate
			// rather than pinning an agent to a model that does not exist.
			continue
		}
		reasoning := ""
		if info, ok := models.SupportedModels[model]; ok && info.CanReason {
			reasoning = "medium"
		}
		cfg.Agents[agent] = Agent{
			Model:           model,
			MaxTokens:       maxTokens,
			ReasoningEffort: reasoning,
		}
		return true
	}
	return false
}

func updateCfgFile(updateCfg func(config *Config)) error {
	if cfg == nil {
		return fmt.Errorf("config not loaded")
	}

	// Get the config file path
	configFile := viper.ConfigFileUsed()
	if configFile == "" {
		// GORILLA OVERRIDE: create the config in the unified
		// ~/.config/gorilla-opencode/config.json, not a home dotfile.
		configFile = GorillaConfigFile()
		if err := ensureConfigDir(ConfigBase()); err != nil {
			return err
		}
		logging.Info("config file not found, creating new one", "path", configFile)
		// Record the path with viper so later writes in this process resolve to
		// the same file through the branch above. See the read below for why
		// that matters.
		viper.SetConfigFile(configFile)
	}

	// GORILLA OVERRIDE: read whatever is on disk, whether or not viper found a
	// file at startup. This branch used to substitute a literal `{}` whenever
	// ConfigFileUsed() was empty — and it stays empty for the whole process when
	// no config.json existed at launch, because nothing re-runs ReadInConfig.
	// So on a fresh install EVERY write re-based from an empty document and
	// discarded the one before it: paste an API key in /connect, then add a local
	// endpoint, and the key was gone. Silent, and only on first-run configs,
	// which is why it survived. Found by a test asserting a removal persisted;
	// nothing was left on disk to remove from.
	var configData []byte
	switch data, err := os.ReadFile(configFile); {
	case err == nil:
		configData = data
	case os.IsNotExist(err):
		configData = []byte(`{}`)
	default:
		return fmt.Errorf("failed to read config file: %w", err)
	}

	// Parse the JSON
	var userCfg *Config
	if err := json.Unmarshal(configData, &userCfg); err != nil {
		return fmt.Errorf("failed to parse config file: %w", err)
	}

	updateCfg(userCfg)

	// Write the updated config back to file
	updatedData, err := json.MarshalIndent(userCfg, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	// GORILLA OVERRIDE: 0o600, not 0o644. This file holds provider API keys in
	// plain text, and every sidecar beside it (loadout.json, ratelimit.json,
	// subagents.json) was already 0o600 — the file with the secrets was the
	// loosest in the directory. writeSecretFile also chmods an existing file, so
	// a config.json left at 0o644 by an older version is tightened on first
	// write rather than staying world-readable forever.
	if err := writeSecretFile(configFile, updatedData); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	return nil
}

// configureLogging installs the process-wide slog handler.
//
// GORILLA OVERRIDE: extracted from Load and moved earlier. Until this runs,
// slog's default handler is in force and it writes to stderr — which the TUI is
// about to cover. Anything that logs before this point leaves text burned onto
// the user's screen for the rest of the session. Call it as early as cfg.Debug
// and cfg.Data.Directory are known, and never log above the call site.
func configureLogging() error {
	defaultLevel := slog.LevelInfo
	if cfg.Debug {
		defaultLevel = slog.LevelDebug
	}

	dest := io.Writer(logging.NewWriter())
	if os.Getenv("GORILLA_OPENCODE_DEV_DEBUG") == "true" {
		loggingFile := filepath.Join(StateBase(), "debug.log")
		messagesPath := filepath.Join(StateBase(), "messages")

		// if file does not exist create it
		if _, err := os.Stat(loggingFile); os.IsNotExist(err) {
			if err := os.MkdirAll(StateBase(), 0o755); err != nil {
				return fmt.Errorf("failed to create directory: %w", err)
			}
			if _, err := os.Create(loggingFile); err != nil {
				return fmt.Errorf("failed to create log file: %w", err)
			}
		}

		if _, err := os.Stat(messagesPath); os.IsNotExist(err) {
			if err := os.MkdirAll(messagesPath, 0o756); err != nil {
				return fmt.Errorf("failed to create directory: %w", err)
			}
		}
		logging.MessageDir = messagesPath

		f, err := os.OpenFile(loggingFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o666)
		if err != nil {
			return fmt.Errorf("failed to open log file: %w", err)
		}
		dest = f
	}

	slog.SetDefault(slog.New(slog.NewTextHandler(dest, &slog.HandlerOptions{
		Level: defaultLevel,
	})))
	return nil
}

// registerLocalEndpoints loads every enabled OpenAI-compatible local endpoint
// into the model registry so their models coexist. GORILLA OVERRIDE: falls
// back to the legacy single LOCAL_ENDPOINT env vars when none are configured,
// so existing setups keep working untouched.
func registerLocalEndpoints() {
	// Backward-compat: the legacy LOCAL_ENDPOINT env (NIM) is not stored in
	// config.json, so append it as a "nim" endpoint whenever it's set and not
	// already listed — regardless of what else is configured. (Previously this
	// only ran when the list was empty, so adding any endpoint via /connect
	// silently dropped NIM.)
	if ep := os.Getenv("LOCAL_ENDPOINT"); ep != "" {
		exists := false
		for _, le := range cfg.LocalEndpoints {
			if le.BaseURL == ep || le.Name == "nim" {
				exists = true
				break
			}
		}
		if !exists {
			cfg.LocalEndpoints = append(cfg.LocalEndpoints, LocalEndpoint{
				Name:    "nim",
				BaseURL: ep,
				APIKey:  os.Getenv("LOCAL_ENDPOINT_API_KEY"),
			})
		}
	}
	// GORILLA OVERRIDE: collapse endpoints that share a baseURL, preferring one
	// that carries a key.
	//
	// Two facts combine into a silent failure. NVIDIA NIM serves /v1/models
	// WITHOUT authentication, so listing succeeds for any entry aimed at that
	// URL — even one with no key or a malformed one — and registration looks
	// healthy. And every entry sharing a baseURL registers the SAME model ids,
	// each overwriting the previous route. So the last entry wins, and if its key
	// is wrong (a key pasted without its "nvapi-" prefix, say) all 102 models are
	// routed through it and every inference returns 401 — while the picker shows
	// a full, apparently working model list.
	seenURL := map[string]LocalEndpoint{}
	order := []string{}
	for _, ep := range cfg.LocalEndpoints {
		if ep.Disabled || ep.BaseURL == "" {
			continue
		}
		key := models.CanonicalEndpointURL(ep.BaseURL)
		prev, dup := seenURL[key]
		if !dup {
			seenURL[key] = ep
			order = append(order, key)
			continue
		}
		// Prefer a keyed entry; between two keyed entries keep the first, so
		// re-adding a connection cannot silently steal a working route.
		if prev.APIKey == "" && ep.APIKey != "" {
			seenURL[key] = ep
		}
		logging.Warn("ignoring duplicate local endpoint",
			"kept", seenURL[key].Name, "ignored", ep.Name, "baseURL", ep.BaseURL, "canonical", key)
	}

	// GORILLA OVERRIDE: apply a previously refreshed model catalogue over the
	// built-in one. Reads a local file only - never the network - so a slow or
	// absent connection cannot delay startup. Any problem with the file is
	// logged and the built-in list stands, because a corrupt cache must not
	// leave someone with no models at all.
	if n, err := models.LoadRefreshedCatalogue(CacheBase()); err != nil {
		logging.Warn("Could not apply refreshed model list", "error", err)
	} else if n > 0 {
		logging.Debug("Applied refreshed model list", "models", n)
	}
	// Same, for the Antigravity catalogue. Separate cache file and separate
	// call because they are separate upstreams with separate lifetimes — one
	// refresh must never invalidate the other.
	if n, err := models.LoadRefreshedAntigravity(CacheBase()); err != nil {
		logging.Warn("Could not apply refreshed Antigravity model list", "error", err)
	} else if n > 0 {
		logging.Debug("Applied refreshed Antigravity model list", "models", n)
	}
	// Same again for the ChatGPT sign-in list. Disk only, never the network:
	// launch must not wait on a listing, which is the same bargain the other two
	// make. A missing cache is the normal first-run state and leaves the
	// built-in list in chatgpt.go working.
	if n, err := models.LoadRefreshedChatGPT(CacheBase()); err != nil {
		logging.Warn("Could not apply refreshed ChatGPT model list", "error", err)
	} else if n > 0 {
		logging.Debug("Applied refreshed ChatGPT model list", "models", n)
	}

	var first models.ModelID
	for _, url := range order {
		ep := seenURL[url]
		if n, id := models.RegisterLocalEndpoint(ep.Name, ep.BaseURL, ep.APIKey); n > 0 && first == "" {
			first = id
		}
	}

	// GORILLA OVERRIDE (2026-09-01): if the user has configured no local endpoint
	// at all, look for the two that are worth looking for.
	//
	// Ollama and LM Studio are running on a great many developer machines and
	// serve an OpenAI-compatible API on a well-known port. Someone who has one
	// running and no cloud key configured would otherwise be told this program
	// has no models — while a perfectly good one is listening on loopback.
	//
	// Deliberately conditional on the list being EMPTY. Probing is never allowed
	// to interfere with, reorder, or duplicate a setup somebody made on purpose;
	// this is a first-run courtesy, not a background scanner. Nothing is written
	// to config.json, so it also cannot accumulate stale entries.
	if len(order) == 0 {
		for _, name := range models.DiscoverLocalRuntimes() {
			logging.Info("using a local model server found on its default port", "name", name)
		}
		if first == "" {
			for id := range models.SupportedModels {
				if models.SupportedModels[id].Provider == models.ProviderLocal {
					first = id
					break
				}
			}
		}
	}
	// Default the agents to a local model only if nothing else set one.
	if first != "" {
		if cfg.Agents == nil {
			cfg.Agents = make(map[AgentName]Agent)
		}
		// GORILLA OVERRIDE: AgentResearch is deliberately NOT in this list.
		//
		// `first` is the first LOCAL endpoint model. Defaulting the research
		// agent to it created an entry pointing at an unrelated provider: a
		// user signed in to Antigravity, chatting to Claude, was shown
		// "helpers run on Llama 3.3 70B" — and that was TRUE, because this
		// loop had pointed them at a local server. Reported 2026-08-14; the
		// reaction was, correctly, "what is this moron talking about".
		//
		// Research has no business having an opinion here. With no entry,
		// researchAgentName() falls back to AgentTask, so helpers run on the
		// same provider as everything else. Someone who wants them elsewhere
		// adds a "research" entry deliberately.
		for _, name := range []AgentName{AgentCoder, AgentSummarizer, AgentTask, AgentTitle} {
			if cfg.Agents[name].Model == "" {
				a := cfg.Agents[name]
				a.Model = first
				cfg.Agents[name] = a
			}
		}
	}
}

// UpsertProviderKey stores/updates a provider's API key (enabling it) and
// persists it to the config file. Used by the /connect dialog.
func UpsertProviderKey(provider models.ModelProvider, apiKey string) error {
	if cfg == nil {
		return fmt.Errorf("config not loaded")
	}
	set := func(c *Config) {
		if c.Providers == nil {
			c.Providers = make(map[models.ModelProvider]Provider)
		}
		p := c.Providers[provider]
		p.APIKey = apiKey
		p.Disabled = false
		c.Providers[provider] = p
	}
	set(cfg)
	return updateCfgFile(set)
}

// SetProviderDisabled toggles a keyed provider on/off and persists it.
func SetProviderDisabled(provider models.ModelProvider, disabled bool) error {
	if cfg == nil {
		return fmt.Errorf("config not loaded")
	}
	set := func(c *Config) {
		if c.Providers == nil {
			c.Providers = make(map[models.ModelProvider]Provider)
		}
		p := c.Providers[provider]
		p.Disabled = disabled
		c.Providers[provider] = p
	}
	set(cfg)
	return updateCfgFile(set)
}

// UpsertLocalEndpoint adds or updates a local endpoint (matched by Name) and
// persists it. Used by the /connect dialog.
func UpsertLocalEndpoint(ep LocalEndpoint) error {
	if cfg == nil {
		return fmt.Errorf("config not loaded")
	}
	apply := func(list []LocalEndpoint) []LocalEndpoint {
		for i := range list {
			if list[i].Name == ep.Name {
				list[i] = ep
				return list
			}
		}
		return append(list, ep)
	}
	cfg.LocalEndpoints = apply(cfg.LocalEndpoints)
	return updateCfgFile(func(c *Config) { c.LocalEndpoints = apply(c.LocalEndpoints) })
}

// RemoveLocalEndpoint deletes a local endpoint by name and persists the change.
//
// GORILLA OVERRIDE: /connect could add and disable endpoints but never remove
// one, so a fumbled paste left a permanent entry that only a hand-edit of
// config.json could clear. One user config accumulated four entries for the same
// NVIDIA URL — the same key four times, twice with its "nvapi-" prefix missing —
// and every launch logged two "ignoring duplicate local endpoint" warnings with
// no way to act on them from inside the app.
//
// Its models and route are dropped too, so the endpoint stops being selectable
// immediately rather than lingering until the next launch.
func RemoveLocalEndpoint(name string) error {
	if cfg == nil {
		return fmt.Errorf("config not loaded")
	}
	found := false
	apply := func(list []LocalEndpoint) []LocalEndpoint {
		kept := make([]LocalEndpoint, 0, len(list))
		for _, ep := range list {
			if ep.Name == name {
				found = true
				continue
			}
			kept = append(kept, ep)
		}
		return kept
	}
	cfg.LocalEndpoints = apply(cfg.LocalEndpoints)
	if !found {
		return fmt.Errorf("no local endpoint named %q", name)
	}
	// By NAME, not by baseURL: the duplicates being cleaned up share their URL
	// with the endpoint that is staying, and dropping by URL would unregister
	// that one's models too.
	models.UnregisterLocalEndpointByName(name)
	return updateCfgFile(func(c *Config) { c.LocalEndpoints = apply(c.LocalEndpoints) })
}

// NormaliseLocalAPIKey repairs the paste mistakes that produce a credential
// which lists models happily and 401s on every actual request.
//
// GORILLA OVERRIDE: NVIDIA keys carry an "nvapi-" prefix, and NIM serves
// /v1/models with NO authentication — so a key pasted without its prefix looks
// perfectly healthy (full model list, endpoint "connected") and fails only at
// inference. Selecting the prefix by double-click drops it, which is how one
// config ended up holding the same key twice with and twice without it. Repair
// it at the moment of entry and say so; a silent fix teaches nothing.
//
// Returns the cleaned key and a note for the user, empty when nothing was wrong.
func NormaliseLocalAPIKey(baseURL, key string) (string, string) {
	cleaned := strings.TrimSpace(key)
	note := ""
	if cleaned != key {
		note = "trimmed surrounding whitespace"
	}
	if cleaned == "" {
		return cleaned, note
	}

	// Only NVIDIA's endpoints have a known prefix to check against; guessing at
	// other providers' key shapes would do more harm than good.
	if strings.Contains(baseURL, "api.nvidia.com") && !strings.HasPrefix(cleaned, nvidiaKeyPrefix) {
		cleaned = nvidiaKeyPrefix + cleaned
		note = "added the missing \"" + nvidiaKeyPrefix + "\" prefix — without it NVIDIA lists models fine but returns 401 on every request"
	}
	return cleaned, note
}

// nvidiaKeyPrefix is the prefix every NVIDIA NIM API key carries.
const nvidiaKeyPrefix = "nvapi-"

// SetLocalEndpointDisabled toggles a local endpoint on/off and persists it.
func SetLocalEndpointDisabled(name string, disabled bool) error {
	if cfg == nil {
		return fmt.Errorf("config not loaded")
	}
	apply := func(list []LocalEndpoint) []LocalEndpoint {
		for i := range list {
			if list[i].Name == name {
				list[i].Disabled = disabled
			}
		}
		return list
	}
	cfg.LocalEndpoints = apply(cfg.LocalEndpoints)
	return updateCfgFile(func(c *Config) { c.LocalEndpoints = apply(c.LocalEndpoints) })
}

// Get returns the current configuration.
// It's safe to call this function multiple times.
func Get() *Config {
	return cfg
}

// WorkingDirectory returns the current working directory from the configuration.
func WorkingDirectory() string {
	if cfg == nil {
		panic("config not loaded")
	}
	return cfg.WorkingDir
}

func UpdateAgentModel(agentName AgentName, modelID models.ModelID) error {
	if cfg == nil {
		panic("config not loaded")
	}

	existingAgentCfg := cfg.Agents[agentName]

	model, ok := models.SupportedModels[modelID]
	if !ok {
		return fmt.Errorf("model %s not supported", modelID)
	}

	maxTokens := existingAgentCfg.MaxTokens
	if model.DefaultMaxTokens > 0 {
		maxTokens = model.DefaultMaxTokens
	}

	newAgentCfg := Agent{
		Model:           modelID,
		MaxTokens:       maxTokens,
		ReasoningEffort: existingAgentCfg.ReasoningEffort,
	}
	cfg.Agents[agentName] = newAgentCfg

	if err := validateAgent(cfg, agentName, newAgentCfg); err != nil {
		// revert config update on failure
		cfg.Agents[agentName] = existingAgentCfg
		return fmt.Errorf("failed to update agent model: %w", err)
	}

	return updateCfgFile(func(config *Config) {
		if config.Agents == nil {
			config.Agents = make(map[AgentName]Agent)
		}
		config.Agents[agentName] = newAgentCfg
	})
}

// UpdateTheme updates the theme in the configuration and writes it to the config file.
func UpdateTheme(themeName string) error {
	if cfg == nil {
		return fmt.Errorf("config not loaded")
	}

	// Update the in-memory config
	cfg.TUI.Theme = themeName

	// Update the file config
	return updateCfgFile(func(config *Config) {
		config.TUI.Theme = themeName
	})
}

// GORILLA CULL (2026-08-21): LoadGitHubToken and the Copilot credential probe
// went with the Copilot provider. The Copilot token lives in a GitHub-hosted
// config that only the Copilot client could use, and there is no Copilot client
// any more. Kept in the quarantine copy of copilot.go if it is ever wanted back.

// FollowCoderModel moves the helper agents (summarizer, task, title) onto
// newModel, but ONLY those that were still sitting on prevCoder — i.e. the ones
// that were following the coder rather than deliberately set to something else.
// Returns how many were moved.
//
// GORILLA FIX: switching the coder model used to strand the helpers.
//
// The every-launch portal sets all four agents (applyAgentModels), but the
// /models dialog set ONLY the coder. So picking a new model left summarizer,
// task and title pointing at the old one — and if that old model had become
// unusable, the sole visible symptom was a recurring "failed to generate title"
// in the status bar, while summarisation and sub-agents were primed to fail
// later, silently, at whatever moment the context finally filled.
//
// Observed 2026-08-05: coder on google/diffusiongemma-26b-a4b-it while all
// three helpers were still on 01-ai/yi-large, a model the account cannot run
// (HTTP 404). Config-level validation cannot catch this: yi-large is a
// perfectly well-formed, registered model — it is only the PROVIDER that
// refuses it, which nothing local can know.
//
// Deliberately conditional on prevCoder rather than overwriting all three: a
// cheap fast model for titles is a legitimate, common choice and must survive
// a coder switch. Only agents that were shadowing the coder keep shadowing it.
//
// GORILLA FIX 2026-08-14: the shadowing rule alone LEFT AGENTS ON ANOTHER
// PROVIDER, which is a second, worse bug than the one above.
//
// The Antigravity portal deliberately splits the agents at login — coder on
// Claude, background agents on Gemini Flash (cmd/provider_portal.go). That
// split is good: the Gemini pool is billed separately, so it roughly doubles a
// free-tier allowance. But it means a fresh install has helpers that do NOT
// equal the coder, and the shadowing rule reads that as "deliberately set,
// leave it". Switch the coder to a local model and four agents stay wired to
// Antigravity — a different account, quietly drawing quota the user believes
// they walked away from. Nothing in the UI said so; it was reported as
// "lots of sob stories could arise from misunderstandings like these".
//
// So there are now TWO reasons to move a helper, and either is sufficient:
//
//	shadowing — it was on prevCoder, so it was following the coder (the
//	            2026-08-05 stranding bug above)
//	stranded  — it is on a DIFFERENT PROVIDER from the new coder, so it bills
//	            a different account
//
// A helper is left alone only when it is both deliberately different AND on the
// coder's own provider — same account, same pot, no surprise. That is the case
// the split exists for, and it survives untouched.
//
// Note the asymmetry: a same-provider difference is housekeeping, a
// cross-provider difference is somebody else's bill. Only the second is a
// surprise worth overriding a user's choice for.
// AgentModelMove records one background agent being moved, so the user can be
// shown exactly what changed and put it back.
type AgentModelMove struct {
	Agent AgentName
	From  models.ModelID
	To    models.ModelID
}

// GORILLA FIX 2026-08-14, third revision. The rule is now: BACKGROUND AGENTS
// ALWAYS FOLLOW THE CODER — and the user is told, in detail, and can undo it.
//
// Revision 1 (upstream) moved only agents sitting on the previous coder model.
// Revision 2 added "or on a different provider", to stop a switched-away
// account quietly drawing quota. Both reasoned about MONEY, and both were blind
// to the thing that actually broke:
//
// A user on Antigravity switched to Claude Opus 4.6 (Thinking). Opus and Gemini
// Flash share a provider, so no rule fired, and research kept running on Flash.
// Same bill, much worse answer, nothing on screen saying so. Nobody selects
// Opus to save money.
//
// So CAPABILITY wins the default, and the price of that default is made visible
// instead of assumed away: every move is recorded and handed back, the caller
// shows what it means for money and quota, and RevertAgentModels puts it back
// exactly. Automatic, never silent, always reversible.
//
// This DOES drag the title agent onto an expensive model, which is real waste —
// naming a session does not need Opus. That is exactly why the moves are
// itemised for the user rather than summarised into a count.
func FollowCoderModel(prevCoder, newModel models.ModelID) ([]AgentModelMove, error) {
	if cfg == nil {
		panic("config not loaded")
	}
	if prevCoder == "" || prevCoder == newModel {
		return nil, nil
	}
	var moves []AgentModelMove
	for _, name := range []AgentName{AgentSummarizer, AgentTask, AgentTitle, AgentResearch} {
		agentCfg, ok := cfg.Agents[name]
		if !ok || agentCfg.Model == newModel {
			continue // absent, or already where it needs to be
		}
		from := agentCfg.Model
		if err := UpdateAgentModel(name, newModel); err != nil {
			return moves, err
		}
		moves = append(moves, AgentModelMove{Agent: name, From: from, To: newModel})
	}
	return moves, nil
}

// RevertAgentModels undoes a set of moves exactly, restoring each agent to the
// model it held before.
//
// Exact restoration, never a re-derivation. The previous state may have been a
// deliberate hand-edit, a provider portal default, or a cheap model chosen for
// one agent on purpose. Guessing at it is how a user's choice gets quietly
// replaced with our idea of a sensible one.
func RevertAgentModels(moves []AgentModelMove) error {
	if cfg == nil {
		return fmt.Errorf("config not loaded")
	}
	for _, mv := range moves {
		if err := UpdateAgentModel(mv.Agent, mv.From); err != nil {
			return fmt.Errorf("restoring %s to %s: %w", mv.Agent, mv.From, err)
		}
	}
	return nil
}

// BackgroundModelForProvider reports a provider's designated cheap background
// model, if it has one. Retained after the always-follow change because the
// FACT it encodes is still true and is now used to WARN rather than to decide:
// Antigravity bills its Gemini pool separately from Claude/GPT, so dragging the
// background agents off Gemini forfeits a second, separate free allowance. The
// user should be told that when it happens.
func BackgroundModelForProvider(p models.ModelProvider) (models.ModelID, bool) {
	id, ok := backgroundModelByProvider[p]
	if !ok {
		return "", false
	}
	if _, registered := models.SupportedModels[id]; !registered {
		return "", false
	}
	return id, true
}

// SUPERSEDED 2026-08-14 by the always-follow rule above. Kept, not deleted:
// it records the two earlier rules and exactly why each was insufficient, and
// its tests still document the failure modes (a stranded helper on a dead
// model; a switched-away account still drawing quota). Do not re-wire it into
// FollowCoderModel — both rules were blind to capability, which is the reason
// the current one exists.
//
// helperMustMove is the whole rule, kept pure so it can be tested across every
// provider combination without touching config or credentials.
//
// Move when EITHER:
//
//	shadowing — the helper was on prevCoder, so it was following the coder and
//	            would otherwise be stranded on a model the account may not run
//	stranded  — the helper is on a different PROVIDER from the new coder, so it
//	            draws a different account's quota after the user has moved on
//
// Stay when it is both deliberately different AND on the coder's own provider.
// Same account, same pot: housekeeping, not a surprise bill, and not ours to
// override.
func helperMustMove(helperModel, prevCoder, newCoder models.ModelID) bool {
	if helperModel == prevCoder {
		return true // shadowing
	}
	newProvider, newKnown := providerOf(newCoder)
	helperProvider, helperKnown := providerOf(helperModel)
	if !newKnown || !helperKnown {
		// An unknown model is left alone rather than guessed at: we cannot show
		// it bills elsewhere, and moving it on a hunch discards a deliberate
		// choice for no demonstrated reason.
		return false
	}
	return helperProvider != newProvider
}

// backgroundModelByProvider names the model a provider's BACKGROUND agents
// (summarizer, task, title, research) should land on when they have to move.
//
// GORILLA OVERRIDE: Antigravity is the reason this exists. Its Gemini pool is
// billed separately from its Claude/GPT pool, so putting background work on
// Gemini Flash leaves the Claude quota for the work the user actually watches —
// on a free tier that is close to twice the usable allowance. The login portal
// applies exactly this split; without an entry here the split would be lost the
// first time the user switched models and came back.
//
// A provider with no entry gets the coder's own model. That is deliberately
// dumb: picking a "cheap equivalent" by inspecting the catalogue is the
// heuristic that once offered an embedding model as a chat model. A curated
// constant per provider, or nothing.
var backgroundModelByProvider = map[models.ModelProvider]models.ModelID{
	models.ProviderAntigravity: models.AGGemini36Flash,
	// A ChatGPT plan has ONE pool, so this is not a separate-quota split like
	// Antigravity's: it is simply the cheaper model against the same limit.
	// On a free plan the cooldown is the scarce resource, and spending GPT-5.5
	// on conversation titles brings it forward for no benefit the user sees.
	models.ProviderChatGPT: models.ChatGPT54Mini,
}

// helperTargetFor returns the model a background agent should move to when the
// coder becomes newCoder.
func helperTargetFor(newCoder models.ModelID) models.ModelID {
	m, ok := models.SupportedModels[newCoder]
	if !ok {
		return newCoder
	}
	bg, ok := backgroundModelByProvider[m.Provider]
	if !ok {
		return newCoder
	}
	if _, registered := models.SupportedModels[bg]; !registered {
		return newCoder // not in this build's catalogue
	}
	return bg
}

// providerOf reports a model's provider, and whether the model is known at all.
func providerOf(id models.ModelID) (models.ModelProvider, bool) {
	m, ok := models.SupportedModels[id]
	return m.Provider, ok
}

// ReregisterLocalEndpoints re-asks every enabled OpenAI-compatible endpoint what
// it serves right now, and reports how many endpoints answered.
//
// GORILLA OVERRIDE (2026-08-20): exported for /update. It reuses
// registerLocalEndpoints rather than iterating cfg.LocalEndpoints from the TUI,
// because that function carries the dedupe-by-baseURL rule that prefers a keyed
// entry — reimplementing it at the call site would drop the protection against
// two entries for one URL, where the last one wins and a wrong key silently
// routes every model through a 401.
func ReregisterLocalEndpoints() int {
	if cfg == nil {
		return 0
	}
	n := 0
	for _, ep := range cfg.LocalEndpoints {
		if !ep.Disabled && ep.BaseURL != "" {
			n++
		}
	}
	registerLocalEndpoints()
	return n
}
