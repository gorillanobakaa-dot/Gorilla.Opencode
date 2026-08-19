package app

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"maps"
	"sync"
	"time"

	"github.com/opencode-ai/opencode/internal/config"
	"github.com/opencode-ai/opencode/internal/db"
	"github.com/opencode-ai/opencode/internal/format"
	"github.com/opencode-ai/opencode/internal/history"
	"github.com/opencode-ai/opencode/internal/llm/agent"
	"github.com/opencode-ai/opencode/internal/llm/prompt"
	"github.com/opencode-ai/opencode/internal/logging"
	"github.com/opencode-ai/opencode/internal/lsp"
	"github.com/opencode-ai/opencode/internal/message"
	"github.com/opencode-ai/opencode/internal/permission"
	"github.com/opencode-ai/opencode/internal/session"
	"github.com/opencode-ai/opencode/internal/tui/theme"
)

type App struct {
	Sessions    session.Service
	Messages    message.Service
	History     history.Service
	Permissions permission.Service

	CoderAgent agent.Service

	LSPClients map[string]*lsp.Client

	clientsMutex sync.RWMutex

	watcherCancelFuncs []context.CancelFunc
	cancelFuncsMutex   sync.Mutex
	watcherWG          sync.WaitGroup
}

func New(ctx context.Context, conn *sql.DB) (*App, error) {
	q := db.New(conn)
	sessions := session.NewService(q)
	messages := message.NewService(q)
	files := history.NewService(q, conn)

	app := &App{
		Sessions:    sessions,
		Messages:    messages,
		History:     files,
		Permissions: permission.NewPermissionService(),
		LSPClients:  make(map[string]*lsp.Client),
	}

	// Initialize theme based on configuration
	app.initTheme()

	// GORILLA OVERRIDE: measure the real per-turn token cost of every
	// tool and the base prompt so the /context loadout reports truth.
	// GORILLA OVERRIDE: register a /context row per prompt section BEFORE
	// calibration, so the measured token cost lands on rows that already exist.
	prompt.RegisterSectionComponents()

	agent.CalibrateLoadout(app.Permissions, app.Sessions, app.Messages, app.History, app.LSPClients)

	// Initialize LSP clients in the background
	go app.initLSPClients(ctx)

	var err error
	app.CoderAgent, err = agent.NewAgent(
		config.AgentCoder,
		app.Sessions,
		app.Messages,
		agent.CoderAgentTools(
			app.Permissions,
			app.Sessions,
			app.Messages,
			app.History,
			app.LSPClients,
		),
	)
	if err != nil {
		logging.Error("Failed to create coder agent", err)
		return nil, err
	}

	return app, nil
}

// initTheme sets the application theme based on the configuration
func (app *App) initTheme() {
	cfg := config.Get()
	if cfg == nil || cfg.TUI.Theme == "" {
		return // Use default theme
	}

	// Try to set the theme from config
	err := theme.SetTheme(cfg.TUI.Theme)
	if err != nil {
		logging.Warn("Failed to set theme from config, using default theme", "theme", cfg.TUI.Theme, "error", err)
	} else {
		logging.Debug("Set theme from config", "theme", cfg.TUI.Theme)
	}
}

