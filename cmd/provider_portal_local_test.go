package cmd

import (
	"strings"
	"testing"

	"github.com/opencode-ai/opencode/internal/llm/models"
)

// GORILLA OVERRIDE (2026-09-01): the local-model rows are for people who do not
// know what a port is.
//
// Before this, "Local Ollama (no key)" was hardcoded Configured:true — the same
// readiness marker a working cloud key gets — with the reachability check
// deferred to after selection. Someone without Ollama running picked the row
// that said it was ready, watched it fail, and had no reason to conclude
// anything other than that the program was broken. LM Studio had no row at all
// and could only be reached by typing a URL into a form.

func TestLocalRowSaysWhatIsActuallyRunning(t *testing.T) {
	row := localRuntimeRow(localRuntimeSpec{
		id:    "ollama",
		label: "Ollama",
		port:  "11434",
		probe: models.LocalProbe{Running: true, Count: 3, Names: []string{"qwen2.5-coder", "llama3.2"}},
	})

	if !row.Configured {
		t.Error("a running server with models is not marked configured, so the picker under-reports what works")
	}
	if !strings.Contains(row.Name, "running") {
		t.Errorf("row name %q does not say the server is running", row.Name)
	}
	if !strings.Contains(row.What, "qwen2.5-coder") {
		t.Error("the description does not name a model that is actually available")
	}
	if row.Warning != "" {
		t.Errorf("a working server carries a warning: %q", row.Warning)
	}
}

// The row must NOT claim readiness for something that is not there, and it must
// say what to do about it — both branches, because from here we cannot tell
// "installed but switched off" from "never installed".
func TestLocalRowThatIsNotRunningSaysHowToFixIt(t *testing.T) {
	row := localRuntimeRow(localRuntimeSpec{
		id:      "lmstudio",
		label:   "LM Studio",
		port:    "1234",
		probe:   models.LocalProbe{},
		getIt:   "Install it free from lmstudio.ai",
		startIt: "Open LM Studio and turn on the local server",
	})

	if row.Configured {
		t.Error("a server that is not running is marked configured — this is the bug that made " +
			"the picker say 'ready' and then fail on selection")
	}
	if !strings.Contains(row.Name, "not running") {
		t.Errorf("row name %q does not say the server is not running", row.Name)
	}
	for _, want := range []string{"lmstudio.ai", "turn on the local server", "1234"} {
		if !strings.Contains(row.Warning, want) {
			t.Errorf("the warning does not mention %q — a closed door must say how to open it:\n%s", want, row.Warning)
		}
	}
}

// A server that is up but has nothing loaded must not be offered as ready:
// selecting it produces a provider with no models.
func TestLocalRowWithNoModelsIsNotOfferedAsReady(t *testing.T) {
	row := localRuntimeRow(localRuntimeSpec{
		id:    "ollama",
		label: "Ollama",
		port:  "11434",
		probe: models.LocalProbe{Running: true, Count: 0},
	})
	if row.Configured {
		t.Error("a server with zero models is marked configured; selecting it gives an empty picker")
	}
}

// Both local runtimes must appear in the real picker, free-tagged. LM Studio is
// as common as Ollama and was entirely absent.
func TestBothLocalRuntimesAppearInThePicker(t *testing.T) {
	loadCfg(t)
	rows, _ := providerPortalRows()

	seen := map[string]bool{}
	for _, r := range rows {
		seen[r.ID] = true
		if r.ID == "ollama" || r.ID == "lmstudio" {
			if !r.Free {
				t.Errorf("%s is not tagged Free — running a model on your own machine costs nothing", r.ID)
			}
			if !strings.Contains(r.What, "nothing you type leaves the machine") {
				t.Errorf("%s does not say the privacy benefit, which is the main reason to choose it", r.ID)
			}
		}
	}
	for _, id := range []string{"ollama", "lmstudio"} {
		if !seen[id] {
			t.Errorf("%s has no row in the provider picker", id)
		}
	}
}
