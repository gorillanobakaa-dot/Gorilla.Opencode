package arsenal

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// The manifest is a teaching document. A field that does not teach and does not
// decide should not be in the binary at all, so every field that IS there has
// to be filled in — an entry with an empty `teaches` is pure weight.
func TestEveryEntryTeachesSomething(t *testing.T) {
	m, err := Load()
	if err != nil {
		t.Fatalf("the embedded manifest does not parse: %v", err)
	}
	if len(m.Series) == 0 {
		t.Fatal("no series")
	}
	seen := map[string]string{}
	for _, s := range m.Series {
		if s.Title == "" || s.Why == "" {
			t.Errorf("series %q has no title or no reason to exist", s.ID)
		}
		if len(s.Entries) == 0 {
			t.Errorf("series %q is empty", s.ID)
		}
		for _, e := range s.Entries {
			if prev, dup := seen[e.ID]; dup {
				t.Errorf("entry id %q appears in both %q and %q; ids are how tagfiles refer to things", e.ID, prev, s.ID)
			}
			seen[e.ID] = s.ID

			if e.Title == "" {
				t.Errorf("%s has no title", e.ID)
			}
			if len(strings.Fields(e.Teaches)) < 12 {
				t.Errorf("%s: `teaches` is %d words. This is the discovery surface — the whole "+
					"reason this manifest exists is that nobody stumbles onto these tools unaided",
					e.ID, len(strings.Fields(e.Teaches)))
			}
			if len(e.Unlocks) == 0 {
				t.Errorf("%s does not say what the agent gains", e.ID)
			}
			if len(e.Detect.Binaries) == 0 {
				t.Errorf("%s has no detection. The manifest may say what a tool WOULD unlock; "+
					"only a successful detection says it IS unlocked", e.ID)
			}
			if e.Caveats == "" {
				t.Errorf("%s has no caveats. That is the field most tools' own documentation "+
					"omits and the one a user most needs — it is the difference between a "+
					"catalogue and honest advice", e.ID)
			}
			if len(e.Packages) == 0 {
				t.Errorf("%s says how to detect it but not how to get it", e.ID)
			}
			switch e.Tier {
			case "MINIMUM", "NICE", "BEST":
			default:
				t.Errorf("%s has tier %q", e.ID, e.Tier)
			}
		}
	}
}

// Detection must be MEASURED. The bug that started this package was a
// capability sitting on the disk that nothing knew about; a manifest that
// merely asserts what is installed reproduces it exactly.
func TestDetectionAgreesWithTheActualMachine(t *testing.T) {
	m, _ := Load()
	for _, s := range m.Series {
		for _, e := range s.Entries {
			st := DetectEntry(e)
			for _, b := range st.Found {
				if _, err := exec.LookPath(b); err != nil {
					t.Errorf("%s: reported %q as present and it is not on PATH", e.ID, b)
				}
			}
			for _, b := range st.Missing {
				if _, err := exec.LookPath(b); err == nil {
					t.Errorf("%s: reported %q as missing and it IS on PATH", e.ID, b)
				}
			}
			if st.Present && len(st.Missing) > 0 {
				t.Errorf("%s claims to be fully present with %v missing", e.ID, st.Missing)
			}
		}
	}
}

// "You have pdftotext but not pdfimages" is actionable. "Not installed" for the
// same state is misleading.
func TestAHalfInstalledEntryIsReportedAsPartial(t *testing.T) {
	st := Status{Found: []string{"a"}, Missing: []string{"b"}}
	if !st.Partial() {
		t.Fatal("a half-present entry did not report as partial")
	}
	if (Status{Found: []string{"a"}}).Partial() {
		t.Error("a fully present entry reported as partial")
	}
}

// An installer is the highest-stakes prompt in the program. The command is
// shown; running it is a separate, explicit act.
func TestInstallCommandIsShownNeverSilentlyPrivileged(t *testing.T) {
	got := InstallCommand([]string{"tesseract-ocr", "tesseract-ocr-eng"}, APT)
	if !strings.HasPrefix(got, "sudo apt-get install") {
		t.Fatalf("unexpected apt command: %q", got)
	}
	if !strings.Contains(got, "tesseract-ocr-eng") {
		t.Error("the command dropped a package the user selected")
	}
	if InstallCommand(nil, APT) != "" {
		t.Error("produced an install command for nothing")
	}
	if InstallCommand([]string{"x"}, Unknown) != "" {
		t.Error("produced a command for an unknown package manager")
	}
}

