// GORILLA OVERRIDE: this file did not exist upstream. It fixes the
// desktop-launch experience. Apps started from the application grid do
// NOT inherit shell environment variables, so API keys exported in
// .bashrc are invisible there: the program found no providers, exited,
// and the terminal window closed before the error could be read —
// which looks exactly like a crash.
//
// `gorilla-opencode launch` (used by the desktop entry) fixes both:
//  1. it creates the key file (~/.config/gorilla-opencode/env) on
//     first run and shows a held-open welcome if no keys are set yet;
//  2. it loads KEY=VALUE lines from that file and execve-replaces
//     itself with the real program — replace, not spawn, so exactly
//     one process owns the terminal (a lingering parent breaks the
//     TUI under some terminal emulators). re-loading env this way is
//     required because LOCAL_ENDPOINT is read at package-init time.
package cmd

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"

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

// ensureEnvTemplate writes the commented key-file template if it does
// not already exist, and reports whether it had to create it. Shared by
// `install` and `launch` so that BOTH the self-installer and the .deb
// (whose desktop entry also calls `launch`) get the key file — the two
// paths drifted in v0.1.1 and .deb users never got it.
func ensureEnvTemplate() (created bool) {
	path := envFilePath()
	if _, err := os.Stat(path); err == nil {
		return false
	}
	if os.MkdirAll(filepath.Dir(path), 0o755) != nil {
		return false
	}
	if os.WriteFile(path, []byte(envTemplate), 0o600) != nil {
		return false
	}
	return true
}

var launchCmd = &cobra.Command{
	Use:    "launch",
	Hidden: true, // desktop-entry plumbing, not part of the CLI surface
	Short:  "Run with keys from the env file; hold the window open on error",
	RunE: func(cmd *cobra.Command, args []string) error {
		// First desktop launch after a .deb install: create the key
		// file and tell the user where it is, instead of flash-dying.
		if ensureEnvTemplate() {
			fmt.Printf("Welcome to Gorilla OpenCode.\n\nNo API keys are set yet. Put them in:\n  %s\n\n", envFilePath())
			fmt.Println("  NVIDIA NIM:  LOCAL_ENDPOINT=https://integrate.api.nvidia.com/v1")
			fmt.Println("               LOCAL_ENDPOINT_API_KEY=nvapi-...")
			fmt.Println("  Google:      GEMINI_API_KEY=...")
			fmt.Println("  Ollama:      LOCAL_ENDPOINT=http://localhost:11434/v1")
			fmt.Print("\nThen launch again. Press Enter to close... ")
			_, _ = bufio.NewReader(os.Stdin).ReadString('\n')
			return nil
		}

		self, err := os.Executable()
		if err != nil {
			return err
		}
		// GORILLA OVERRIDE: replace this process with the real binary
		// via execve, rather than spawning it as a child. The child
		// approach left `launch` alive holding the terminal while the
		// TUI tried to take control of the same PTY — under a real
		// terminal emulator (gnome-terminal, no `script` PTY in
		// between) the bubbletea TUI cannot become the controlling
		// process and dies instantly: the "flash-die" on the desktop
		// icon. execve leaves exactly one process owning the terminal,
		// which is how every env-loading launcher (env, direnv, …)
		// hands off. We keep the keys loaded from the env file.
		env := append(os.Environ(), loadEnvFile(envFilePath())...)
		if err := syscall.Exec(self, []string{self}, env); err != nil {
			fmt.Fprintf(os.Stderr, "\nfailed to start gorilla-opencode: %v\n", err)
			fmt.Fprint(os.Stderr, "Press Enter to close this window... ")
			_, _ = bufio.NewReader(os.Stdin).ReadString('\n')
			return err
		}
		return nil // unreachable after a successful exec
	},
}

func init() {
	rootCmd.AddCommand(launchCmd)
}
