package cmd

import (
	"context"
	"fmt"
	"os"
	"sync"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	zone "github.com/lrstanley/bubblezone"
	"github.com/opencode-ai/opencode/internal/app"
	"github.com/opencode-ai/opencode/internal/config"
	"github.com/opencode-ai/opencode/internal/db"
	"github.com/opencode-ai/opencode/internal/format"
	"github.com/opencode-ai/opencode/internal/llm/agent"
	"github.com/opencode-ai/opencode/internal/logging"
	"github.com/opencode-ai/opencode/internal/plain"
	"github.com/opencode-ai/opencode/internal/pubsub"
	"github.com/opencode-ai/opencode/internal/tui"
	"github.com/opencode-ai/opencode/internal/tui/startup"
	"github.com/opencode-ai/opencode/internal/tui/theme"
	"github.com/opencode-ai/opencode/internal/version"
	"github.com/spf13/cobra"
)

// resolveWorkspace decides which folder this session works in, and reports
// whether the user asked not to be prompted again. An empty first return means
// the user quit the picker and the launch must abort.
//
// GORILLA OVERRIDE: this whole step is new. The problem it solves is that the
// desktop entry is `Exec=gorilla-opencode launch` with no Path=, so an icon
// click — how nearly everyone starts the program — inherits $HOME. On a machine
// holding a kernel tree and a browser tree, that puts over a million files
// inside the agent's default reach before a word is typed, and the saved
// workspace could not rescue it because config.json's "wd" was write-only
// (see config.PeekStartupWorkspace).
//
// The precedence is deliberate:
//  1. An explicit -c/--cwd always wins. The user was specific; do not
//     second-guess them, and never interrupt a scripted invocation.
//  2. Non-interactive runs (-p) never prompt — there is no one to answer. They
//     fall back to the saved workspace, then to the real cwd.
//  3. Otherwise ask, unless the user has turned the question off, in which case
//     the saved workspace is used silently.
func resolveWorkspace(flagCwd string, nonInteractive bool) (dir, alreadySaved string, remember bool, err error) {
	saved := config.PeekStartupWorkspace()

	if flagCwd != "" {
		abs, err := startup.ResolveDir(flagCwd)
		if err != nil {
			return "", "", false, fmt.Errorf("--cwd: %v", err)
		}
		return abs, saved.WorkingDir, false, nil
	}

	realCwd, err := os.Getwd()
	if err != nil {
		return "", "", false, fmt.Errorf("failed to get current working directory: %v", err)
	}

	if nonInteractive || !saved.Ask || !interactiveTerminal() {
		if saved.WorkingDir != "" {
			return saved.WorkingDir, saved.WorkingDir, false, nil
		}
		return realCwd, saved.WorkingDir, false, nil
	}

	choice, err := startup.Ask(startup.Options{
		LastUsed: saved.WorkingDir,
		Cwd:      realCwd,
		Recent:   saved.Recent,
		Home:     homeDir(),
	})
	if err != nil {
		// A picker that cannot run must not block the program. Fall back to what
		// the non-interactive path would have done and say why.
		fmt.Fprintf(os.Stderr, "could not show the workspace picker (%v); using %s\n", err, realCwd)
		return realCwd, saved.WorkingDir, false, nil
	}
	if choice.Quit {
		return "", saved.WorkingDir, false, nil
	}
	return choice.Dir, saved.WorkingDir, choice.Remember, nil
}

// noProgramOption is the option that changes nothing. tea offers no no-op
// ProgramOption, and the alternative — building the option slice conditionally at
// every call site — is where a mode gets applied to one path and forgotten on
// another.
func noProgramOption() tea.ProgramOption { return func(*tea.Program) {} }

// mouseOption asks the terminal for mouse events only where doing so can buy
// anything. See config.RequestMouseEvents: without the alternate screen the
// terminal is already scrolling the conversation, so requesting mouse events
// would trade a working wheel for a broken text selection.
func mouseOption() tea.ProgramOption {
	if config.RequestMouseEvents() {
		return tea.WithMouseCellMotion()
	}
	return noProgramOption()
}

