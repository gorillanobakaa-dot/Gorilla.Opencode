package basic

import (
	"archive/zip"
	"bytes"
	"encoding/hex"
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
)

// Result is one check's verdict with the reason, in words.
type Result struct {
	Kind string `json:"kind"`
	OK   bool   `json:"ok"`
	Why  string `json:"why"`
}

// RunChecks runs every check of j against the folder dir and the model's
// answer. It never looks at what the model said it did, only at the disk and
// at the answer text the check names.
func RunChecks(j Job, dir string, vars Vars, answer string) []Result {
	out := make([]Result, 0, len(j.Checks))
	for _, c := range j.Checks {
		ok, why := runCheck(c, dir, vars, answer)
		out = append(out, Result{Kind: c.Kind, OK: ok, Why: why})
	}
	return out
}

// AllPassed is true when every result is OK (and there is at least one).
func AllPassed(rs []Result) bool {
	for _, r := range rs {
		if !r.OK {
			return false
		}
	}
	return len(rs) > 0
}

func runCheck(c Check, dir string, vars Vars, answer string) (bool, string) {
	path := func(rel string) string { return filepath.Join(dir, filepath.FromSlash(vars.Expand(rel))) }
	switch c.Kind {
	case "absent":
		if _, err := os.Lstat(path(c.Path)); err == nil {
			return false, c.Path + " still exists"
		}
		return true, c.Path + " is gone"
	case "exists":
		if _, err := os.Stat(path(c.Path)); err != nil {
			return false, c.Path + " does not exist"
		}
		return true, c.Path + " exists"
	case "content":
		data, err := os.ReadFile(path(c.Path))
		if err != nil {
			return false, err.Error()
		}
		return matchText(c, vars, string(data), c.Path)
	case "answer":
		return checkAnswer(c, vars, answer)
	case "magic":
		want, err := hex.DecodeString(vars.Expand(c.Hex))
		if err != nil {
			return false, "bad hex: " + err.Error()
		}
		data, err := os.ReadFile(path(c.Path))
		if err != nil {
			return false, err.Error()
		}
		if c.Offset+len(want) > len(data) || !bytes.Equal(data[c.Offset:c.Offset+len(want)], want) {
			return false, fmt.Sprintf("%s: bytes at %d are not %s", c.Path, c.Offset, c.Hex)
		}
		return true, fmt.Sprintf("%s: bytes at %d are %s", c.Path, c.Offset, c.Hex)
	case "zip_has":
		zr, err := zip.OpenReader(path(c.Path))
		if err != nil {
			return false, c.Path + ": not a readable zip: " + err.Error()
		}
		defer zr.Close()
		have := map[string]bool{}
		for _, f := range zr.File {
			have[strings.TrimPrefix(filepath.ToSlash(f.Name), "./")] = true
		}
		for _, n := range c.Names {
			n = vars.Expand(n)
			if !have[n] && !hasSuffixEntry(have, n) {
				return false, fmt.Sprintf("%s: %q is not inside", c.Path, n)
			}
		}
		return true, c.Path + ": every named file is inside"
	case "dims":
		f, err := os.Open(path(c.Path))
		if err != nil {
			return false, err.Error()
		}
		defer f.Close()
		cfg, _, err := image.DecodeConfig(f)
		if err != nil {
			return false, c.Path + ": not a readable image: " + err.Error()
		}
		if cfg.Width != c.W || cfg.Height != c.H {
			return false, fmt.Sprintf("%s is %dx%d, want %dx%d", c.Path, cfg.Width, cfg.Height, c.W, c.H)
		}
		return true, fmt.Sprintf("%s is %dx%d", c.Path, cfg.Width, cfg.Height)
	case "same_tree":
		return sameTree(path(c.Path), path(c.Other))
	case "size_min":
		st, err := os.Stat(path(c.Path))
		if err != nil {
			return false, c.Path + " does not exist"
		}
		if st.Size() < int64(c.Min) {
			return false, fmt.Sprintf("%s is %d bytes, want at least %d", c.Path, st.Size(), c.Min)
		}
		return true, fmt.Sprintf("%s is %d bytes", c.Path, st.Size())
	case "recycled":
		return checkRecycled(c, path(c.Path))
	}
	return false, "unknown check " + c.Kind
}

func matchText(c Check, vars Vars, text, label string) (bool, string) {
	if c.Equals != "" {
		want := vars.Expand(c.Equals)
		if strings.TrimSpace(text) != strings.TrimSpace(want) {
			return false, fmt.Sprintf("%s: content is %q, want %q", label, clip(text), clip(want))
		}
		return true, label + ": content equals"
	}
	if c.Contains != "" {
		if !strings.Contains(text, vars.Expand(c.Contains)) {
			return false, fmt.Sprintf("%s: does not contain %q (has %q)", label, c.Contains, clip(text))
		}
		return true, label + ": contains " + c.Contains
	}
	re, err := regexp.Compile(vars.Expand(c.Regex))
	if err != nil {
		return false, "bad regex: " + err.Error()
	}
	if !re.MatchString(text) {
		return false, fmt.Sprintf("%s: no match for %s (has %q)", label, c.Regex, clip(text))
	}
	return true, label + ": matches " + c.Regex
}

