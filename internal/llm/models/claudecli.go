// GORILLA OVERRIDE: this file did not exist upstream. It registers the Claude
// models reachable by driving the user's own, already signed-in `claude` binary
// as a subprocess, rather than by calling an HTTP API with a key.
//
// # WHY A SUBPROCESS AND NOT AN HTTP CLIENT
//
// A Claude Pro or Max subscription is not an API credential. Anthropic's Claude
// Code legal terms say so in as many words: OAuth sign-in "is intended
// exclusively for purchasers of Claude Free, Pro, Max, Team, and Enterprise
// subscription plans and is designed to support ordinary use of Claude Code and
// other native Anthropic applications", and third-party developers may not
// "offer Claude.ai login into their own applications, or to route requests
// through Free, Pro, or Max plan credentials on behalf of their users", nor
// "collect, store, or intermediate Claude.ai credentials or session tokens".
//
// Every third-party tool that reached a subscription anyway did it by sending
// Claude Code's OAuth client id and claiming, on the wire, to be Claude Code.
// Anthropic shipped server-side blocks for exactly that on 9 January 2026.
// OpenCode, the project this one is forked from, was one of the tools broken by
// it. Building that here would mean writing the impersonation and then watching
// it fail on the next header check.
//
// The same terms permit the thing we actually do: "Nor does it prevent an end
// user from signing in to the unmodified Claude Code binary with their own
// Claude subscription." So the user runs `claude login` once, the CLI owns its
// own token, and this program spawns it.
//
// The security property that falls out of this is worth more than the legal one:
// THIS PROGRAM NEVER SEES A CREDENTIAL. There is no token in config.json, no
// refresh loop, nothing to leak into a log or a bug report, and nothing for
// directive §7 to redact. The provider is configured by the presence of a
// working binary, not by a secret.
//
// # WHY THE ALIAS IS THE APIModel AND NOT THE RESOLVED ID
//
// `claude --model` takes either an alias (fable, opus, sonnet, haiku) or a full
// model name. We pass the ALIAS. Probed 2026-10-12 against claude 2.1.296 on a
// Max plan, the aliases resolved as:
//
//	fable  -> claude-fable-5-1
//	opus   -> claude-opus-5-5
//	sonnet -> claude-sonnet-5-5
//	haiku  -> claude-haiku-5-5
//
// Those resolutions are shown to the user and are NOT what we send. An alias
// always points at the current model of that family, so this list cannot rot
// the way a pinned id does: when Anthropic ships the next Sonnet, "sonnet"
// follows it with no change here and no release. That is why this provider is
// in the fixed-list category described in catalogue_fetch.go alongside
// Antigravity and the ChatGPT sign-in, and not in LiveCatalogues. There is no
// list to fetch, because there are only four names and they are permanent.
package models

const ProviderClaudeCLI ModelProvider = "claude-cli"

const (
	ClaudeCLIOpus   ModelID = "claude-cli.opus"
	ClaudeCLIFable  ModelID = "claude-cli.fable"
	ClaudeCLISonnet ModelID = "claude-cli.sonnet"
	ClaudeCLIHaiku  ModelID = "claude-cli.haiku"
)

// ClaudeCLIContextWindow is a deliberate FLOOR, not a measurement.
//
// CLAIM SOURCING (directive §14): nothing here measured the real context window
// of the 5.5 generation. 200K is the smallest window any current Claude model
// has, so sizing against it can only ever make this program compact EARLIER
// than it had to, never send a request that is refused for length. The CLI also
// runs its own auto-compaction (`--autocompact`), so the floor being low costs a
// little efficiency and risks nothing.
//
// Raise it only against a measurement, not against a documentation page.
const ClaudeCLIContextWindow int64 = 200_000

// ClaudeCLIModels is the catalogue shown in the model picker.
//
// Every cost field is 0 on purpose. This route bills nothing per token: the
// entitlement is the subscription the user already pays for. The `claude` CLI
// does report a `total_cost_usd` per turn, which is what the same work WOULD
// have cost on the API, and the provider deliberately does not write it into
// these fields. Showing it as money would tell the user they are being charged
// for something their plan already covers. See internal/llm/provider/claudecli.go.
var ClaudeCLIModels = map[ModelID]Model{
	ClaudeCLIOpus: {
		ID:          ClaudeCLIOpus,
		Name:        "Claude Opus (your Claude subscription)",
		Description: "Anthropic's most capable model, through the Claude Code CLI you are signed in to.",
		Detail: "Runs by starting your own `claude` program, which is already signed in to your " +
			"Claude account. No API key and no per-token bill: the work counts against the " +
			"Pro or Max plan you already pay for, and the five-hour and seven-day windows of " +
			"that plan are what limits it. Gorilla OpenCode never sees your password or token.",
		Provider:            ProviderClaudeCLI,
		APIModel:            "opus",
		ContextWindow:       ClaudeCLIContextWindow,
		DefaultMaxTokens:    32_000,
		CanReason:           true,
		SupportsAttachments: true,
		Rank:                1,
	},
	ClaudeCLIFable: {
		ID:          ClaudeCLIFable,
		Name:        "Claude Fable (your Claude subscription)",
		Description: "Frontier model tuned for long agentic work.",
		Detail: "Runs by starting your own `claude` program, which is already signed in to your " +
			"Claude account. No API key and no per-token bill. Counts against your Pro or Max " +
			"plan's own limits rather than a card.",
		Provider:            ProviderClaudeCLI,
		APIModel:            "fable",
		ContextWindow:       ClaudeCLIContextWindow,
		DefaultMaxTokens:    32_000,
		CanReason:           true,
		SupportsAttachments: true,
		Rank:                2,
	},
	ClaudeCLISonnet: {
		ID:          ClaudeCLISonnet,
		Name:        "Claude Sonnet (your Claude subscription)",
		Description: "The balanced everyday model, and what the CLI picks by default.",
		Detail: "Runs by starting your own `claude` program, which is already signed in to your " +
			"Claude account. No API key and no per-token bill. Faster than Opus and easier on " +
			"your plan's limits, which matters because those limits, not money, are what you " +
			"run out of on this route.",
		Provider:            ProviderClaudeCLI,
		APIModel:            "sonnet",
		ContextWindow:       ClaudeCLIContextWindow,
		DefaultMaxTokens:    32_000,
		CanReason:           true,
		SupportsAttachments: true,
		Rank:                3,
	},
	ClaudeCLIHaiku: {
		ID:          ClaudeCLIHaiku,
		Name:        "Claude Haiku (your Claude subscription)",
		Description: "Small and fast, for titles, summaries and simple edits.",
		Detail: "Runs by starting your own `claude` program, which is already signed in to your " +
			"Claude account. The lightest model on this route and the gentlest on your plan's " +
			"five-hour window, which is why it is the one used for background work.",
		Provider:            ProviderClaudeCLI,
		APIModel:            "haiku",
		ContextWindow:       ClaudeCLIContextWindow,
		DefaultMaxTokens:    16_000,
		CanReason:           true,
		SupportsAttachments: true,
		Rank:                4,
	},
}

func init() {
	// Additive copy into the global registry, mirroring the Antigravity,
	// ChatGPT sign-in and Gemini Code Assist registrations.
	for id, m := range ClaudeCLIModels {
		SupportedModels[id] = m
	}
	// Rank alongside the other sign-in routes so these sort near the top of the
	// picker rather than landing last as "unranked".
	ProviderPopularity[ProviderClaudeCLI] = 1
}
