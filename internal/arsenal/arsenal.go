// GORILLA OVERRIDE: this package did not exist upstream. It is /arsenal — the
// agent telling you what it could become, and what that would cost you.
//
// WHY IT EXISTS. On 2026-08-18 a model was handed a screenshot and reported
// that it could not read images. That was true of the model and false of the
// machine: tesseract 5.5.0 was installed, working, and three inches away. The
// capability was sitting on the disk unused because nothing told anybody it
// was there.
//
// That is not a missing feature. It is a missing MAP. Nobody stumbles onto
// binwalk, sleuthkit, libesedb or ssdeep unaided, and someone who does not
// know a thing exists cannot ask for it. The barrier is discovery, not
// bandwidth.
//
// THE GOVERNING RULE: ship the knowledge, not the bulk. The binary carries a
// manifest of a few tens of kilobytes that knows what each tool is, what it
// unlocks, how to detect it, and the exact command to fetch it. The user's own
// package manager does the downloading, from their own distribution's mirrors,
// only for what they chose. Nothing is redistributed and nothing rots at a
// vendored version. This is the same doctrine as the Microsoft fonts decision
// of 2026-08-03: ship the method, never the binaries.
//
// AND THE RULE THAT DECIDES ARGUMENTS: costs are INFORMATION, NOT A GATE. Show
// the megabytes and the hours, then let the user choose "everything" if that is
// what they want. In the owner's words: "poor kids are usually patient, they
// are used to slow downloads — you don't know what you don't know." A number
// presented so someone can choose is respect. The same number used to steer
// them toward a smaller option is condescension wearing a helpful face.
package arsenal

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"

	"github.com/opencode-ai/opencode/internal/config"
)

//go:embed manifest.json
var manifestJSON []byte

// Manifest is the whole catalogue.
type Manifest struct {
	Version   int      `json:"manifest_version"`
	Generated string   `json:"generated"`
	Series    []Series `json:"series"`
}

// Series is a coherent group, in the Slackware sense: something you can take
// wholesale, walk item by item, or ignore.
type Series struct {
	ID      string  `json:"id"`
	Title   string  `json:"title"`
	Why     string  `json:"why"`
	Entries []Entry `json:"entries"`
}

// Entry is one capability.
//
// Every field exists to answer a question the user would otherwise have to ask
// somebody else. If a field does not teach and does not decide, it should not
// be here — the manifest is a teaching document that happens to be
// machine-readable, and bloat in it is bloat in the binary.
type Entry struct {
	ID    string `json:"id"`
	Title string `json:"title"`
	// Teaches is the discovery surface and the reason this package exists:
	// "why would I ever want this?", answered for somebody who has never heard
	// of the tool.
	Teaches string `json:"teaches"`
	// Unlocks is what the AGENT gains, concretely.
	Unlocks []string `json:"unlocks"`
	Detect  Detect   `json:"detect"`
	// Platforms lists the operating systems (runtime.GOOS values) this entry
	// can exist on. Empty means everywhere this program runs.
	//
	// GORILLA FIX (2026-10-05), from the /arsenal audit (A10): on Windows the
	// "minimum" series could never read 8/8, because bubblewrap is built on
	// Linux kernel namespaces and has no Windows form at all — and the series
	// "understanding Windows files on Linux" was listed ON Windows with every
	// entry N/A. An entry that cannot exist here is not a missing capability,
	// and counting it as one makes the score unreachable. See Applicable and
	// ForPlatform.
	Platforms []string `json:"platforms,omitempty"`
	// Packages is per package manager, because "how do I get it" has a
	// different answer on every distribution and a wrong one is useless.
	Packages map[string][]string `json:"packages"`
	Needs    Needs               `json:"needs"`
	Tier     string              `json:"tier"`
	// Caveats is the field most tools' own documentation omits and the one a
	// user most needs: what will disappoint you about this. It is the
	// difference between a catalogue and honest advice.
	Caveats string `json:"caveats"`
}

