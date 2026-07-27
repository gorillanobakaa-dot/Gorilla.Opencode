package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestConfigBaseHonoursXDGConfigHome(t *testing.T) {
	xdg := os.Getenv("XDG_CONFIG_HOME")
	if xdg == "" {
		t.Fatal("XDG_CONFIG_HOME unset — TestMain isolation is not in effect")
	}
	want := filepath.Join(xdg, gorillaConfigDir)
	if got := ConfigBase(); got != want {
		t.Errorf("ConfigBase() = %q, want %q", got, want)
	}
	// gorillaConfigBase() is retained as an in-package alias; it must not drift
	// from ConfigBase() the way loadoutConfigBase() did.
	if got := gorillaConfigBase(); got != ConfigBase() {
		t.Errorf("gorillaConfigBase() = %q but ConfigBase() = %q — the duplicate is back", got, ConfigBase())
	}
}

// Every path under the config directory must resolve through ConfigBase(), or
// the TestMain isolation has a hole and tests can reach the real config.
func TestEveryConfigPathIsUnderConfigBase(t *testing.T) {
	base := ConfigBase()
	for name, got := range map[string]string{
		"config.json":    GorillaConfigFile(),
		"env":            EnvFilePath(),
		"prompts dir":    PromptsDir(),
		"loadout.json":   loadoutPath(),
		"ratelimit.json": rateLimitPath(),
		"subagents.json": subAgentsPath(),
	} {
		if filepath.Dir(got) != base && got != base {
			t.Errorf("%s resolves to %q, which is not directly under ConfigBase() %q", name, got, base)
		}
	}
}

// config.json carries provider API keys in plain text. It was written 0o644 —
// readable by every account on the machine — while the sidecars beside it were
// already 0o600, making the file with the secrets the loosest in the directory.
func TestConfigFileIsWrittenSecret(t *testing.T) {
	dir := t.TempDir()
	if _, err := Load(dir, false); err != nil {
		t.Fatalf("Load: %v", err)
	}

	if err := updateCfgFile(func(c *Config) { c.Debug = false }); err != nil {
		t.Fatalf("updateCfgFile: %v", err)
	}

	path := GorillaConfigFile()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("config file not written: %v", err)
	}
	if got := info.Mode().Perm(); got != secretFileMode {
		t.Errorf("config.json mode = %04o, want %04o — it holds API keys", got, secretFileMode)
	}
}

// os.WriteFile only applies its mode when CREATING a file, so a config.json left
// at 0o644 by an older version would keep that mode forever. The explicit Chmod
// in writeSecretFile is what tightens it. Without that Chmod this test fails
// while TestConfigFileIsWrittenSecret still passes — which is exactly the gap.
func TestExistingLooseConfigFileIsTightenedOnWrite(t *testing.T) {
	dir := t.TempDir()
	if _, err := Load(dir, false); err != nil {
		t.Fatalf("Load: %v", err)
	}

	path := GorillaConfigFile()
	if err := ensureConfigDir(filepath.Dir(path)); err != nil {
		t.Fatal(err)
	}
	// Simulate the file an older version left behind. The explicit Chmod is
	// required, not belt-and-braces: os.WriteFile does not touch the mode of an
	// existing file, and a previous test in this package may already have created
	// this path at 0600 — which is the very behaviour under test here, and it
	// silently broke this setup on the first attempt.
	if err := os.WriteFile(path, []byte(`{"debug":false}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(path, 0o644); err != nil {
		t.Fatal(err)
	}
	if info, _ := os.Stat(path); info.Mode().Perm() != 0o644 {
		t.Fatalf("setup failed: wanted a 0644 file, got %04o", info.Mode().Perm())
	}

	if err := updateCfgFile(func(c *Config) { c.Debug = false }); err != nil {
		t.Fatalf("updateCfgFile: %v", err)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := info.Mode().Perm(); got != secretFileMode {
		t.Errorf("pre-existing 0644 config.json still %04o after a write, want %04o — an upgraded install keeps leaking keys", got, secretFileMode)
	}
}

func TestOverrideRoundTrip(t *testing.T) {
	dir := PromptsDir()

	if _, ok := readOverride(dir, "coder.txt"); ok {
		t.Fatal("readOverride reported an override that was never written")
	}

	const body = "you are a test prompt\n"
	if err := writeOverride(dir, "coder.txt", body); err != nil {
		t.Fatalf("writeOverride: %v", err)
	}

	got, ok := readOverride(dir, "coder.txt")
	if !ok || got != body {
		t.Errorf("readOverride = (%q, %v), want (%q, true)", got, ok, body)
	}

	info, err := os.Stat(filepath.Join(dir, "coder.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if mode := info.Mode().Perm(); mode != secretFileMode {
		t.Errorf("override mode = %04o, want %04o", mode, secretFileMode)
	}

	if err := removeOverride(dir, "coder.txt"); err != nil {
		t.Fatalf("removeOverride: %v", err)
	}
	if _, ok := readOverride(dir, "coder.txt"); ok {
		t.Error("override still readable after removal")
	}

	// Removing an absent override is the caller asking for "no override", which
	// is the resulting state either way — it must not be an error.
	if err := removeOverride(dir, "coder.txt"); err != nil {
		t.Errorf("removing an absent override returned %v, want nil", err)
	}
}

// writeSecretFile has to create the parent directory, or the first write to a
// fresh install fails and nothing persists.
func TestWriteSecretFileCreatesParentDirectory(t *testing.T) {
	path := filepath.Join(ConfigBase(), "deeply", "nested", "thing.json")
	if err := writeSecretFile(path, []byte("{}")); err != nil {
		t.Fatalf("writeSecretFile: %v", err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Errorf("file not created: %v", err)
	}
	t.Cleanup(func() { os.RemoveAll(filepath.Join(ConfigBase(), "deeply")) })
}
