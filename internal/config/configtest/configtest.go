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
func Isolate(m *testing.M) int { return IsolateWith(m, nil) }

// IsolateWith is Isolate for packages that must call config.Load themselves.
// setup runs AFTER the redirect is in place and BEFORE any test — the only
// ordering that works, because config.Load resolves the config path when it
// runs, so a package that loads first reads the developer's real file.
//
// GORILLA OVERRIDE (2026-08-09): internal/llm/tools did exactly that. Its
// TestMain called config.Load(tempWorkingDir) with no redirect, so config.Get()
// had been returning the REAL config all along. It went unnoticed for as long as
// no config value changed behaviour — then websearch.go started reading
// cfg.SearxNGURL, a developer configured one, and four stubbed tests silently
// began querying their live SearXNG instead of the httptest server they had set
// up. They failed with "want 2 hits, got 8", which was luck: had the live
// instance happened to return two results, the suite would have passed while
// testing nothing at all.
func IsolateWith(m *testing.M, setup func()) int {
	tmp, err := os.MkdirTemp("", "gorilla-cfgtest-*")
	if err != nil {
		panic("configtest: cannot create temp config dir: " + err.Error())
	}
	defer os.RemoveAll(tmp)

	os.Setenv("XDG_CONFIG_HOME", tmp)

	if got := config.GorillaConfigFile(); !strings.HasPrefix(got, tmp+string(filepath.Separator)) {
		panic("configtest: isolation failed — tests would write the real config at " + got)
	}
	if setup != nil {
		setup()
	}
	return m.Run()
}