// Detect is how to find out whether it is ALREADY HERE. Never claimed, always
// checked — the manifest says what a tool WOULD unlock; only a successful
// detection says it IS unlocked.
type Detect struct {
	Binaries []string `json:"binaries"`
	// Mode is "all" (default) or "any".
	//
	// GORILLA FIX (2026-08-19): "all" was the only behaviour, and it was wrong
	// for entries whose binaries are ALTERNATIVE NAMES for the same thing.
	// ast-grep ships as `ast-grep` or as `sg` depending on how it was
	// installed; requiring both reported it as missing on a machine where it
	// was installed and working — which is precisely the bug this whole
	// feature exists to fix, reproduced inside the fix. Caught by driving the
	// real binary, not by any test.
	//
	// "all" stays the default because it is right for the common case:
	// poppler-utils genuinely gives you pdftotext AND pdfimages AND pdfinfo,
	// and having one of the three is worth reporting as partial.
	Mode string `json:"mode,omitempty"`
	// Impostors names the cases where a detect name resolves to something that
	// is NOT this tool.
	//
	// GORILLA FIX (2026-10-05), from the /arsenal audit (A1): `convert` is
	// ImageMagick on Linux and the FAT-to-NTFS disk converter on Windows
	// (C:\Windows\System32\convert.exe), so every Windows machine reported
	// ImageMagick as HAVE whether or not it was installed. A false HAVE is the
	// fault this feature exists to prevent, pointing the other way. The same
	// trap sits on Linux: /usr/bin/sg is a link to newgrp, not ast-grep, which
	// the astgrep caveat already said in words while detection ignored it.
	Impostors []Impostor `json:"impostors,omitempty"`
	// Paths lists, per operating system (runtime.GOOS), well-known install
	// locations to check when nothing was found on PATH. ${NAME} is an
	// environment variable; ${SCOOP} is the Scoop root. Only meaningful with
	// Mode "any": one existing file is the whole capability.
	//
	// GORILLA FIX (2026-10-05), from the /arsenal audit (A6): Scoop's
	// libreoffice package creates Start-menu shortcuts only — no shim, nothing
	// on PATH. Measured on a Windows machine with it installed: soffice.exe was
	// on the disk and /arsenal reported it missing, and would have gone on
	// offering the install command for ever.
	Paths map[string][]string `json:"paths,omitempty"`
}

// Impostor is one known name collision: on OS, a file called Binary that
// matches Under or Target is a different program.
type Impostor struct {
	Binary string `json:"binary"`
	// OS is a runtime.GOOS value.
	OS string `json:"os"`
	// Under is the name of an environment variable holding a directory. A hit
	// anywhere inside that directory is the operating system's own program.
	Under string `json:"under,omitempty"`
	// Target is the file name (without extension) the hit resolves to once
	// symbolic links are followed.
	Target string `json:"target,omitempty"`
	// Is says, in plain words, what the impostor really is. Shown to the user.
	Is string `json:"is"`
}

// Seams for tests. Production code never reassigns them.
var (
	lookPath = exec.LookPath
	goos     = runtime.GOOS
)

func (i Impostor) matches(binary, path string) bool {
	if i.Binary != binary || i.OS != goos {
		return false
	}
	if i.Under != "" {
		if root := os.Getenv(i.Under); root != "" && pathWithin(path, root) {
			return true
		}
	}
	if i.Target != "" {
		real, err := filepath.EvalSymlinks(path)
		if err != nil {
			real = path
		}
		base := filepath.Base(real)
		base = strings.TrimSuffix(base, filepath.Ext(base))
		if real != path && strings.EqualFold(base, i.Target) {
			return true
		}
	}
	return false
}

// pathWithin reports whether path lies inside dir. Case-insensitive on Windows,
// where C:\WINDOWS and C:\Windows are the same directory.
func pathWithin(path, dir string) bool {
	p, d := filepath.Clean(path), filepath.Clean(dir)
	if goos == "windows" {
		p, d = strings.ToLower(p), strings.ToLower(d)
	}
	return strings.HasPrefix(p, d+string(filepath.Separator))
}

// Applicable reports whether an entry can exist on this operating system.
func Applicable(e Entry) bool {
	if len(e.Platforms) == 0 {
		return true
	}
	for _, p := range e.Platforms {
		if p == goos {
			return true
		}
	}
	return false
}

// ForPlatform returns the manifest as it applies to this operating system:
// entries that cannot exist here are removed, and a series left with nothing in
// it is not listed at all.
func ForPlatform(m Manifest) Manifest {
	out := Manifest{Version: m.Version, Generated: m.Generated}
	for _, s := range m.Series {
		kept := Series{ID: s.ID, Title: s.Title, Why: s.Why}
		for _, e := range s.Entries {
			if Applicable(e) {
				kept.Entries = append(kept.Entries, e)
			}
		}
		if len(kept.Entries) > 0 {
			out.Series = append(out.Series, kept)
		}
	}
	return out
}

