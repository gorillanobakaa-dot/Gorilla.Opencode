// Package pathlook answers "is this program on PATH" without walking PATH again
// every time the question is asked.
//
// GORILLA FIX (2026-10-27): exec.LookPath on Windows stats every PATH directory
// times every PATHEXT extension (about 30 x 12 on the owner's laptop). The
// bash tool's "is this PowerShell" question and /arsenal asked it thousands of
// times per test run: `go test ./internal/tui/components/dialog` in cacheable
// mode wrote a testlog of 1,289,552 entries (1,267,273 stats), and go.exe then
// hashed every one of them for hours after the tests had passed. Measured, not
// guessed: Scripts/gohang/results/2026-10-27-dialog-testlog.txt.
//
// Two answers here:
//   - PowerShell: which PowerShell this machine runs, resolved once per PATH
//     value (a process that changes PATH gets a fresh answer, nobody else pays).
//   - Snapshot: one batch of lookups (the /arsenal screen asks about ~70 names)
//     that lists each PATH directory once and probes only the hits, instead of
//     stat-ing names x directories x extensions.
package pathlook

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// Seams for tests. Production code never reassigns them.
var (
	lookPath = exec.LookPath
	readDir  = os.ReadDir
	goos     = runtime.GOOS
)

// Calls counts the work this package did, so a test can hold the number of
// PATH probes to a ceiling. Every field only ever grows.
type Calls struct {
	// LookPaths is calls to exec.LookPath, each a full or confirming probe.
	LookPaths int64
	// DirLists is PATH directories read by a Snapshot (an unchanged directory
	// already read is not read again).
	DirLists int64
	// DirStats is PATH directories a Snapshot looked at (one stat each, at
	// most once per directory per snapshot).
	DirStats int64
}

var lookPaths, dirLists, dirStats atomic.Int64

// Counted returns the work done so far in this process.
func Counted() Calls {
	return Calls{LookPaths: lookPaths.Load(), DirLists: dirLists.Load(), DirStats: dirStats.Load()}
}

func look(name string) (string, error) {
	lookPaths.Add(1)
	return lookPath(name)
}

// ─── PowerShell, once per PATH ────────────────────────────────────────────

type psAnswer struct {
	env        string // PATH and PATHEXT the answer was worked out for
	name, full string
	ok         bool
}

var (
	psMu   sync.Mutex
	psLast *psAnswer
)

// PowerShell reports the PowerShell this machine would run: 7+ (pwsh.exe) when
// present, else the 5.1 that ships with Windows (powershell.exe). name is the
// bare program name, full its resolved path; ok is false when neither is on
// PATH. Resolved once for a given PATH and PATHEXT.
func PowerShell() (name, full string, ok bool) {
	env := os.Getenv("PATH") + "\x00" + os.Getenv("PATHEXT")
	psMu.Lock()
	defer psMu.Unlock()
	if last := psLast; last != nil && last.env == env {
		return last.name, last.full, last.ok
	}
	a := &psAnswer{env: env}
	for _, n := range []string{"pwsh.exe", "powershell.exe"} {
		if p, err := look(n); err == nil {
			a.name, a.full, a.ok = n, p, true
			break
		}
	}
	psLast = a
	return a.name, a.full, a.ok
}

// ─── Snapshot: one batch of lookups ───────────────────────────────────────

// Snapshot is PATH as it was when the snapshot was taken. Each directory is
// looked at the first time a lookup needs it and never again, so a batch of N
// names costs at most one look per directory plus one confirming probe per
// hit. Take a new one when the answer must reflect programs installed since.
// Not safe for concurrent use.
//
// A look at a directory is one stat; it is listed again only when its
// modification time moved (adding, removing or renaming a file in it moves it),
// so a tool installed meanwhile is seen and an unchanged PATH is not re-read.
// Re-reading mattered beyond the reads themselves: go test hashes every
// directory a test opened by listing it again, once per open.
type Snapshot struct {
	dirs   []string
	exts   []string // Windows only: PATHEXT, lower case, each with its dot
	listed map[string]map[string]bool
	found  map[string]answer // names already asked, with their answer
}

