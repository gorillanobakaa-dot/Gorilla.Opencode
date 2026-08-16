package fileutil

import (
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/bmatcuk/doublestar/v4"
	"github.com/opencode-ai/opencode/internal/logging"
)

var (
	rgPath  string
	fzfPath string
)

func init() {
	var err error
	rgPath, err = exec.LookPath("rg")
	if err != nil {
		logging.Warn("Ripgrep (rg) not found in $PATH. Some features might be limited or slower.")
		rgPath = ""
	}
	fzfPath, err = exec.LookPath("fzf")
	if err != nil {
		// GORILLA OVERRIDE: was Warn — it printed on every run, including
		// non-interactive/scripted ones where fzf is irrelevant noise.
		logging.Debug("FZF not found in $PATH. Some features might be limited or slower.")
		fzfPath = ""
	}
}

func GetRgCmd(globPattern string) *exec.Cmd {
	if rgPath == "" {
		return nil
	}
	rgArgs := []string{
		"--files",
		"-L",
		"--null",
	}
	if globPattern != "" {
		if !filepath.IsAbs(globPattern) && !strings.HasPrefix(globPattern, "/") {
			globPattern = "/" + globPattern
		}
		rgArgs = append(rgArgs, "--glob", globPattern)
	}
	// GORILLA OVERRIDE: search EVERY workspace root, not just the process cwd.
	// cmd.Dir = "." made ripgrep list only the primary root, so @-file completion
	// could not see files in a directory added with /add-dir — the root was
	// registered, its context loaded, its permissions scoped, and yet you still
	// could not @-mention a file in it. Passing the roots as explicit search
	// paths fixes that; ripgrep prints paths relative to each argument it was
	// given, so absolute roots yield absolute results the tools can open.
	if roots := workspaceRoots(); len(roots) > 0 {
		rgArgs = append(rgArgs, roots...)
	}
	cmd := exec.Command(rgPath, rgArgs...)
	cmd.Dir = "."
	return cmd
}

// workspaceRoots is set by config at startup. fileutil cannot import config
// (config is the higher layer), so the dependency is inverted with a hook —
// keeping this package free of config knowledge while still honouring /add-dir.
var workspaceRootsFn func() []string

// SetWorkspaceRootsFn registers the accessor. Called once from config.Load.
func SetWorkspaceRootsFn(fn func() []string) { workspaceRootsFn = fn }

func workspaceRoots() []string {
	if workspaceRootsFn == nil {
		return nil
	}
	return workspaceRootsFn()
}

func GetFzfCmd(query string) *exec.Cmd {
	if fzfPath == "" {
		return nil
	}
	fzfArgs := []string{
		"--filter",
		query,
		"--read0",
		"--print0",
	}
	cmd := exec.Command(fzfPath, fzfArgs...)
	cmd.Dir = "."
	return cmd
}

type FileInfo struct {
	Path    string
	ModTime time.Time
}

func SkipHidden(path string) bool {
	// Check for hidden files (starting with a dot)
	base := filepath.Base(path)
	if base != "." && strings.HasPrefix(base, ".") {
		return true
	}

	commonIgnoredDirs := map[string]bool{
		".gorilla-opencode": true,
		// GORILLA OVERRIDE: keep ".opencode" ignored too. v0.1.85 moved this
		// program's data out of the per-project ".opencode" folder without
		// migrating it, so those folders survive in users' projects holding an
		// old SQLite database, debug.log and messages/. Dropping the entry from
		// this list would make file search and @-completion walk straight into
		// abandoned conversation history — un-ignoring data in the same release
		// that orphans it.
		".opencode":        true,
		"node_modules":     true,
		"vendor":           true,
		"dist":             true,
		"build":            true,
		"target":           true,
		".git":             true,
		".idea":            true,
		".vscode":          true,
		"__pycache__":      true,
		"bin":              true,
		"obj":              true,
		"out":              true,
		"coverage":         true,
		"tmp":              true,
		"temp":             true,
		"logs":             true,
		"generated":        true,
		"bower_components": true,
		"jspm_packages":    true,
	}

	parts := strings.Split(path, string(os.PathSeparator))
	for _, part := range parts {
		if commonIgnoredDirs[part] {
			return true
		}
	}
	return false
}

func GlobWithDoublestar(pattern, searchPath string, limit int) ([]string, bool, error) {
	fsys := os.DirFS(searchPath)
	relPattern := strings.TrimPrefix(pattern, "/")
	var matches []FileInfo

	err := doublestar.GlobWalk(fsys, relPattern, func(path string, d fs.DirEntry) error {
		if d.IsDir() {
			return nil
		}
		if SkipHidden(path) {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return nil
		}
		absPath := path
		if !strings.HasPrefix(absPath, searchPath) && searchPath != "." {
			absPath = filepath.Join(searchPath, absPath)
		} else if !strings.HasPrefix(absPath, "/") && searchPath == "." {
			absPath = filepath.Join(searchPath, absPath) // Ensure relative paths are joined correctly
		}

		matches = append(matches, FileInfo{Path: absPath, ModTime: info.ModTime()})
		if limit > 0 && len(matches) >= limit*2 {
			return fs.SkipAll
		}
		return nil
	})
	if err != nil {
		return nil, false, fmt.Errorf("glob walk error: %w", err)
	}

	sort.Slice(matches, func(i, j int) bool {
		return matches[i].ModTime.After(matches[j].ModTime)
	})

	truncated := false
	if limit > 0 && len(matches) > limit {
		matches = matches[:limit]
		truncated = true
	}

	results := make([]string, len(matches))
	for i, m := range matches {
		results[i] = m.Path
	}
	return results, truncated, nil
}
