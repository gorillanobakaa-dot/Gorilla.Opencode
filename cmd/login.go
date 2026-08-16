// GORILLA OVERRIDE: this file did not exist upstream. `gorilla-opencode login`
// authenticates with ANY supported provider, not just Google. The original
// version offered two options and called auth.Login (Google OAuth) for both, so
// a user with a Groq / Anthropic / Cerebras key had no CLI path and had to
// hand-edit ~/.config/gorilla-opencode/config.json.
package cmd

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/opencode-ai/opencode/internal/auth"
	"github.com/opencode-ai/opencode/internal/config"
	"github.com/opencode-ai/opencode/internal/llm/models"
	"github.com/spf13/cobra"
)

var (
	loginProject string
	loginChatGPT bool
)

// loginOption is one row in the interactive menu.
//
// provider == "" and gcpPath == false -> Google OAuth, auto free tier.
// gcpPath == true                     -> Google OAuth, ask for a GCP project id.
// otherwise                           -> API-key path for that provider.
// envHint names the env var that would have skipped this command entirely.
type loginOption struct {
	label    string
	provider models.ModelProvider
	envHint  string
	gcpPath  bool
}

var loginOptions = []loginOption{
	{label: "Google (Code Assist — free tier, OAuth)"},
	{label: "Anthropic (Claude)", provider: models.ProviderAnthropic, envHint: "ANTHROPIC_API_KEY"},
	{label: "OpenAI (GPT / o-series)", provider: models.ProviderOpenAI, envHint: "OPENAI_API_KEY"},
	{label: "Google Gemini (API key)", provider: models.ProviderGemini, envHint: "GEMINI_API_KEY"},
	{label: "Groq (fast inference — free tier available)", provider: models.ProviderGROQ, envHint: "GROQ_API_KEY"},
	{label: "Cerebras (fast inference)", provider: models.ProviderCerebras, envHint: "CEREBRAS_API_KEY"},
	{label: "xAI (Grok)", provider: models.ProviderXAI, envHint: "XAI_API_KEY"},
	{label: "OpenRouter (multi-provider gateway)", provider: models.ProviderOpenRouter, envHint: "OPENROUTER_API_KEY"},
	{label: "Google — use a specific Cloud project id (billing/quota)", gcpPath: true},
}

