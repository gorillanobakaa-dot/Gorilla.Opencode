package politehttp

import (
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"
)

// reset clears the shared per-host state so one test cannot pace another.
func reset() {
	statesMu.Lock()
	states = map[string]*hostState{}
	statesMu.Unlock()
}

func TestQPSForKnownAndUnknownHosts(t *testing.T) {
	if q := QPSFor("eutils.ncbi.nlm.nih.gov"); q != 3 {
		t.Errorf("NCBI = %v, want 3 (their documented keyless limit)", q)
	}
	if q := QPSFor("EUTILS.NCBI.NLM.NIH.GOV"); q != 3 {
		t.Errorf("host matching must be case-insensitive, got %v", q)
	}
	if q := QPSFor("export.arxiv.org"); q != 0.33 {
		t.Errorf("arXiv = %v, want 0.33 (they ask for one per three seconds)", q)
	}
	if q := QPSFor("somewhere.nobody.knows"); q != defaultQPS {
		t.Errorf("unknown host = %v, want the timid default %v", q, defaultQPS)
	}
	// A subdomain of a known host inherits, rather than falling to the default.
	if q := QPSFor("beta.api.openalex.org"); q != 10 {
		t.Errorf("subdomain of a known host = %v, want 10", q)
	}
}

// The point of the whole package: consecutive calls to one host are spaced.
func TestWaitSpacesRequestsToTheSameHost(t *testing.T) {
	reset()
	// A fake host with the default budget: 2/sec, so 500ms apart.
	l := &Limiter{} // cross-process off: no temp files in tests
	start := time.Now()
	l.Wait("test.invalid")
	l.Wait("test.invalid")
	elapsed := time.Since(start)

	want := time.Duration(float64(time.Second)/defaultQPS) - 50*time.Millisecond
	if elapsed < want {
		t.Errorf("two calls took %v; the second should have waited ~%v",
			elapsed, time.Duration(float64(time.Second)/defaultQPS))
	}
}

// Different hosts must not queue behind each other. A limiter that serialises
// everything would turn a parallel research run into a sequential one.
func TestDifferentHostsDoNotBlockEachOther(t *testing.T) {
	reset()
	l := &Limiter{}
	start := time.Now()
	l.Wait("a.invalid")
	l.Wait("b.invalid")
	l.Wait("c.invalid")
	if elapsed := time.Since(start); elapsed > 100*time.Millisecond {
		t.Errorf("three different hosts took %v; they should not wait on each other", elapsed)
	}
}

// The documented failure mode: many helpers, one host, same instant.
func TestParallelCallersToOneHostAreSerialised(t *testing.T) {
	reset()
	l := &Limiter{}
	const n = 4
	start := time.Now()
	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); l.Wait("burst.invalid") }()
	}
	wg.Wait()
	elapsed := time.Since(start)

	// n callers means n-1 gaps.
	interval := time.Duration(float64(time.Second) / defaultQPS)
	want := time.Duration(n-1)*interval - 100*time.Millisecond
	if elapsed < want {
		t.Errorf("%d parallel callers finished in %v; expected at least ~%v of spacing",
			n, elapsed, want)
	}
}

// The transport is the integration point, so prove requests really go through
// it and really come back intact.
func TestTransportPacesAndPreservesTheResponse(t *testing.T) {
	reset()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		w.Write([]byte("ok"))
	}))
	defer srv.Close()

	tr := NewTransport(nil)
	tr.Limiter = &Limiter{} // no temp files in tests
	client := &http.Client{Transport: tr, Timeout: 5 * time.Second}

	start := time.Now()
	for i := 0; i < 2; i++ {
		resp, err := client.Get(srv.URL)
		if err != nil {
			t.Fatalf("request %d: %v", i, err)
		}
		if resp.StatusCode != 200 {
			t.Fatalf("request %d: status %d", i, resp.StatusCode)
		}
		resp.Body.Close()
	}
	// 127.0.0.1 is not in the table, so it gets the default budget.
	if elapsed := time.Since(start); elapsed < 300*time.Millisecond {
		t.Errorf("two requests through the transport took %v; they were not paced", elapsed)
	}
}

// A 429 with Retry-After is the host stating its own budget, and it must win
// over the table.
func TestRetryAfterPushesTheNextCallOut(t *testing.T) {
	reset()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Retry-After", "1")
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer srv.Close()

	tr := NewTransport(nil)
	tr.Limiter = &Limiter{}
	client := &http.Client{Transport: tr, Timeout: 5 * time.Second}

	resp, err := client.Get(srv.URL)
	if err != nil {
		t.Fatalf("first: %v", err)
	}
	resp.Body.Close()

	start := time.Now()
	resp2, err := client.Get(srv.URL)
	if err != nil {
		t.Fatalf("second: %v", err)
	}
	resp2.Body.Close()
	if elapsed := time.Since(start); elapsed < 700*time.Millisecond {
		t.Errorf("after Retry-After: 1 the next call waited only %v", elapsed)
	}
}

func TestRetryAfterParsing(t *testing.T) {
	if d := retryAfter("2"); d != 2*time.Second {
		t.Errorf("delay-seconds: got %v", d)
	}
	if d := retryAfter(""); d != 0 {
		t.Errorf("empty: got %v", d)
	}
	if d := retryAfter("not-a-number"); d != 0 {
		t.Errorf("garbage must not panic or block: got %v", d)
	}
	// A host asking for an hour must not freeze a research run for an hour.
	if d := retryAfter("3600"); d != 60*time.Second {
		t.Errorf("3600 should be capped at 60s, got %v", d)
	}
	if d := retryAfter("-5"); d != 0 {
		t.Errorf("negative: got %v", d)
	}
	// The HTTP-date form.
	if d := retryAfter(time.Now().Add(2 * time.Second).UTC().Format(http.TimeFormat)); d <= 0 {
		t.Errorf("HTTP-date form was not understood: got %v", d)
	}
	// A date in the past means "now".
	if d := retryAfter(time.Now().Add(-time.Hour).UTC().Format(http.TimeFormat)); d != 0 {
		t.Errorf("past date should mean no wait, got %v", d)
	}
}

func TestSafeNameCannotEscapeTheTempDirectory(t *testing.T) {
	for _, in := range []string{"../../etc/passwd", "a/b", `c:\windows`, "host:8080"} {
		got := safeName(in)
		for _, bad := range []string{"/", `\`, ":", ".."} {
			if bad == ".." && got == ".." {
				t.Errorf("safeName(%q) = %q", in, got)
			}
			if bad != ".." && contains(got, bad) {
				t.Errorf("safeName(%q) = %q, still contains %q", in, got, bad)
			}
		}
	}
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
