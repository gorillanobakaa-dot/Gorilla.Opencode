// GORILLA OVERRIDE: this file did not exist upstream. It estimates how fast the
// user's link is WITHOUT ever spending a byte to find out.
//
// WHY THERE IS NO SPEED TEST. The obvious design is to download a test file.
// On the connection this program is actually for, that is backwards: at 2 KB/s a
// 100 KB probe costs 50 seconds and real money from a metered allowance, spent
// to tell someone their connection is slow — which they already knew. Worse, it
// would not even be correct. An Iridium round trip is 1-2 seconds, so a small
// probe measures LATENCY, not throughput, and would report a slow-but-usable
// link as far worse than it is.
//
// So nothing here initiates a transfer. It times the transfers that were going
// to happen anyway — fetching a model list, a completion response — and derives
// an estimate for free. Zero extra bytes, on principle.
//
// WHY THE MAXIMUM AND NOT THE AVERAGE. Every sample is biased DOWNWARD: it
// includes connection setup, TLS, round-trip latency and any time the server
// spent thinking, none of which is the wire's fault. A slow sample can mean a
// slow link OR a busy server; a FAST sample can only happen on a fast link.
// So the best observation is the closest to the truth, and the estimate is
// reported as "at least X" rather than "X" — an honest floor beats a confident
// wrong number.
package config

import (
	"encoding/json"
	"os"
	"sync"
	"time"
)

// Samples shorter than these are latency measurements wearing a throughput
// costume, and are discarded rather than averaged in.
//
// THESE NUMBERS WERE WRONG ONCE, AND THE WAY THEY WERE WRONG IS INSTRUCTIVE.
// The first version demanded 16 KB. On a 2 KB/s Iridium link a transfer must run
// EIGHT SECONDS to reach 16 KB, so the threshold silently excluded the slowest
// links — the exact users the whole profile ladder exists for. The gate had been
// written from a fast-link assumption, which is the same blind spot the feature
// is meant to correct. Caught by a test asserting 2 KB/s maps to Austere.
//
// What actually separates throughput from latency is DURATION, not size: a
// transfer still running after half a second is bounded by the wire, because no
// plausible round trip is that long twice over. 4 KB is kept only as a floor
// against pathologically tiny bodies.
//
// The remaining risk is a fast link with a slow server: a small body arriving
// late looks like a slow wire. That is why the estimate takes the MAXIMUM across
// samples — one honest fast observation discards any number of pessimistic ones.
const (
	minSampleBytes    = 4 * 1024
	minSampleDuration = 500 * time.Millisecond
	maxSamples        = 8
)

type linkSample struct {
	KBps float64   `json:"kbps"`
	At   time.Time `json:"at"`
}

var (
	linkMu      sync.RWMutex
	linkSamples []linkSample
	linkLoaded  bool
)

// RecordTransfer notes that n bytes moved in d. Call it from anywhere a real
// transfer completes; it is cheap, safe from any goroutine, and silently
// ignores anything too small to be meaningful.
func RecordTransfer(n int64, d time.Duration) {
	if n < minSampleBytes || d < minSampleDuration {
		return
	}
	kbps := float64(n) / 1024 / d.Seconds()
	if kbps <= 0 {
		return
	}
	linkMu.Lock()
	linkSamples = append(linkSamples, linkSample{KBps: kbps, At: time.Now()})
	if len(linkSamples) > maxSamples {
		linkSamples = linkSamples[len(linkSamples)-maxSamples:]
	}
	snapshot := append([]linkSample(nil), linkSamples...)
	linkMu.Unlock()
	saveLinkSamples(snapshot)
}

// EstimatedKBps reports the best observed throughput, and whether there is one.
// The value is a FLOOR: the link is at least this fast.
func EstimatedKBps() (float64, bool) {
	loadLinkSamples()
	linkMu.RLock()
	defer linkMu.RUnlock()
	best := 0.0
	for _, s := range linkSamples {
		if s.KBps > best {
			best = s.KBps
		}
	}
	return best, best > 0
}

