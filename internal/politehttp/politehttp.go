// GORILLA (2026-09-02): ask for less, get more.
//
// Gorilla already handles 429 well — fetch.go reads Retry-After, websearch.go
// reports the failure honestly. What it never did was try not to earn one.
//
// The cost of that is written into this codebase twice. From websearch.go:
//
//	"On 2026-08-07 a model was asked for papers outside PubMed and arXiv ...
//	 every one returned 403, 404 or 429 - and then, cornered, fabricated a
//	 citation table"
//
// and from research-tool.go, on raising the helper cap to eleven:
//
//	"The 429 risk is real and has not gone away — it is now VISIBLE instead
//	 of hidden"
//
// Visible is better than hidden. Not happening is better than either. Eleven
// research helpers hitting one host in the same second is a self-inflicted
// rate limit, and the first thing it costs is the quality of an answer the
// user has already paid for.
//
// The idea is borrowed from polite_http, the Python package behind the
// Science Skills: give every HOST a request budget and make callers wait
// their turn, rather than sprinting into a wall and reading the sign
// afterwards.
//
// WHAT THIS GUARANTEES, precisely, because a limiter that overpromises is
// worse than none:
//
//   - Within one process, the per-host interval is enforced exactly. This is
//     the documented failure: parallel research helpers, one address space.
//   - Between processes — a second Gorilla window — it is BEST EFFORT. A
//     timestamp file is shared through the OS temp directory under a lock
//     that can be stolen if stale. Two processes can still overlap in a
//     narrow race. That is deliberate: a correct cross-process lock that
//     deadlocks when a process is killed would be a worse bug than the one
//     being fixed, and this stays safe when the file cannot be created at
//     all.
package politehttp

