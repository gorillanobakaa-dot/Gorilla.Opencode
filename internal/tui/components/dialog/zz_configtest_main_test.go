package dialog

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/opencode-ai/opencode/internal/config"
	"github.com/opencode-ai/opencode/internal/config/configtest"
	"github.com/opencode-ai/opencode/internal/llm/models"
)

// GORILLA OVERRIDE: tests in this package call config.Load, and some of them
// write settings. Without this, those writes land in the developer's real
// ~/.config/gorilla-opencode/config.json — which is exactly what happened once.
//
// GORILLA FIX (2026-10-25): configtest isolates the config root only. The
// cache root holds this machine's measured history (helper-timing.json,
// research-runs.json), and the helper timing is read once per process, so the
// first test to touch it loaded the owner's real 20 samples and every later
// test saw them, whatever XDG_CACHE_HOME it set. TestAPricedSupervisedDialog-
// FitsTheScreenToo failed on the owner's laptop and passed on CI for that
// reason. Cache, data and state now start empty for the whole package; a test
// that wants history records it.
//
// The same test had a second input from the machine: config.Load, with no
// local endpoint configured, asks Ollama and LM Studio on their default ports
// what they serve. On the owner's laptop LM Studio answered, its Gemma became
// the chat model, the helper model differed from it, and the HELPERS RUN ON
// block added rows CI never rendered. Tests here never probe loopback.
func TestMain(m *testing.M) { os.Exit(isolateMachineState(m)) }

func isolateMachineState(m *testing.M) int {
	models.DefaultLocalRuntimes = nil
	tmp, err := os.MkdirTemp("", "gorilla-dialogtest-*")
	if err != nil {
		panic("dialog tests: cannot create temp state dir: " + err.Error())
	}
	defer os.RemoveAll(tmp)
	for _, v := range []string{"XDG_CACHE_HOME", "XDG_DATA_HOME", "XDG_STATE_HOME"} {
		os.Setenv(v, filepath.Join(tmp, v))
	}
	return configtest.Isolate(m)
}

// The isolation above is itself a rule: without it these tests pass or fail
// by what the machine holds, and they ask a running model server for its list.
func TestThisPackageReadsNoMachineState(t *testing.T) {
	if n := len(models.DefaultLocalRuntimes); n != 0 {
		t.Errorf("%d default local runtimes would be probed on loopback", n)
	}
	home, _ := os.UserHomeDir()
	for name, dir := range map[string]string{
		"cache": config.CacheBase(), "data": config.DataBase(), "state": config.StateBase(),
	} {
		if home != "" && strings.HasPrefix(dir, home) && !strings.HasPrefix(dir, os.TempDir()) {
			t.Errorf("the %s root is the machine's own: %s", name, dir)
		}
	}
}
