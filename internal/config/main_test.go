package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestMain redirects the whole package's test binary at a throwaway config
// directory BEFORE any test runs.
//
// This is not optional hygiene. AddDir, RemoveDir, SetWorkingDir,
// UpsertProviderKey and friends all persist through updateCfgFile, which writes
// viper.ConfigFileUsed() or falls back to GorillaConfigFile() — i.e. the
// developer's REAL ~/.config/gorilla-opencode/config.json. Writing tests here
// without this hook rewrote the real config's "wd" to a t.TempDir() path that
// no longer existed after the run. It happened; hence this file.
//
// gorillaConfigBase() honours XDG_CONFIG_HOME, and configureViper() resolves the
// path inside Load(), so setting the variable before any Load() call is enough.
// Doing it in TestMain rather than per-test means a future test cannot forget.
func TestMain(m *testing.M) {
	tmp, err := os.MkdirTemp("", "gorilla-config-test-*")
	if err != nil {
		panic("cannot create temp config dir: " + err.Error())
	}
	defer os.RemoveAll(tmp)

	os.Setenv("XDG_CONFIG_HOME", filepath.Join(tmp, "config"))
	os.Setenv("XDG_DATA_HOME", filepath.Join(tmp, "data"))
	os.Setenv("XDG_CACHE_HOME", filepath.Join(tmp, "cache"))
	os.Setenv("XDG_STATE_HOME", filepath.Join(tmp, "state"))

	// Guard the guard: if the resolved path is not inside the temp dir, the
	// isolation assumption has broken and tests must not run.
	if got := GorillaConfigFile(); !strings.HasPrefix(got, tmp+string(filepath.Separator)) {
		panic("config isolation failed — tests would write the real config at " + got)
	}

	code := m.Run()
	os.RemoveAll(tmp)
	os.Exit(code)
}
