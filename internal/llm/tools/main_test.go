package tools

import (
	"os"
	"testing"

	"github.com/opencode-ai/opencode/internal/config"
	"github.com/opencode-ai/opencode/internal/config/configtest"
)

// TestMain loads a minimal config before the package's tests run, inside an
// isolated config directory.
//
// GORILLA FIX (inherited upstream bug): ls_test.go's "handles empty path
// parameter" case calls the ls tool with an empty Path, which falls back to
// config.WorkingDirectory() (ls.go). That function panics with "config not
// loaded" when the global config was never initialised — and this test never
// initialised it. The whole `tools` test binary panicked as a result. It went
// unnoticed because the project has no CI running `go test ./...`. Loading a
// throwaway config here gives WorkingDirectory() a valid value so the tools
// that depend on it can be exercised.
//
// GORILLA FIX (2026-08-09): the load above was NOT isolated, so config.Get()
// returned the developer's real ~/.config/gorilla-opencode/config.json. Harmless
// until a config value started changing behaviour: websearch.go reads
// cfg.SearxNGURL, and the moment a real one was configured the SearXNG tests
// stopped talking to their httptest stubs and queried the live instance instead.
// Caught by "want 2 hits, got 8" — a number that only differed by luck.
//
// configtest.IsolateWith redirects XDG_CONFIG_HOME first and runs the load after,
// which is the ordering that matters: config.Load resolves the path when it runs.
func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "opencode-tools-test")
	if err != nil {
		panic(err)
	}
	code := configtest.IsolateWith(m, func() {
		if _, err := config.Load(dir, false); err != nil {
			os.RemoveAll(dir)
			panic(err)
		}
	})
	os.RemoveAll(dir)
	os.Exit(code)
}
