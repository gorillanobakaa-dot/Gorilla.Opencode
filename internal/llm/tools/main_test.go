package tools

import (
	"os"
	"testing"

	"github.com/opencode-ai/opencode/internal/config"
)

// TestMain loads a minimal config before the package's tests run.
//
// GORILLA FIX (inherited upstream bug): ls_test.go's "handles empty path
// parameter" case calls the ls tool with an empty Path, which falls back to
// config.WorkingDirectory() (ls.go). That function panics with "config not
// loaded" when the global config was never initialised — and this test never
// initialised it. The whole `tools` test binary panicked as a result. It went
// unnoticed because the project has no CI running `go test ./...`. Loading a
// throwaway config here gives WorkingDirectory() a valid value so the tools
// that depend on it can be exercised.
func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "opencode-tools-test")
	if err != nil {
		panic(err)
	}
	if _, err := config.Load(dir, false); err != nil {
		os.RemoveAll(dir)
		panic(err)
	}
	code := m.Run()
	os.RemoveAll(dir)
	os.Exit(code)
}
