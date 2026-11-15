package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/opencode-ai/opencode/internal/config"
	"github.com/opencode-ai/opencode/internal/featurecheck"
	"github.com/opencode-ai/opencode/internal/llm/models"
)

var doctorFeaturesCmd = &cobra.Command{
	Use:   "features",
	Short: "Checklist after a model or tool change: every surface that must know it",
	Long: `Reads the checklist rules (internal/featurecheck/rules.yaml) and checks this
source tree: every surface that names a model must name one the catalogue
still has, every --new-tool must appear on every tool surface, and saved
configs must not point at removed models. Exit 3 when anything is stale.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		root, _ := cmd.Flags().GetString("repo")
		tools, _ := cmd.Flags().GetStringSlice("new-tool")
		// config.Load first: it applies the cached live catalogues.
		saved := map[string]string{}
		if cwd, err := os.Getwd(); err == nil {
			if cfg, err := config.Load(cwd, false); err == nil {
				for name, a := range cfg.Agents {
					saved[string(name)] = string(a.Model)
				}
			}
		}
		cat := map[string]bool{}
		for id, m := range models.SupportedModels {
			cat[string(id)] = true
			if m.APIModel != "" {
				cat[m.APIModel] = true
			}
		}
		found, err := featurecheck.Check(featurecheck.Input{Root: root, Catalogue: cat, NewTools: tools, Saved: saved})
		if err != nil {
			return err
		}
		for _, f := range found {
			fmt.Println(f)
		}
		if len(found) > 0 {
			fmt.Printf("%d stale surface(s). NEXT: update each, then run again.\n", len(found))
			os.Exit(3)
		}
		fmt.Println("all surfaces current. NEXT: nothing.")
		return nil
	},
}

func init() {
	doctorFeaturesCmd.Flags().String("repo", ".", "source tree to check")
	doctorFeaturesCmd.Flags().StringSlice("new-tool", nil, "a new tool name every tool surface must mention")
	doctorCmd.AddCommand(doctorFeaturesCmd)
}