// profileCeilings is the upper bound of each profile's band, in KB/s, in the
// same order as ConnProfiles. A measurement picks the first band it fits under.
//
// The top entry is deliberately huge rather than the band's nominal 5 MB/s
// start: Unconstrained is the catch-all for "faster than everything else", and
// a link that exceeds every band must still land somewhere.
var profileCeilings = []float64{9, 60, 250, 5000, 1 << 30}

// RecommendProfile maps a measured speed to a profile. Returns false when there
// is no measurement — in which case the caller must say so rather than invent a
// recommendation, because a made-up suggestion on this screen is worse than none.
func RecommendProfile() (ConnProfileID, float64, bool) {
	kbps, ok := EstimatedKBps()
	if !ok {
		return "", 0, false
	}
	for i, ceil := range profileCeilings {
		if kbps <= ceil {
			return ConnProfiles[i].ID, kbps, true
		}
	}
	return ConnProfiles[len(ConnProfiles)-1].ID, kbps, true
}

// profileIndex is a profile's rung on the ladder, 0 = slowest.
func profileIndex(id ConnProfileID) int {
	for i, p := range ConnProfiles {
		if p.ID == id {
			return i
		}
	}
	return -1
}

// ProfileMismatch reports how many rungs the recommendation is from the active
// profile. Positive means the link is FASTER than the profile assumes (the user
// is being needlessly patient and frugal); negative means SLOWER (turns will
// fail). Only meaningful when ok is true.
//
// The caller uses the magnitude to decide whether to interrupt someone. One rung
// is inside the noise of a single free sample and must never trigger a prompt;
// two or more means something really changed — a flight, a boat, a different
// SIM — which is exactly when being asked is welcome rather than a nag.
func ProfileMismatch() (rungs int, ok bool) {
	rec, _, ok := RecommendProfile()
	if !ok {
		return 0, false
	}
	ri, ci := profileIndex(rec), profileIndex(CurrentConnProfile().ID)
	if ri < 0 || ci < 0 {
		return 0, false
	}
	return ri - ci, true
}

// ShouldOfferProfilePicker is the whole trigger policy in one place: first run,
// or a mismatch of two rungs or more. Everything else is silence.
func ShouldOfferProfilePicker() bool {
	if !profileEverChosen() {
		return true
	}
	rungs, ok := ProfileMismatch()
	if !ok {
		return false
	}
	if rungs < 0 {
		rungs = -rungs
	}
	return rungs >= 2
}

// ---- persistence, sharing connection.json with the profile itself ----

type linkFile struct {
	Profile string       `json:"profile,omitempty"`
	Chosen  bool         `json:"chosen,omitempty"`
	Samples []linkSample `json:"samples,omitempty"`
}

func readLinkFile() linkFile {
	var f linkFile
	if data, err := os.ReadFile(connProfilePath()); err == nil {
		_ = json.Unmarshal(data, &f)
	}
	return f
}

func writeLinkFile(f linkFile) error {
	data, err := json.MarshalIndent(f, "", "  ")
	if err != nil {
		return err
	}
	// 0700, not 0755: this directory holds config.json, which carries provider
	// API keys. 0755 lets any other account on the machine list it. Flagged by
	// gosec on the diff and it is right.
	if err := os.MkdirAll(ConfigBase(), 0o700); err != nil {
		return err
	}
	return os.WriteFile(connProfilePath(), append(data, '\n'), 0o600)
}

func loadLinkSamples() {
	linkMu.Lock()
	defer linkMu.Unlock()
	if linkLoaded {
		return
	}
	linkLoaded = true
	linkSamples = readLinkFile().Samples
}

func saveLinkSamples(s []linkSample) {
	f := readLinkFile()
	f.Samples = s
	_ = writeLinkFile(f)
}

// profileEverChosen distinguishes "running on the shipped default" from "the
// user looked at the picker and picked this". Without it, first run cannot be
// told apart from a deliberate choice of the default.
func profileEverChosen() bool { return readLinkFile().Chosen }

// MarkProfileChosen records that the user made an explicit choice, so the picker
// stops volunteering itself.
func MarkProfileChosen() error {
	f := readLinkFile()
	f.Chosen = true
	return writeLinkFile(f)
}