// A cost we could not measure must SAY so. A zero reads as "free", which is the
// one thing it must never be mistaken for.
func TestAnUnmeasurableCostSaysSoRatherThanShowingZero(t *testing.T) {
	c := MeasureCost([]string{"x"}, Unknown)
	if c.Measured {
		t.Error("claimed to have measured a cost with no package manager")
	}
	if c.Note == "" {
		t.Error("failed silently — a zero with no note reads as free")
	}
}

func TestEmptySelectionCostsNothingAndKnowsIt(t *testing.T) {
	c := MeasureCost(nil, APT)
	if !c.Measured || c.DownloadBytes != 0 {
		t.Fatalf("empty selection priced as %+v", c)
	}
}

// A decimal comma would silently turn 29.5 MB into 295 MB, which is why the
// package manager is run under LC_ALL=C.
func TestSizeParsingHandlesTheUnitsAptEmits(t *testing.T) {
	for _, tc := range []struct {
		num, unit string
		want      int64
	}{
		{"72.3", "k", 72300},
		{"29.5", "M", 29500000},
		{"1.4", "G", 1400000000},
		{"190", "", 190},
		{"1,024", "k", 1024000},
	} {
		if got := parseSize(tc.num, tc.unit); got != tc.want {
			t.Errorf("parseSize(%q,%q) = %d, want %d", tc.num, tc.unit, got, tc.want)
		}
	}
}

// §8: download size is time. A figure in megabytes hides the cost from exactly
// the person who most needs to see it.
func TestDownloadTimeIsStatedInTheUnitSomeoneWaitingFeels(t *testing.T) {
	if got := DownloadTime(18*1000*1000, 8); !strings.Contains(got, "minutes") {
		t.Errorf("18 MB at 8 KB/s reported as %q; it is about 37 minutes", got)
	}
	if got := DownloadTime(1500*1000*1000, 8); !strings.Contains(got, "hours") {
		t.Errorf("1.5 GB at 8 KB/s reported as %q", got)
	}
	if DownloadTime(0, 8) != "" {
		t.Error("invented a wait for nothing to download")
	}
}

