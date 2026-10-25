package models

// GORILLA OVERRIDE (2026-10-25): an id says where its model runs, by the host
// of the endpoint that listed it. See onmachine.go for the incident.

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestEndpointIDPrefixIsDecidedByTheHost(t *testing.T) {
	for _, u := range []string{
		"http://127.0.0.1:1234/v1", "http://localhost:11434/v1", "http://[::1]:1234/v1",
		"http://127.0.0.2:8080/v1", "http://0.0.0.0:1234/v1",
	} {
		if got := EndpointIDPrefix(u); got != LocalIDPrefix {
			t.Errorf("%s: prefix %q, want %q", u, got, LocalIDPrefix)
		}
	}
	for _, u := range []string{
		"https://integrate.api.nvidia.com/v1", "http://192.168.1.50:1234/v1",
		"http://localhost.example.com/v1", "http://my-local-box:1234/v1", "not a url",
	} {
		if got := EndpointIDPrefix(u); got != RemoteIDPrefix {
			t.Errorf("%s: prefix %q, want %q", u, got, RemoteIDPrefix)
		}
	}
}

// fakeModelServer answers a model listing with ids, and counts requests.
func fakeModelServer(t *testing.T, ids ...string) *httptest.Server {
	t.Helper()
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasSuffix(r.URL.Path, "/models") {
			http.NotFound(w, r)
			return
		}
		var data []map[string]any
		for _, id := range ids {
			data = append(data, map[string]any{"id": id, "object": "model"})
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"object": "list", "data": data})
	}))
	t.Cleanup(ts.Close)
	return ts
}

// A loopback endpoint's models are "local.", a remote host's are "remote.",
// whatever the endpoint is called.
func TestRegisteredIDsSayWhereTheModelRuns(t *testing.T) {
	restore := IsolateLocalEndpointsForTest()
	defer restore()

	lm := fakeModelServer(t, "qwen3.6-35b-a3b")
	if n, _ := RegisterLocalEndpoint("local-sounding-but-loopback", lm.URL+"/v1", ""); n != 1 {
		t.Fatalf("loopback listing registered %d models", n)
	}
	if !IsOnMachine("local.qwen3.6-35b-a3b") {
		t.Fatal("a loopback model is not local.qwen3.6-35b-a3b on this machine")
	}

	// The same listing, served for a remote host name. Every dial goes to the
	// fake server: nothing leaves this machine.
	nim := fakeModelServer(t, "nvidia/nemotron-3-ultra-550b-a55b")
	prev := localHTTP
	localHTTP = &http.Client{Timeout: 5 * time.Second, Transport: &http.Transport{
		DialContext: func(ctx context.Context, network, _ string) (net.Conn, error) {
			return (&net.Dialer{}).DialContext(ctx, network, nim.Listener.Addr().String())
		},
	}}
	defer func() { localHTTP = prev }()
	if n, _ := RegisterLocalEndpoint("local", "http://integrate.api.nvidia.com/v1", ""); n != 1 {
		t.Fatalf("remote listing registered %d models", n)
	}
	if _, ok := SupportedModels["local.nvidia/nemotron-3-ultra-550b-a55b"]; ok {
		t.Fatal("a model of integrate.api.nvidia.com was registered as local.")
	}
	id := ModelID("remote.nvidia/nemotron-3-ultra-550b-a55b")
	if !IsRemoteEndpointModel(id) || IsOnMachine(id) {
		t.Fatalf("%s: remote=%v onMachine=%v", id, IsRemoteEndpointModel(id), IsOnMachine(id))
	}
	if ok, why := AvailableOnMachine(id); ok || !strings.Contains(why, "not on this machine") {
		t.Errorf("AvailableOnMachine(%s) = %v, %q", id, ok, why)
	}
	for _, on := range OnMachineModels() {
		if on == id {
			t.Fatal("a remote model is listed as available on this machine")
		}
	}
	// The old id of that model resolves to the new one.
	if got, ok := RenamedEndpointID("local.nvidia/nemotron-3-ultra-550b-a55b"); !ok || got != id {
		t.Errorf("RenamedEndpointID = %s, %v", got, ok)
	}
	// And a probe verdict filed under the old id still counts.
	setProbeVerdict("local.nvidia/nemotron-3-ultra-550b-a55b", ProbeVerdict{Outcome: ProbeGone, At: time.Now()})
	defer ClearProbeVerdictForTest("local.nvidia/nemotron-3-ultra-550b-a55b")
	if v, ok := ProbeVerdictFor(id); !ok || v.Outcome != ProbeGone {
		t.Errorf("a verdict saved under the old id was lost: %+v %v", v, ok)
	}
	// PreferredLocalModel takes the machine's own model first.
	if got := PreferredLocalModel(); got != "local.qwen3.6-35b-a3b" {
		t.Errorf("PreferredLocalModel = %s, want the loopback model", got)
	}
}

func TestAvailableOnMachineSaysWhyNot(t *testing.T) {
	restore := IsolateLocalEndpointsForTest()
	defer restore()

	if ok, why := AvailableOnMachine("local.gone"); ok || !strings.Contains(why, "not listed") {
		t.Errorf("unknown model: %v %q", ok, why)
	}
	lm := fakeModelServer(t, "text-embedding-nomic-embed-text-v1.5", "gemma-4-26b-a4b-it")
	RegisterLocalEndpoint("lmstudio", lm.URL+"/v1", "")
	if ok, _ := AvailableOnMachine("local.text-embedding-nomic-embed-text-v1.5"); ok {
		t.Error("an embedder is offered as a chat model")
	}
	if ok, why := AvailableOnMachine("local.gemma-4-26b-a4b-it"); !ok {
		t.Errorf("a listed chat model on loopback is not available: %s", why)
	}
	if got := OnMachineEndpointNames(); len(got) != 1 || got[0] != "lmstudio" {
		t.Errorf("OnMachineEndpointNames = %v", got)
	}
}