import (
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

// perHostQPS is what each host is asked for, not what it can survive.
//
// Where a publisher states a number, that number is used and the source is
// named. Where none is stated the value is deliberately timid: the cost of
// being too slow is a few seconds on a research run, and the cost of being
// too fast is a blocked host and, once already, a fabricated citation table.
var perHostQPS = map[string]float64{
	// NCBI documents 3/sec without an API key and 10/sec with one. Gorilla
	// sends no key, so 3 is the ceiling, and E-utilities is strict about it.
	"eutils.ncbi.nlm.nih.gov": 3,
	"pubmed.ncbi.nlm.nih.gov": 3,
	"pmc.ncbi.nlm.nih.gov":    3,
	"api.ncbi.nlm.nih.gov":    3,

	// arXiv's own terms ask for one request every three seconds. It is the
	// slowest limit here and the one most often ignored.
	"export.arxiv.org": 0.33,

	// EBI asks for "no more than 10 concurrent"; 10/sec is a safe reading and
	// covers EuropePMC, UniProt, InterPro and the rest of the estate.
	"www.ebi.ac.uk":    10,
	"rest.uniprot.org": 10,

	// Crossref's polite pool is generous, but generosity is not an invitation.
	"api.crossref.org":  20,
	"api.openalex.org":  10,
	"api.unpaywall.org": 10,
	"doaj.org":          5,
	"api.biorxiv.org":   5,

	// Structured biology and chemistry.
	"rest.ensembl.org":             15,
	"data.rcsb.org":                10,
	"search.rcsb.org":              10,
	"files.rcsb.org":               10,
	"alphafold.ebi.ac.uk":          5,
	"gnomad.broadinstitute.org":    3,
	"gtexportal.org":               5,
	"www.ebi.ac.uk/chembl":         5,
	"pubchem.ncbi.nlm.nih.gov":     5,
	"api.fda.gov":                  4,
	"api.platform.opentargets.org": 5,
	"string-db.org":                3,
	"reactome.org":                 5,

	// Reference works that are cheap for us and expensive for them.
	"en.wikipedia.org": 5,
	"archive.org":      2,
}

// defaultQPS applies to any host without an entry above. Two per second is
// slower than a human clicking and faster than anything a research run needs.
const defaultQPS = 2.0

// projectName namespaces the shared timestamp files so Gorilla's politeness
// state cannot collide with another tool's in the same temp directory.
const projectName = "gorilla-opencode"

type hostState struct {
	mu   sync.Mutex
	last time.Time
}

var (
	statesMu sync.Mutex
	states   = map[string]*hostState{}
)

// Limiter enforces a per-host minimum interval.
type Limiter struct {
	// CrossProcess enables the best-effort shared timestamp file. Off in
	// tests, where a real temp file would make results depend on whatever ran
	// before them.
	CrossProcess bool
}

func stateFor(host string) *hostState {
	statesMu.Lock()
	defer statesMu.Unlock()
	s := states[host]
	if s == nil {
		s = &hostState{}
		states[host] = s
	}
	return s
}

// QPSFor reports the budget a host is given. Exported so a test can assert
// the table is actually consulted rather than quietly bypassed.
func QPSFor(host string) float64 {
	host = strings.ToLower(host)
	if q, ok := perHostQPS[host]; ok {
		return q
	}
	// A subdomain inherits from its parent: "www.uniprot.org" should not get
	// the timid default when "uniprot.org" is known.
	for known, q := range perHostQPS {
		if strings.HasSuffix(host, "."+known) {
			return q
		}
	}
	return defaultQPS
}

// Wait blocks until this host may be called again.
func (l *Limiter) Wait(host string) {
	if host == "" {
		return
	}
	host = strings.ToLower(host)
	interval := time.Duration(float64(time.Second) / QPSFor(host))

	s := stateFor(host)
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	if !s.last.IsZero() {
		if gap := interval - now.Sub(s.last); gap > 0 {
			time.Sleep(gap)
		}
	}
	if l.CrossProcess {
		l.waitShared(host, interval)
	}
	s.last = time.Now()
}

// waitShared is the between-processes half. Every failure path here returns
// silently: this is an optimisation, and a politeness mechanism that stops
// the program when the temp directory is read-only would be indefensible.
func (l *Limiter) waitShared(host string, interval time.Duration) {
	path := filepath.Join(os.TempDir(), projectName+"-"+safeName(host)+".stamp")
	lock := path + ".lock"

	if !acquire(lock) {
		return // someone else holds it; the in-process limit still applied
	}
	defer os.Remove(lock)

	if b, err := os.ReadFile(path); err == nil {
		if ns, err := strconv.ParseInt(strings.TrimSpace(string(b)), 10, 64); err == nil {
			if gap := interval - time.Since(time.Unix(0, ns)); gap > 0 && gap < 10*time.Second {
				time.Sleep(gap)
			}
		}
	}
	_ = os.WriteFile(path, []byte(strconv.FormatInt(time.Now().UnixNano(), 10)), 0o600)
}

// acquire takes the lock, or steals one left behind by a process that died
// holding it. O_EXCL is atomic on both Windows and POSIX, so no platform
// build tags and no x/sys dependency are needed for this.
func acquire(lock string) bool {
	for i := 0; i < 20; i++ {
		f, err := os.OpenFile(lock, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
		if err == nil {
			f.Close()
			return true
		}
		// A lock older than two seconds belonged to something that is gone.
		// Nothing here holds it for longer than one sleep interval.
		if fi, err := os.Stat(lock); err == nil && time.Since(fi.ModTime()) > 2*time.Second {
			os.Remove(lock)
			continue
		}
		time.Sleep(10 * time.Millisecond)
	}
	return false
}

// safeName keeps a host usable as a filename on Windows, where ':' is illegal
// and a stray path separator would write outside the temp directory.
func safeName(host string) string {
	var b strings.Builder
	for _, r := range host {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9', r == '.', r == '-':
			b.WriteRune(r)
		default:
			b.WriteByte('_')
		}
	}
	return b.String()
}

// Transport wraps a RoundTripper so every request through it waits its turn.
//
// A RoundTripper rather than a helper function so existing call sites do not
// change: anything that already builds an http.Client gets politeness by
// wrapping its Transport, and nothing has to remember to call Wait. A rule
// that must be remembered at nineteen call sites is a rule that will be
// missed at one of them.
type Transport struct {
	Base    http.RoundTripper
	Limiter *Limiter
}

// NewTransport wraps base, defaulting to http.DefaultTransport.
func NewTransport(base http.RoundTripper) *Transport {
	if base == nil {
		base = http.DefaultTransport
	}
	return &Transport{Base: base, Limiter: &Limiter{CrossProcess: true}}
}

func (t *Transport) RoundTrip(req *http.Request) (*http.Response, error) {
	lim := t.Limiter
	if lim == nil {
		lim = &Limiter{}
	}
	lim.Wait(req.URL.Hostname())

	resp, err := t.Base.RoundTrip(req)
	if err != nil || resp == nil {
		return resp, err
	}

	// A 429 or 503 with Retry-After is the host stating its own budget, which
	// beats anything in the table above. Record it so the NEXT caller waits,
	// rather than sleeping here: this request is already answered, and
	// blocking inside RoundTrip would stall a caller that may not even want to
	// retry.
	if resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode == http.StatusServiceUnavailable {
		if d := retryAfter(resp.Header.Get("Retry-After")); d > 0 {
			s := stateFor(strings.ToLower(req.URL.Hostname()))
			s.mu.Lock()
			// Push "last called" into the future by the stated delay.
			if until := time.Now().Add(d); until.After(s.last) {
				s.last = until
			}
			s.mu.Unlock()
		}
	}
	return resp, nil
}

// retryAfter reads both forms the header takes: delay-seconds, and an HTTP
// date. Capped, because a host answering "3600" should not silently freeze a
// research run for an hour.
func retryAfter(v string) time.Duration {
	v = strings.TrimSpace(v)
	if v == "" {
		return 0
	}
	const max = 60 * time.Second
	if secs, err := strconv.Atoi(v); err == nil {
		d := time.Duration(secs) * time.Second
		if d > max {
			return max
		}
		if d < 0 {
			return 0
		}
		return d
	}
	if t, err := http.ParseTime(v); err == nil {
		if d := time.Until(t); d > 0 {
			if d > max {
				return max
			}
			return d
		}
	}
	return 0
}
