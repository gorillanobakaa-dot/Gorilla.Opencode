// Package toolbox carries the helper programs gorilla-opencode ships inside its
// own executable (pfind, uutils coreutils, ast-grep, difftastic, sd, hyperfine,
// ninja) and hands them out safely.
//
// Owner decision (2026-10-26): every helper is EMBEDDED in gorilla-opencode.exe,
// extracted on first use into the private per-user cache under a name taken from
// its sha256, and its hash is checked before every exec. The helpers are only
// embedded when the binary is built with `-tags toolbox` (scripts/build-toolbox.ps1
// puts them in bin/<goos>-<goarch>/ first); a plain `go build` embeds none and
// every lookup answers ErrNotEmbedded, so a fresh clone still builds and tests.
//
// What is trusted: SHA256SUMS in this folder, compiled into the binary. An
// embedded helper whose bytes do not match its line there, or that has no line,
// is refused. An extracted file is hashed again before each exec and rewritten
// from the embedded bytes when it does not match.
//
// Layout of the cache: <UserCacheDir>/gorilla-opencode/toolbox/<sha256>/<name>.exe.
// The FOLDER carries the hash and the file keeps its plain name, so the shell
// can call `sd` or `ast-grep` by name once the folder is on PATH, and a new
// build with a new helper never runs a stale extract.
package toolbox

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"sync"
)

//go:embed SHA256SUMS
var pinsText string

// ErrNotEmbedded: this build carries no such helper (a plain `go build`, or a
// helper that is not built for this platform).
var ErrNotEmbedded = errors.New("not embedded in this build")

// Box is one set of embedded helpers. The package-level functions use the
// binary's own; tests build their own with New.
type Box struct {
	fsys      fs.FS             // holds <platform>/<file> or <platform>/<file>.gz
	platform  string            // runtime.GOOS + "-" + runtime.GOARCH
	pins      map[string]string // "<platform>/<file>" -> lower-case hex sha256
	cacheRoot func() (string, error)

	mu    sync.Mutex
	bytes map[string][]byte // embedded bytes already checked against their pin
}

// New makes a Box over fsys. cacheRoot returns the folder the toolbox folder
// is made in (normally os.UserCacheDir).
func New(fsys fs.FS, platform string, pins map[string]string, cacheRoot func() (string, error)) *Box {
	return &Box{fsys: fsys, platform: platform, pins: pins, cacheRoot: cacheRoot, bytes: map[string][]byte{}}
}

// ParsePins reads a sha256sum-style list: "<64 hex>  <platform>/<file>" per
// line; blank lines and lines starting with # are skipped. Anything else is an
// error, so a damaged list fails the build's tests instead of pinning nothing.
func ParsePins(text string) (map[string]string, error) {
	pins := map[string]string{}
	sc := bufio.NewScanner(strings.NewReader(text))
	n := 0
	for sc.Scan() {
		n++
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		f := strings.Fields(line)
		if len(f) != 2 {
			return nil, fmt.Errorf("SHA256SUMS line %d: want \"<sha256>  <platform>/<file>\", got %q", n, line)
		}
		sum, name := strings.ToLower(f[0]), strings.TrimPrefix(f[1], "*")
		if b, err := hex.DecodeString(sum); err != nil || len(b) != sha256.Size {
			return nil, fmt.Errorf("SHA256SUMS line %d: %q is not a sha256", n, f[0])
		}
		if _, dup := pins[name]; dup {
			return nil, fmt.Errorf("SHA256SUMS line %d: %s is pinned twice", n, name)
		}
		pins[name] = sum
	}
	return pins, sc.Err()
}

// Names lists the helpers embedded for this platform, by plain name ("sd").
func (b *Box) Names() []string {
	if b == nil || b.fsys == nil {
		return nil
	}
	entries, err := fs.ReadDir(b.fsys, b.platform)
	if err != nil {
		return nil
	}
	var names []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		n := strings.TrimSuffix(e.Name(), ".gz")
		if n == "README.md" {
			continue
		}
		names = append(names, strings.TrimSuffix(n, ".exe"))
	}
	sort.Strings(names)
	return names
}

// fileName is the helper's file name on this platform: "sd.exe" on Windows.
func (b *Box) fileName(name string) string {
	if strings.HasPrefix(b.platform, "windows-") {
		return name + ".exe"
	}
	return name
}