// Needs answers the question that decides everything for this audience: will
// this ask me for an account or a card?
type Needs struct {
	Account          bool `json:"account"`
	Card             bool `json:"card"`
	NetworkAtRuntime bool `json:"network_at_runtime"`
}

var (
	loadOnce sync.Once
	loaded   Manifest
	loadErr  error
)

// Load parses the embedded manifest. Parsed once; the result is read-only.
func Load() (Manifest, error) {
	loadOnce.Do(func() { loadErr = json.Unmarshal(manifestJSON, &loaded) })
	return loaded, loadErr
}

// Status is what is true about one entry ON THIS MACHINE.
type Status struct {
	Entry Entry
	// Present is measured, never assumed.
	Present bool
	// Found lists the binaries that were actually located, so the display can
	// say WHICH part is present when an entry covers several.
	Found []string
	// Missing lists the ones that were not.
	Missing []string
	// Ignored lists names that DID resolve, to a different program. They are
	// also in Missing; this is the explanation, so the screen can say why a
	// file the user can see on their own disk was not counted.
	Ignored []IgnoredHit
	// OffPath is true when the entry was found only at a well-known install
	// location, not on PATH. Found then holds the full path, because that is
	// the only way to call it.
	OffPath bool
}

// IgnoredHit is one detect name that resolved to an impostor.
type IgnoredHit struct {
	Binary string
	Path   string
	Is     string
}

// Partial reports an entry that is half-installed — some binaries present,
// some not. Worth distinguishing: "you have pdftotext but not pdfimages" is
// actionable, "not installed" is misleading.
func (s Status) Partial() bool { return len(s.Found) > 0 && len(s.Missing) > 0 }

// Detect probes this machine for one entry.
func DetectEntry(e Entry) Status {
	st := Status{Entry: e}
	for _, b := range e.Detect.Binaries {
		path, err := lookPath(b)
		if err != nil {
			st.Missing = append(st.Missing, b)
			continue
		}
		impostor := false
		for _, imp := range e.Detect.Impostors {
			if imp.matches(b, path) {
				st.Ignored = append(st.Ignored, IgnoredHit{Binary: b, Path: path, Is: imp.Is})
				impostor = true
				break
			}
		}
		if impostor {
			st.Missing = append(st.Missing, b)
			continue
		}
		st.Found = append(st.Found, b)
	}
	if e.Detect.Mode == "any" {
		// Any one name is the whole capability, so nothing is "missing" once
		// one is found — reporting the other spellings as missing would read
		// as a half-install that does not exist.
		st.Present = len(st.Found) > 0
		if !st.Present {
			if p := firstExistingPath(e.Detect.Paths[goos]); p != "" {
				st.Found, st.Present, st.OffPath = []string{p}, true, true
			}
		}
		if st.Present {
			st.Missing = nil
		}
		return st
	}
	st.Present = len(st.Found) > 0 && len(st.Missing) == 0
	return st
}

// scoopRoot is where Scoop keeps itself: $SCOOP when the user moved it,
// otherwise the scoop folder in the home directory. "" when neither is known.
func scoopRoot() string {
	if r := os.Getenv("SCOOP"); r != "" {
		return r
	}
	if home, err := os.UserHomeDir(); err == nil {
		return filepath.Join(home, "scoop")
	}
	return ""
}

// firstExistingPath expands each candidate and returns the first that is a
// file on this disk. A candidate naming a variable that is not set is skipped
// rather than expanded to a path that was never meant.
func firstExistingPath(candidates []string) string {
	for _, raw := range candidates {
		unset := false
		p := os.Expand(raw, func(name string) string {
			v := os.Getenv(name)
			if name == "SCOOP" {
				v = scoopRoot()
			}
			if v == "" {
				unset = true
			}
			return v
		})
		if unset {
			continue
		}
		p = filepath.FromSlash(p)
		if fi, err := os.Stat(p); err == nil && !fi.IsDir() {
			return p
		}
	}
	return ""
}

// PackageManager is what this machine actually uses to install things.
type PackageManager string

