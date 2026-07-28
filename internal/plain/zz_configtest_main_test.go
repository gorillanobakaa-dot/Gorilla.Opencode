package plain

import (
	"os"
	"testing"

	"github.com/opencode-ai/opencode/internal/config/configtest"
)

// GORILLA OVERRIDE: these tests call config.Load and SetExtra. Without isolation
// those writes land in the developer's real config — which has happened before.
func TestMain(m *testing.M) { os.Exit(configtest.Isolate(m)) }
