package provider

import (
	"os"
	"testing"

	"github.com/opencode-ai/opencode/internal/llm/models"
)

func TestPromptCacheIsAskedOnlyOfAModelOnThisMachine(t *testing.T) {
	restore := models.IsolateLocalEndpointsForTest()
	defer restore()
	models.RegisterLocalRouteForTest("local.p22-x", "http://127.0.0.1:1234/v1", "")
	models.RegisterLocalRouteForTest("remote.p22-y", "https://integrate.api.nvidia.com/v1", "")
	if len(localPromptCache("local.p22-x")) != 1 {
		t.Error("a loopback model must ask for prompt caching")
	}
	if len(localPromptCache("remote.p22-y")) != 0 {
		t.Error("a remote endpoint must not be sent cache_prompt (NIM answers 400 to unknown parameters)")
	}
	if len(localPromptCache("unregistered")) != 0 {
		t.Error("an unregistered model is not known to be on this machine")
	}
}

func TestTuneRunsOncePerModelAndNeverForRemote(t *testing.T) {
	restore := models.IsolateLocalEndpointsForTest()
	defer restore()
	models.RegisterLocalRouteForTest("local.p22-z", "http://127.0.0.1:1234/v1", "")
	models.RegisterLocalRouteForTest("remote.p22-w", "https://example.com/v1", "")
	dir := t.TempDir()
	log := dir + "/calls"
	script := "#!/bin/sh\necho \"$@\" >> " + log + "\n"
	if err := os.WriteFile(dir+"/fieldkit", []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+":"+os.Getenv("PATH"))
	tunedOnce.Range(func(k, _ any) bool { tunedOnce.Delete(k); return true })

	ensureTuned(t.Context(), "local.p22-z", "p22-z")
	ensureTuned(t.Context(), "local.p22-z", "p22-z")
	ensureTuned(t.Context(), "remote.p22-w", "p22-w")
	b, _ := os.ReadFile(log)
	if string(b) != "models tune p22-z --apply\n" {
		t.Fatalf("fieldkit calls = %q, want exactly one tune --apply for the local model", b)
	}
	t.Setenv("GORILLA_LOCAL_TUNE", "0")
	ensureTuned(t.Context(), "local.p22-z2", "p22-z2")
	if b2, _ := os.ReadFile(log); string(b2) != string(b) {
		t.Error("GORILLA_LOCAL_TUNE=0 must stop the call")
	}
}
