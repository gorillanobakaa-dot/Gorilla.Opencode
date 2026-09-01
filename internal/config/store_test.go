package config

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
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
	assertSecretFile(t, path, info)
}

// assertSecretFile checks the protection that actually exists on this platform.
//
// GORILLA OVERRIDE (2026-09-01): the 0600 assertion is unachievable on Windows,
// and asserting it anyway hid the question rather than answering it.
//
// Go's os.Chmod on Windows sets one bit — the read-only attribute. It does not
// write a DACL, and os.Stat reports a synthesised 0666 for any writable file. So
// this test could only ever fail on Windows, and its failure said nothing about
// whether the API keys were actually exposed.
//
// What protects the file on Windows is its LOCATION: everything under the user's
// profile inherits a DACL granting that user and administrators, and nobody
// else. That is the real control, so that is what is checked — together with the
// file not having been left world-writable in the one way Windows does model.
func assertSecretFile(t *testing.T, path string, info os.FileInfo) {
	t.Helper()
	if runtime.GOOS != "windows" {
		if got := info.Mode().Perm(); got != secretFileMode {
			t.Errorf("config.json mode = %04o, want %04o — it holds API keys", got, secretFileMode)
		}
		return
	}
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skip("cannot determine the user profile directory")
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		t.Fatalf("Abs(%s): %v", path, err)
	}
	rel, err := filepath.Rel(home, abs)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		t.Errorf("config.json is at %s, outside the user profile %s — on Windows that "+
			"location IS the access control, and it holds API keys", abs, home)
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
	// GORILLA OVERRIDE (2026-09-01): the premise of this test is a Unix mode
	// bit. Windows has no 0644 to leave behind and no 0600 to tighten to — Go's
	// Chmod there sets only the read-only attribute — so there is no "loose old
	// file" state to create and nothing this test could observe.
	if runtime.GOOS == "windows" {
		t.Skip("file modes are not how Windows protects a file; see assertSecretFile")
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
	if runtime.GOOS != "windows" {
		if mode := info.Mode().Perm(); mode != secretFileMode {
			t.Errorf("override mode = %04o, want %04o", mode, secretFileMode)
		}
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