type answer struct {
	path string
	err  error
}

// NewSnapshot captures PATH (and PATHEXT on Windows) now.
func NewSnapshot() *Snapshot {
	s := &Snapshot{listed: map[string]map[string]bool{}, found: map[string]answer{}}
	for _, d := range filepath.SplitList(os.Getenv("PATH")) {
		// exec.LookPath refuses a hit in a relative directory (exec.ErrDot);
		// every caller here treats that as "missing", so it is never listed.
		if d != "" && filepath.IsAbs(d) {
			s.dirs = append(s.dirs, d)
		}
	}
	if goos == "windows" {
		s.exts = pathExts(os.Getenv("PATHEXT"))
	}
	return s
}

// pathExts parses PATHEXT the way exec.LookPath does.
func pathExts(x string) []string {
	if x == "" {
		return []string{".com", ".exe", ".bat", ".cmd"}
	}
	var exts []string
	for _, e := range strings.Split(strings.ToLower(x), ";") {
		if e == "" {
			continue
		}
		if e[0] != '.' {
			e = "." + e
		}
		exts = append(exts, e)
	}
	return exts
}

// foldCase: file names on Windows and macOS do not care about case.
func foldCase() bool { return goos == "windows" || goos == "darwin" }

type listing struct {
	mod   time.Time
	fold  bool
	names map[string]bool
}

var (
	listMu   sync.Mutex
	listings = map[string]listing{}
)

func (s *Snapshot) names(dir string) map[string]bool {
	if m, ok := s.listed[dir]; ok {
		return m
	}
	m := map[string]bool{} // a missing PATH directory simply holds nothing
	dirStats.Add(1)
	if fi, err := os.Stat(dir); err == nil && fi.IsDir() {
		m = listed(dir, fi.ModTime())
	}
	s.listed[dir] = m
	return m
}

// listed returns dir's file names, from the last listing when dir has not been
// modified since.
func listed(dir string, mod time.Time) map[string]bool {
	fold := foldCase()
	listMu.Lock()
	l, ok := listings[dir]
	listMu.Unlock()
	if ok && l.fold == fold && l.mod.Equal(mod) {
		return l.names
	}
	dirLists.Add(1)
	m := map[string]bool{}
	entries, _ := readDir(dir)
	for _, e := range entries {
		n := e.Name()
		if fold {
			n = strings.ToLower(n)
		}
		m[n] = true
	}
	listMu.Lock()
	listings[dir] = listing{mod: mod, fold: fold, names: m}
	listMu.Unlock()
	return m
}

// candidates are the file names exec.LookPath would try for name, in its order.
func (s *Snapshot) candidates(name string) []string {
	if goos != "windows" {
		return []string{name}
	}
	var c []string
	if strings.Contains(name, ".") {
		c = append(c, name)
	}
	for _, e := range s.exts {
		c = append(c, name+e)
	}
	return c
}

// LookPath answers like exec.LookPath for this snapshot: the first PATH
// directory, in order, holding an executable called name (with a PATHEXT
// extension on Windows). The current directory is never searched. A name with
// a path separator in it is handed to exec.LookPath unchanged.
func (s *Snapshot) LookPath(name string) (string, error) {
	if name == "" || strings.ContainsAny(name, `/\:`) {
		return look(name)
	}
	if a, ok := s.found[name]; ok {
		return a.path, a.err
	}
	p, err := s.search(name)
	s.found[name] = answer{p, err}
	return p, err
}

func (s *Snapshot) search(name string) (string, error) {
	cands := s.candidates(name)
	for _, dir := range s.dirs {
		have := s.names(dir)
		for _, c := range cands {
			n := c
			if foldCase() {
				n = strings.ToLower(c)
			}
			if !have[n] {
				continue
			}
			// The listing says a file of that name exists; exec.LookPath on the
			// full path makes the same "is it a runnable file" check as before
			// (not a directory; the execute bit outside Windows), one probe.
			if p, err := look(filepath.Join(dir, c)); err == nil {
				return p, nil
			}
		}
	}
	return "", &exec.Error{Name: name, Err: exec.ErrNotFound}
}
