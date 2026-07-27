package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/viper"
)

// writeEnvFile puts content at the path EnvFilePath() resolves to. TestMain
// has already pointed XDG_CONFIG_HOME at a temp dir, so this never touches the
// developer's real ~/.config/gorilla-opencode/env.
func writeEnvFile(t *testing.T, content string) {
	t.Helper()
	path := EnvFilePath()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write env file: %v", err)
	}
	t.Cleanup(func() { os.Remove(path) })
}

// EnvFilePath must land beside config.json in the one gorilla-opencode folder,
// and must honour XDG_CONFIG_HOME — the test isolation in main_test.go depends
// on that, and so does cmd/launch.go, which now shares this path.
func TestEnvFilePathHonoursXDGConfigHome(t *testing.T) {
	xdg := os.Getenv("XDG_CONFIG_HOME")
	if xdg == "" {
		t.Fatal("XDG_CONFIG_HOME unset — TestMain isolation is not in effect")
	}
	want := filepath.Join(xdg, gorillaConfigDir, "env")
	if got := EnvFilePath(); got != want {
		t.Errorf("EnvFilePath() = %q, want %q", got, want)
	}
	if got, cfgFile := filepath.Dir(EnvFilePath()), filepath.Dir(GorillaConfigFile()); got != cfgFile {
		t.Errorf("env file dir %q != config.json dir %q", got, cfgFile)
	}
}

func TestParseEnvFile(t *testing.T) {
	writeEnvFile(t, `# a comment
GORILLA_TEST_PLAIN=value

  GORILLA_TEST_SPACED  =  spaced value
GORILLA_TEST_DQUOTED="quoted"
GORILLA_TEST_SQUOTED='quoted'
GORILLA_TEST_URL=https://example.com/v1?a=b
GORILLA_TEST_EMPTY=
=novalue
not_a_pair
`)
	for _, k := range []string{
		"GORILLA_TEST_PLAIN", "GORILLA_TEST_SPACED", "GORILLA_TEST_DQUOTED",
		"GORILLA_TEST_SQUOTED", "GORILLA_TEST_URL", "GORILLA_TEST_EMPTY",
	} {
		t.Setenv(k, "") // empty counts as unset, and restores after the test
	}

	got := map[string]string{}
	for _, kv := range ParseEnvFile(EnvFilePath()) {
		k, v, _ := strings.Cut(kv, "=")
		got[k] = v
	}

	want := map[string]string{
		"GORILLA_TEST_PLAIN":   "value",
		"GORILLA_TEST_SPACED":  "spaced value",
		"GORILLA_TEST_DQUOTED": "quoted",
		"GORILLA_TEST_SQUOTED": "quoted",
		"GORILLA_TEST_URL":     "https://example.com/v1?a=b",
		"GORILLA_TEST_EMPTY":   "",
	}
	for k, v := range want {
		if got[k] != v {
			t.Errorf("%s = %q, want %q", k, got[k], v)
		}
	}
	if len(got) != len(want) {
		t.Errorf("parsed %d entries (%v), want %d — a comment or malformed line leaked through", len(got), got, len(want))
	}
}

func TestParseEnvFileMissingFileIsNotAnError(t *testing.T) {
	if got := ParseEnvFile(filepath.Join(t.TempDir(), "nope")); got != nil {
		t.Errorf("ParseEnvFile(missing) = %v, want nil", got)
	}
}

// The documented contract of the key file: an explicit export in the user's
// shell always beats the file. Breaking this would make a terminal user's
// `GEMINI_API_KEY=... gorilla-opencode` silently use the stale file value.
func TestParseEnvFileProcessEnvironmentWins(t *testing.T) {
	writeEnvFile(t, "GORILLA_TEST_WINNER=from-file\nGORILLA_TEST_LOSER=from-file\n")
	t.Setenv("GORILLA_TEST_WINNER", "from-shell")
	t.Setenv("GORILLA_TEST_LOSER", "")

	got := map[string]string{}
	for _, kv := range ParseEnvFile(EnvFilePath()) {
		k, v, _ := strings.Cut(kv, "=")
		got[k] = v
	}
	if _, ok := got["GORILLA_TEST_WINNER"]; ok {
		t.Error("value already in the environment was overridden by the file")
	}
	if got["GORILLA_TEST_LOSER"] != "from-file" {
		t.Errorf("unset variable not taken from the file: %q", got["GORILLA_TEST_LOSER"])
	}
}

// applyEnvFile is what makes a plain terminal run behave like a desktop
// launch: config.Load exports the file's entries so every later os.Getenv —
// setProviderDefaults, registerLocalEndpoints, backfillProviderKeysFromEnv —
// sees the keys. Before this, only the hidden `launch` subcommand loaded them.
func TestApplyEnvFileExportsMissingVarsAndFeedsProviderDefaults(t *testing.T) {
	writeEnvFile(t, "GEMINI_API_KEY=gemini-from-file\nLOCAL_ENDPOINT=http://localhost:11434/v1\nANTHROPIC_API_KEY=anthropic-from-shell-wins\n")
	t.Setenv("GEMINI_API_KEY", "")
	t.Setenv("LOCAL_ENDPOINT", "")
	t.Setenv("ANTHROPIC_API_KEY", "already-set")

	applyEnvFile()

	if got := os.Getenv("GEMINI_API_KEY"); got != "gemini-from-file" {
		t.Errorf("GEMINI_API_KEY = %q, want %q", got, "gemini-from-file")
	}
	if got := os.Getenv("LOCAL_ENDPOINT"); got != "http://localhost:11434/v1" {
		t.Errorf("LOCAL_ENDPOINT = %q, want the file value", got)
	}
	if got := os.Getenv("ANTHROPIC_API_KEY"); got != "already-set" {
		t.Errorf("ANTHROPIC_API_KEY = %q — the file overrode the environment", got)
	}

	// The point of loading early in Load(): the provider defaults pick it up,
	// so the gemini provider is configured instead of silently disabled.
	setProviderDefaults()
	if got := viper.GetString("providers.gemini.apiKey"); got != "gemini-from-file" {
		t.Errorf("providers.gemini.apiKey = %q, want the key from the env file", got)
	}
}
