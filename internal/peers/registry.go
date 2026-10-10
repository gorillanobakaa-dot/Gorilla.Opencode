package peers

// GORILLA (2026-10-10): the register. One JSON file per running session, in a
// folder of the user's own state directory:
//
//	<state>/gorilla-opencode/peers/<pid>.json
//
// Written when a session starts, rewritten when it changes between busy and
// idle or is renamed, removed when it exits. A session that dies without
// removing its file is cleaned up by the next listing, which checks that the
// process is still alive.
//
// Nothing in a file is trusted beyond what can be checked: the process number
// must match the file name, and the endpoint must be exactly the one this
// program would have created for that process. A planted file pointing a
// client at some other pipe or socket is ignored.

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"
)

// Entry is one running session's register file.
type Entry struct {
	Name     string    `json:"name"`
	Folder   string    `json:"folder"`
	PID      int       `json:"pid"`
	Endpoint string    `json:"endpoint"`
	Version  string    `json:"version"`
	Started  time.Time `json:"started"`
	Busy     bool      `json:"busy"`

	// key is the file name without .json: "<pid>", or "<pid>-<tag>" for the
	// second session a test runs inside one process.
	key string
}

const (
	dirMode       os.FileMode = 0o700
	fileMode      os.FileMode = 0o600
	maxEntryBytes             = 16 * 1024
)

var keyRe = regexp.MustCompile(`^([0-9]{1,10})(-[a-z0-9]{1,16})?$`)

// Registry is the register folder.
type Registry struct{ Dir string }

// DefaultDir is the register folder under the program's state directory
// (config.StateBase(), which already ends in gorilla-opencode).
func DefaultDir(stateBase string) string { return filepath.Join(stateBase, "peers") }

// ensure creates the folder 0700 and, on Unix, refuses to use one that another
// account owns or can read. On Windows the folder inherits the user profile's
// ACL (the user, SYSTEM and Administrators); the access check that matters
// there is the pipe's own security descriptor.
func (r Registry) ensure() error {
	if r.Dir == "" {
		return errors.New("no register folder was given")
	}
	if err := os.MkdirAll(r.Dir, dirMode); err != nil {
		return err
	}
	if runtime.GOOS == "windows" {
		return nil
	}
	// MkdirAll applies the mode only to folders it creates; an older folder is
	// tightened here, and fails closed if it cannot be.
	if err := os.Chmod(r.Dir, dirMode); err != nil {
		return err
	}
	st, err := os.Stat(r.Dir)
	if err != nil {
		return err
	}
	if st.Mode().Perm()&0o077 != 0 {
		return fmt.Errorf("the register folder %s is open to other accounts (%v)", r.Dir, st.Mode().Perm())
	}
	return checkDirOwner(r.Dir)
}

func (r Registry) path(key string) string { return filepath.Join(r.Dir, key+".json") }

// write replaces an entry atomically: a temporary file, then a rename, so a
// reader never sees half an entry.
func (r Registry) write(e Entry) error {
	if !keyRe.MatchString(e.key) {
		return fmt.Errorf("invalid register key %q", e.key)
	}
	if err := r.ensure(); err != nil {
		return err
	}
	data, err := json.MarshalIndent(e, "", "  ")
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(r.Dir, ".entry-*.tmp")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	_, werr := tmp.Write(data)
	cerr := tmp.Close()
	if werr != nil || cerr != nil {
		_ = os.Remove(tmpName)
		if werr != nil {
			return werr
		}
		return cerr
	}
	_ = os.Chmod(tmpName, fileMode)
	// On Windows a rename onto a file another session is reading at that moment
	// fails with a sharing violation; a reader holds it for microseconds.
	for i := 0; ; i++ {
		err = os.Rename(tmpName, r.path(e.key))
		if err == nil || i == 4 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if err != nil {
		_ = os.Remove(tmpName)
	}
	return err
}

// remove deletes an entry and, on Unix, its socket file.
func (r Registry) remove(key string) {
	if !keyRe.MatchString(key) {
		return
	}
	_ = os.Remove(r.path(key))
	removeEndpointFile(r.Dir, key)
}

// List returns every live, valid entry, sorted by name. Entries of processes
// that are no longer running are deleted on the way.
func (r Registry) List() ([]Entry, error) {
	des, err := os.ReadDir(r.Dir)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var out []Entry
	for _, de := range des {
		name := de.Name()
		if de.IsDir() || !strings.HasSuffix(name, ".json") {
			continue
		}
		key := strings.TrimSuffix(name, ".json")
		m := keyRe.FindStringSubmatch(key)
		if m == nil {
			continue // not a name this program writes; left alone
		}
		pid, err := strconv.Atoi(m[1])
		if err != nil {
			continue
		}
		if !pidAlive(pid) {
			r.remove(key)
			continue
		}
		if e, ok := r.read(key, pid); ok {
			out = append(out, e)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Name != out[j].Name {
			return out[i].Name < out[j].Name
		}
		return out[i].key < out[j].key
	})
	return out, nil
}

// read loads and checks one entry.
func (r Registry) read(key string, pid int) (Entry, bool) {
	f, err := os.Open(r.path(key))
	if err != nil {
		return Entry{}, false
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, maxEntryBytes+1))
	if err != nil || len(data) > maxEntryBytes {
		return Entry{}, false
	}
	var e Entry
	if err := json.Unmarshal(data, &e); err != nil {
		return Entry{}, false
	}
	if e.PID != pid || e.Endpoint != endpointFor(r.Dir, key) {
		return Entry{}, false
	}
	e.Name = cleanName(e.Name)
	if e.Name == "" {
		return Entry{}, false
	}
	e.Folder = cleanLine(e.Folder, maxFolderLen)
	e.Version = cleanLine(e.Version, 64)
	e.key = key
	return e, true
}

// baseName is the default session name for a working folder.
func baseName(folder string) string {
	n := cleanName(filepath.Base(folder))
	if n == "" {
		n = "session"
	}
	return n
}

// uniqueName returns base, or base-2, base-3 ... whichever is not taken. taken
// holds lower-cased names, because two sessions called Proj and proj would be
// one name to anyone typing it.
func uniqueName(base string, taken map[string]bool) string {
	if !taken[strings.ToLower(base)] {
		return base
	}
	for i := 2; ; i++ {
		suffix := "-" + strconv.Itoa(i)
		b := base
		if len(b)+len(suffix) > maxNameLen {
			b = b[:maxNameLen-len(suffix)]
		}
		if c := b + suffix; !taken[strings.ToLower(c)] {
			return c
		}
	}
}
