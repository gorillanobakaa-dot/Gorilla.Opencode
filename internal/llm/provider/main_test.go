package provider

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/opencode-ai/opencode/internal/config"
)

// TestMain points this package's tests at a throwaway config directory before any
// of them run.
//
// This package reads config.Get() on the request path, and since reasoning
// capture became a persisted user setting, tests need to write that setting to
// exercise it. Without isolation, config.Load and SetExtra would resolve to the
// developer's real ~/.config/gorilla-opencode/config.json — which has been
// clobbered by tests twice before in this project. Hence the guard, copied from
// internal/config/main_test.go: if the resolved path is not inside the temp dir,
// refuse to run at all rather than risk it.
// realConfigHome is the developer's actual XDG_CONFIG_HOME, captured before the
// isolation below replaces it.
//
// It exists for exactly one caller: the opt-in live backend probes, which must
// READ a real OAuth credential to talk to a real server. The isolation above
// exists to stop tests WRITING the developer's config; reading one token file
// does not weaken that, and the alternative — skipping the only test that can
// verify an undocumented wire format — is worse. Nothing may write through this.
var realConfigHome = os.Getenv("XDG_CONFIG_HOME")

func TestMain(m *testing.M) {
	tmp, err := os.MkdirTemp("", "gorilla-provider-test-*")
	if err != nil {
		panic("cannot create temp config dir: " + err.Error())
	}
	os.Setenv("XDG_CONFIG_HOME", tmp)

	if got := config.GorillaConfigFile(); !strings.HasPrefix(got, tmp+string(filepath.Separator)) {
		panic("config isolation failed — tests would write the real config at " + got)
	}

	// A loaded config is needed for the extras lookup to see anything other than
	// registry defaults. The working dir is throwaway too.
	work, err := os.MkdirTemp("", "gorilla-provider-work-*")
	if err != nil {
		panic("cannot create temp work dir: " + err.Error())
	}
	if _, err := config.Load(work, false); err != nil {
		panic("config.Load in TestMain: " + err.Error())
	}

	code := m.Run()
	os.RemoveAll(tmp)
	os.RemoveAll(work)
	os.Exit(code)
}

// withReasoningEnabled turns the paid extra on for one test and restores it after.
// Reasoning defaults to OFF — deliberately, because it spends more — so anything
// asserting the reasoning path must opt in explicitly, exactly as a user does.
func withReasoningEnabled(t *testing.T) {
	t.Helper()
	before := config.ExtraEnabled("extras-reasoning-generate")
	if err := config.SetExtra("extras-reasoning-generate", true); err != nil {
		t.Fatalf("enabling reasoning: %v", err)
	}
	t.Cleanup(func() {
		if err := config.SetExtra("extras-reasoning-generate", before); err != nil {
			t.Errorf("restoring reasoning setting: %v", err)
		}
	})
}
