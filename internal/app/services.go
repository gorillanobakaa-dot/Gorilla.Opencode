package app

import (
	"context"
	"database/sql"

	"github.com/kujtimiihoxha/termai/internal/config"
	"github.com/kujtimiihoxha/termai/internal/db"
	"github.com/kujtimiihoxha/termai/internal/logging"
	"github.com/kujtimiihoxha/termai/internal/lsp"
	"github.com/kujtimiihoxha/termai/internal/lsp/watcher"
	"github.com/kujtimiihoxha/termai/internal/message"
	"github.com/kujtimiihoxha/termai/internal/permission"
	"github.com/kujtimiihoxha/termai/internal/session"
)

type App struct {
	Context context.Context

	Sessions    session.Service
	Messages    message.Service
	Permissions permission.Service

	LSPClients map[string]*lsp.Client
}

func New(ctx context.Context, conn *sql.DB) *App {
	cfg := config.Get()
	logging.Info("Debug mode enabled")

	q := db.New(conn)
	sessions := session.NewService(ctx, q)
	messages := message.NewService(ctx, q)

	app := &App{
		Context:     ctx,
		Sessions:    sessions,
		Messages:    messages,
		Permissions: permission.NewPermissionService(),
		LSPClients:  make(map[string]*lsp.Client),
	}

	for name, client := range cfg.LSP {
		lspClient, err := lsp.NewClient(ctx, client.Command, client.Args...)
		workspaceWatcher := watcher.NewWorkspaceWatcher(lspClient)
		if err != nil {
			logging.Error("Failed to create LSP client for", name, err)
			continue
		}

		_, err = lspClient.InitializeLSPClient(ctx, config.WorkingDirectory())
		if err != nil {
			logging.Error("Initialize failed", "error", err)
			continue
		}
		go workspaceWatcher.WatchWorkspace(ctx, config.WorkingDirectory())
		app.LSPClients[name] = lspClient
	}
	return app
}
