// GORILLA OVERRIDE: `gorilla-opencode models refresh-antigravity`.
//
// The Antigravity list was five models typed out by hand from `agy models`
// (client 1.1.10). Measured 2026-08-14 the backend served twenty usable ones,
// and Gemini 3.7 was unreachable purely because nobody had retyped it.
//
// Separate command from `models refresh` on purpose. That one is anonymous and
// hits openrouter.ai; this one needs the user's own Google login and hits
// daily-cloudcode-pa. Folding them together would make an account-less refresh
// silently start requiring an account.
package cmd

import (
	"context"
	"fmt"
	"os"

	"github.com/opencode-ai/opencode/internal/auth"
	"github.com/opencode-ai/opencode/internal/config"
	"github.com/opencode-ai/opencode/internal/llm/models"
	"github.com/spf13/cobra"
)

var modelsRefreshAntigravityCmd = &cobra.Command{
	Use:   "refresh-antigravity",
	Short: "Ask Google Antigravity which models your account can actually use",
	Long: `Ask the Antigravity backend for the current list of models.

The list that ships with this program was correct on the day it was built.
Google adds and retires models constantly, so it goes stale - and a model that
is missing from the list is simply unreachable, however new it is.

This needs the Antigravity login you already have (nothing is sent about you
beyond the request itself) and downloads roughly 40 KB. If it fails, the list
you already have keeps working.

Why not just copy what the 'agy' client prints: measured 2026-08-14, three of
the model names it displays for Gemini 3.7 are rejected by the backend with
NOT_FOUND. It expands one entry into three tier names for its own display, and
those names are not model ids. This command asks the backend directly and takes
the ids it actually honours.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		creds, err := auth.LoadAntigravityCreds()
		if err != nil {
			fmt.Fprintln(os.Stderr, "\n  Not logged in to Antigravity.")
			fmt.Fprintln(os.Stderr, "  Use \"Login with Google (Antigravity)\" in /connect first.")
			return err
		}

		fmt.Println()
		fmt.Println("  Asking daily-cloudcode-pa which models your account can use")
		fmt.Printf("  About 40 KB — %s on a single-digit-KB/s connection.\n", humanSeconds(40*1024))
		fmt.Println("  Uses your existing Antigravity login. Ctrl+C to stop.")
		fmt.Println()

		fetched, err := creds.FetchAvailableModels(context.Background())
		if err != nil {
			fmt.Fprintf(os.Stderr, "\n  Refresh failed: %v\n", err)
			fmt.Fprintln(os.Stderr, "  Your existing model list is untouched and still works.")
			return err
		}

		rows := make([]models.AntigravityRow, 0, len(fetched))
		for id, m := range fetched {
			rows = append(rows, models.AntigravityRow{
				ID:               id,
				DisplayName:      m.DisplayName,
				APIProvider:      m.APIProvider,
				MaxTokens:        m.MaxTokens,
				MaxOutputTokens:  m.MaxOutputTokens,
				SupportsImages:   m.SupportsImages,
				SupportsThinking: m.SupportsThinking,
				IsInternal:       m.IsInternal,
			})
		}

		res, err := models.RefreshAntigravity(config.CacheBase(), rows)
		if err != nil {
			fmt.Fprintf(os.Stderr, "\n  Refresh failed: %v\n", err)
			fmt.Fprintln(os.Stderr, "  Your existing model list is untouched and still works.")
			return err
		}

		fmt.Printf("  The backend offers %d models; %d are usable in a chat.\n", res.Fetched, res.Usable)
		if res.Skipped > 0 {
			fmt.Printf("  %d skipped: internal scaffolding, editor tab-completion, or image-only endpoints.\n", res.Skipped)
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
				fmt.Printf("    %s\n", s)
			}
			fmt.Println()
		}
		report("New since the built-in list", res.Added, 20)
		// "No longer offered" is reported but NOT removed from the catalogue —
		// a model someone has configured must not vanish because one refresh
		// did not mention it.
		report("No longer offered to this account, removed from the picker", res.Removed, 10)

		fmt.Println("  Saved. Pick one with /models.")
		fmt.Println()
		return nil
	},
}

func init() {
	modelsCmd.AddCommand(modelsRefreshAntigravityCmd, modelsDoctorCmd, modelsProbeCmd)
}

// modelsDoctorCmd prints what the model doctor finds, changing nothing unless
// --fix is given. The same checks /update, /model and /provider run.
var modelsDoctorCmd = &cobra.Command{
	Use:   "doctor",
	Short: "Check the model lists, saved keys and agent settings for known faults",
	Long: `Runs every check in the model doctor and prints what it finds.

No network, no AI: the same rules over the same files give the same answer
every time. It looks for an agent set to a model that no longer exists or that
was found retired, saved keys that are a failed paste, two connections to the
same server, rows in the model list that read the same, a ranking that is
upside down, and lists that have not been fetched for a week.

Without --fix nothing is changed. Exit status 3 when a problem is left.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		fix, _ := cmd.Flags().GetBool("fix")
		wd, _ := os.Getwd()
		if _, err := config.Load(wd, false); err != nil {
			return err
		}
		var set config.AgentSetter
		if fix {
			set = config.UpdateAgentModel
		}
		findings := config.RunModelDoctor(config.CacheBase(), set)
		if len(findings) == 0 {
			fmt.Println("Nothing wrong found.")
			return nil
		}
		left := 0
		for _, line := range config.DoctorSummary(findings) {
			fmt.Println(line)
		}
		for _, f := range findings {
			if !f.Fixed {
				left++
			}
		}
		if left > 0 {
			os.Exit(3)
		}
		return nil
	},
}

// modelsProbeCmd asks the likeliest models on each remote connection whether
// they answer, the way /update does.
var modelsProbeCmd = &cobra.Command{
	Use:   "probe",
	Short: "Ask your remote connections' models whether they actually answer",
	Long: `Sends one small request, carrying one ordinary tool, to the likeliest models
on each remote connection you have saved (NVIDIA NIM, Cloudflare, ...) and
records what came back: answered with a tool call, retired, key refused,
provider error. At most ten requests per connection.

A model list is not evidence that a model works. This is.
It is never sent to a model server on this machine.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		wd, _ := os.Getwd()
		if _, err := config.Load(wd, false); err != nil {
			return err
		}
		eps := models.ProbedEndpoints()
		if len(eps) == 0 {
			fmt.Println("No remote connection is saved, so there is nothing to ask.")
			return nil
		}
		for _, ep := range eps {
			fmt.Println(models.ProbeEndpoint(ep, config.CacheBase()).Note())
			if id := models.PreferredOnEndpoint(ep); id != "" {
				fmt.Printf("  default for %s: %s\n", ep, models.SupportedModels[id].APIModel)
			}
		}
		return nil
	},
}

func init() {
	modelsDoctorCmd.Flags().Bool("fix", false, "repair what has exactly one right repair")
}