// embedded returns the helper's bytes and pin after checking one against the
// other (once per Box; the bytes are part of the running binary).
func (b *Box) embedded(name string) ([]byte, string, error) {
	if b == nil || b.fsys == nil {
		return nil, "", fmt.Errorf("%s: %w", name, ErrNotEmbedded)
	}
	file := b.fileName(name)
	key := b.platform + "/" + file
	pin, pinned := b.pins[key]

	b.mu.Lock()
	defer b.mu.Unlock()
	if data, ok := b.bytes[key]; ok {
		return data, pin, nil
	}
	data, err := fs.ReadFile(b.fsys, path.Join(b.platform, file))
	if errors.Is(err, fs.ErrNotExist) {
		gz, gerr := fs.ReadFile(b.fsys, path.Join(b.platform, file+".gz"))
		if gerr != nil {
			return nil, "", fmt.Errorf("%s: %w", name, ErrNotEmbedded)
		}
		zr, zerr := gzip.NewReader(bytes.NewReader(gz))
		if zerr != nil {
			return nil, "", fmt.Errorf("%s: embedded copy is damaged: %w", name, zerr)
		}
		data, err = io.ReadAll(zr)
	}
	if err != nil {
		return nil, "", fmt.Errorf("%s: reading the embedded copy: %w", name, err)
	}
	if !pinned {
		return nil, "", fmt.Errorf("%s: embedded but not pinned in SHA256SUMS; refusing to run it", name)
	}
	if got := hashBytes(data); got != pin {
		return nil, "", fmt.Errorf("%s: embedded copy has sha256 %s, SHA256SUMS pins %s; refusing to run it", name, got, pin)
	}
	b.bytes[key] = data
	return data, pin, nil
}

// Path returns the helper's extracted, hash-checked file, ready to exec.
// It is called before EVERY exec: the file is hashed each time and, when it
// does not match its pin (deleted, damaged, replaced), rewritten from the
// embedded bytes and hashed again.
func (b *Box) Path(name string) (string, error) {
	data, pin, err := b.embedded(name)
	if err != nil {
		return "", err
	}
	root, err := b.cacheRoot()
	if err != nil {
		return "", fmt.Errorf("%s: no cache folder: %w", name, err)
	}
	base := filepath.Join(root, "gorilla-opencode", "toolbox")
	dir := filepath.Join(base, pin)
	target := filepath.Join(dir, b.fileName(name))
	if hashFile(target) == pin {
		return target, nil
	}
	for _, d := range []string{filepath.Join(root, "gorilla-opencode"), base, dir} {
		if err := ensurePrivateDir(d); err != nil {
			return "", fmt.Errorf("%s: %w", name, err)
		}
	}
	if err := writeAtomic(dir, target, data); err != nil {
		return "", fmt.Errorf("%s: extracting to %s: %w", name, dir, err)
	}
	if got := hashFile(target); got != pin {
		return "", fmt.Errorf("%s: extracted file %s has sha256 %q, want %s; refusing to run it", name, target, got, pin)
	}
	return target, nil
}

// Dirs extracts and checks every embedded helper and returns their folders,
// for PATH. A helper that fails is left out and named in the error.
func (b *Box) Dirs() ([]string, error) {
	var dirs []string
	var errs []error
	for _, n := range b.Names() {
		p, err := b.Path(n)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		dirs = append(dirs, filepath.Dir(p))
	}
	return dirs, errors.Join(errs...)
}

// Mentioned lists the embedded helpers a shell command calls by name.
func (b *Box) Mentioned(command string) []string {
	var hit []string
	for _, n := range b.Names() {
		re := regexp.MustCompile(`(?i)(^|[\s;&|(` + "`" + `{'"=/\\])` + regexp.QuoteMeta(n) + `(\.exe)?($|[\s;&|)'"}])`)
		if re.MatchString(command) {
			hit = append(hit, n)
		}
	}
	return hit
}

