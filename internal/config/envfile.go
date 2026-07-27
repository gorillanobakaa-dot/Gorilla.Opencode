// GORILLA OVERRIDE: this file did not exist upstream.
//
// ~/.config/gorilla-opencode/env is the key file created for desktop
// launches (apps started from the application grid inherit no shell
// environment). It used to be parsed ONLY by `gorilla-opencode launch`,
// the hidden subcommand the .desktop entry runs. That meant a terminal
// user running the binary directly never got those keys: GEMINI_API_KEY
// and friends were invisible, providers were silently disabled, and the
// desktop icon and the shell behaved differently for the same install.
//
// Loading the file from config.Load instead makes both paths identical.
package config

import (
	"os"
	"path/filepath"
	"strings"
)

// EnvFilePath returns ~/.config/gorilla-opencode/env (or the equivalent
// under $XDG_CONFIG_HOME). It sits beside config.json and loadout.json in
// the one clearly-named folder — see GorillaConfigFile.
func EnvFilePath() string {
	return filepath.Join(gorillaConfigBase(), "env")
}

// ParseEnvFile reads simple KEY=VALUE lines from path; '#' starts a
// comment and surrounding quotes are stripped from the value. It returns
// "KEY=VALUE" strings suitable for appending to os.Environ().
//
// Variables already present in the process environment are skipped, so a
// terminal user's explicit exports always win over the file. That is the
// documented contract of the key file and both callers rely on it.
//
// A missing or unreadable file is not an error: it just yields nothing.
func ParseEnvFile(path string) []string {
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

// applyEnvFile parses the key file and exports the entries the process is
// missing, so every later os.Getenv (setProviderDefaults,
// registerLocalEndpoints, backfillProviderKeysFromEnv, ...) sees them.
// Existing environment variables are left untouched.
func applyEnvFile() {
	for _, kv := range ParseEnvFile(EnvFilePath()) {
		if k, v, ok := strings.Cut(kv, "="); ok {
			// ParseEnvFile already filtered out anything already set.
			_ = os.Setenv(k, v)
		}
	}
}
