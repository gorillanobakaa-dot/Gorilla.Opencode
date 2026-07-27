package app

import (
	"context"
	"os/exec"
	"strings"
	"time"

	"github.com/opencode-ai/opencode/internal/config"
	"github.com/opencode-ai/opencode/internal/logging"
	"github.com/opencode-ai/opencode/internal/lsp"
	"github.com/opencode-ai/opencode/internal/lsp/watcher"
)

func (app *App) initLSPClients(ctx context.Context) {
	cfg := config.Get()

	// GORILLA OVERRIDE: de-duplicate by (command, args) before starting anything.
	// The config maps a LANGUAGE to a server, but several languages share one
	// binary — clangd serves both "c" and "cpp", typescript-language-server
	// serves both "javascript" and "typescript". Starting one process per
	// language spawned redundant servers: an observed run had TWO clangd
	// processes (PIDs 228951/228952) idling on the same workspace, and clangd
	// on a large tree is 500MB-2GB each. One process per distinct command
	// answers for every language it covers.
	started := make(map[string]string, len(cfg.LSP))
	for name, clientConfig := range cfg.LSP {
		// Honour the per-server /context toggle. Skipping here means the process
		// never starts — the real saving is memory and CPU, not tokens. A toggle
		// applies on the next launch; we do not hot-stop a running server.
		if !config.LSPEnabled(name) {
			logging.Info("LSP client disabled, not starting", "name", name)
			continue
		}
		// Resolve the command to its absolute path first, so "gopls" and
		// "/home/gorilla/go/bin/gopls" — two config entries naming the SAME
		// binary — collapse to one fingerprint. Without this a bare name that
		// is not on PATH also starts a doomed second process alongside the
		// working absolute-path one.
		resolved := clientConfig.Command
		if abs, err := exec.LookPath(clientConfig.Command); err == nil {
			resolved = abs
		}
		fingerprint := resolved + "\x00" + strings.Join(clientConfig.Args, "\x00")
		if first, dup := started[fingerprint]; dup {
			logging.Info("LSP client shares a command with an already-started server, reusing it",
				"name", name, "already_started_as", first, "command", clientConfig.Command)
			continue
		}
		started[fingerprint] = name
		// Start each client initialization in its own goroutine
		go app.createAndStartLSPClient(ctx, name, clientConfig.Command, clientConfig.Args...)
	}
	logging.Info("LSP clients initialization started in background")
}

// createAndStartLSPClient creates a new LSP client, initializes it, and starts its workspace watcher
func (app *App) createAndStartLSPClient(ctx context.Context, name string, command string, args ...string) {
	// Create a specific context for initialization with a timeout
	logging.Info("Creating LSP client", "name", name, "command", command, "args", args)

	// Create the LSP client
	lspClient, err := lsp.NewClient(ctx, command, args...)
	if err != nil {
		logging.Error("Failed to create LSP client for", name, err)
		return
	}

	// Create a longer timeout for initialization (some servers take time to start)
	initCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	// Initialize with the initialization context
	_, err = lspClient.InitializeLSPClient(initCtx, config.WorkingDirectory())
	if err != nil {
		logging.Error("Initialize failed", "name", name, "error", err)
		// Clean up the client to prevent resource leaks
		lspClient.Close()
		return
	}

	// Wait for the server to be ready
	if err := lspClient.WaitForServerReady(initCtx); err != nil {
		logging.Error("Server failed to become ready", "name", name, "error", err)
		// We'll continue anyway, as some functionality might still work
		lspClient.SetServerState(lsp.StateError)
	} else {
		logging.Info("LSP server is ready", "name", name)
		lspClient.SetServerState(lsp.StateReady)
	}

	logging.Info("LSP client initialized", "name", name)

	// Create a child context that can be canceled when the app is shutting down
	watchCtx, cancelFunc := context.WithCancel(ctx)

	// Create a context with the server name for better identification
	watchCtx = context.WithValue(watchCtx, "serverName", name)

	// Create the workspace watcher
	workspaceWatcher := watcher.NewWorkspaceWatcher(lspClient)

	// Store the cancel function to be called during cleanup
	app.cancelFuncsMutex.Lock()
	app.watcherCancelFuncs = append(app.watcherCancelFuncs, cancelFunc)
	app.cancelFuncsMutex.Unlock()

	// Add the watcher to a WaitGroup to track active goroutines
	app.watcherWG.Add(1)

	// Add to map with mutex protection before starting goroutine
	app.clientsMutex.Lock()
	app.LSPClients[name] = lspClient
	app.clientsMutex.Unlock()

	go app.runWorkspaceWatcher(watchCtx, name, workspaceWatcher)
}

// runWorkspaceWatcher executes the workspace watcher for an LSP client
func (app *App) runWorkspaceWatcher(ctx context.Context, name string, workspaceWatcher *watcher.WorkspaceWatcher) {
	defer app.watcherWG.Done()
	defer logging.RecoverPanic("LSP-"+name, func() {
		// Try to restart the client
		app.restartLSPClient(ctx, name)
	})

	// GORILLA OVERRIDE: watch EVERY workspace root, not only the primary. An
	// /add-dir root whose files are never watched gets no diagnostics on change,
	// which is half the point of registering it.
	//
	// WatchWorkspace blocks for the life of the context, so extra roots each get
	// their own goroutine and the primary keeps this one — preserving the
	// "watcher stopped" signal that the caller's defer relies on.
	roots := config.Roots()
	for _, extra := range roots[1:] {
		go func(path string) {
			defer logging.RecoverPanic("LSP-watch-"+name+"-"+path, nil)
			logging.Info("Watching additional workspace root", "client", name, "path", path)
			workspaceWatcher.WatchWorkspace(ctx, path)
		}(extra)
	}
	workspaceWatcher.WatchWorkspace(ctx, roots[0])
	logging.Info("Workspace watcher stopped", "client", name)
}

// restartLSPClient attempts to restart a crashed or failed LSP client
func (app *App) restartLSPClient(ctx context.Context, name string) {
	// Get the original configuration
	cfg := config.Get()
	clientConfig, exists := cfg.LSP[name]
	if !exists {
		logging.Error("Cannot restart client, configuration not found", "client", name)
		return
	}

	// Clean up the old client if it exists
	app.clientsMutex.Lock()
	oldClient, exists := app.LSPClients[name]
	if exists {
		delete(app.LSPClients, name) // Remove from map before potentially slow shutdown
	}
	app.clientsMutex.Unlock()

	if exists && oldClient != nil {
		// Try to shut it down gracefully, but don't block on errors
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		_ = oldClient.Shutdown(shutdownCtx)
		cancel()
	}

	// Create a new client using the shared function
	app.createAndStartLSPClient(ctx, name, clientConfig.Command, clientConfig.Args...)
	logging.Info("Successfully restarted LSP client", "client", name)
}
