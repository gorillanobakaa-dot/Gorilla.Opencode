// GORILLA (2026-10-27, job 06): `gorilla-opencode system-prompt`.
//
// Prints the coder's system prompt exactly as it would be sent from this
// folder on this machine: the base prompt, the <env> block with the startup
// probe (OS, PowerShell edition, CPUs and robocopy's thread limit, the
// embedded toolbox, PowerToys when installed), the "Your tools" table built
// from the real tool set, and the how-to card line. Read-only: no model is
// called, nothing is written.
package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/opencode-ai/opencode/internal/config"
	"github.com/opencode-ai/opencode/internal/llm/agent"
)

var systemPromptCmd = &cobra.Command{
	Use:   "system-prompt",
	Short: "Print the system prompt the coder model is sent, as built on this machine",
	Long: `Prints the coder's system prompt exactly as a session in this folder would send it:
the instructions, the environment block (OS, shell and its edition, CPUs, robocopy's
thread limit, installed tools, the embedded toolbox), the table of your tools and
the how-to cards available here.

No model is called and nothing is written. MCP servers are not started unless
--mcp is given, so their tools are left out by default.

  gorilla-opencode system-prompt
  gorilla-opencode system-prompt --cwd C:\path\to\project --mcp`,
	RunE: func(cmd *cobra.Command, args []string) error {
		cwd, _ := cmd.Flags().GetString("cwd")
		withMCP, _ := cmd.Flags().GetBool("mcp")
		if cwd == "" {
			var err error
			if cwd, err = os.Getwd(); err != nil {
				return err
			}
		}
		if err := os.Chdir(cwd); err != nil {
			return err
		}
		config.SetRunWithoutScreen(true)
		if _, err := config.Load(cwd, false); err != nil {
			// A model server that is not running fails validation, but the
			// prompt does not depend on reaching the model: say so and print it.
			if config.Get() == nil {
				return err
			}
			fmt.Fprintf(cmd.ErrOrStderr(), "note: %v (the prompt below does not depend on it)\n", err)
		}
		fmt.Fprint(cmd.OutOrStdout(), agent.CoderSystemPrompt(!withMCP))
		return nil
	},
}

func init() {
	systemPromptCmd.Flags().StringP("cwd", "c", "", "Working directory (default: the current one)")
	systemPromptCmd.Flags().Bool("mcp", false, "Start the configured MCP servers and include their tools")
	rootCmd.AddCommand(systemPromptCmd)
}
