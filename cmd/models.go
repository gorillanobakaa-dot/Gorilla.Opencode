// GORILLA OVERRIDE: `gorilla-opencode models refresh` — let the person running
// the program update the model list themselves.
//
// Before this, the OpenRouter list could only change when someone cut a release.
// It had drifted so far that nine of its twenty-two models no longer existed and
// two of the dead ones were the defaults for every agent: configuring OpenRouter
// produced something that could not answer. Nothing failed loudly, because a
// stale list does not crash, it just stops being true.
//
// Deliberately a command and not a background task. Directive §8: this ships to
// people on single-digit-KB/s links, so nothing fetches unless they ask, the
// cost is stated in bytes AND in seconds at 8 KB/s before anything is
// downloaded, and a failed refresh leaves the built-in list working.
package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/opencode-ai/opencode/internal/config"
	"github.com/opencode-ai/opencode/internal/llm/models"
	"github.com/spf13/cobra"
)

// slowLinkBytesPerSec is the reference connection this project is built for.
// Used to translate a download size into the unit that actually matters to
// someone waiting for it.
const slowLinkBytesPerSec = 8 * 1024

func humanSeconds(bytes int64) string {
	s := float64(bytes) / slowLinkBytesPerSec
	if s < 90 {
		return fmt.Sprintf("~%.0f seconds", s)
	}
	return fmt.Sprintf("~%.0f minutes", s/60)
}

var modelsCmd = &cobra.Command{
	Use:   "models",
	Short: "Inspect and refresh the list of available AI models",
}

var modelsRefreshCmd = &cobra.Command{
	Use:   "refresh",
	Short: "Download the current OpenRouter model list",
	Long: `Download the current list of models from OpenRouter.

The list that ships with this program was correct on the day it was built.
Providers add and retire models constantly, so it goes out of date - and a
retired model does not fail politely, it just returns an error when you pick it.

This downloads roughly 650 KB (about 80 seconds on a very slow connection) and
stores it next to your config. Nothing is sent about you, no account is needed,
and if the download fails the list you already have keeps working.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		dir := config.ConfigBase()

		fmt.Println()
		fmt.Println("  Refreshing the model list from openrouter.ai")
		fmt.Printf("  About 650 KB — %s on a single-digit-KB/s connection.\n", humanSeconds(650*1024))
		fmt.Println("  Nothing about you is sent. No account needed. Ctrl+C to stop.")
		fmt.Println()

		res, err := models.RefreshOpenRouter(dir)
		if err != nil {
			// Say plainly that nothing was broken by the failure — otherwise a
			// red error message reads as "you have now damaged your install".
			fmt.Fprintf(os.Stderr, "\n  Refresh failed: %v\n", err)
			fmt.Fprintln(os.Stderr, "  Your existing model list is untouched and still works.")
			return err
		}

		fmt.Printf("  Downloaded %d models (%.0f KB).\n", res.Fetched, float64(res.Bytes)/1024)
		fmt.Printf("  %d can be used by an agent; %d of those are FREE.\n", res.Usable, res.Free)
		fmt.Printf("  %d were skipped — they cannot call tools, so they cannot edit files.\n",
			res.Fetched-res.Usable)
		fmt.Println()

		report := func(label string, items []string, limit int) {
			if len(items) == 0 {
				return
			}
			fmt.Printf("  %s (%d):\n", label, len(items))
			for i, s := range items {
				if i == limit {
					fmt.Printf("    … and %d more\n", len(items)-limit)
					break
				}
				fmt.Printf("    %s\n", strings.TrimPrefix(s, "openrouter."))
			}
			fmt.Println()
		}
		report("New", res.Added, 10)
		report("No longer offered", res.Removed, 10)
		report("Price changed", res.PriceChanged, 10)

		if len(res.Added)+len(res.Removed)+len(res.PriceChanged) == 0 {
			fmt.Println("  Nothing changed — your list was already current.")
			fmt.Println()
		}
		fmt.Println("  Done. Pick a model with /models inside the app.")
		fmt.Println()
		return nil
	},
}

func init() {
	modelsCmd.AddCommand(modelsRefreshCmd)
	rootCmd.AddCommand(modelsCmd)
}