func checkAnswer(c Check, vars Vars, answer string) (bool, string) {
	low := strings.ToLower(answer)
	has := func(s string) bool { return strings.Contains(low, strings.ToLower(vars.Expand(s))) }
	if c.Contains != "" && !has(c.Contains) {
		return false, fmt.Sprintf("answer lacks %q", c.Contains)
	}
	for _, s := range c.All {
		if !has(s) {
			return false, fmt.Sprintf("answer lacks %q", s)
		}
	}
	if len(c.Any) > 0 {
		found := false
		for _, s := range c.Any {
			if has(s) {
				found = true
				break
			}
		}
		if !found {
			return false, fmt.Sprintf("answer has none of %v", c.Any)
		}
	}
	for _, s := range c.None {
		if has(s) {
			return false, fmt.Sprintf("answer wrongly contains %q", s)
		}
	}
	if c.Regex != "" {
		re, err := regexp.Compile(vars.Expand(c.Regex))
		if err != nil {
			return false, "bad regex: " + err.Error()
		}
		distinct := map[string]bool{}
		for _, m := range re.FindAllString(answer, -1) {
			distinct[m] = true
		}
		want := c.Min
		if want < 1 {
			want = 1
		}
		if len(distinct) < want {
			return false, fmt.Sprintf("answer has %d distinct match(es) of %s, want %d", len(distinct), c.Regex, want)
		}
	}
	return true, "answer has what was asked"
}

// sameTree compares two folders: the same relative files with the same bytes.
func sameTree(a, b string) (bool, string) {
	list := func(root string) (map[string][]byte, error) {
		m := map[string][]byte{}
		err := filepath.Walk(root, func(p string, info os.FileInfo, err error) error {
			if err != nil || info.IsDir() {
				return err
			}
			rel, _ := filepath.Rel(root, p)
			data, err := os.ReadFile(p)
			m[filepath.ToSlash(rel)] = data
			return err
		})
		return m, err
	}
	ma, err := list(a)
	if err != nil {
		return false, err.Error()
	}
	mb, err := list(b)
	if err != nil {
		return false, err.Error()
	}
	if len(ma) == 0 {
		return false, "the source folder is empty or missing"
	}
	for name, data := range ma {
		other, ok := mb[name]
		if !ok {
			return false, name + " is missing from the copy"
		}
		if !bytes.Equal(data, other) {
			return false, name + " differs in the copy"
		}
	}
	if len(ma) != len(mb) {
		return false, fmt.Sprintf("%d files in the source, %d in the copy", len(ma), len(mb))
	}
	return true, fmt.Sprintf("%d files, identical", len(ma))
}

// checkRecycled passes when the file is gone from its place AND an item with
// its unique name is in the Recycle Bin (Windows) or the trash folder (Linux).
// A permanent delete fails it.
func checkRecycled(c Check, full string) (bool, string) {
	if _, err := os.Lstat(full); err == nil {
		return false, c.Path + " still exists"
	}
	name := filepath.Base(full)
	stem := strings.TrimSuffix(name, filepath.Ext(name))
	if runtime.GOOS == "windows" {
		script := `$ErrorActionPreference='Stop'; (New-Object -ComObject Shell.Application).NameSpace(10).Items() | ForEach-Object { $_.Name }`
		out, err := exec.Command("powershell", "-NoProfile", "-NonInteractive", "-Command", script).Output()
		if err != nil {
			return false, "cannot list the Recycle Bin: " + err.Error()
		}
		for _, line := range strings.Split(string(out), "\n") {
			if strings.Contains(line, stem) {
				return true, c.Path + " is in the Recycle Bin"
			}
		}
		return false, c.Path + " is gone but not in the Recycle Bin (deleted for good)"
	}
	home, _ := os.UserHomeDir()
	trash := filepath.Join(home, ".local", "share", "Trash", "files")
	if x := os.Getenv("XDG_DATA_HOME"); x != "" {
		trash = filepath.Join(x, "Trash", "files")
	}
	entries, _ := os.ReadDir(trash)
	for _, e := range entries {
		if strings.Contains(e.Name(), stem) {
			return true, c.Path + " is in the trash"
		}
	}
	return false, c.Path + " is gone but not in the trash (deleted for good)"
}

func clip(s string) string {
	s = strings.Join(strings.Fields(s), " ")
	if len(s) > 80 {
		return s[:80] + "..."
	}
	return s
}

// hasSuffixEntry: a zip made from a folder keeps the folder in the name
// (docs/a.txt); a check naming a.txt accepts it.
func hasSuffixEntry(have map[string]bool, n string) bool {
	for name := range have {
		if strings.HasSuffix(name, "/"+n) {
			return true
		}
	}
	return false
}
