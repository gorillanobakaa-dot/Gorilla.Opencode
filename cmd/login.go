// GORILLA OVERRIDE: this file did not exist upstream. It adds
// `gorilla-opencode login` — the "Sign in with Google" entry point that
// mirrors the Gemini CLI / Antigravity login menu, so a user with a normal
// Gmail account can use Google's free Code Assist tier. See
// internal/auth/gemini_oauth.go for the flow and the transparency note.
package cmd

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/opencode-ai/opencode/internal/auth"
	"github.com/spf13/cobra"
)

var loginProject string

var loginCmd = &cobra.Command{
	Use:   "login",
	Short: "Sign in to Google (Gemini free tier) via OAuth",
	Long: `Sign in with your Google account to use Gemini through Google's
Code Assist free tier — the same login the Gemini CLI and Antigravity use.
A browser window opens for consent; tokens are stored (0600) in
~/.config/gorilla-opencode/gemini-oauth.json and refreshed automatically.

Two methods:
  1. Google OAuth            — personal Gmail, auto-provisioned free tier.
  2. Use a Google Cloud project — your own GCP project id (billing/quota).`,
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()

		project := loginProject
		if project == "" && !cmd.Flags().Changed("project") {
			// Interactive method picker (matches the CLI screenshot).
			project = pickLoginMethod()
		}

		fmt.Println("\nStarting Google sign-in...")
		creds, err := auth.Login(ctx)
		if err != nil {
			return fmt.Errorf("sign-in failed: %w", err)
		}
		if err := creds.Save(); err != nil {
			return fmt.Errorf("could not save credentials: %w", err)
		}

		who := creds.Email
		if who == "" {
			who = "your Google account"
		}
		fmt.Printf("\nSigned in as %s.\n", who)

		fmt.Println("Setting up Code Assist (free tier)...")
		if err := creds.SetupCodeAssist(ctx, project); err != nil {
			// Login worked even if onboarding hiccups; tell the user plainly.
			fmt.Fprintf(os.Stderr, "\nSigned in, but Code Assist onboarding failed: %v\n", err)
			fmt.Fprintln(os.Stderr, "Your token is saved; you can retry with 'gorilla-opencode login'.")
			return nil
		}

		fmt.Printf("Ready. Project: %s (tier: %s)\n", creds.ProjectID, creds.Tier)
		fmt.Println("\nStart the app and choose a Gemini model — it will use this login.")
		return nil
	},
}

// pickLoginMethod prints the two-option menu and returns a project id
// ("" for the free-tier OAuth method).
func pickLoginMethod() string {
	fmt.Println("\nWelcome to Gorilla OpenCode. You are currently not signed in.")
	fmt.Println("\nSelect login method:")
	fmt.Println("  1. Google OAuth (personal Gmail, free tier)")
	fmt.Println("  2. Use a Google Cloud project")
	fmt.Print("\nEnter 1 or 2 [1]: ")

	reader := bufio.NewReader(os.Stdin)
	choice, _ := reader.ReadString('\n')
	choice = strings.TrimSpace(choice)

	if choice == "2" {
		fmt.Print("Enter your Google Cloud project id: ")
		p, _ := reader.ReadString('\n')
		return strings.TrimSpace(p)
	}
	return ""
}

func init() {
	loginCmd.Flags().StringVar(&loginProject, "project", "", "Google Cloud project id (method 2; omit for free-tier OAuth)")
	rootCmd.AddCommand(loginCmd)
}