// altScreenOption decides where the interface draws.
//
// GORILLA OVERRIDE: off by default. The alternate screen is a buffer the terminal
// keeps no scrollback for, which is why nothing drawn there could be scrolled
// back to, selected or copied. With it off, finished messages are printed into
// the terminal's real output and only the prompt is redrawn in place — see
// config.AlternateScreenEnabled for the measurements.
func altScreenOption() tea.ProgramOption {
	if config.AlternateScreenEnabled() {
		return tea.WithAltScreen()
	}
	return noProgramOption()
}

// interactiveTerminal reports whether there is a human at a terminal to answer.
// Piped or redirected stdin means a script, and a prompt would hang it.
func interactiveTerminal() bool {
	info, err := os.Stdin.Stat()
	if err != nil {
		return false
	}
	return info.Mode()&os.ModeCharDevice != 0
}

func homeDir() string {
	h, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return h
}

var rootCmd = &cobra.Command{
	Use:   appBinName,
	Short: "Terminal-based AI assistant for software development",
	Long: `Gorilla OpenCode is a terminal-based AI assistant that helps with software
development tasks. It provides an interactive chat interface with AI capabilities,
code analysis, and LSP integration to assist developers in writing, debugging, and
understanding code directly from the terminal.`,
	// GORILLA OVERRIDE: a runtime failure (bad key, unreachable endpoint)
	// used to dump this entire usage text after the error, burying it.
	// Usage now prints only for actual usage mistakes.
	SilenceUsage: true,
	Example: `
  # Run in interactive mode
  ` + appBinName + `

  # Run with debug logging
  ` + appBinName + ` -d

  # Run with debug logging in a specific directory
  ` + appBinName + ` -d -c /path/to/project

  # Print version
  ` + appBinName + ` -v

  # Run a single non-interactive prompt
  ` + appBinName + ` -p "Explain the use of context in Go"

  # Run a single non-interactive prompt with JSON output format
  ` + appBinName + ` -p "Explain the use of context in Go" -f json
  `,
	RunE: func(cmd *cobra.Command, args []string) error {
		// If the help flag is set, show the help message
		if cmd.Flag("help").Changed {
			cmd.Help()
			return nil
		}
		if cmd.Flag("version").Changed {
			fmt.Println(version.Version)
			return nil
		}

		// Load the config
		debug, _ := cmd.Flags().GetBool("debug")
		cwd, _ := cmd.Flags().GetString("cwd")
		prompt, _ := cmd.Flags().GetString("prompt")
		outputFormat, _ := cmd.Flags().GetString("output-format")
		quiet, _ := cmd.Flags().GetBool("quiet")
		plainMode, _ := cmd.Flags().GetBool("plain")

		// Validate format option
		if !format.IsValid(outputFormat) {
			return fmt.Errorf("invalid format option: %s\n%s", outputFormat, format.GetHelpText())
		}

		// GORILLA OVERRIDE: resolve the working folder BEFORE config.Load, because
		// everything that follows is derived from it and cannot be re-pointed
		// afterwards. See resolveWorkspace.
		cwd, saved, remember, err := resolveWorkspace(cwd, prompt != "")
		if err != nil {
			return err
		}
		if cwd == "" {
			// The user quit the picker. Nothing has been started yet, so there is
			// nothing to report — leaving silently is the correct outcome.
			return nil
		}
		if err := os.Chdir(cwd); err != nil {
			return fmt.Errorf("failed to change directory: %v", err)
		}

		cfg, err := config.Load(cwd, debug)
		if err != nil {
			return err
		}

		// Persist the choice only now that Load has succeeded, so a launch that
		// dies on a bad config does not also rewrite the saved workspace.
		//
		// keepOld is FALSE deliberately. Picking a folder at startup means "work
		// here", not "work here as well as wherever I was last time" — keeping
		// the previous primary would add one root per launch, quietly growing the
		// scope this whole feature exists to shrink. Roots added on purpose with
		// /add-dir are not affected; SetWorkingDir only drops the old primary and
		// any root that contains the new one.
		if cwd != config.WorkingDirectory() || cwd != saved {
			if _, err := config.SetWorkingDir(cwd, false); err != nil {
				logging.Warn("could not save the working folder: %v", err)
			}
		}
		if remember {
			if err := config.SetSkipWorkspacePrompt(true); err != nil {
				logging.Warn("could not save the don't-ask-again choice: %v", err)
			}
		}

		// GORILLA OVERRIDE: ask once, on first run, whether to show the agent's
		// working — and be straight that one of those settings makes the model
		// generate more. Anything that spends more of someone's allowance should
		// be a decision, not a default they never saw.
		//
		// After config.Load because the answer is persisted, and skipped entirely
		// when non-interactive (-p) or already answered.
		if prompt == "" && !config.ExtrasChoiceMade() {
			if err := askExtrasOnce(); err != nil {
				// Never fatal: a session must still start if the question fails.
				logging.Warn("could not ask about the optional extras", "err", err)
			}
		}

		// GORILLA OVERRIDE: fill the /settings theme row's options from the theme
		// registry. theme imports config, so config cannot import it back — the
		// list is pushed in here, the same inversion used for prompt sections and
		// the fileutil roots hook.
		config.SetThemeOptions(theme.AvailableThemes())

		// GORILLA OVERRIDE: without any provider the old code died later
		// with the cryptic "agent coder not found". Say what is actually
		// wrong and what to do about it, up front.
		if _, ok := cfg.Agents[config.AgentCoder]; !ok {
			return fmt.Errorf(`no AI provider is configured.

Set one of these and try again:
  NVIDIA NIM:  LOCAL_ENDPOINT=https://integrate.api.nvidia.com/v1 LOCAL_ENDPOINT_API_KEY=nvapi-...
  Google:      GEMINI_API_KEY=...
  Ollama:      LOCAL_ENDPOINT=http://localhost:11434/v1
  (or ANTHROPIC_API_KEY, OPENAI_API_KEY, GROQ_API_KEY, ... — see README)

Desktop launches read keys from ~/.config/%s/env`, appBinName)
		}

		// Connect DB, this will also run migrations
		conn, err := db.Connect()
		if err != nil {
			return err
		}

		// Create main context for the application
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		app, err := app.New(ctx, conn)
		if err != nil {
			logging.Error("Failed to create app: %v", err)
			return err
		}
		// Defer shutdown here so it runs for both interactive and non-interactive modes
		defer app.Shutdown()

		// Initialize MCP tools early for both modes
		initMCPTools(ctx, app)

		// Non-interactive mode
		if prompt != "" {
			// Run non-interactive flow using the App method
			return app.RunNonInteractive(ctx, prompt, outputFormat, quiet)
		}

		// GORILLA OVERRIDE: plain interactive mode. Deliberately before the TUI
		// setup so none of the screen handling runs at all.
		//
		// The flag is an override, not the only way in. The desktop entry runs
		// `gorilla-opencode launch` with no arguments, so a flag-only mode would be
		// unreachable for anyone who starts the program by clicking its icon — and
		// that is most people. The persisted setting is what makes the choice stick.
		if plainMode || config.InterfaceMode() == config.InterfacePlain {
			return plain.New(app, os.Stdin, os.Stdout).Run(ctx)
		}

		// Interactive mode
		// Set up the TUI
		zone.NewGlobal()
		program := tea.NewProgram(
			tui.New(app),
			// GORILLA OVERRIDE: the alternate screen is now a SETTING, off by
			// default, rather than something every launch takes. It is a buffer the
			// terminal keeps no scrollback for, so everything drawn in it was
			// unscrollable, unselectable and uncopyable — the actual reason the
			// interface felt like it was missing ordinary terminal behaviour.
			altScreenOption(),
			// GORILLA OVERRIDE: mouse reporting is opt-in AND only requested when the
			// alternate screen is on. Requesting it takes drag-to-select away from the
			// terminal, and the only modes bubbletea offers report one event per cell
			// crossed — a single drag fires hundreds, which stalled the loop badly
			// enough that raw escape codes leaked into the editor. Without the
			// alternate screen the terminal scrolls the conversation itself, so there
			// is nothing left to buy.
			mouseOption(),
		)
		// Let background goroutines push messages into the event loop. The OAuth
		// flow needs this: it must report its sign-in URL while blocking on the
		// browser callback, and printing it instead paints over a screen Bubble
		// Tea owns, where it can never be cleared.
		tui.SetProgram(program)

		// Setup the subscriptions, this will send services events to the TUI
		ch, cancelSubs := setupSubscriptions(app, ctx)

		// Create a context for the TUI message handler
		tuiCtx, tuiCancel := context.WithCancel(ctx)
		var tuiWg sync.WaitGroup
		tuiWg.Add(1)

		// Set up message handling for the TUI
		go func() {
			defer tuiWg.Done()
			defer logging.RecoverPanic("TUI-message-handler", func() {
				attemptTUIRecovery(program)
			})

			for {
				select {
				case <-tuiCtx.Done():
					logging.Info("TUI message handler shutting down")
					return
				case msg, ok := <-ch:
					if !ok {
						logging.Info("TUI message channel closed")
						return
					}
					program.Send(msg)
				}
			}
		}()

		// Cleanup function for when the program exits
		cleanup := func() {
			// Shutdown the app
			app.Shutdown()

			// Cancel subscriptions first
			cancelSubs()

			// Then cancel TUI message handler
			tuiCancel()

			// Wait for TUI message handler to finish
			tuiWg.Wait()

			logging.Info("All goroutines cleaned up")
		}

		// Run the TUI
		result, err := program.Run()
		cleanup()

		if err != nil {
			logging.Error("TUI error: %v", err)
			return fmt.Errorf("TUI error: %v", err)
		}

		logging.Info("TUI exited with result: %v", result)
		return nil
	},
}