const (
	APT    PackageManager = "apt"
	Pacman PackageManager = "pacman"
	// GORILLA OVERRIDE (2026-09-01): Scoop, so /arsenal is not inert on Windows.
	//
	// Before this, DetectPackageManager could only return apt, pacman or
	// Unknown, so on Windows every entry reported "no supported package manager
	// found" and InstallCommand returned "". The screen listed thirty-three
	// capabilities and could not tell you how to obtain a single one of them -
	// which is the exact failure /arsenal exists to prevent: a capability that
	// is available and unknown.
	//
	// Scoop rather than winget or choco because it needs no administrator
	// rights, which matches this program's decision not to require elevation
	// (see winres.json). Package names were verified against the real main and
	// extras buckets on a Windows machine, not recalled - 24 of the 33 entries
	// have one, and the nine that do not are left absent so UnavailableNote can
	// say so honestly rather than offering a name that does not resolve.
	Scoop   PackageManager = "scoop"
	Unknown PackageManager = ""
)

// DetectPackageManager looks for the tool rather than reading /etc/os-release,
// because what matters is what can actually be run here.
func DetectPackageManager() PackageManager {
	if goos == "windows" {
		// scoop is a PowerShell function on a shim, so LookPath finds
		// scoop.cmd / scoop.ps1 rather than an .exe. Checking the shim
		// directory as well means a working Scoop is not missed just because
		// PATH has not been refreshed in this shell yet.
		if _, err := lookPath("scoop"); err == nil {
			return Scoop
		}
		if root := scoopRoot(); root != "" {
			if _, err := os.Stat(filepath.Join(root, "shims", "scoop.cmd")); err == nil {
				return Scoop
			}
		}
		return Unknown
	}
	if _, err := lookPath("apt-get"); err == nil {
		return APT
	}
	if _, err := lookPath("pacman"); err == nil {
		return Pacman
	}
	return Unknown
}

// CanMeasure reports whether this package manager can price a selection
// before installing it.
//
// GORILLA FIX (2026-10-05), from the /arsenal audit (A3): on Scoop the screen
// said "p measure the real cost" in three places and p could only ever answer
// that Scoop does not report sizes. An offer the program knows it cannot keep
// is not made; the screen asks this function before printing the key.
func CanMeasure(pm PackageManager) bool { return pm == APT || pm == Pacman }

// CannotMeasureNote says why not, in the words shown on screen.
func CannotMeasureNote(pm PackageManager) string {
	switch {
	case CanMeasure(pm):
		return ""
	case pm == Scoop:
		// Scoop has no equivalent of `apt-get --print-uris`: sizes live inside
		// each manifest's architecture block and are often absent entirely.
		return "scoop does not report download size before installing"
	}
	return "no supported package manager found on this machine"
}

// PackagesFor returns the package names for this machine's package manager,
// or nil if this entry has none for it.
func PackagesFor(e Entry, pm PackageManager) []string {
	if pm == Unknown {
		return nil
	}
	return e.Packages[string(pm)]
}

// Available reports whether this entry can be installed by this machine's
// package manager at all.
//
// GORILLA FIX (2026-08-19): caught by the first real measurement run against
// this machine. ast-grep is not in Debian, so its apt package list is empty —
// and MeasureCost dutifully priced it at 0 B, which on screen reads as FREE
// next to "not installed". It is not free; it is unobtainable this way, and
// those are opposite facts.
//
// This is the same trap the rest of this file is written against, arriving in
// my own code within an hour of writing the rule.
func Available(e Entry, pm PackageManager) bool {
	return len(PackagesFor(e, pm)) > 0
}

// UnavailableNote says WHY an entry cannot be offered here, so the user gets a
// route rather than a shrug.
func UnavailableNote(e Entry, pm PackageManager) string {
	if Available(e, pm) {
		return ""
	}
	if pm == Unknown {
		return "no supported package manager found — install it however this system does"
	}
	return "not packaged for " + string(pm) + " — it exists, but your package manager cannot fetch it"
}

// InstallCommand is the exact command, shown to the user and never run behind
// their back.
//
// It is a string rather than an exec.Cmd on purpose. /arsenal must NEVER sudo
// silently: an installer is the highest-stakes prompt in the program, and the
// August 2026 audit established that a prompt describing less than what happens
// is worse than no prompt at all. So the command is DISPLAYED, and running it
// is a separate, explicit act.
func InstallCommand(pkgs []string, pm PackageManager) string {
	return strings.Join(InstallCommands(pkgs, pm), "\n")
}

