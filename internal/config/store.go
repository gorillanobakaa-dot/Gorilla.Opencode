// GORILLA OVERRIDE: this file did not exist upstream. It is the single owner of
// ~/.config/gorilla-opencode/ — where the path comes from, what mode files are
// written with, and how user override files are read and written.
//
// Two problems it exists to fix:
//
//  1. gorillaConfigBase() (config.go) and loadoutConfigBase() (loadout.go) had
//     byte-identical bodies. Two functions resolving the same directory is two
//     places for the answer to drift.
//  2. config.json holds provider API keys and was written 0o644 — readable by
//     every account on the machine. The sidecars beside it (loadout.json,
//     ratelimit.json, subagents.json) were already 0o600, so the file with the
//     secrets in it was the loosest one in the directory.
package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/opencode-ai/opencode/internal/logging"
)

// File modes for everything under ConfigBase().
//
// secretFileMode is used for any file that can contain credentials —
// config.json (provider apiKey fields) and env (the key file). There is no
// legitimate reason for another account to read either.
const (
	secretFileMode os.FileMode = 0o600
	configDirMode  os.FileMode = 0o755
)

// ConfigBase is ~/.config/gorilla-opencode, or $XDG_CONFIG_HOME/gorilla-opencode
// when that is set. Every path under this directory resolves through here, which
// is also what makes the TestMain isolation in main_test.go total.
func ConfigBase() string {
	if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" {
		return filepath.Join(xdg, gorillaConfigDir)
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", gorillaConfigDir)
}

// DataBase is the XDG data root for durable application data such as sessions
// and the SQLite database.
func DataBase() string {
	if xdg := os.Getenv("XDG_DATA_HOME"); xdg != "" {
		return filepath.Join(xdg, gorillaConfigDir)
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".local", "share", gorillaConfigDir)
}

// CacheBase is the XDG cache root for rebuildable catalogues and other caches.
func CacheBase() string {
	if xdg := os.Getenv("XDG_CACHE_HOME"); xdg != "" {
		return filepath.Join(xdg, gorillaConfigDir)
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".cache", gorillaConfigDir)
}

// StateBase is the XDG state root for logs and other non-cache history.
func StateBase() string {
	if xdg := os.Getenv("XDG_STATE_HOME"); xdg != "" {
		return filepath.Join(xdg, gorillaConfigDir)
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".local", "state", gorillaConfigDir)
}

// PromptsDir is ConfigBase()/prompts, holding user overrides of the system
// prompts. A prompt is not a credential, but it lives under the same roof and is
// written with the same mode for consistency.
func PromptsDir() string { return filepath.Join(ConfigBase(), "prompts") }

// ensureConfigDir creates dir with configDirMode if it does not exist.
func ensureConfigDir(dir string) error {
	if err := os.MkdirAll(dir, configDirMode); err != nil {
		return fmt.Errorf("failed to create %s: %w", dir, err)
	}
	return nil
}

// writeSecretFile writes data to path with secretFileMode, creating the parent
// directory if needed.
//
// os.WriteFile only applies its mode when CREATING a file, so a file left at
// 0o644 by an older version keeps that mode forever. The explicit Chmod tightens
// those on first write. It is deliberately not fatal: on a filesystem that
// cannot represent the mode the data still needs to be saved, and a caller that
// failed to write is a worse outcome than one that wrote with a loose mode.
func writeSecretFile(path string, data []byte) error {
	if err := ensureConfigDir(filepath.Dir(path)); err != nil {
		return err
	}
	if err := os.WriteFile(path, data, secretFileMode); err != nil {
		return err
	}
	if err := os.Chmod(path, secretFileMode); err != nil {
		logging.Warn("could not tighten permissions on config file",
			"path", path, "wanted_mode", secretFileMode, "error", err)
	}
	return nil
}

// readOverride returns the contents of ConfigBase()/<dir>/<name> and whether it
// exists. A missing file is not an error — it means "no override", which is the
// normal case.
func readOverride(dir, name string) (string, bool) {
	data, err := os.ReadFile(filepath.Join(dir, name))
	if err != nil {
		return "", false
	}
	return string(data), true
}

// writeOverride saves an override file.
func writeOverride(dir, name, content string) error {
	return writeSecretFile(filepath.Join(dir, name), []byte(content))
}

// removeOverride deletes an override, restoring whatever the built-in default
// is. Removing something that is already absent succeeds — the caller asked for
// "no override" and that is the resulting state either way.
func removeOverride(dir, name string) error {
	err := os.Remove(filepath.Join(dir, name))
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

// ReadPromptOverride returns the contents of PromptsDir()/name and whether it
// exists. A missing file is the normal case — it means "no override".
func ReadPromptOverride(name string) (string, bool) {
	return readOverride(PromptsDir(), name)
}

// WritePromptOverride saves a user-edited prompt (0o600, dir created as needed).
func WritePromptOverride(name, content string) error {
	return writeOverride(PromptsDir(), name, content)
}

// RemovePromptOverride deletes an override, restoring the built-in default.
// Removing one that is already absent succeeds: the caller asked for "no
// override" and that is the resulting state either way.
func RemovePromptOverride(name string) error {
	return removeOverride(PromptsDir(), name)
}

// PromptOverridePath is the on-disk path for a prompt override, for the UI to
// display and for $EDITOR to open.
func PromptOverridePath(name string) string {
	return filepath.Join(PromptsDir(), name)
}
