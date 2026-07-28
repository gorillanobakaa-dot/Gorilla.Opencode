package chat

import (
	"os"
	"testing"

	"github.com/opencode-ai/opencode/internal/config/configtest"
)

// GORILLA OVERRIDE: tests in this package call config.Load, and some of them
// write settings. Without this, those writes land in the developer's real
// ~/.config/gorilla-opencode/config.json — which is exactly what happened once.
func TestMain(m *testing.M) { os.Exit(configtest.Isolate(m)) }