// scoopHasBucket reports whether a Scoop bucket is already added on this
// machine. A variable so a test can answer for a machine it is not running on.
var scoopHasBucket = func(name string) bool {
	root := scoopRoot()
	if root == "" {
		return false
	}
	fi, err := os.Stat(filepath.Join(root, "buckets", name))
	return err == nil && fi.IsDir()
}

// InstallCommands is the same thing as separate lines, one command each, in
// the order they must be run.
//
// GORILLA FIX (2026-10-05), from the /arsenal audit (A11): the Scoop form was
// one line, "scoop bucket add extras; scoop install ...". The semicolon is
// PowerShell. Pasted into cmd.exe it is handed to scoop as part of the bucket
// name and the whole line fails. Two lines work in both shells. The bucket
// line is also left out when the bucket is already there, which is checked on
// the disk rather than assumed either way.
func InstallCommands(pkgs []string, pm PackageManager) []string {
	if len(pkgs) == 0 {
		return nil
	}
	switch pm {
	case APT:
		return []string{"sudo apt-get install -y " + strings.Join(pkgs, " ")}
	case Pacman:
		return []string{"sudo pacman -S --needed " + strings.Join(pkgs, " ")}
	case Scoop:
		// No sudo: Scoop installs into the user's own profile, which is the
		// reason it was chosen over winget and choco. A package written
		// "bucket/name" lives outside the default bucket, so the command that
		// adds the bucket comes first - otherwise the install fails with a
		// bucket error that says nothing about buckets.
		var out []string
		seen := map[string]bool{}
		for _, p := range pkgs {
			i := strings.Index(p, "/")
			if i <= 0 {
				continue
			}
			bucket := p[:i]
			if seen[bucket] || scoopHasBucket(bucket) {
				continue
			}
			seen[bucket] = true
			out = append(out, "scoop bucket add "+bucket)
		}
		return append(out, "scoop install "+strings.Join(pkgs, " "))
	}
	return nil
}

// Cost is what a selection will really take, on THIS machine.
type Cost struct {
	DownloadBytes int64
	DiskBytes     int64
	// Measured distinguishes a real figure from an unavailable one. A cost we
	// could not measure must say so rather than show a zero, because a zero
	// reads as "free".
	Measured bool
	// DiskMeasured is the same distinction for DiskBytes alone. pacman -Sp
	// reports download size only, and until 2026-10-05 the screen printed that
	// as "0 B on disk" — a zero standing in for "not reported".
	DiskMeasured bool
	// Note carries why, when Measured is false.
	Note string
}

// aptSizeRe matches apt-get's summary lines. Sizes come with a unit and a
// decimal point, both locale-dependent, which is why the command is run under
// LC_ALL=C below.
var (
	aptNeedRe  = regexp.MustCompile(`Need to get ([0-9.,]+) ?([kMG]?)B`)
	aptDiskRe  = regexp.MustCompile(`After this operation, ([0-9.,]+) ?([kMG]?)B of additional disk space`)
	aptFreedRe = regexp.MustCompile(`After this operation, ([0-9.,]+) ?([kMG]?)B disk space will be freed`)
)

// MeasureCost asks the package manager what a selection would really cost,
// installing nothing.
//
// This is the honest way to price it, and it is better than any static table
// could be: `apt-get --print-uris` resolves the FULL dependency closure against
// what is ALREADY on this machine. A user who happens to have half the
// dependencies is told the truth about their own remaining cost, not a
// worst-case figure from a spreadsheet.
//
// The research measured a 147x gap between the two: poppler-utils costs 0.2 MB
// on a fully-loaded desktop and 29.5 MB on a fresh netinst. Quoting either
// number as universal would be misleading by omission.
func MeasureCost(pkgs []string, pm PackageManager) Cost {
	if !CanMeasure(pm) {
		// Reporting an unmeasured 0 here would read as FREE next to "not
		// installed", which is the trap the rest of this file is written
		// against - so it says plainly that it does not know.
		return Cost{Note: CannotMeasureNote(pm)}
	}
	if len(pkgs) == 0 {
		// GORILLA FIX (2026-10-05), from the /arsenal audit (A2): this returned
		// Measured with zero bytes, and the screen turned that into "nothing to
		// download - all of it is already here". The usual way to arrive here
		// is the opposite: every selected entry is one this package manager
		// CANNOT fetch, so there were no packages to ask about. Nothing was
		// asked, so nothing was measured.
		return Cost{Note: "no packages in this selection, so there was nothing to ask the package manager"}
	}
	if pm == APT {
		return measureAPT(pkgs)
	}
	return measurePacman(pkgs)
}