// attemptTUIRecovery tries to recover the TUI after a panic
func attemptTUIRecovery(program *tea.Program) {
	logging.Info("Attempting to recover TUI after panic")

	// We could try to restart the TUI or gracefully exit
	// For now, we'll just quit the program to avoid further issues
	program.Quit()
}

func initMCPTools(ctx context.Context, app *app.App) {
	go func() {
		defer logging.RecoverPanic("MCP-goroutine", nil)

		// Create a context with timeout for the initial MCP tools fetch
		ctxWithTimeout, cancel := context.WithTimeout(ctx, 30*time.Second)
		defer cancel()

		// Set this up once with proper error handling
		agent.GetMcpTools(ctxWithTimeout, app.Permissions)
		logging.Info("MCP message handling goroutine exiting")
	}()
}

func setupSubscriber[T any](
	ctx context.Context,
	wg *sync.WaitGroup,
	name string,
	subscriber func(context.Context) <-chan pubsub.Event[T],
	outputCh chan<- tea.Msg,
) {
	wg.Add(1)
	go func() {
		defer wg.Done()
		defer logging.RecoverPanic(fmt.Sprintf("subscription-%s", name), nil)

		subCh := subscriber(ctx)

		for {
			select {
			case event, ok := <-subCh:
				if !ok {
					logging.Info("subscription channel closed", "name", name)
					return
				}

				var msg tea.Msg = event

				select {
				case outputCh <- msg:
				case <-time.After(2 * time.Second):
					logging.Warn("message dropped due to slow consumer", "name", name)
				case <-ctx.Done():
					logging.Info("subscription cancelled", "name", name)
					return
				}
			case <-ctx.Done():
				logging.Info("subscription cancelled", "name", name)
				return
			}
		}
	}()
}