var loginCmd = &cobra.Command{
	Use:   "login",
	Short: "Sign in to any supported provider (Google OAuth, API keys, …)",
	Long: `Choose how to connect to an AI provider.

Google OAuth:  personal Gmail, Google's free Code Assist tier — no key needed.
               A browser window opens for consent; tokens are stored (0o600) at
               ~/.config/gorilla-opencode/gemini-oauth.json and auto-refreshed.

API-key path:  paste a key for Anthropic, OpenAI, Groq, Cerebras, xAI,
               OpenRouter, or Gemini. Saved to
               ~/.config/gorilla-opencode/config.json (mode 0o600).

Tip: export the provider's env var (ANTHROPIC_API_KEY, GROQ_API_KEY, …) and skip
this command — the app picks env-var keys up automatically and /model shows them
without any /connect ceremony.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()

		// Scripted path: --chatgpt skips the menu entirely.
		if loginChatGPT {
			return runChatGPTLogin(ctx)
		}

		// Scripted path: --project skips the menu entirely.
		if cmd.Flags().Changed("project") && loginProject != "" {
			return runGoogleLogin(ctx, loginProject)
		}

		// ONE bufio.Reader for the whole flow. A second reader would see EOF
		// on piped input, because the first buffered the remaining lines.
		stdin := bufio.NewReader(os.Stdin)
		chosen, gcpProject := pickLoginOption(stdin, loginOptions)

		switch {
		case chosen.gcpPath:
			return runGoogleLogin(ctx, gcpProject)
		case chosen.provider == "":
			return runGoogleLogin(ctx, "")
		default:
			return runAPIKeyLogin(chosen, stdin)
		}
	},
}

// runChatGPTLogin signs in with a ChatGPT account and then MEASURES whether the
// resulting token is usable from this client.
//
// OpenAI states that Codex is included across ChatGPT plans "including Free and
// Go", which is the access this project exists to reach. What OpenAI does not
// state, anywhere, is whether a client other than their own may present that
// token. So this does not assume an answer in either direction: it signs in,
// asks the backend one read-only question, and prints exactly what came back.
func runChatGPTLogin(ctx context.Context) error {
	fmt.Println("\nStarting ChatGPT sign-in...")
	fmt.Printf("A browser window will open. This uses port %d, which OpenAI\n"+
		"registered for this sign-in — it cannot use any other port.\n", 1455)

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
	fmt.Printf("\nSigned in as %s.\n", who)
	if creds.PlanType != "" {
		fmt.Printf("Plan: %s\n", creds.PlanType)
	}
	fmt.Printf("Token saved (0600) at %s\n", auth.ChatGPTCredsPath())

	fmt.Println("\nChecking whether this client may use that sign-in...")
	fmt.Printf("  asking:     GET %s/models\n", auth.ChatGPTBackend)
	fmt.Printf("  identifying as: originator=%s\n", auth.ChatGPTOriginator)

	status, body, err := creds.ProbeBackend(ctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "\nCould not complete the check: %v\n", err)
		fmt.Fprintln(os.Stderr, "The sign-in itself worked and your token is saved.")
		return nil
	}

	if len(body) > 600 {
		body = body[:600] + " …(truncated)"
	}
	fmt.Printf("\n  HTTP %d\n", status)
	if body != "" {
		fmt.Printf("  %s\n", body)
	}

	switch {
	case status >= 200 && status < 300:
		fmt.Println("\nAccepted. The ChatGPT plan is reachable from this client.")
	case status == 401 || status == 403:
		fmt.Println("\nRefused. The token is valid but this client is not accepted at that")
		fmt.Println("endpoint. Nothing further will be attempted: making this work would mean")
		fmt.Println("presenting ourselves as a different client, and that is a decision for the")
		fmt.Println("project owner to make deliberately, not something to slip into a release.")
	default:
		fmt.Println("\nUnexpected reply — read the status and body above before drawing a")
		fmt.Println("conclusion. Neither 'it works' nor 'it is blocked' is established by this.")
	}
	return nil
}

// runGoogleLogin runs OAuth + Code Assist onboarding. Empty project = free tier.
func runGoogleLogin(ctx context.Context, project string) error {
	fmt.Println("\nStarting Google sign-in...")
	creds, err := auth.Login(ctx)
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
	fmt.Println("Setting up Code Assist (free tier)...")
	if err := creds.SetupCodeAssist(ctx, project); err != nil {
		// Sign-in landed; only onboarding hiccuped. Say so, do not fail.
		fmt.Fprintf(os.Stderr, "\nSigned in, but Code Assist onboarding failed: %v\n", err)
		fmt.Fprintln(os.Stderr, "Your token is saved; retry with 'gorilla-opencode login'.")
		return nil
	}
	fmt.Printf("Ready. Project: %s (tier: %s)\n", creds.ProjectID, creds.Tier)
	fmt.Println("\nStart the app and choose a Gemini model — it will use this login.")
	return nil
}

// runAPIKeyLogin prompts for a key (or reuses an exported env var) and saves it.
func runAPIKeyLogin(chosen loginOption, stdin *bufio.Reader) error {
	if chosen.envHint != "" {
		if k := strings.TrimSpace(os.Getenv(chosen.envHint)); k != "" {
			fmt.Printf("\n%s is already set in your environment; saving that value.\n", chosen.envHint)
			return persistProviderKey(chosen, k)
		}
	}
	fmt.Printf("\nPaste your %s API key: ", chosen.label)
	line, err := stdin.ReadString('\n')
	if err != nil {
		return fmt.Errorf("reading key: %w", err)
	}
	key := strings.TrimSpace(line)
	if key == "" {
		return fmt.Errorf("no key entered")
	}
	return persistProviderKey(chosen, key)
}

func persistProviderKey(chosen loginOption, key string) error {
	// `login` runs before the app starts, so config.Load has not run and cfg is
	// nil — UpsertProviderKey would fail with "config not loaded". Load lazily.
	// The working directory is irrelevant to a config write; updateCfgFile always
	// resolves to ~/.config/gorilla-opencode/config.json.
	if config.Get() == nil {
		cwd, err := os.Getwd()
		if err != nil {
			cwd = "."
		}
		if _, err := config.Load(cwd, false); err != nil {
			return fmt.Errorf("loading config: %w", err)
		}
	}
	if err := config.UpsertProviderKey(chosen.provider, key); err != nil {
		return fmt.Errorf("saving key: %w", err)
	}
	fmt.Printf("\n\u2713 Saved %s key to ~/.config/gorilla-opencode/config.json (mode 0600).\n", chosen.label)
	if chosen.envHint != "" {
		fmt.Printf("  (You can also export %s=… next time and skip this command.)\n", chosen.envHint)
	}
	fmt.Println("\nStart the app — /model will show this provider as a tab.")
	return nil
}

// pickLoginOption renders the menu and reads the choice. `opts` is a parameter
// rather than a global read so the selection logic is unit-testable without
// touching stdin of the real process or any config file.
func pickLoginOption(stdin *bufio.Reader, opts []loginOption) (loginOption, string) {
	fmt.Println("\nWelcome to Gorilla OpenCode.")
	fmt.Println("\nSelect a provider to connect:")
	for i, o := range opts {
		fmt.Printf("  %d. %s\n", i+1, o.label)
	}
	fmt.Print("\nEnter a number [1]: ")

	raw, _ := stdin.ReadString('\n')
	idx := selectedIndex(strings.TrimSpace(raw), len(opts))
	chosen := opts[idx]

	gcp := ""
	if chosen.gcpPath {
		fmt.Print("Enter your Google Cloud project id: ")
		p, _ := stdin.ReadString('\n')
		gcp = strings.TrimSpace(p)
	}
	return chosen, gcp
}

// selectedIndex maps raw menu input to a 0-based index, defaulting to 0 for
// empty input and for anything out of range or unparseable. Split out so the
// boundary behaviour is testable.
func selectedIndex(raw string, n int) int {
	if raw == "" || n == 0 {
		return 0
	}
	var v int
	if _, err := fmt.Sscanf(raw, "%d", &v); err == nil && v >= 1 && v <= n {
		return v - 1
	}
	return 0
}

func init() {
	loginCmd.Flags().BoolVar(&loginChatGPT, "chatgpt", false,
		"sign in with a ChatGPT account (Codex is included on ChatGPT plans, including Free)")
	loginCmd.Flags().StringVar(&loginProject, "project", "",
		"Google Cloud project id (Google OAuth with your own project; omit for the free tier or the interactive menu)")
	rootCmd.AddCommand(loginCmd)
}