func measureAPT(pkgs []string) Cost {
	args := append([]string{"--print-uris", "install", "-y"}, pkgs...)
	cmd := exec.Command("apt-get", args...)
	// LC_ALL=C so the numbers parse the same everywhere. A decimal comma would
	// silently change 29.5 MB into 295 MB.
	cmd.Env = append(cmd.Environ(), "LC_ALL=C", "DEBIAN_FRONTEND=noninteractive")
	out, err := cmd.CombinedOutput()
	if err != nil {
		// apt exits non-zero when a package name is unknown. Report that
		// rather than a zero, which would read as "free".
		return Cost{Note: firstUsefulLine(string(out), "could not price this — "+err.Error())}
	}
	return parseAPTCost(string(out))
}

// parseAPTCost reads apt-get's summary lines.
//
// GORILLA FIX (2026-10-05), from the /arsenal audit (A4): the cost was marked
// Measured BEFORE the output was read, so an answer matching neither pattern —
// a changed format, a translated line that slipped past LC_ALL=C — stayed
// "measured" at 0 B and reached the screen as "nothing to download". An
// answer this program cannot read is not a measurement of zero. Each figure
// now counts only if its own line was found.
func parseAPTCost(out string) Cost {
	need := aptNeedRe.FindStringSubmatch(out)
	if need == nil {
		return Cost{Note: "apt answered, but not in a form this program can read — not measured"}
	}
	c := Cost{Measured: true, DownloadBytes: parseSize(need[1], need[2])}
	switch {
	case aptDiskRe.MatchString(out):
		m := aptDiskRe.FindStringSubmatch(out)
		c.DiskBytes, c.DiskMeasured = parseSize(m[1], m[2]), true
	case aptFreedRe.MatchString(out):
		// Installing frees space only when it replaces something larger. No
		// ADDITIONAL space is a true zero, not a missing figure.
		c.DiskMeasured = true
	default:
		c.Note = "download only — apt's disk figure could not be read"
	}
	return c
}

func measurePacman(pkgs []string) Cost {
	args := append([]string{"-Sp", "--print-format", "%s"}, pkgs...)
	cmd := exec.Command("pacman", args...)
	cmd.Env = append(cmd.Environ(), "LC_ALL=C")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return Cost{Note: firstUsefulLine(string(out), "could not price this — "+err.Error())}
	}
	return parsePacmanCost(string(out))
}

// parsePacmanCost sums the per-package sizes `pacman -Sp --print-format %s`
// prints, one number per line. Same rule as parseAPTCost: with no line it can
// read as a size, nothing was measured.
func parsePacmanCost(out string) Cost {
	var c Cost
	sizes := 0
	for _, line := range strings.Split(strings.TrimSpace(out), "\n") {
		if n, err := strconv.ParseInt(strings.TrimSpace(line), 10, 64); err == nil && n >= 0 {
			c.DownloadBytes += n
			sizes++
		}
	}
	if sizes == 0 {
		return Cost{Note: "pacman answered, but not in a form this program can read — not measured"}
	}
	c.Measured = true
	// pacman -Sp reports download size only. Saying so beats reporting 0 for
	// disk, which would read as "takes no space".
	c.Note = "download only — pacman does not report installed size here"
	return c
}

func parseSize(num, unit string) int64 {
	num = strings.ReplaceAll(num, ",", "")
	f, err := strconv.ParseFloat(num, 64)
	if err != nil {
		return 0
	}
	switch unit {
	case "k":
		f *= 1000
	case "M":
		f *= 1000 * 1000
	case "G":
		f *= 1000 * 1000 * 1000
	}
	return int64(f)
}

func firstUsefulLine(out, fallback string) string {
	for _, l := range strings.Split(out, "\n") {
		l = strings.TrimSpace(l)
		if l != "" && !strings.HasPrefix(l, "Reading") && !strings.HasPrefix(l, "Building") {
			return l
		}
	}
	return fallback
}