func setupSubscriptions(app *app.App, parentCtx context.Context) (chan tea.Msg, func()) {
	ch := make(chan tea.Msg, 100)

	wg := sync.WaitGroup{}
	ctx, cancel := context.WithCancel(parentCtx) // Inherit from parent context

	setupSubscriber(ctx, &wg, "logging", logging.Subscribe, ch)
	setupSubscriber(ctx, &wg, "sessions", app.Sessions.Subscribe, ch)
	setupSubscriber(ctx, &wg, "messages", app.Messages.Subscribe, ch)
	setupSubscriber(ctx, &wg, "permissions", app.Permissions.Subscribe, ch)
	setupSubscriber(ctx, &wg, "coderAgent", app.CoderAgent.Subscribe, ch)
	// GORILLA OVERRIDE: live sub-agent spawn/exit events → status bar, /tasks
	// list, and spawn toasts. Keeps the user aware of every helper deployed.
	setupSubscriber(ctx, &wg, "subAgents", agent.SubAgentSubscribe, ch)

	cleanupFunc := func() {
		logging.Info("Cancelling all subscriptions")
		cancel() // Signal all goroutines to stop

		waitCh := make(chan struct{})
		go func() {
			defer logging.RecoverPanic("subscription-cleanup", nil)
			wg.Wait()
			close(waitCh)
		}()

		select {
		case <-waitCh:
			logging.Info("All subscription goroutines completed successfully")
			close(ch) // Only close after all writers are confirmed done
		case <-time.After(5 * time.Second):
			logging.Warn("Timed out waiting for some subscription goroutines to complete")
			close(ch)
		}
	}
	return ch, cleanupFunc
}