// VerifyMentioned runs Path (hash check, re-extract when needed) for every
// helper the command calls by name, so the shell never execs an unchecked one.
func (b *Box) VerifyMentioned(command string) error {
	var errs []error
	for _, n := range b.Mentioned(command) {
		if _, err := b.Path(n); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

// ensurePrivateDir makes dir (0700) and refuses it when other local users
// could write to it. On Windows the check is by location (see
// CacheDirIsPrivate), so it runs BEFORE the folder is made: nothing is ever
// created in a shared place.
func ensurePrivateDir(dir string) error {
	if runtime.GOOS == "windows" {
		if err := CacheDirIsPrivate(dir); err != nil {
			return err
		}
		return os.MkdirAll(dir, 0o700)
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	return CacheDirIsPrivate(dir)
}

// writeAtomic writes then renames, so a concurrent session never execs a
// half-written file.
func writeAtomic(dir, target string, data []byte) error {
	tmp, err := os.CreateTemp(dir, ".extract-*.tmp")
	if err != nil {
		return err
	}
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		os.Remove(tmp.Name())
		return err
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmp.Name())
		return err
	}
	if err := os.Chmod(tmp.Name(), 0o700); err != nil {
		os.Remove(tmp.Name())
		return err
	}
	if err := os.Rename(tmp.Name(), target); err != nil {
		os.Remove(tmp.Name())
		return err
	}
	return nil
}

func hashBytes(b []byte) string {
	s := sha256.Sum256(b)
	return hex.EncodeToString(s[:])
}

// hashFile is "" when the file cannot be read.
func hashFile(p string) string {
	f, err := os.Open(p)
	if err != nil {
		return ""
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return ""
	}
	return hex.EncodeToString(h.Sum(nil))
}

// CacheRoot is where the binary's own toolbox folder goes: the user cache, or
// a per-user folder under the temp dir when there is none (cron, containers).
func CacheRoot() (string, error) {
	if d, err := os.UserCacheDir(); err == nil {
		return d, nil
	}
	return filepath.Join(os.TempDir(), fmt.Sprintf("gorilla-opencode-%d", os.Getuid())), nil
}

// PrependPath puts dirs at the front of PATH in env (case-insensitive key on
// Windows, where it is usually "Path"). A missing PATH is created.
func PrependPath(env []string, dirs []string) []string {
	if len(dirs) == 0 {
		return env
	}
	add := strings.Join(dirs, string(os.PathListSeparator))
	out := append([]string(nil), env...)
	for i, kv := range out {
		eq := strings.IndexByte(kv, '=')
		if eq <= 0 {
			continue
		}
		k := kv[:eq]
		if k == "PATH" || (runtime.GOOS == "windows" && strings.EqualFold(k, "PATH")) {
			if v := kv[eq+1:]; v != "" {
				add += string(os.PathListSeparator) + v
			}
			out[i] = k + "=" + add
			return out
		}
	}
	return append(out, "PATH="+add)
}

var (
	defaultOnce sync.Once
	defaultBox  *Box
	defaultErr  error
)

// Default is the binary's own toolbox. Its error is a damaged SHA256SUMS.
func Default() (*Box, error) {
	defaultOnce.Do(func() {
		pins, err := ParsePins(pinsText)
		if err != nil {
			defaultErr = err
			return
		}
		defaultBox = New(embeddedFS(), runtime.GOOS+"-"+runtime.GOARCH, pins, CacheRoot)
	})
	return defaultBox, defaultErr
}

// Path is Default().Path: the helper's checked file, or ErrNotEmbedded.
func Path(name string) (string, error) {
	b, err := Default()
	if err != nil {
		return "", err
	}
	return b.Path(name)
}

// Names is Default().Names.
func Names() []string {
	b, _ := Default()
	return b.Names()
}

// Dirs is Default().Dirs.
func Dirs() ([]string, error) {
	b, err := Default()
	if err != nil {
		return nil, err
	}
	return b.Dirs()
}

// VerifyMentioned is Default().VerifyMentioned.
func VerifyMentioned(command string) error {
	b, err := Default()
	if err != nil {
		return err
	}
	return b.VerifyMentioned(command)
}

// IsToolboxFile reports whether p is inside the binary's toolbox cache folder.
func IsToolboxFile(p string) bool {
	root, err := CacheRoot()
	if err != nil {
		return false
	}
	rel, err := filepath.Rel(filepath.Join(root, "gorilla-opencode", "toolbox"), p)
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) && !filepath.IsAbs(rel)
}
