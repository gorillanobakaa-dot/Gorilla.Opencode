// GORILLA OVERRIDE (2026-09-01): `gorilla-opencode doctor`.
//
// The Windows environment checks used to run on every single launch, printing a
// banner to stdout before cobra had parsed anything — so `--version` and
// `-p "..."` emitted it too, straight into whatever was reading this program's
// output. Checks worth running are worth running on purpose; this is where they
// live now.
package cmd

import (
	"github.com/spf13/cobra"

	"github.com/opencode-ai/opencode/internal/bootstrap"
)

var doctorCmd = &cobra.Command{
	Use:   "doctor",
	Short: "Check this machine for settings and tools that affect Gorilla OpenCode",
	Long: `Reports Windows settings and optional tools that change how well this
program works: long-path support, Developer Mode, Defender exclusions, OneDrive
placeholder files, and the external tools the search, review and OCR tools use.

Everything it reports is optional. Nothing here is required to run.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		bootstrap.RunDoctor()
		return nil
	},
}

func init() {
	rootCmd.AddCommand(doctorCmd)
}