// RunNonInteractive handles the execution flow when a prompt is provided via CLI flag.
func (a *App) RunNonInteractive(ctx context.Context, prompt string, outputFormat string, quiet bool) error {
	logging.Info("Running in non-interactive mode")

	// GORILLA OVERRIDE (2026-08-18): a wall-clock deadline, because there is
	// nobody here to press ESC.
	//
	// provider/httpclient.go deliberately sets no client Timeout and no
	// ResponseHeaderTimeout, so a big model over a satellite link is never
	// killed mid-answer. That is right — and it assumes a human is watching,
	// able to cancel. Measured on a link that went silent (socket held open,
	// nothing forwarded, which is what a real dropout looks like): the
	// interactive path can be escaped, and this one hung for as long as it was
	// left, with no error and no output. A cron job or a script would hang
	// forever.
	//
	// So the deadline exists only on this path, generous enough that a slow
	// model on a slow link finishes comfortably, and it says what happened.
	if d := config.NonInteractiveDeadline(); d > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, d)
		defer cancel()
	}

	// Start spinner if not in quiet mode
	var spinner *format.Spinner
	if !quiet {
		spinner = format.NewSpinner("Thinking...")
		spinner.Start()
		defer spinner.Stop()
	}

	const maxPromptLengthForTitle = 100
	titlePrefix := "Non-interactive: "
	var titleSuffix string

	if len(prompt) > maxPromptLengthForTitle {
		titleSuffix = prompt[:maxPromptLengthForTitle] + "..."
	} else {
		titleSuffix = prompt
	}
	title := titlePrefix + titleSuffix

	sess, err := a.Sessions.Create(ctx, title)
	if err != nil {
		return fmt.Errorf("failed to create session for non-interactive mode: %w", err)
	}
	logging.Info("Created session for non-interactive run", "session_id", sess.ID)

	// Automatically approve all permission requests for this non-interactive session
	a.Permissions.AutoApproveSession(sess.ID)
	// GORILLA FIX (2026-08-19): and say that nobody is watching, so the
	// auto-approve carve-outs log instead of publishing a question no
	// subscriber exists to answer. Without this a headless fetch would block
	// for the full ten-minute PermissionWait and then be denied.
	a.Permissions.SetUnattended(true)

	done, err := a.CoderAgent.Run(ctx, sess.ID, prompt)
	if err != nil {
		return fmt.Errorf("failed to start agent processing stream: %w", err)
	}

	result := <-done

	// GORILLA OVERRIDE (2026-08-18): a deadline is a FAILURE, not a cancellation.
	//
	// The branch below returns nil — exit code 0 — for a cancelled run, which is
	// right when a human pressed ESC. But the headless deadline added above
	// surfaces through the same path, and measured against a link that went
	// silent this printed "No content available" and exited 0. A script or a
	// cron job reads that as success with a strange answer, which is worse than
	// the hang it replaced: the hang at least never claimed to have worked.
	//
	// So the deadline is checked FIRST and separately, and it fails loudly.
	if ctx.Err() == context.DeadlineExceeded || errors.Is(result.Error, context.DeadlineExceeded) {
		return fmt.Errorf(
			"gave up after %s: no answer arrived. The connection may have gone silent — "+
				"on a satellite or mobile link that is common, and there is nobody here to "+
				"cancel by hand. Nothing was written. Raise or remove the limit with "+
				"GORILLA_OPENCODE_HEADLESS_TIMEOUT (a duration such as 45m, or 0 to wait "+
				"indefinitely)", config.NonInteractiveDeadline())
	}

	if result.Error != nil {
		if errors.Is(result.Error, context.Canceled) || errors.Is(result.Error, agent.ErrRequestCancelled) {
			logging.Info("Agent processing cancelled", "session_id", sess.ID)
			return nil
		}
		return fmt.Errorf("agent processing failed: %w", result.Error)
	}

	// Stop spinner before printing output
	if !quiet && spinner != nil {
		spinner.Stop()
	}

	// Get the text content from the response
	content := "No content available"
	if result.Message.Content().String() != "" {
		content = result.Message.Content().String()
	}

	fmt.Println(format.FormatOutput(content, outputFormat))

	logging.Info("Non-interactive run completed", "session_id", sess.ID)

	return nil
}

// Shutdown performs a clean shutdown of the application
func (app *App) Shutdown() {
	// Cancel all watcher goroutines
	app.cancelFuncsMutex.Lock()
	for _, cancel := range app.watcherCancelFuncs {
		cancel()
	}
	app.cancelFuncsMutex.Unlock()
	app.watcherWG.Wait()

	// Perform additional cleanup for LSP clients
	app.clientsMutex.RLock()
	clients := make(map[string]*lsp.Client, len(app.LSPClients))
	maps.Copy(clients, app.LSPClients)
	app.clientsMutex.RUnlock()

	for name, client := range clients {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		if err := client.Shutdown(shutdownCtx); err != nil {
			logging.Error("Failed to shutdown LSP client", "name", name, "error", err)
		}
		cancel()
	}
}

// GORILLA OVERRIDE: rebuild the coder agent's tool set from the current
// context loadout so /context toggles take effect without a restart.
//
// Returns true when the system-prompt rebuild had to be DEFERRED because a turn
// is in flight. The tool set swaps immediately either way (it is read under a
// lock per tool call), but the provider cannot be replaced mid-request. Callers
// must surface a deferred result — reporting a new token count for a change that
// has not taken effect is exactly the silent-failure this returns to prevent.
func (app *App) ReloadCoderTools() (deferred bool) {
	if app.CoderAgent == nil {
		return false
	}
	app.CoderAgent.ReloadTools(agent.CoderAgentTools(
		app.Permissions,
		app.Sessions,
		app.Messages,
		app.History,
		app.LSPClients,
	))
	// GORILLA OVERRIDE: also re-render the system prompt so env/LSP
	// loadout toggles (the env block can be thousands of tokens) take
	// effect immediately, not on restart.
	return app.CoderAgent.RebuildProvider()
}
