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
}

// Partial reports an entry that is half-installed — some binaries present,
// some not. Worth distinguishing: "you have pdftotext but not pdfimages" is
// actionable, "not installed" is misleading.
func (s Status) Partial() bool { return len(s.Found) > 0 && len(s.Missing) > 0 }

// Detect probes this machine for one entry.
func DetectEntry(e Entry) Status {
	st := Status{Entry: e}
	for _, b := range e.Detect.Binaries {
		if _, err := exec.LookPath(b); err == nil {
			st.Found = append(st.Found, b)
		} else {
			st.Missing = append(st.Missing, b)
		}
	}
	if e.Detect.Mode == "any" {
		// Any one name is the whole capability, so nothing is "missing" once
		// one is found — reporting the other spellings as missing would read
		// as a half-install that does not exist.
		st.Present = len(st.Found) > 0
		if st.Present {
			st.Missing = nil
		}
		return st
	}
	st.Present = len(st.Found) > 0 && len(st.Missing) == 0
	return st
}

// PackageManager is what this machine actually uses to install things.
type PackageManager string

const (
	APT     PackageManager = "apt"
	Pacman  PackageManager = "pacman"
	Unknown PackageManager = ""
)

// DetectPackageManager looks for the tool rather than reading /etc/os-release,
// because what matters is what can actually be run here.
func DetectPackageManager() PackageManager {
	if _, err := exec.LookPath("apt-get"); err == nil {
		return APT
	}
	if _, err := exec.LookPath("pacman"); err == nil {
		return Pacman
	}
	return Unknown
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
	if len(pkgs) == 0 {
		return ""
	}
	switch pm {
	case APT:
		return "sudo apt-get install -y " + strings.Join(pkgs, " ")
	case Pacman:
		return "sudo pacman -S --needed " + strings.Join(pkgs, " ")
	}
	return ""
}

// Cost is what a selection will really take, on THIS machine.
type Cost struct {
	DownloadBytes int64
	DiskBytes     int64
	// Measured distinguishes a real figure from an unavailable one. A cost we
	// could not measure must say so rather than show a zero, because a zero
	// reads as "free".
	Measured bool
	// Note carries why, when Measured is false.
	Note string
}

// aptSizeRe matches apt-get's summary lines. Sizes come with a unit and a
// decimal point, both locale-dependent, which is why the command is run under
// LC_ALL=C below.
var (
	aptNeedRe = regexp.MustCompile(`Need to get ([0-9.,]+) ?([kMG]?)B`)
	aptDiskRe = regexp.MustCompile(`After this operation, ([0-9.,]+) ?([kMG]?)B of additional disk space`)
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
	if len(pkgs) == 0 {
		return Cost{Measured: true}
	}
	switch pm {
	case APT:
		return measureAPT(pkgs)
	case Pacman:
		return measurePacman(pkgs)
	}
	return Cost{Note: "no supported package manager found on this machine"}
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
	c := Cost{Measured: true}
	if m := aptNeedRe.FindStringSubmatch(string(out)); m != nil {
		c.DownloadBytes = parseSize(m[1], m[2])
	}
	if m := aptDiskRe.FindStringSubmatch(string(out)); m != nil {
		c.DiskBytes = parseSize(m[1], m[2])
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
	c := Cost{Measured: true}
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if n, err := strconv.ParseInt(strings.TrimSpace(line), 10, 64); err == nil {
			c.DownloadBytes += n
		}
	}
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
