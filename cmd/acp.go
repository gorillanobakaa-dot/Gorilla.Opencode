package cmd

import (
	"context"
	"fmt"
	"os"

	"github.com/opencode-ai/opencode/internal/acp"
	"github.com/opencode-ai/opencode/internal/app"
	"github.com/opencode-ai/opencode/internal/config"
	"github.com/opencode-ai/opencode/internal/db"
	"github.com/opencode-ai/opencode/internal/logging"
	"github.com/opencode-ai/opencode/internal/version"
	"github.com/spf13/cobra"
)

// GORILLA (2026-10-09): `gorilla-opencode acp` makes the program an Agent
// Client Protocol agent, so Zed, JetBrains and other ACP editors can drive it
// in place of the terminal. The editor starts this as a subprocess and talks
// JSON-RPC on stdin/stdout; nothing else may be printed to stdout.
//
// No account, no key of its own, no network of its own: the editor gets the
// same program with the same providers, the same permission questions (shown
// by the editor) and the same receipt at the end of each answer.
var acpCmd = &cobra.Command{
	Use:   "acp",
	Short: "Run as an Agent Client Protocol agent over stdio (for Zed, JetBrains and other editors)",
	Long: `Run Gorilla OpenCode as an Agent Client Protocol (ACP) agent.

An editor starts this command as a subprocess and talks JSON-RPC to it on
stdin and stdout. Every tool call is reported to the editor as it happens,
every permission question is shown by the editor, and each answer ends with
the program's own record of what ran.

Zed, in settings.json:
  "agent_servers": {
    "Gorilla OpenCode": { "type": "custom", "command": "gorilla-opencode", "args": ["acp"] }
  }

Nothing is printed to stdout but the protocol. Logs go to the usual log file.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		cwd, _ := cmd.Flags().GetString("cwd")
		if cwd == "" {
			var err error
			if cwd, err = os.Getwd(); err != nil {
				return err
			}
		}
		if err := os.Chdir(cwd); err != nil {
			return err
		}
		cfg, err := config.Load(cwd, false)
		if err != nil {
			return err
		}
		if _, ok := cfg.Agents[config.AgentCoder]; !ok {
			return fmt.Errorf("no AI provider is configured: start gorilla-opencode once in a terminal and choose one")
		}
		conn, err := db.Connect()
		if err != nil {
			return err
		}
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		a, err := app.New(ctx, conn)
		if err != nil {
			return err
		}
		defer a.Shutdown()
		initMCPTools(ctx, a)

		eng := acp.Engine{
			Sessions:    a.Sessions,
			Messages:    a.Messages,
			Permissions: a.Permissions,
			Agent:       a.CoderAgent,
			Receipt:     a.ReceiptText,
		}
		logging.Info("acp: serving on stdio", "cwd", cwd, "version", version.Version)
		return acp.New(eng, version.Version, os.Stdin, os.Stdout).Serve(ctx)
	},
}

func init() {
	acpCmd.Flags().StringP("cwd", "c", "", "Working directory (the editor's session folder is adopted on session/new)")
	rootCmd.AddCommand(acpCmd)
}
