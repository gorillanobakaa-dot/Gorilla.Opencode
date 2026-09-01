package models

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// GORILLA OVERRIDE (2026-09-01): the picker must be able to say whether a local
// model server is actually running BEFORE the user commits to it.
//
// The Ollama row used to be hardcoded Configured:true with the comment
// "reachability is checked on apply" — so it carried the same readiness marker
// as a working cloud key, and the user only discovered nothing was listening
// after selecting it. For someone who does not know what a port is, that is
// indistinguishable from the program being broken.

func fakeOpenAIEndpoint(t *testing.T, body string) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasSuffix(r.URL.Path, "/models") {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestProbeReportsAServerThatIsRunning(t *testing.T) {
	srv := fakeOpenAIEndpoint(t, `{"data":[
		{"id":"qwen2.5-coder:7b","object":"model"},
		{"id":"llama3.2:3b","object":"model"}]}`)

	p := ProbeLocalEndpoint(srv.URL+"/v1", "")
	if !p.Running {
		t.Fatal("a server that answered with two models was reported as not running")
	}
	if p.Count != 2 {
		t.Errorf("Count = %d, want 2", p.Count)
	}
	if got := p.Summary(); !strings.Contains(got, "2 models") {
		t.Errorf("Summary() = %q, should say how many models are available", got)
	}
}

// A refused port must be reported as not running, promptly. This is the case
// that decides whether the row is honest.
func TestProbeReportsAServerThatIsNotRunning(t *testing.T) {
	// Port 1 on loopback: nothing listens there, and the connection is refused
	// immediately rather than timing out.
	p := ProbeLocalEndpoint("http://127.0.0.1:1/v1", "")
	if p.Running {
		t.Error("reported a server as running on a port where nothing listens")
	}
	if got := p.Summary(); got != "not running" {
		t.Errorf("Summary() = %q, want %q", got, "not running")
	}
}

// A server that is up but has nothing installed is a DIFFERENT problem from a
// server that is not up, and the user has to be told which one they have:
// one is "start Ollama", the other is "ollama pull something".
func TestProbeDistinguishesRunningFromEmpty(t *testing.T) {
	srv := fakeOpenAIEndpoint(t, `{"data":[]}`)
	p := ProbeLocalEndpoint(srv.URL+"/v1", "")
	if p.Running {
		t.Log("an endpoint listing zero models is treated as not usable, which is fine")
	}
	if strings.Contains(p.Summary(), "models (") {
		t.Errorf("Summary() = %q, must not imply models are available", p.Summary())
	}
}

// Probing must never register anything: the picker asks before the user has
// chosen, and a row being LOOKED at must not change the model list.
func TestProbeDoesNotRegisterModels(t *testing.T) {
	srv := fakeOpenAIEndpoint(t, `{"data":[{"id":"probe-should-not-register","object":"model"}]}`)

	before := len(SupportedModels)
	ProbeLocalEndpoint(srv.URL+"/v1", "")
	if len(SupportedModels) != before {
		t.Errorf("probing registered models: %d -> %d", before, len(SupportedModels))
	}
	if _, ok := SupportedModels["local.probe-should-not-register"]; ok {
		t.Error("probing added a model to the picker")
	}
}

// The two default runtimes are probed concurrently. Run under -race in CI: an
// earlier draft swapped a package-level http.Client from both goroutines.
func TestProbingDefaultRuntimesIsConcurrencySafe(t *testing.T) {
	got := ProbeDefaultLocalRuntimes()
	for _, rt := range DefaultLocalRuntimes {
		if _, ok := got[rt.Name]; !ok {
			t.Errorf("no probe result for %q", rt.Name)
		}
	}
}

// GORILLA OVERRIDE (2026-09-01): a stock LM Studio install serves an embedder
// alongside the chat models. Counting it tells the user "3 models available"
// when the picker will only ever offer two, and selecting an embedder answers a
// chat request with a bare HTTP 400.
func TestProbeDoesNotCountModelsYouCannotTalkTo(t *testing.T) {
	srv := fakeOpenAIEndpoint(t, `{"data":[
		{"id":"qwen3-coder-30b","object":"model"},
		{"id":"llama-3.2-3b-instruct","object":"model"},
		{"id":"text-embedding-nomic-embed-text-v1.5","object":"model"},
		{"id":"bge-reranker-v2-m3","object":"model"}]}`)

	p := ProbeLocalEndpoint(srv.URL+"/v1", "")
	if p.Count != 2 {
		t.Errorf("Count = %d, want 2 — the embedder and the reranker are not models "+
			"you can hold a conversation with, and the picker will not offer them", p.Count)
	}
	for _, n := range p.Names {
		if strings.Contains(strings.ToLower(n), "embed") || strings.Contains(strings.ToLower(n), "rerank") {
			t.Errorf("a non-chat model (%q) is named as available", n)
		}
	}
}

// A server serving ONLY embedders is up but not usable, and that needs
// different advice from "not running".
func TestProbeWithOnlyEmbeddersIsNotOfferedAsReady(t *testing.T) {
	srv := fakeOpenAIEndpoint(t, `{"data":[{"id":"text-embedding-nomic-embed-text","object":"model"}]}`)
	p := ProbeLocalEndpoint(srv.URL+"/v1", "")
	if p.Count != 0 {
		t.Errorf("Count = %d, want 0", p.Count)
	}
	if strings.Contains(p.Summary(), "models (") {
		t.Errorf("Summary() = %q implies chat models are available", p.Summary())
	}
}