func TestHumanBytes(t *testing.T) {
	for _, tc := range []struct {
		in   int64
		want string
	}{{0, "0 B"}, {512, "512 B"}, {72300, "72 KB"}, {29500000, "29.5 MB"}, {1400000000, "1.40 GB"}} {
		if got := HumanBytes(tc.in); got != tc.want {
			t.Errorf("HumanBytes(%d) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

// GORILLA OVERRIDE (2026-08-19): caught by the first real measurement run.
// ast-grep has no Debian package, so its apt list is empty and MeasureCost
// priced it at 0 B — which on screen, next to "not installed", reads as FREE.
// It is not free; it is unobtainable that way. Opposite facts.
func TestAnEntryWithNoPackageIsUnavailableNotFree(t *testing.T) {
	e := Entry{ID: "x", Packages: map[string][]string{"apt": {}, "pacman": {"x"}}}
	if Available(e, APT) {
		t.Error("an entry with an empty apt list reported as installable via apt")
	}
	if !Available(e, Pacman) {
		t.Error("an entry with a pacman package reported as unavailable")
	}
	note := UnavailableNote(e, APT)
	if note == "" {
		t.Fatal("no explanation offered; the user gets a shrug instead of a route")
	}
	if !strings.Contains(note, "apt") {
		t.Errorf("the explanation does not name the package manager: %q", note)
	}
	if UnavailableNote(e, Pacman) != "" {
		t.Error("explained away an entry that is perfectly available")
	}
}

// Every entry must be obtainable SOMEWHERE, or it is a tease.
func TestEveryEntryIsAvailableOnAtLeastOnePackageManager(t *testing.T) {
	m, _ := Load()
	for _, s := range m.Series {
		for _, e := range s.Entries {
			if !Available(e, APT) && !Available(e, Pacman) {
				t.Errorf("%s cannot be installed by either package manager — it is a tease", e.ID)
			}
		}
	}
}

// A tagfile is a SELECTION AS A FILE — the part of the Slackware installer that
// mattered most and is least obvious. Someone who works out a good forensics
// selection can post the file, and the next person gets the map for free.
func TestTagfileRoundTrips(t *testing.T) {
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	m, _ := Load()
	want := []string{"tesseract", "poppler", "binwalk"}

	path, err := SaveTagfile(want, m, APT)
	if err != nil {
		t.Fatalf("save: %v", err)
	}
	got, unknown, err := LoadTagfile(path, m)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if len(unknown) != 0 {
		t.Errorf("round trip invented unknown ids: %v", unknown)
	}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Errorf("got %v, want %v", got, want)
	}
}

// It has to be readable and editable by a person with none of this software.
func TestATagfileIsPlainTextAPersonCanEdit(t *testing.T) {
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	m, _ := Load()
	path, err := SaveTagfile([]string{"tesseract"}, m, APT)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	body := string(raw)
	if !strings.Contains(body, "#") {
		t.Error("no comments — the file does not explain itself to someone who receives it")
	}
	if !strings.Contains(body, "Read text out of images") {
		t.Error("the human-readable title is missing; the id alone teaches nobody anything")
	}
	if strings.Contains(body, "{") {
		t.Error("this looks like JSON; a tagfile must be editable by someone who does not know JSON")
	}
}

// A tagfile from a newer version naming a capability this build lacks is a FACT
// the user should hear, not something to swallow silently.
func TestUnknownIdsAreReportedNotDropped(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "t.tagfile")
	body := "# from a friend\ntesseract\nquantum-decompiler   # not a real thing\n\n  poppler\n"
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	m, _ := Load()
	ids, unknown, err := LoadTagfile(path, m)
	if err != nil {
		t.Fatal(err)
	}
	if len(ids) != 2 {
		t.Errorf("read %v, want the two real ids", ids)
	}
	if len(unknown) != 1 || unknown[0] != "quantum-decompiler" {
		t.Errorf("unknown = %v; an id this build does not have must be reported", unknown)
	}
}

// GORILLA OVERRIDE (2026-08-19): caught by driving the real binary, not by any
// test here.
//
// ast-grep ships as `ast-grep` or as `sg`, depending on how it was installed.
// Detection required ALL listed binaries, so a machine with a working `sg`
// reported the capability as MISSING — the exact bug this whole feature exists
// to fix, reproduced inside the fix.
func TestAlternativeBinaryNamesCountAsPresent(t *testing.T) {
	e := Entry{
		ID:     "x",
		Detect: Detect{Binaries: []string{"definitely-not-installed-xyz", "sh"}, Mode: "any"},
	}
	st := DetectEntry(e)
	if !st.Present {
		t.Fatal("an entry whose alternative name IS installed reported as missing")
	}
	if len(st.Missing) != 0 {
		t.Errorf("reported %v as missing; with alternative names there is nothing missing "+
			"once one is found, and saying otherwise reads as a half-install that does not exist", st.Missing)
	}
}

// "all" must stay the default: poppler-utils genuinely gives you pdftotext AND
// pdfimages AND pdfinfo, and having one of three is worth reporting as partial.
func TestAllRemainsTheDefaultAndStillReportsPartial(t *testing.T) {
	e := Entry{ID: "x", Detect: Detect{Binaries: []string{"sh", "definitely-not-installed-xyz"}}}
	st := DetectEntry(e)
	if st.Present {
		t.Fatal("a half-installed entry reported as fully present")
	}
	if !st.Partial() {
		t.Fatal("a half-installed entry did not report as partial")
	}
}

// Only entries whose binaries really are alternative spellings may use "any".
// Marking poppler "any" would claim the whole capability from pdfinfo alone.
func TestAnyModeIsOnlyUsedWhereTheNamesAreAlternatives(t *testing.T) {
	m, _ := Load()
	allowed := map[string]bool{
		"astgrep": true, "imagemagick": true, "p7zip": true,
		"apkinspect": true, "libreoffice": true,
	}
	for _, s := range m.Series {
		for _, e := range s.Entries {
			if e.Detect.Mode == "any" && !allowed[e.ID] {
				t.Errorf("%s uses any-mode detection; that claims the whole capability from one "+
					"binary, which is only honest when the names are alternatives for one tool", e.ID)
			}
		}
	}
}
