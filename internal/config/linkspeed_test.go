package config

import (
	"testing"
	"time"
)

func resetLink(t *testing.T) {
	t.Helper()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	linkMu.Lock()
	linkSamples = nil
	linkLoaded = true // skip disk load; the temp dir is empty anyway
	linkMu.Unlock()
}

// A tiny or instant transfer measures round-trip latency, not throughput.
// Counting it would misreport a fine link as terrible, or a slow one as fast.
func TestTinyAndInstantSamplesAreIgnored(t *testing.T) {
	resetLink(t)
	RecordTransfer(200, 2*time.Second)         // too few bytes
	RecordTransfer(1<<20, 10*time.Millisecond) // too fast to be real
	if _, ok := EstimatedKBps(); ok {
		t.Error("a latency measurement was accepted as throughput")
	}
}

// The estimate must be the BEST sample, not the average: every sample is biased
// downward by setup and server think-time, so the fastest is closest to truth.
func TestEstimateUsesTheBestSample(t *testing.T) {
	resetLink(t)
	RecordTransfer(100*1024, 10*time.Second) // 10 KB/s
	RecordTransfer(100*1024, 2*time.Second)  // 50 KB/s
	RecordTransfer(100*1024, 20*time.Second) // 5 KB/s
	got, ok := EstimatedKBps()
	if !ok {
		t.Fatal("no estimate")
	}
	if got < 49 || got > 51 {
		t.Errorf("got %.1f KB/s, want ~50 (the best sample, not the mean)", got)
	}
}

func TestRecommendationMapsSpeedToProfile(t *testing.T) {
	cases := []struct {
		kbps float64
		want ConnProfileID
	}{
		{2, ProfileAustere},           // Iridium
		{9, ProfileAustere},           // top of the austere band
		{25, ProfileConstrained},      // EDGE
		{120, ProfileModest},          // BGAN
		{1500, ProfileBroadband},      // HSPA+
		{40000, ProfileUnconstrained}, // Starlink
	}
	for _, c := range cases {
		resetLink(t)
		RecordTransfer(int64(c.kbps*1024*4), 4*time.Second)
		got, _, ok := RecommendProfile()
		if !ok {
			t.Fatalf("%.0f KB/s: no recommendation", c.kbps)
		}
		if got != c.want {
			t.Errorf("%.0f KB/s -> %s, want %s", c.kbps, got, c.want)
		}
	}
}

// With no measurement the caller must be told so, not handed a guess.
func TestNoSamplesMeansNoRecommendation(t *testing.T) {
	resetLink(t)
	if _, _, ok := RecommendProfile(); ok {
		t.Error("invented a recommendation with no data")
	}
}

// The trigger policy is the thing that decides whether to interrupt someone.
// One rung must never nag; two must.
func TestPickerTriggerPolicy(t *testing.T) {
	resetLink(t)
	if !ShouldOfferProfilePicker() {
		t.Error("first run should offer the picker")
	}
	if err := MarkProfileChosen(); err != nil {
		t.Fatal(err)
	}
	if err := SetConnProfile(ProfileModest); err != nil { // rung 2
		t.Fatal(err)
	}

	linkMu.Lock()
	linkSamples = nil
	linkMu.Unlock()
	RecordTransfer(30*1024*4, 4*time.Second) // ~30 KB/s -> Constrained, one rung
	if ShouldOfferProfilePicker() {
		t.Error("a one-rung difference must not interrupt the user")
	}

	linkMu.Lock()
	linkSamples = nil
	linkMu.Unlock()
	RecordTransfer(3*1024*4, 4*time.Second) // ~3 KB/s -> Austere, two rungs
	if !ShouldOfferProfilePicker() {
		t.Error("a two-rung difference should offer the picker")
	}
}

// Saving a profile must not erase the samples or the chosen flag: they share one
// file, and losing them would look like a trigger bug rather than a lost field.
func TestSavingProfileKeepsSamplesAndChosenFlag(t *testing.T) {
	resetLink(t)
	RecordTransfer(100*1024, 4*time.Second)
	if err := MarkProfileChosen(); err != nil {
		t.Fatal(err)
	}
	if err := SetConnProfile(ProfileAustere); err != nil {
		t.Fatal(err)
	}
	f := readLinkFile()
	if len(f.Samples) == 0 {
		t.Error("samples were erased by saving the profile")
	}
	if !f.Chosen {
		t.Error("chosen flag was erased by saving the profile")
	}
	if f.Profile != string(ProfileAustere) {
		t.Errorf("profile not saved: %q", f.Profile)
	}
}
