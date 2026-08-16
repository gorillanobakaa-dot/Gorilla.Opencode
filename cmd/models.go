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
	"sort"
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

// providerAliases are what someone may reasonably type. Kept generous - the
// point is that "/update nvidia" works, not that people learn our spelling.
var providerAliases = map[string]models.ModelProvider{
	"openrouter": models.ProviderOpenRouter, "or": models.ProviderOpenRouter,
	"groq":     models.ProviderGROQ,
	"cerebras": models.ProviderCerebras,
	"openai":   models.ProviderOpenAI, "gpt": models.ProviderOpenAI,
	"xai": models.ProviderXAI, "grok": models.ProviderXAI,
}

var modelsCheckCmd = &cobra.Command{
	Use:   "check [provider...]",
	Short: "Ask your providers whether the models we list still exist",
	Long: `Check the models this program offers against what each provider actually has.

Providers retire models constantly, and a retired model does not fail politely -
it errors the moment you pick it. This asks each provider for its current list
and tells you which of ours have gone.

It only reports. Nothing is changed, because for most providers the published
list is bare identifiers with no context size or price, and replacing a curated
entry with a bare name would be a downgrade dressed up as an update.

  models check                  every provider you have a key for
  models check groq+cerebras    just those
  models check all              every provider that publishes a list

Each check is a few kilobytes. Providers you have no key for are named, with
where to get one - most have a free tier.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg := config.Get()
		want := map[models.ModelProvider]bool{}
		all := len(args) == 1 && strings.EqualFold(args[0], "all")
		for _, a := range args {
			for _, part := range strings.Split(strings.ToLower(a), "+") {
				if p, ok := providerAliases[strings.TrimSpace(part)]; ok {
					want[p] = true
				} else if !strings.EqualFold(part, "all") {
					return fmt.Errorf("unknown provider %q — try one of: openrouter groq cerebras openai xai", part)
				}
			}
		}

		if want[models.ProviderOpenRouter] {
			fmt.Println()
			fmt.Println("  OpenRouter publishes prices and context sizes, so its list can be")
			fmt.Println("  rebuilt outright rather than only checked:")
			fmt.Println()
			fmt.Println("      gorilla-opencode models refresh")
			fmt.Println()
			delete(want, models.ProviderOpenRouter)
			if len(want) == 0 {
				return nil
			}
		}

		var todo []models.ModelProvider
		for p := range models.CatalogueEndpoints {
			configured := false
			if cfg != nil {
				if pc, ok := cfg.Providers[p]; ok && strings.TrimSpace(pc.APIKey) != "" && !pc.Disabled {
					configured = true
				}
			}
			switch {
			case len(want) > 0:
				if want[p] {
					todo = append(todo, p)
				}
			case all || configured:
				todo = append(todo, p)
			}
		}
		sort.Slice(todo, func(i, j int) bool { return todo[i] < todo[j] })

		if len(todo) == 0 {
			fmt.Println()
			fmt.Println("  Nothing to check — no API keys configured for a provider that publishes a list.")
			fmt.Println("  Most have a free tier. Go and get one, then come back:")
			// Free first, deliberately: map order is random and would otherwise
			// put the paid ones at the top of a list read by people with no card.
			var eps []models.CatalogueEndpoint
			for _, ep := range models.CatalogueEndpoints {
				eps = append(eps, ep)
			}
			sort.Slice(eps, func(i, j int) bool {
				if eps[i].Free != eps[j].Free {
					return eps[i].Free
				}
				return eps[i].Name < eps[j].Name
			})
			for _, ep := range eps {
				tag := "paid"
				if ep.Free {
					tag = "FREE"
				}
				fmt.Printf("    %-10s %-4s  %s\n", ep.Name, tag, ep.KeyHint)
			}
			fmt.Println()
			return nil
		}

		fmt.Println()
		for _, p := range todo {
			key := ""
			if cfg != nil {
				if pc, ok := cfg.Providers[p]; ok {
					key = pc.APIKey
				}
			}
			r := models.VerifyProvider(p, key)
			label := r.Name
			if label == "" {
				label = string(p)
			}
			if r.Err != nil {
				fmt.Printf("  %-10s  %v\n", label, r.Err)
				continue
			}
			fmt.Printf("  %-10s  we list %d, they offer %d\n", label, r.Listed, r.Upstream)
			if len(r.Missing) > 0 {
				fmt.Printf("              GONE (these error if you pick them): %s\n", strings.Join(r.Missing, ", "))
			}
			if n := len(r.NewThere); n > 0 {
				fmt.Printf("              %d model(s) they have that we do not list\n", n)
			}
			if len(r.Missing) == 0 {
				fmt.Printf("              all good\n")
			}
		}
		fmt.Println()
		return nil
	},
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
		dir := config.CacheBase()

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
		if res.NoTools > 0 {
			fmt.Printf("  %d skipped: cannot call tools, so they cannot edit files.\n", res.NoTools)
		}
		if res.Batch > 0 {
			fmt.Printf("  %d skipped: batch endpoints — replies arrive hours later, not in a chat.\n", res.Batch)
		}
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
	modelsCmd.AddCommand(modelsRefreshCmd, modelsCheckCmd)
	rootCmd.AddCommand(modelsCmd)
}
