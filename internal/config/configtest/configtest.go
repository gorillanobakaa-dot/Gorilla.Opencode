// Package configtest isolates a test binary from the developer's real config.
//
// GORILLA OVERRIDE: this exists because it has now gone wrong three times. Any
// test that calls config.Load and then any setter — SetExtra, SetWorkingDir,
// UpsertProviderKey, AddDir — writes through updateCfgFile to
// GorillaConfigFile(), which is the REAL ~/.config/gorilla-opencode/config.json
// unless XDG_CONFIG_HOME says otherwise. internal/config had a TestMain guarding
// this; four other packages that call config.Load did not, and a single new
// writing test in internal/tui/components/chat duly wrote a stray key into the
// developer's live config.
//
// It is a separate package rather than an exported helper in config so it cannot
// be reached from production code, and so adding it to a package is one line.
package configtest

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/opencode-ai/opencode/internal/config"
)

// Isolate points config at a throwaway directory and runs the tests.
//
// Use it as the whole body of TestMain:
//
//	func TestMain(m *testing.M) { os.Exit(configtest.Isolate(m)) }
//
// It PANICS rather than continuing if the redirect did not take effect. A test
// run that silently falls back to the real config is worse than one that refuses
// to start, because the damage is invisible until someone notices their settings
// changed.
func Isolate(m *testing.M) int {
	tmp, err := os.MkdirTemp("", "gorilla-cfgtest-*")
	if err != nil {
		panic("configtest: cannot create temp config dir: " + err.Error())
	}
	defer os.RemoveAll(tmp)

	os.Setenv("XDG_CONFIG_HOME", tmp)

	if got := config.GorillaConfigFile(); !strings.HasPrefix(got, tmp+string(filepath.Separator)) {
		panic("configtest: isolation failed — tests would write the real config at " + got)
	}
	return m.Run()
}
