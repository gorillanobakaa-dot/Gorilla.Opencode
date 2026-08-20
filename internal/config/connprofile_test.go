package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// Every profile must be internally coherent. A ladder whose numbers do not move
// monotonically is worse than no ladder: the user picks "slower" and gets
// something less patient, with no way to tell.
func TestConnProfileLadderIsMonotonic(t *testing.T) {
	for i := 1; i < len(ConnProfiles); i++ {
		prev, cur := ConnProfiles[i-1], ConnProfiles[i]
		if cur.FirstByte > prev.FirstByte {
			t.Errorf("%s first-byte %v is MORE patient than the slower %s (%v); ladder is inverted",
				cur.ID, cur.FirstByte, prev.ID, prev.FirstByte)
		}
		if cur.StreamStall > prev.StreamStall {
			t.Errorf("%s stall %v > slower %s %v", cur.ID, cur.StreamStall, prev.ID, prev.StreamStall)
		}
		if cur.UploadMB < prev.UploadMB {
			t.Errorf("%s upload budget %.1fMB is SMALLER than the slower %s (%.1fMB)",
				cur.ID, cur.UploadMB, prev.ID, prev.UploadMB)
		}
	}
}

// Austere is the whole point of the feature: it must be the most patient and the
// most frugal, and it must refuse rather than overspend.
func TestAustereIsTheExtreme(t *testing.T) {
	a, ok := lookupConnProfile(ProfileAustere)
	if !ok {
		t.Fatal("austere profile missing")
	}
	// Refusing before sending is universal (budgetTransport), so what makes
	// austere strict is its LIMIT, not a flag. Assert the limit instead.
	if a.UploadMB > 1.0 {
		t.Errorf("austere upload budget %.1fMB is too generous for a 1-9 KB/s link", a.UploadMB)
	}
	if a.FirstByte < 10*time.Minute {
		t.Errorf("austere first-byte %v is too short for a 1-9 KB/s link", a.FirstByte)
	}
	for _, p := range ConnProfiles {
		if p.FirstByte > a.FirstByte {
			t.Errorf("%s is more patient than austere", p.ID)
		}
		if p.UploadMB < a.UploadMB {
			t.Errorf("%s is more frugal than austere", p.ID)
		}
	}
}

// Every profile needs the human-facing fields filled in. A blank Layman makes
// the picker useless to exactly the person it is for.
func TestEveryProfileIsSelfDescribing(t *testing.T) {
	for _, p := range ConnProfiles {
		if p.Name == "" || p.Rate == "" || p.Links == "" || p.Layman == "" {
			t.Errorf("%s has an empty human-facing field", p.ID)
		}
		if p.MaxRetries < 1 {
			t.Errorf("%s allows no attempt at all", p.ID)
		}
	}
}

// An explicit env var is a deliberate act and must beat the profile. This is the
// precedence rule the whole file is built on, so it gets a test.
func TestEnvOverridesProfile(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	t.Setenv("GORILLA_OPENCODE_FIRST_BYTE_TIMEOUT", "7s")
	if got := FirstByteTimeout(); got != 7*time.Second {
		t.Errorf("env override ignored: got %v, want 7s", got)
	}
	os.Unsetenv("GORILLA_OPENCODE_FIRST_BYTE_TIMEOUT")
	if got := FirstByteTimeout(); got == 7*time.Second {
		t.Error("env value persisted after unset; profile should apply")
	}
}

// A profile id we do not recognise must fall back, never invent behaviour.
func TestUnknownProfileFallsBack(t *testing.T) {
	if err := SetConnProfile("no-such-profile"); err == nil {
		t.Error("expected an error for an unknown profile id")
	}
	if CurrentConnProfile().ID == "no-such-profile" {
		t.Error("unknown id was accepted")
	}
}

// A hand-edited or future config must not brick the profile.
func TestCorruptProfileFileKeepsDefault(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, connProfileFileName), []byte("{not json"), 0o600); err != nil {
		t.Fatal(err)
	}
	if got := CurrentConnProfile(); got.ID == "" {
		t.Error("corrupt file produced an empty profile")
	}
}

// The settings row's Default must be a value the row can actually take.
func TestSettingsDefaultIsAValidOption(t *testing.T) {
	def := connProfileNameByID(DefaultConnProfile)
	for _, n := range ConnProfileNames() {
		if n == def {
			return
		}
	}
	t.Errorf("default %q is not among the options %v", def, ConnProfileNames())
}

// The slow profiles must not stream: that is the 27x data saving the whole
// feature turns on. The fast ones must, because live typing is the comfortable
// default and costs nothing that matters on a fat link.
func TestOnlySlowProfilesTurnStreamingOff(t *testing.T) {
	want := map[ConnProfileID]bool{
		ProfileAustere: false, ProfileConstrained: false,
		ProfileModest: true, ProfileBroadband: true, ProfileUnconstrained: true,
	}
	for id, w := range want {
		p, ok := lookupConnProfile(id)
		if !ok {
			t.Fatalf("%s missing", id)
		}
		if p.Stream != w {
			t.Errorf("%s Stream=%v, want %v", id, p.Stream, w)
		}
	}
}

// An explicit env override must beat the profile, same rule as every other knob.
func TestStreamEnvOverridesProfile(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	if err := SetConnProfile(ProfileAustere); err != nil {
		t.Fatal(err)
	}
	if StreamRepliesEnabled() {
		t.Fatal("austere should not stream by default")
	}
	t.Setenv("GORILLA_OPENCODE_STREAM", "1")
	if !StreamRepliesEnabled() {
		t.Error("env override ignored; a deliberate setting must win")
	}
}
