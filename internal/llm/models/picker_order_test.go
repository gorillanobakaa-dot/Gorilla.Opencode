package models

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func withVerdicts(t *testing.T, set map[ModelID]ProbeVerdict) {
	t.Helper()
	probeMu.Lock()
	before := probeVerdicts
	probeVerdicts = map[ModelID]ProbeVerdict{}
	for id, v := range set {
		probeVerdicts[id] = v
	}
	probeMu.Unlock()
	t.Cleanup(func() {
		probeMu.Lock()
		probeVerdicts = before
		probeMu.Unlock()
	})
}

// On 2026-10-05 the picker led with an untested model, then two that had not
// answered, and carried fifteen retired rows and the provider's embedders.
func TestThePickerShowsWhatAnsweredFirstAndLeavesOutWhatCannot(t *testing.T) {
	now := time.Now().UTC()
	withVerdicts(t, map[ModelID]ProbeVerdict{
		"local.a/works-70b":   {Outcome: ProbeWorks, At: now},
		"local.a/timeout-90b": {Outcome: ProbeTimeout, At: now},
		"local.a/retired-34b": {Outcome: ProbeGone, Status: 410, At: now},
		"local.a/err-550b":    {Outcome: ProbeError, Status: 500, At: now},
	})
	shown, retired, notChat := PickerOrder([]ModelID{
		"local.a/untested-8b", "local.a/timeout-90b", "local.a/retired-34b",
		"local.a/nv-embed-1b", "local.a/works-70b", "local.a/err-550b", "local.a/llama-guard-12b",
	})
	if retired != 1 || notChat != 2 {
		t.Fatalf("retired=%d notChat=%d, want 1 and 2", retired, notChat)
	}
	if len(shown) != 4 {
		t.Fatalf("shown %v, want 4 rows", shown)
	}
	if shown[0] != "local.a/works-70b" {
		t.Errorf("first row is %s; the model seen answering must lead", shown[0])
	}
	if shown[1] != "local.a/untested-8b" {
		t.Errorf("second row is %s; an untested model ranks above one that failed its test", shown[1])
	}
	for _, id := range shown {
		if strings.Contains(string(id), "retired") || strings.Contains(string(id), "embed") || strings.Contains(string(id), "guard") {
			t.Errorf("%s must not be shown", id)
		}
	}
}

// A model labelled "provider error 500" by one test question carried a whole
// conversation the same evening, and its row went on saying it had failed.
func TestUseOverridesAFailedTest(t *testing.T) {
	const id ModelID = "local.a/err-550b"
	withVerdicts(t, map[ModelID]ProbeVerdict{id: {Outcome: ProbeError, Status: 500, At: time.Now().UTC()}})
	_, routes, commit := beginRegistryEdit()
	routes[id] = localRouteInfo{Endpoint: "A"}
	commit()
	t.Cleanup(func() {
		_, routes, commit := beginRegistryEdit()
		delete(routes, id)
		commit()
	})

	NoteAnsweredInUse(id, t.TempDir())
	if v, _ := ProbeVerdictFor(id); !v.Usable() {
		t.Fatalf("after answering in use the record is %q", v.Outcome)
	}

	// A model no endpoint serves is none of this record's business.
	NoteAnsweredInUse("gpt-unrelated", t.TempDir())
	if _, ok := ProbeVerdictFor("gpt-unrelated"); ok {
		t.Fatal("a model with no endpoint was recorded")
	}
}

// One 500 is the server having a bad second. It is asked again before the
// model is marked.
func TestAServerErrorIsAskedOnceMore(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if calls.Add(1) == 1 {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[{"message":{"tool_calls":[{"function":{"name":"get_weather"}}]}}]}`))
	}))
	defer srv.Close()
	probeAllowLoopback, probeRetryPause = true, 0
	t.Cleanup(func() { probeAllowLoopback, probeRetryPause = false, 2*time.Second })

	v := ProbeChat(srv.URL, "", "m")
	if !v.Usable() || calls.Load() != 2 {
		t.Fatalf("outcome %q after %d request(s); want works after 2", v.Outcome, calls.Load())
	}

	// A retired model is NOT asked twice: 410 is an answer.
	calls.Store(0)
	gone := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		w.WriteHeader(http.StatusGone)
	}))
	defer gone.Close()
	if v := ProbeChat(gone.URL, "", "m"); v.Outcome != ProbeGone || calls.Load() != 1 {
		t.Fatalf("outcome %q after %d request(s); want gone after 1", v.Outcome, calls.Load())
	}
}

// The labels for failures that may pass must say so.
func TestATransientFailureDoesNotReadAsAVerdict(t *testing.T) {
	at := time.Date(2026, 10, 5, 0, 0, 0, 0, time.UTC)
	for _, v := range []ProbeVerdict{
		{Outcome: ProbeError, Status: 500, At: at}, {Outcome: ProbeTimeout, At: at}, {Outcome: ProbeBusy, At: at},
	} {
		if l := v.Label(); !strings.Contains(l, "may work now") {
			t.Errorf("%s: label %q reads as final", v.Outcome, l)
		}
	}
	if l := (ProbeVerdict{Outcome: ProbeGone, At: at}).Label(); strings.Contains(l, "may work now") {
		t.Errorf("retired is final: %q", l)
	}
}

// No sentence and no price typed into the bundle reaches an endpoint's model.
func TestAnEndpointModelCarriesNoTypedOpinionOrPrice(t *testing.T) {
	for id, meta := range modelMetaByID {
		if meta.Description == "" && meta.CostIn == 0 {
			continue
		}
		m := convertLocalModel(localModel{ID: id})
		if m.CostPer1MIn != 0 || m.CostPer1MOut != 0 {
			t.Fatalf("%s: typed price %.2f/%.2f reached the model", id, m.CostPer1MIn, m.CostPer1MOut)
		}
		if strings.Contains(m.Name, "(") {
			t.Fatalf("%s: a typed remark reached the model's name: %q", id, m.Name)
		}
		if m.Description != "" && !strings.HasPrefix(m.Description, "tested here: ") {
			t.Fatalf("%s: typed description reached the model: %q", id, m.Description)
		}
	}
}