// HumanBytes formats a size the way a person reads it.
func HumanBytes(n int64) string {
	switch {
	case n <= 0:
		return "0 B"
	case n < 1000:
		return fmt.Sprintf("%d B", n)
	case n < 1000*1000:
		return fmt.Sprintf("%.0f KB", float64(n)/1000)
	case n < 1000*1000*1000:
		return fmt.Sprintf("%.1f MB", float64(n)/(1000*1000))
	}
	return fmt.Sprintf("%.2f GB", float64(n)/(1000*1000*1000))
}

// DownloadTime states the cost in the unit somebody waiting actually feels.
//
// kbPerSec defaults to the audience this project is built for. §8: "download
// size is time" — 18 MB at 8 KB/s is roughly forty minutes of someone's life,
// and a figure in megabytes hides that from the person who most needs to know.
func DownloadTime(n int64, kbPerSec float64) string {
	if n <= 0 || kbPerSec <= 0 {
		return ""
	}
	secs := float64(n) / (kbPerSec * 1000)
	switch {
	case secs < 90:
		return fmt.Sprintf("%.0f seconds", secs)
	case secs < 90*60:
		return fmt.Sprintf("%.0f minutes", secs/60)
	}
	return fmt.Sprintf("%.1f hours", secs/3600)
}

// ── tagfiles ─────────────────────────────────────────────────────────────

// TagfileDir is where selections are kept: alongside the user's other config,
// not in the working directory, because a selection is about the MACHINE and
// would otherwise end up committed to whatever repository happened to be open.
func TagfileDir() string {
	return filepath.Join(config.CacheBase(), "arsenal")
}

// TagfilePath is the default selection file.
func TagfilePath() string { return filepath.Join(TagfileDir(), "selection.tagfile") }

// Tagfiles lists every selection file in TagfileDir: the user's own first,
// then the rest by name.
//
// GORILLA FIX (2026-10-05), from the /arsenal audit (A12): loading could read
// exactly one path, the same one saving writes. The stated point of a tagfile
// is that somebody else can send you theirs — and the only way to load theirs
// was to overwrite your own with it. Any file ending .tagfile that is dropped
// into this folder can now be loaded, and nothing is overwritten.
func Tagfiles() []string {
	matches, _ := filepath.Glob(filepath.Join(TagfileDir(), "*.tagfile"))
	sort.Strings(matches)
	own := TagfilePath()
	out := make([]string, 0, len(matches))
	for _, p := range matches {
		if p == own {
			out = append([]string{p}, out...)
			continue
		}
		out = append(out, p)
	}
	return out
}

// SaveTagfile writes a selection as plain text and returns the path.
//
// The format is deliberately the simplest thing that can be shared: one id per
// line, # for comments. A person must be able to open it in any editor,
// understand it, and edit it, without this program and without knowing JSON.
// The header carries the human-readable titles so the file explains itself to
// somebody who receives it with no context.
func SaveTagfile(ids []string, m Manifest, pm PackageManager) (string, error) {
	dir := TagfileDir()
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}
	titles := map[string]string{}
	for _, s := range m.Series {
		for _, e := range s.Entries {
			titles[e.ID] = e.Title
		}
	}

	var b strings.Builder
	b.WriteString("# gorilla-opencode arsenal selection\n")
	b.WriteString("# One capability id per line. # starts a comment.\n")
	b.WriteString("# Edit it, share it, hand it to someone else — /arsenal reads it back.\n#\n")
	for _, id := range ids {
		if t := titles[id]; t != "" {
			fmt.Fprintf(&b, "%-16s # %s\n", id, t)
		} else {
			fmt.Fprintf(&b, "%s\n", id)
		}
	}

	path := TagfilePath()
	if err := os.WriteFile(path, []byte(b.String()), 0o600); err != nil {
		return "", err
	}
	return path, nil
}

// LoadTagfile reads a selection back. Unknown ids are RETURNED rather than
// dropped: a tagfile from a newer version naming a capability this build does
// not have is a fact the user should hear, not something to swallow silently.
func LoadTagfile(path string, m Manifest) (ids []string, unknown []string, err error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, nil, err
	}
	known := map[string]bool{}
	for _, s := range m.Series {
		for _, e := range s.Entries {
			known[e.ID] = true
		}
	}
	for _, line := range strings.Split(string(raw), "\n") {
		if i := strings.Index(line, "#"); i >= 0 {
			line = line[:i]
		}
		id := strings.TrimSpace(line)
		if id == "" {
			continue
		}
		if known[id] {
			ids = append(ids, id)
		} else {
			unknown = append(unknown, id)
		}
	}
	return ids, unknown, nil
}
