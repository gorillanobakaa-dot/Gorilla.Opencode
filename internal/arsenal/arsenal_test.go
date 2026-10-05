package arsenal

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
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
			// GORILLA FIX (2026-10-05), audit A1: a name that resolves to a
			// declared impostor is on PATH and is correctly "missing" — Windows'
			// convert.exe is not ImageMagick. Those are excused here by name and
			// checked on their own in the impostor tests below.
			ignored := map[string]bool{}
			for _, ig := range st.Ignored {
				ignored[ig.Binary] = true
			}
			for _, b := range st.Missing {
				if _, err := exec.LookPath(b); err == nil && !ignored[b] {
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

// GORILLA FIX (2026-10-05), audit A2. This test used to be called
// TestEmptySelectionCostsNothingAndKnowsIt and asserted the defect: zero
// packages came back Measured at 0 B, and the screen printed that as "nothing
// to download - all of it is already here" for a selection of tools the
// package manager could not fetch at all. Nothing asked is nothing measured.
func TestZeroPackagesIsNotMeasuredNeverFree(t *testing.T) {
	for _, pm := range []PackageManager{APT, Pacman, Scoop, Unknown} {
		c := MeasureCost(nil, pm)
		if c.Measured {
			t.Errorf("%q: zero packages reported as a measurement: %+v", pm, c)
		}
		if c.Note == "" {
			t.Errorf("%q: zero packages gave no reason; a bare zero reads as free", pm)
		}
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

// GORILLA FIX (2026-10-05), audit A13: the test above never looked at Scoop, so
// nothing checked the Windows column of the manifest at all.
//
// It cannot demand a Scoop package for every entry — several real tools have
// none, and saying "not packaged for scoop" is the honest answer for them. What
// it can demand is that the column is consistent: no empty list posing as a
// package, no Scoop package on an entry that cannot exist on Windows, and an
// install command for exactly the entries that claim one.
func TestTheScoopColumnIsConsistent(t *testing.T) {
	useMachine(t, "windows", nil)
	m, _ := Load()
	offered := 0
	for _, s := range m.Series {
		for _, e := range s.Entries {
			pkgs, has := e.Packages["scoop"]
			if has && len(pkgs) == 0 {
				t.Errorf("%s has an empty scoop list; leave the key out so the entry reads as unavailable", e.ID)
			}
			if has && !Applicable(e) {
				t.Errorf("%s cannot exist on Windows and still names a scoop package", e.ID)
			}
			for _, p := range pkgs {
				if p == "" || strings.ContainsAny(p, " ;&|") || strings.Count(p, "/") > 1 {
					t.Errorf("%s: scoop package %q is not a plain name or bucket/name", e.ID, p)
				}
			}
			if Available(e, Scoop) != (InstallCommand(PackagesFor(e, Scoop), Scoop) != "") {
				t.Errorf("%s: available via scoop and having an install command disagree", e.ID)
			}
			if !Available(e, Scoop) && Applicable(e) && UnavailableNote(e, Scoop) == "" {
				t.Errorf("%s is listed on Windows, has no scoop package, and offers no explanation", e.ID)
			}
			if Available(e, Scoop) {
				offered++
			}
		}
	}
	if offered == 0 {
		t.Error("no entry is obtainable through scoop; /arsenal would be inert on Windows again")
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
			// Paths count one existing file as the whole capability, which is
			// the same claim any-mode makes and is only honest in the same case.
			if len(e.Detect.Paths) > 0 && e.Detect.Mode != "any" {
				t.Errorf("%s lists install paths without any-mode detection", e.ID)
			}
		}
	}
}

// ── 2026-10-05: tests for the /arsenal audit, defects A1 to A13 ────────────

// useMachine points detection at an imagined machine: operating system `os`,
// with `onPath` (name -> full path) as everything PATH can resolve. Restored
// when the test ends.
func useMachine(t *testing.T, os string, onPath map[string]string) {
	t.Helper()
	oldLook, oldOS, oldBucket := lookPath, goos, scoopHasBucket
	t.Cleanup(func() { lookPath, goos, scoopHasBucket = oldLook, oldOS, oldBucket })
	goos = os
	lookPath = func(name string) (string, error) {
		if p, ok := onPath[name]; ok {
			return p, nil
		}
		return "", exec.ErrNotFound
	}
}

func entryByID(t *testing.T, id string) Entry {
	t.Helper()
	m, err := Load()
	if err != nil {
		t.Fatalf("manifest: %v", err)
	}
	for _, s := range m.Series {
		for _, e := range s.Entries {
			if e.ID == id {
				return e
			}
		}
	}
	t.Fatalf("no manifest entry %q", id)
	return Entry{}
}

// GORILLA FIX (2026-10-05), audit A1, the worst /arsenal finding: on Windows
// `convert` is C:\Windows\System32\convert.exe, the FAT-to-NTFS disk converter.
// Every Windows machine reported ImageMagick as HAVE. Run on the audited
// machine: `where convert` answered C:\Windows\System32\convert.exe.
func TestWindowsDiskConverterIsNotImageMagick(t *testing.T) {
	root := filepath.Join(t.TempDir(), "Windows")
	t.Setenv("SystemRoot", root)
	sys := filepath.Join(root, "System32", "convert.exe")
	e := entryByID(t, "imagemagick")

	useMachine(t, "windows", map[string]string{"convert": sys})
	st := DetectEntry(e)
	if st.Present {
		t.Fatal("Windows' own convert.exe was counted as ImageMagick — a false HAVE")
	}
	if len(st.Found) != 0 {
		t.Errorf("found = %v; nothing of ImageMagick is on this imagined machine", st.Found)
	}
	if len(st.Ignored) != 1 || st.Ignored[0].Binary != "convert" || st.Ignored[0].Path != sys || st.Ignored[0].Is == "" {
		t.Errorf("the impostor was not reported with its path and what it is: %+v", st.Ignored)
	}

	// The same file name in upper case is the same directory on Windows.
	useMachine(t, "windows", map[string]string{"convert": filepath.Join(strings.ToUpper(root), "SYSTEM32", "convert.EXE")})
	if DetectEntry(e).Present {
		t.Error("C:\\WINDOWS and C:\\Windows were treated as different directories")
	}

	// The real thing next to the impostor is still found.
	useMachine(t, "windows", map[string]string{"convert": sys, "magick": filepath.Join(root, "..", "ImageMagick", "magick.exe")})
	st = DetectEntry(e)
	if !st.Present || len(st.Found) != 1 || st.Found[0] != "magick" {
		t.Errorf("ImageMagick installed beside the impostor was not detected by magick alone: %+v", st)
	}

	// A convert.exe that is NOT under the Windows directory is ImageMagick's
	// own legacy name and must still count.
	useMachine(t, "windows", map[string]string{"convert": filepath.Join(root, "..", "ImageMagick", "convert.exe")})
	if !DetectEntry(e).Present {
		t.Error("ImageMagick's own convert.exe outside the Windows directory was rejected")
	}

	// And on Linux `convert` is ImageMagick wherever it lives.
	useMachine(t, "linux", map[string]string{"convert": sys})
	if !DetectEntry(e).Present {
		t.Error("the Windows rule was applied on Linux, where convert IS ImageMagick")
	}
}

// The guard A1 asked for: no detect name anywhere in the manifest may be
// satisfied by a program Windows itself ships. It asks the real System32 of
// the machine running the test, so a collision introduced by a future manifest
// entry fails here instead of on a user's screen.
func TestNoDetectNameIsSatisfiedByAWindowsSystemBinary(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("needs a real Windows System32 to ask; the rule itself is covered by TestWindowsDiskConverterIsNotImageMagick")
	}
	sys32 := filepath.Join(os.Getenv("SystemRoot"), "System32")
	if _, err := os.Stat(sys32); err != nil {
		t.Skipf("no System32 at %q", sys32)
	}
	m, _ := Load()
	collisions := 0
	for _, s := range m.Series {
		for _, e := range s.Entries {
			for _, b := range e.Detect.Binaries {
				for _, ext := range []string{".exe", ".com", ".cmd", ".bat"} {
					p := filepath.Join(sys32, b+ext)
					if _, err := os.Stat(p); err != nil {
						continue
					}
					collisions++
					useMachine(t, "windows", map[string]string{b: p})
					st := DetectEntry(e)
					if st.Present || len(st.Found) != 0 {
						t.Errorf("%s: detect name %q is satisfied by %s, which belongs to Windows. "+
							"Declare it under detect.impostors in manifest.json", e.ID, b, p)
					}
				}
			}
		}
	}
	t.Logf("detect names that also exist in %s: %d", sys32, collisions)
}

// The same defect on Linux, which the astgrep caveat described in words while
// detection ignored it: /usr/bin/sg is a link to newgrp.
func TestTheSystemSgIsNotAstGrep(t *testing.T) {
	dir := t.TempDir()
	newgrp := filepath.Join(dir, "newgrp")
	if err := os.WriteFile(newgrp, []byte("x"), 0o755); err != nil {
		t.Fatal(err)
	}
	sg := filepath.Join(dir, "sg")
	if err := os.Symlink(newgrp, sg); err != nil {
		t.Skipf("cannot make a symbolic link here: %v", err)
	}
	e := entryByID(t, "astgrep")

	useMachine(t, "linux", map[string]string{"sg": sg})
	st := DetectEntry(e)
	if st.Present {
		t.Fatal("the system's sg (a link to newgrp) was counted as ast-grep")
	}
	if len(st.Ignored) != 1 || st.Ignored[0].Binary != "sg" {
		t.Errorf("the impostor was not reported: %+v", st.Ignored)
	}

	// A real file called sg — ast-grep's own short name — still counts.
	real := filepath.Join(t.TempDir(), "sg")
	if err := os.WriteFile(real, []byte("x"), 0o755); err != nil {
		t.Fatal(err)
	}
	useMachine(t, "linux", map[string]string{"sg": real})
	if !DetectEntry(e).Present {
		t.Error("ast-grep installed under its short name sg was rejected")
	}
}

// Every declared impostor must be checkable: it names a binary the entry
// actually detects, an operating system, a rule, and what the thing really is.
func TestImpostorDeclarationsAreComplete(t *testing.T) {
	m, _ := Load()
	for _, s := range m.Series {
		for _, e := range s.Entries {
			names := map[string]bool{}
			for _, b := range e.Detect.Binaries {
				names[b] = true
			}
			for _, imp := range e.Detect.Impostors {
				if !names[imp.Binary] {
					t.Errorf("%s: impostor %q is not one of its detect names", e.ID, imp.Binary)
				}
				if imp.OS == "" || imp.Is == "" || (imp.Under == "") == (imp.Target == "") {
					t.Errorf("%s: impostor %+v needs an os, what it is, and exactly one of under/target", e.ID, imp)
				}
			}
		}
	}
}

// GORILLA FIX (2026-10-05), audit A6: Scoop's libreoffice package makes
// shortcuts only, so soffice.exe is on the disk and not on PATH. Confirmed on
// the audited machine, where it was installed and reported missing.
func TestAToolInstalledOffPathIsFoundByItsLocation(t *testing.T) {
	scoop := filepath.Join(t.TempDir(), "scoop")
	t.Setenv("SCOOP", scoop)
	t.Setenv("ProgramFiles", filepath.Join(t.TempDir(), "nothing-here"))
	e := entryByID(t, "libreoffice")
	useMachine(t, "windows", nil)

	if st := DetectEntry(e); st.Present {
		t.Fatalf("reported LibreOffice present on an empty machine: %+v", st)
	}

	exe := filepath.Join(scoop, "apps", "libreoffice", "current", "LibreOffice", "program", "soffice.exe")
	if err := os.MkdirAll(filepath.Dir(exe), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(exe, []byte("x"), 0o755); err != nil {
		t.Fatal(err)
	}
	st := DetectEntry(e)
	if !st.Present {
		t.Fatal("LibreOffice installed by Scoop was reported missing")
	}
	if !st.OffPath {
		t.Error("it was reported as if it were on PATH; its short name will not run")
	}
	if len(st.Found) != 1 || st.Found[0] != exe {
		t.Errorf("found = %v, want the full path %q — the only way to call it", st.Found, exe)
	}
	if len(st.Missing) != 0 {
		t.Errorf("missing = %v on an entry that is present", st.Missing)
	}

	// On PATH wins, and is then not an off-path find.
	useMachine(t, "windows", map[string]string{"soffice": exe})
	if st := DetectEntry(e); !st.Present || st.OffPath || st.Found[0] != "soffice" {
		t.Errorf("a soffice that IS on PATH was not reported by name: %+v", st)
	}

	// The Windows locations are not probed on Linux.
	useMachine(t, "linux", nil)
	if DetectEntry(e).Present {
		t.Error("a Windows install path satisfied detection on Linux")
	}
}

// A location naming a variable that is not set must be skipped, not expanded
// into a path nobody meant.
func TestAnUnsetVariableSkipsTheLocation(t *testing.T) {
	t.Setenv("ARSENAL_TEST_UNSET", "")
	if p := firstExistingPath([]string{"${ARSENAL_TEST_UNSET}/go.mod", "${ARSENAL_TEST_UNSET}arsenal.go"}); p != "" {
		t.Errorf("an unset variable expanded to %q", p)
	}
}

// GORILLA FIX (2026-10-05), audit A10: on Windows "The minimum" could never
// read 8/8 because bubblewrap is Linux-only, and the series about reading
// Windows files ON LINUX was listed on Windows with every entry N/A.
func TestEntriesThatCannotExistHereAreNotListedOrCounted(t *testing.T) {
	full, _ := Load()
	total := 0
	for _, s := range full.Series {
		total += len(s.Entries)
	}

	useMachine(t, "windows", nil)
	win := ForPlatform(full)
	winTotal := 0
	for _, s := range win.Series {
		if len(s.Entries) == 0 {
			t.Errorf("series %q is listed on Windows with nothing in it", s.ID)
		}
		if s.ID == "windows" {
			t.Errorf("the series %q is listed on Windows", s.Title)
		}
		for _, e := range s.Entries {
			winTotal++
			if e.ID == "bubblewrap" {
				t.Error("bubblewrap is counted on Windows; \"The minimum\" can then never be complete")
			}
			if !Applicable(e) {
				t.Errorf("%s is listed on a system it cannot exist on", e.ID)
			}
		}
	}
	if winTotal == 0 || winTotal >= total {
		t.Errorf("Windows lists %d of %d entries; expected fewer than all and more than none", winTotal, total)
	}

	useMachine(t, "linux", nil)
	lin := ForPlatform(full)
	linTotal := 0
	for _, s := range lin.Series {
		linTotal += len(s.Entries)
	}
	if len(lin.Series) != len(full.Series) || linTotal != total {
		t.Errorf("Linux lists %d entries in %d series, the manifest has %d in %d — nothing in it is Windows-only",
			linTotal, len(lin.Series), total, len(full.Series))
	}

	// The embedded manifest itself is not altered by filtering.
	again, _ := Load()
	n := 0
	for _, s := range again.Series {
		n += len(s.Entries)
	}
	if n != total {
		t.Errorf("filtering changed the loaded manifest: %d entries, was %d", n, total)
	}

	// Only operating systems this program is built for may be named.
	for _, s := range full.Series {
		for _, e := range s.Entries {
			for _, p := range e.Platforms {
				if p != "linux" && p != "windows" {
					t.Errorf("%s names platform %q", e.ID, p)
				}
			}
		}
	}
}

// GORILLA FIX (2026-10-05), audit A13: DetectPackageManager had no test.
func TestDetectPackageManager(t *testing.T) {
	empty := t.TempDir()
	t.Setenv("SCOOP", empty)

	useMachine(t, "linux", map[string]string{"apt-get": "/usr/bin/apt-get", "pacman": "/usr/bin/pacman"})
	if got := DetectPackageManager(); got != APT {
		t.Errorf("apt-get and pacman both present: got %q, want apt", got)
	}
	useMachine(t, "linux", map[string]string{"pacman": "/usr/bin/pacman"})
	if got := DetectPackageManager(); got != Pacman {
		t.Errorf("pacman only: got %q", got)
	}
	useMachine(t, "linux", map[string]string{"scoop": "/usr/local/bin/scoop"})
	if got := DetectPackageManager(); got != Unknown {
		t.Errorf("a scoop on Linux was accepted: %q", got)
	}
	useMachine(t, "windows", map[string]string{"apt-get": `C:\cygwin\bin\apt-get.exe`})
	if got := DetectPackageManager(); got != Unknown {
		t.Errorf("an apt-get on Windows was accepted: %q", got)
	}
	useMachine(t, "windows", map[string]string{"scoop": `C:\Users\x\scoop\shims\scoop.cmd`})
	if got := DetectPackageManager(); got != Scoop {
		t.Errorf("scoop on PATH: got %q", got)
	}

	// Not on PATH yet, but the shim is on the disk.
	useMachine(t, "windows", nil)
	if got := DetectPackageManager(); got != Unknown {
		t.Errorf("no scoop anywhere: got %q", got)
	}
	shims := filepath.Join(empty, "shims")
	if err := os.MkdirAll(shims, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(shims, "scoop.cmd"), []byte("x"), 0o755); err != nil {
		t.Fatal(err)
	}
	if got := DetectPackageManager(); got != Scoop {
		t.Errorf("scoop shim on disk, PATH not refreshed: got %q", got)
	}
}

// GORILLA FIX (2026-10-05), audit A11 and A13: the Scoop command was one line
// joined with ";", which is PowerShell and fails in cmd.exe.
func TestScoopCommandsAreOnePerLineAndRunInAnyShell(t *testing.T) {
	useMachine(t, "windows", nil)
	scoopHasBucket = func(string) bool { return false }

	got := InstallCommands([]string{"7zip", "extras/libreoffice", "extras/testdisk"}, Scoop)
	want := []string{"scoop bucket add extras", "scoop install 7zip extras/libreoffice extras/testdisk"}
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Fatalf("got %q, want %q", got, want)
	}
	for _, line := range got {
		if strings.ContainsAny(line, ";&|\n") {
			t.Errorf("%q joins commands with shell syntax; cmd.exe and PowerShell do not agree on any", line)
		}
		if strings.Contains(line, "sudo") {
			t.Errorf("%q asks for elevation; Scoop installs into the user's own folder", line)
		}
	}
	if got := InstallCommands([]string{"jq"}, Scoop); len(got) != 1 || got[0] != "scoop install jq" {
		t.Errorf("a main-bucket package produced %q", got)
	}

	// A bucket that is already added is not added again.
	scoopHasBucket = func(name string) bool { return name == "extras" }
	if got := InstallCommands([]string{"extras/libreoffice"}, Scoop); len(got) != 1 || got[0] != "scoop install extras/libreoffice" {
		t.Errorf("with the extras bucket present, got %q", got)
	}

	// The single-string form is the same lines, never a ";".
	scoopHasBucket = func(string) bool { return false }
	if s := InstallCommand([]string{"extras/libreoffice"}, Scoop); strings.Contains(s, ";") ||
		s != "scoop bucket add extras\nscoop install extras/libreoffice" {
		t.Errorf("InstallCommand = %q", s)
	}
	if InstallCommands(nil, Scoop) != nil || InstallCommands([]string{"x"}, Unknown) != nil {
		t.Error("produced commands for nothing, or for no package manager")
	}
}

// The bucket check reads the disk.
func TestScoopBucketPresenceIsReadFromTheDisk(t *testing.T) {
	root := t.TempDir()
	t.Setenv("SCOOP", root)
	if scoopHasBucket("extras") {
		t.Fatal("reported a bucket in an empty Scoop folder")
	}
	if err := os.MkdirAll(filepath.Join(root, "buckets", "extras"), 0o755); err != nil {
		t.Fatal(err)
	}
	if !scoopHasBucket("extras") {
		t.Fatal("did not see a bucket that is on the disk")
	}
}

// GORILLA FIX (2026-10-05), audit A4 and A13: nothing fed apt's answer through
// the parser, and an answer matching neither pattern stayed "measured" at 0 B.
//
// The texts below follow apt-get's summary format as the two patterns in
// arsenal.go expect it. They were written for this test on a Windows machine
// with no apt; they are not captures of a real run.
func TestAptAnswerIsParsedOrDeclaredUnmeasured(t *testing.T) {
	const head = "Reading package lists...\nBuilding dependency tree...\nReading state information...\n" +
		"The following NEW packages will be installed:\n  zbar-tools\n" +
		"0 upgraded, 1 newly installed, 0 to remove and 0 not upgraded.\n"

	c := parseAPTCost(head + "Need to get 72.3 kB of archives.\nAfter this operation, 190 kB of additional disk space will be used.\n")
	if !c.Measured || !c.DiskMeasured || c.DownloadBytes != 72300 || c.DiskBytes != 190000 {
		t.Errorf("plain answer parsed as %+v", c)
	}

	// Part already in apt's cache: the first figure is what is left to fetch.
	c = parseAPTCost(head + "Need to get 0 B/1,234 kB of archives.\nAfter this operation, 3,400 kB of additional disk space will be used.\n")
	if !c.Measured || c.DownloadBytes != 0 || !c.DiskMeasured || c.DiskBytes != 3400000 {
		t.Errorf("cached answer parsed as %+v", c)
	}

	c = parseAPTCost(head + "Need to get 97.9 MB of archives.\nAfter this operation, 12.0 kB disk space will be freed.\n")
	if !c.Measured || !c.DiskMeasured || c.DiskBytes != 0 || c.DownloadBytes != 97900000 {
		t.Errorf("space-freed answer parsed as %+v", c)
	}

	// The download line is there and the disk line is not: one figure, and
	// the other is declared missing rather than zero.
	c = parseAPTCost(head + "Need to get 72.3 kB of archives.\n")
	if !c.Measured || c.DiskMeasured || c.Note == "" {
		t.Errorf("download-only answer parsed as %+v; the disk figure must be marked unreported", c)
	}

	// THE DEFECT: an answer with neither line.
	for _, out := range []string{
		"",
		head,
		// The same facts in wording the patterns do not know.
		head + "Download size: 72.3 kB\nSpace needed: 190 kB\n",
	} {
		c = parseAPTCost(out)
		if c.Measured || c.DiskMeasured {
			t.Errorf("an unreadable answer was reported as a measurement of %+v:\n%s", c, out)
		}
		if c.Note == "" {
			t.Errorf("an unreadable answer gave no reason:\n%s", out)
		}
	}
}

func TestPacmanAnswerIsParsedOrDeclaredUnmeasured(t *testing.T) {
	c := parsePacmanCost("1234\n5678\r\n")
	if !c.Measured || c.DownloadBytes != 6912 {
		t.Errorf("two sizes parsed as %+v", c)
	}
	if c.DiskMeasured {
		t.Error("pacman -Sp reports no installed size; it must not be presented as measured")
	}
	if c.Note == "" {
		t.Error("no note that the figure is download only")
	}
	for _, out := range []string{"", "\n", "warning: zbar-0.23-1 is up to date -- reinstalling\n"} {
		c = parsePacmanCost(out)
		if c.Measured {
			t.Errorf("an answer with no size in it was reported as measured: %q -> %+v", out, c)
		}
		if c.Note == "" {
			t.Errorf("no reason given for %q", out)
		}
	}
}

// GORILLA FIX (2026-10-05), audit A3: the screen asks CanMeasure before it
// offers the p key, so this is the fact the offer rests on.
func TestOnlyAptAndPacmanCanPriceBeforeInstalling(t *testing.T) {
	for pm, want := range map[PackageManager]bool{APT: true, Pacman: true, Scoop: false, Unknown: false} {
		if CanMeasure(pm) != want {
			t.Errorf("CanMeasure(%q) = %v", pm, !want)
		}
		if (CannotMeasureNote(pm) == "") != want {
			t.Errorf("CannotMeasureNote(%q) = %q", pm, CannotMeasureNote(pm))
		}
		if !want {
			if c := MeasureCost([]string{"x"}, pm); c.Measured || c.Note != CannotMeasureNote(pm) {
				t.Errorf("MeasureCost on %q returned %+v", pm, c)
			}
		}
	}
}

// GORILLA FIX (2026-10-05), audit A12: a selection received from somebody else
// can be loaded without overwriting your own.
func TestEveryTagfileInTheFolderCanBeFoundOwnFirst(t *testing.T) {
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	if got := Tagfiles(); len(got) != 0 {
		t.Fatalf("found selection files in an empty folder: %v", got)
	}
	m, _ := Load()
	own, err := SaveTagfile([]string{"tesseract"}, m, APT)
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"a-friend.tagfile", "zoe.tagfile", "notes.txt"} {
		if err := os.WriteFile(filepath.Join(TagfileDir(), name), []byte("poppler\n"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	got := Tagfiles()
	if len(got) != 3 {
		t.Fatalf("got %v, want the three .tagfile files and not notes.txt", got)
	}
	if got[0] != own {
		t.Errorf("the user's own selection is not first: %v", got)
	}
	if filepath.Base(got[1]) != "a-friend.tagfile" || filepath.Base(got[2]) != "zoe.tagfile" {
		t.Errorf("the others are not in name order: %v", got)
	}
}

// The help text and the screen both say "everything listed is free and needs
// no account". This is what makes that sentence true; the day it fails, the
// sentence has to change with it.
func TestNothingInTheManifestNeedsAnAccountOrACard(t *testing.T) {
	m, _ := Load()
	for _, s := range m.Series {
		for _, e := range s.Entries {
			if e.Needs.Account || e.Needs.Card {
				t.Errorf("%s needs an account or a card; /arsenal's help says nothing listed does", e.ID)
			}
		}
	}
}
