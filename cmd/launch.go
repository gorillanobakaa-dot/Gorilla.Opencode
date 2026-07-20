// GORILLA OVERRIDE: this file did not exist upstream. It fixes the
// desktop-launch experience. Apps started from the application grid do
// NOT inherit shell environment variables, so API keys exported in
// .bashrc are invisible there: the program found no providers, exited,
// and the terminal window closed before the error could be read —
// which looks exactly like a crash.
//
// `gorilla-opencode launch` (used by the desktop entry) fixes both:
//  1. it loads KEY=VALUE lines from ~/.config/gorilla-opencode/env
//     (written as a commented template by `install`, chmod 600) and
//     re-runs the real program with them — re-exec is required because
//     LOCAL_ENDPOINT is read at package-init time, before main();
//  2. if the program exits with an error, it holds the window open
//     until Enter is pressed, so the error can actually be read.
package cmd

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

// envFilePath returns ~/.config/gorilla-opencode/env.
func envFilePath() string {
	if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" {
		return filepath.Join(xdg, appBinName, "env")
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", appBinName, "env")
}

// loadEnvFile parses simple KEY=VALUE lines; '#' starts a comment.
// Variables already present in the process environment win, so a
// terminal user's explicit exports are never overridden.
func loadEnvFile(path string) []string {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var extra []string
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		k, v, ok := strings.Cut(line, "=")
		if !ok || strings.TrimSpace(k) == "" {
			continue
		}
		k = strings.TrimSpace(k)
		v = strings.Trim(strings.TrimSpace(v), `"'`)
		if os.Getenv(k) == "" {
			extra = append(extra, k+"="+v)
		}
	}
	return extra
}

const envTemplate = `# Gorilla OpenCode — API keys for desktop launches.
# Lines are KEY=VALUE. '#' starts a comment. This file is read by
# 'gorilla-opencode launch' (the desktop entry) because apps started
# from the application grid do not see variables from your .bashrc.
# Uncomment and fill in what you use:
#
# NVIDIA NIM:
#LOCAL_ENDPOINT=https://integrate.api.nvidia.com/v1
#LOCAL_ENDPOINT_API_KEY=nvapi-...
#
# Google AI Studio (Gemini):
#GEMINI_API_KEY=...
#
# Local Ollama (no key needed):
#LOCAL_ENDPOINT=http://localhost:11434/v1
`

var launchCmd = &cobra.Command{
	Use:    "launch",
	Hidden: true, // desktop-entry plumbing, not part of the CLI surface
	Short:  "Run with keys from the env file; hold the window open on error",
	RunE: func(cmd *cobra.Command, args []string) error {
		self, err := os.Executable()
		if err != nil {
			return err
		}
		child := exec.Command(self)
		child.Stdin, child.Stdout, child.Stderr = os.Stdin, os.Stdout, os.Stderr
		child.Env = append(os.Environ(), loadEnvFile(envFilePath())...)
		runErr := child.Run()
		if runErr != nil {
			fmt.Fprintf(os.Stderr, "\ngorilla-opencode exited with an error: %v\n", runErr)
			fmt.Fprintf(os.Stderr, "Hint: put your API keys in %s\n", envFilePath())
			fmt.Fprint(os.Stderr, "Press Enter to close this window... ")
			_, _ = bufio.NewReader(os.Stdin).ReadString('\n')
		}
		return nil
	},
}

func init() {
	rootCmd.AddCommand(launchCmd)
}
