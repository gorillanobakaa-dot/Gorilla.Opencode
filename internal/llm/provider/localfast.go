package provider

import (
	"context"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/openai/openai-go/option"

	"github.com/opencode-ai/opencode/internal/llm/models"
	"github.com/opencode-ai/opencode/internal/logging"
)

// GORILLA (2026-11-09, plan P22, owner idea I-24): a model on this machine
// should reach its first word fast. Two things are done here, both only for a
// model served on a loopback address (models.IsOnMachine), never for a remote
// endpoint: a prompt-cache request, and one call to Fieldkit's tuner.

// localPromptCache asks llama.cpp / LM Studio to keep the KV cache of the prompt
// it just read and reuse the longest common prefix on the next request. The
// system prompt and the tool schemas come first and are byte-stable turn to
// turn (tools.VisibleTools appends discoveries, never inserts), so the next turn
// re-reads only what is new. `cache_prompt` is llama.cpp's field; a server that
// does not know it ignores an unknown JSON field, which is why it is limited to
// on-machine endpoints we can rely on and not sent to NIM (which answers 400 to
// unknown parameters, see cacheOptions).
func localPromptCache(id models.ModelID) []option.RequestOption {
	if !models.IsOnMachine(id) {
		return nil
	}
	return []option.RequestOption{option.WithJSONSet("cache_prompt", true)}
}

var tunedOnce sync.Map // model name -> struct{}

// tuneTimeout bounds the one call: loading a large model is slow, hanging is not allowed.
const tuneTimeout = 15 * time.Minute

// ensureTuned runs `fieldkit models tune MODEL --apply` once per process and
// model, before the first request, when fieldkit is installed. The tuner reads
// the host and the GGUF, picks the load settings, and loads the model through
// LM Studio's lms only if it is not loaded already (it never loads a second
// copy). Nothing is sent anywhere: both programs are local. Any failure is
// logged and ignored; the request goes on with whatever LM Studio has loaded.
// GORILLA_LOCAL_TUNE=0 switches it off.
func ensureTuned(ctx context.Context, id models.ModelID, apiModel string) {
	if apiModel == "" || !models.IsOnMachine(id) || os.Getenv("GORILLA_LOCAL_TUNE") == "0" {
		return
	}
	if _, dup := tunedOnce.LoadOrStore(apiModel, struct{}{}); dup {
		return
	}
	bin, err := exec.LookPath("fieldkit")
	if err != nil {
		logging.Debug("local tune skipped: fieldkit is not installed")
		return
	}
	ctx, cancel := context.WithTimeout(ctx, tuneTimeout)
	defer cancel()
	out, err := exec.CommandContext(ctx, bin, "models", "tune", apiModel, "--apply").CombinedOutput()
	line := lastLines(string(out), 3)
	if err != nil {
		logging.Debug("local tune did not apply", "model", apiModel, "err", err.Error(), "output", line)
		return
	}
	logging.Debug("local tune applied", "model", apiModel, "output", line)
}

func lastLines(s string, n int) string {
	lines := strings.Split(strings.TrimSpace(s), "\n")
	if len(lines) > n {
		lines = lines[len(lines)-n:]
	}
	return strings.Join(lines, " | ")
}
