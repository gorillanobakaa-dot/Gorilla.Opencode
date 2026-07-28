package styles

import (
	"testing"

	"github.com/opencode-ai/opencode/internal/config/configtest"
)

// This package's tests write config, and writing the REAL config from a test has
// already happened three times in this project. Isolate panics rather than
// politely skipping, because a silent fallback is what let it happen before.
func TestMain(m *testing.M) { configtest.Isolate(m) }
