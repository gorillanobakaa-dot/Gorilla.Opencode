package models

import (
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// fakeProvider answers /chat/completions per model the way NVIDIA did on
// 2026-10-05: some answer with a tool call, some are retired (410), one fails
// (500), one answers in prose. It counts requests and records the keys it saw.
func fakeProvider(t *testing.T, behaviour map[string]int, hits *int32) *httptest.Server {
	t.Helper()
	// The probe refuses loopback on purpose, and httptest only listens there.
	// A second loopback name the probe does not recognise is not available, so
	// the refusal is lifted for the test server's address alone.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(hits, 1)
		raw, _ := io.ReadAll(r.Body)
		var req struct {
			Model string `json:"model"`
			Tools []any  `json:"tools"`
		}
		_ = json.Unmarshal(raw, &req)
		if len(req.Tools) != 1 {
			t.Errorf("the probe sent %d tools, want exactly one", len(req.Tools))
		}
		if r.Header.Get("Authorization") != "Bearer test-key" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		switch code := behaviour[req.Model]; code {
		case 200:
			_, _ = w.Write([]byte(`{"choices":[{"message":{"tool_calls":[{"function":{"name":"get_weather","arguments":"{\"city\":\"Bucharest\"}"}}]}}]}`))
		case 299: // answers, but in prose
			_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"It is sunny."}}]}`))
		case 0:
			w.WriteHeader(http.StatusNotFound)
		default:
			w.WriteHeader(code)
			_, _ = w.Write([]byte(`{"detail":"Bearer test-key was rejected"}`)) // a provider echoing the key
		}
	}))
	t.Cleanup(srv.Close)
	return srv
}

// allowLoopbackFor lets the probe reach the test server and nothing else.
func allowLoopbackFor(t *testing.T) {
	t.Helper()
	old := probeAllowLoopback
	probeAllowLoopback = true
	t.Cleanup(func() { probeAllowLoopback = old })
}

func registerFake(t *testing.T, name, baseURL string, apiModels ...string) {
	t.Helper()
	savedVerdicts := map[ModelID]ProbeVerdict{}
	probeMu.Lock()
	for k, v := range probeVerdicts {
		savedVerdicts[k] = v
	}
	probeMu.Unlock()
	var ids []ModelID
	for _, api := range apiModels {
		id := ModelID("local." + api)
		SupportedModels[id] = Model{ID: id, Name: api, Provider: ProviderLocal, APIModel: api}
		localRoute[id] = localRouteInfo{BaseURL: baseURL, APIKey: "test-key", Endpoint: name}
		ids = append(ids, id)
	}
	t.Cleanup(func() {
		for _, id := range ids {
			delete(SupportedModels, id)
			delete(localRoute, id)
		}
		probeMu.Lock()
		probeVerdicts = savedVerdicts
		probeMu.Unlock()
	})
}

func TestProbeClassifiesWhatAProviderActuallyAnswers(t *testing.T) {
	allowLoopbackFor(t)
	var hits int32
	srv := fakeProvider(t, map[string]int{"good": 200, "prose": 299, "retired": 410, "broken": 500, "paid": 402, "busy": 429}, &hits)
	for model, want := range map[string]string{
		"good": ProbeWorks, "prose": ProbeNoToolCall, "retired": ProbeGone, "missing": ProbeGone,
		"broken": ProbeError, "paid": ProbeNeedsPay, "busy": ProbeBusy,
	} {
		v := ProbeChat(srv.URL, "test-key", model)
		if v.Outcome != want {
			t.Errorf("%s: outcome %q, want %q", model, v.Outcome, want)
		}
		if strings.Contains(v.Detail, "test-key") {
			t.Errorf("%s: the verdict kept the provider's reply, which repeats the key", model)
		}
		if v.At.IsZero() {
			t.Errorf("%s: a verdict with no date is not a measurement", model)
		}
	}
	if v := ProbeChat(srv.URL, "wrong", "good"); v.Outcome != ProbeRefused {
		t.Errorf("a refused key gave %q", v.Outcome)
	}
	// Only a model seen to call the tool may be a default.
	if !(ProbeVerdict{Outcome: ProbeWorks}).Usable() || (ProbeVerdict{Outcome: ProbeNoToolCall}).Usable() ||
		(ProbeVerdict{Outcome: ProbeBusy}).Usable() || (ProbeVerdict{}).Usable() {
		t.Error("Usable() accepts something that was not seen to work")
	}
}

// A local runtime loads a model when asked for it. The probe must never ask.
func TestAModelServerOnThisMachineIsNeverProbed(t *testing.T) {
	var hits int32
	srv := fakeProvider(t, map[string]int{"m": 200}, &hits)
	if v := ProbeChat(srv.URL, "test-key", "m"); v.Outcome != ProbeUnreached {
		t.Errorf("a loopback endpoint was probed: %q", v.Outcome)
	}
	registerFake(t, "Mine", srv.URL, "m")
	rep := ProbeEndpoint("Mine", t.TempDir())
	if rep.Asked != 0 || rep.Skipped == "" {
		t.Errorf("ProbeEndpoint asked %d model(s) on this machine", rep.Asked)
	}
	if hits != 0 {
		t.Fatalf("%d request(s) reached a model server on this machine", hits)
	}
	for _, u := range []string{"http://localhost:1234/v1", "http://127.0.0.1:11434/v1", "http://[::1]:8080/v1"} {
		if !isLoopback(u) {
			t.Errorf("%s is not recognised as this machine", u)
		}
	}
	if isLoopback("https://integrate.api.nvidia.com/v1") {
		t.Error("a remote endpoint was taken for this machine")
	}
}

// What NVIDIA looked like on 2026-10-05: listed models that are retired, the
// alphabetically first one useless, and the default therefore wrong.
func TestTheDefaultIsAModelThatWasSeenToAnswer(t *testing.T) {
	allowLoopbackFor(t)
	var hits int32
	srv := fakeProvider(t, map[string]int{
		"vendor/small-coder-7b":  410,
		"vendor/big-coder-120b":  410,
		"vendor/glm-next":        500,
		"vendor/qwen-works-30b":  200,
		"vendor/llama-works-70b": 200,
	}, &hits)
	registerFake(t, "Far", srv.URL,
		"01-ai/yi-large", "baai/bge-m3-embed", "vendor/small-coder-7b", "vendor/big-coder-120b",
		"vendor/glm-next", "vendor/qwen-works-30b", "vendor/llama-works-70b")

	// Before any check, the embedder can never be chosen and the choice is the
	// same every time.
	first := PreferredOnEndpoint("Far")
	if strings.Contains(string(first), "embed") || first != PreferredOnEndpoint("Far") {
		t.Fatalf("unchecked default = %s", first)
	}

	dir := t.TempDir()
	rep := ProbeEndpoint("Far", dir)
	if len(rep.Working) != 2 {
		t.Fatalf("working = %v, want the two that answer; report: %s", rep.Working, rep.Note())
	}
	if got := PreferredOnEndpoint("Far"); got != "local.vendor/llama-works-70b" {
		t.Errorf("default after the check = %s, want the larger of the models that answered", got)
	}
	if hits > probeMaxRequests {
		t.Errorf("%d requests sent; the budget is %d", hits, probeMaxRequests)
	}
	if strings.Contains(rep.Note(), "test-key") {
		t.Error("the report contains the key")
	}

	// The record survives a restart, and a model found retired is not asked
	// again within the week.
	probeMu.Lock()
	probeVerdicts = map[ModelID]ProbeVerdict{}
	probeMu.Unlock()
	if n := LoadProbeVerdicts(dir); n == 0 {
		t.Fatal("nothing was saved")
	}
	if v, ok := ProbeVerdictFor("local.vendor/big-coder-120b"); !ok || !v.Dead() {
		t.Error("the retired model's verdict was not kept")
	}
	before := hits
	ProbeEndpoint("Far", dir)
	if again := hits - before; again > 4 {
		t.Errorf("the second check sent %d requests; retired models must not be re-asked", again)
	}
	if got := PreferredOnSameEndpoint("local.vendor/big-coder-120b"); got != "local.vendor/llama-works-70b" {
		t.Errorf("successor of a retired model = %s", got)
	}
}

// A model that never replies is that model's fault; the check must go on to the
// next one. A key that is refused is not: more requests prove nothing.
func TestOneSilentModelDoesNotEndTheCheckButARefusedKeyDoes(t *testing.T) {
	allowLoopbackFor(t)
	old := chatProbeHTTP
	chatProbeHTTP = &http.Client{Timeout: 300 * time.Millisecond}
	defer func() { chatProbeHTTP = old }()

	var hits int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&hits, 1)
		raw, _ := io.ReadAll(r.Body)
		if strings.Contains(string(raw), "silent") {
			time.Sleep(900 * time.Millisecond)
			return
		}
		_, _ = w.Write([]byte(`{"choices":[{"message":{"tool_calls":[{"function":{"name":"get_weather"}}]}}]}`))
	}))
	defer srv.Close()
	registerFake(t, "Slow", srv.URL, "vendor/qwen-silent-900b", "vendor/qwen-ok-10b")
	rep := ProbeEndpoint("Slow", t.TempDir())
	if len(rep.Working) != 1 || rep.Failed["local.vendor/qwen-silent-900b"].Outcome != ProbeTimeout {
		t.Errorf("report: %s", rep.Note())
	}

	hits = 0
	bad := fakeProvider(t, map[string]int{}, &hits)
	registerFake(t, "Refusing", bad.URL, "vendor/qwen-a", "vendor/qwen-b", "vendor/qwen-c")
	for id, r := range localRoute {
		if r.Endpoint == "Refusing" {
			r.APIKey = "wrong"
			localRoute[id] = r
		}
	}
	if rep := ProbeEndpoint("Refusing", t.TempDir()); rep.Asked != 1 {
		t.Errorf("a refused key was tried %d times", rep.Asked)
	}
}

func TestNoTwoModelsOfOneEndpointShareAName(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasSuffix(r.URL.Path, "/v1/models") {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		_, _ = w.Write([]byte(`{"data":[{"id":"nvidia/nemotron-parse","object":"model"},{"id":"nvidia/nemotron-parse-2.0","object":"model"},{"id":"openai/gpt-oss-20b","object":"model"}]}`))
	}))
	defer srv.Close()
	host, port, _ := net.SplitHostPort(strings.TrimPrefix(srv.URL, "http://"))
	_ = host
	base := "http://127.0.0.1:" + port + "/v1"
	defer UnregisterLocalEndpoint(base)

	for round := 0; round < 2; round++ { // twice: the repair must not stack
		if n, _ := RegisterLocalEndpoint("Names", base, ""); n != 3 {
			t.Fatalf("registered %d", n)
		}
		a, b := SupportedModels["local.nvidia/nemotron-parse"].Name, SupportedModels["local.nvidia/nemotron-parse-2.0"].Name
		if a == b {
			t.Fatalf("round %d: two models are both named %q", round, a)
		}
		if strings.Count(a, "[") > 1 {
			t.Fatalf("round %d: the id was appended twice: %q", round, a)
		}
	}
}

func TestSizeAndFamilyAreReadFromTheIDNotTyped(t *testing.T) {
	for id, want := range map[string]float64{
		"nvidia/nemotron-3-super-120b-a12b": 120, "openai/gpt-oss-20b": 20, "qwen/qwen3.5-397b-a17b": 397,
		"moonshotai/kimi-k3": 0, "google/gemma-4-e2b": 2, "x/model-1.5b:free": 1.5,
	} {
		if got := sizeHint(id); got != want {
			t.Errorf("sizeHint(%q) = %v, want %v", id, got, want)
		}
	}
	for id, want := range map[string]int{
		"baai/bge-m3": 9, "nvidia/nemotron-parse-2.0": 9, "nvidia/riva-translate-4b-instruct-v2": 9,
		"moonshotai/kimi-k9": 1, "z-ai/glm-99": 1, "meta/llama-9-70b": 2, "01-ai/yi-large": 3,
	} {
		if got := chatTier(id); got != want {
			t.Errorf("chatTier(%q) = %d, want %d", id, got, want)
		}
	}
}