func Execute() {
	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}

func init() {
	rootCmd.Flags().BoolP("help", "h", false, "Help")
	rootCmd.Flags().BoolP("version", "v", false, "Version")
	rootCmd.Flags().BoolP("debug", "d", false, "Debug")
	rootCmd.Flags().StringP("cwd", "c", "", "Current working directory")
	rootCmd.Flags().StringP("prompt", "p", "", "Prompt to run in non-interactive mode")

	// Add format flag with validation logic
	rootCmd.Flags().StringP("output-format", "f", format.Text.String(),
		"Output format for non-interactive mode (text, json)")

	// Add quiet flag to hide spinner in non-interactive mode
	rootCmd.Flags().BoolP("quiet", "q", false, "Hide spinner in non-interactive mode")
	// GORILLA OVERRIDE: --plain runs an interactive session with no TUI, so every
	// byte goes to ordinary terminal scrollback and the whole conversation can be
	// selected and copied. The TUI uses the terminal's ALTERNATE screen, which has
	// no scrollback by design — measured: lines pushed via tea.Println reach the
	// terminal 0 times out of 3 with the altscreen active, 3 of 3 without. No
	// amount of work inside the TUI can make Ctrl+A work there.
	rootCmd.Flags().Bool("plain", false, "Interactive mode with no TUI, so the whole session can be selected and copied")

	// Register custom validation for the format flag
	rootCmd.RegisterFlagCompletionFunc("output-format", func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		return format.SupportedFormats, cobra.ShellCompDirectiveNoFileComp
	})
}

// askExtrasOnce shows the first-run consent screen and persists the answers.
//
// GORILLA OVERRIDE: the startup package cannot import config — config is the
// lower layer and startup runs before it — so the rows are built here, shown
// there, and written back here. Same inversion as the workspace picker.
func askExtrasOnce() error {
	rows := make([]startup.ExtraRow, 0, len(config.Extras))
	for _, e := range config.Extras {
		rows = append(rows, startup.ExtraRow{
			ID:    e.ID,
			Name:  e.Name,
			What:  e.What,
			Costs: e.Cost == config.CostGeneration,
			On:    config.ExtraEnabled(e.ID),
		})
	}

	choice, err := startup.AskExtras(rows)
	if err != nil {
		return err
	}
	if choice.Quit {
		// Treat esc as "not now": leave every setting at its default and ask
		// again next launch rather than recording a decision nobody made.
		return nil
	}

	for _, r := range choice.Rows {
		if r.On == config.ExtraEnabled(r.ID) {
			continue
		}
		if err := config.SetExtra(r.ID, r.On); err != nil {
			return err
		}
	}
	return config.MarkExtrasChoiceMade()
}
