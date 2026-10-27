package tools

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"unicode"

	"github.com/opencode-ai/opencode/internal/permission"
)

// Bulk rename, the owner's two PowerShell lines as one call:
//
//	Get-ChildItem -File | Rename-Item -NewName { $_.Name -replace 'old', 'new' }
//	$i = 1; Get-ChildItem -File | Rename-Item -NewName { "{0:D2}_{1}" -f $script:i++, $_.Name }
//
// find/replace is a regex on the whole name, then case, then template
// ({n}, {n:D2}, {name}, {stem}, {ext}). Every new name is worked out and
// checked first: a name used twice, a name that is already taken by a file
// not being renamed, or a name Windows cannot hold stops the whole rename
// before anything moves. Renames run in two steps (through temporary names),
// so a swap (a->b, b->a) and a case-only change both work, and a failure
// half way puts back what had moved.

type renameStep struct{ from, to string }

var templateToken = regexp.MustCompile(`\{(n(?::[Dd](\d+))?|name|stem|ext)\}`)

// windowsReserved are device names Windows refuses as file names.
var windowsReserved = map[string]bool{"con": true, "prn": true, "aux": true, "nul": true,
	"com1": true, "com2": true, "com3": true, "com4": true, "com5": true, "com6": true, "com7": true, "com8": true, "com9": true,
	"lpt1": true, "lpt2": true, "lpt3": true, "lpt4": true, "lpt5": true, "lpt6": true, "lpt7": true, "lpt8": true, "lpt9": true}

// badName says why a new name cannot be used, or "".
func badName(name string) string {
	switch {
	case strings.TrimSpace(name) == "":
		return "empty name"
	case name == "." || name == "..":
		return "not a name"
	case strings.ContainsAny(name, `/\`):
		return "contains a folder separator"
	case strings.ContainsAny(name, "<>:\"|?*") || strings.IndexFunc(name, func(r rune) bool { return r < 32 }) >= 0:
		return `contains a character Windows does not allow (< > : " | ? * or a control character)`
	case strings.HasSuffix(name, ".") || strings.HasSuffix(name, " "):
		return "ends with a dot or a space (Windows drops them)"
	}
	stem := strings.ToLower(strings.TrimSuffix(name, filepath.Ext(name)))
	if windowsReserved[stem] {
		return "a reserved device name on Windows"
	}
	return ""
}

func foldName(s string) string {
	if runtime.GOOS == "windows" || runtime.GOOS == "darwin" {
		return strings.ToLower(s)
	}
	return s
}

// newName applies find/replace, case and template to one name.
func newName(name string, re *regexp.Regexp, replace, caseMode, template string, n int) string {
	if re != nil {
		name = re.ReplaceAllString(name, replace)
	}
	switch strings.ToLower(caseMode) {
	case "lower":
		name = strings.ToLower(name)
	case "upper":
		name = strings.ToUpper(name)
	case "title":
		prev := ' '
		name = strings.Map(func(r rune) rune {
			out := r
			if !unicode.IsLetter(prev) && !unicode.IsDigit(prev) && prev != '\'' {
				out = unicode.ToUpper(r)
			} else {
				out = unicode.ToLower(r)
			}
			prev = r
			return out
		}, name)
	}
	if template == "" {
		return name
	}
	ext := filepath.Ext(name)
	stem := strings.TrimSuffix(name, ext)
	return templateToken.ReplaceAllStringFunc(template, func(tok string) string {
		m := templateToken.FindStringSubmatch(tok)
		switch m[1] {
		case "name":
			return name
		case "stem":
			return stem
		case "ext":
			return ext
		}
		width, _ := strconv.Atoi(m[2])
		return fmt.Sprintf("%0*d", width, n)
	})
}

// planRename works out every step and refuses the whole set on any problem.
func planRename(p filesParams) ([]renameStep, error) {
	fi, err := os.Stat(p.Path)
	if err != nil {
		return nil, err
	}
	var re *regexp.Regexp
	if p.Find != "" {
		if re, err = regexp.Compile(p.Find); err != nil {
			return nil, fmt.Errorf("find is not a valid regex: %v", err)
		}
	}
	if re == nil && p.Template == "" && p.Case == "" && (fi.IsDir() || p.Dest == "") {
		return nil, fmt.Errorf("rename needs find/replace, template or case (or dest for one file)")
	}
	dir := p.Path
	var names []string
	if fi.IsDir() {
		entries, err := os.ReadDir(dir)
		if err != nil {
			return nil, err
		}
		for _, e := range entries {
			if e.Type().IsRegular() && nameMatches(p.Pattern, e.Name()) {
				names = append(names, e.Name())
			}
		}
		sort.Strings(names)
	} else {
		dir = filepath.Dir(p.Path)
		names = []string{filepath.Base(p.Path)}
	}
	start := p.Start
	if start == 0 {
		start = 1
	}
	var steps []renameStep
	for i, name := range names {
		to := ""
		if !fi.IsDir() && p.Dest != "" {
			to = filepath.Base(strings.TrimSpace(p.Dest))
		} else {
			to = newName(name, re, p.Replace, p.Case, p.Template, start+i)
		}
		if to != name {
			steps = append(steps, renameStep{from: name, to: to})
		}
	}
	// Checks, all before anything moves.
	moving := map[string]bool{}
	for _, s := range steps {
		moving[foldName(s.from)] = true
	}
	taken := map[string]string{}
	var problems []string
	for _, s := range steps {
		if why := badName(s.to); why != "" {
			problems = append(problems, fmt.Sprintf("%s -> %q: %s", s.from, s.to, why))
			continue
		}
		key := foldName(s.to)
		if other, dup := taken[key]; dup {
			problems = append(problems, fmt.Sprintf("%s and %s would both become %s", other, s.from, s.to))
			continue
		}
		taken[key] = s.from
		if !moving[key] {
			if _, err := os.Lstat(filepath.Join(dir, s.to)); err == nil {
				problems = append(problems, fmt.Sprintf("%s -> %s: a file named %s already exists", s.from, s.to, s.to))
			}
		}
	}
	if len(problems) > 0 {
		var b strings.Builder
		fmt.Fprintf(&b, "refused, nothing was renamed: %d problem(s)\n", len(problems))
		writeList(&b, "  ", problems, true)
		return nil, fmt.Errorf("%s", b.String())
	}
	for i := range steps {
		steps[i].from = filepath.Join(dir, steps[i].from)
		steps[i].to = filepath.Join(dir, steps[i].to)
	}
	return steps, nil
}

// runRename moves every file to a temporary name, then to its new name, and
// on a failure moves back what had moved.
func runRename(steps []renameStep) error {
	tmp := make([]string, len(steps))
	undo := func(upto int, final bool) {
		for i := upto - 1; i >= 0; i-- {
			if final {
				_ = os.Rename(steps[i].to, tmp[i])
			}
		}
		for i := range steps {
			if tmp[i] != "" {
				_ = os.Rename(tmp[i], steps[i].from)
			}
		}
	}
	for i, s := range steps {
		t := filepath.Join(filepath.Dir(s.from), fmt.Sprintf(".gorilla-rename-%d-%d.tmp", os.Getpid(), i))
		if err := os.Rename(s.from, t); err != nil {
			undo(0, false)
			return fmt.Errorf("%s: %v; nothing was renamed", s.from, err)
		}
		tmp[i] = t
	}
	for i, s := range steps {
		if err := os.Rename(tmp[i], s.to); err != nil {
			undo(i, true)
			return fmt.Errorf("%s -> %s: %v; every name was put back", s.from, s.to, err)
		}
	}
	return nil
}

func (t filesTool) rename(ctx context.Context, p filesParams) (ToolResponse, error) {
	steps, err := planRename(p)
	if err != nil {
		return NewTextErrorResponse(err.Error()), nil
	}
	lines := make([]string, len(steps))
	for i, s := range steps {
		lines[i] = filepath.Base(s.from) + " -> " + filepath.Base(s.to)
	}
	var b strings.Builder
	fmt.Fprintf(&b, "DRY RUN (nothing changed yet): %d file(s) renamed in %s\n", len(steps), filepath.Dir(firstOr(steps, p.Path)))
	writeList(&b, "  ", lines, p.Full)
	preview := b.String()
	if p.DryRun || len(steps) == 0 {
		return NewTextResponse(preview), nil
	}
	if !t.ask(ctx, "rename", p.Path, "rename:"+p.Path, preview, "", lines) {
		return ToolResponse{}, permission.ErrorPermissionDenied
	}
	if err := runRename(steps); err != nil {
		return NewTextErrorResponse("rename failed: " + err.Error()), nil
	}
	return NewTextResponse(strings.Replace(preview, "DRY RUN (nothing changed yet)", "DONE", 1)), nil
}

func firstOr(steps []renameStep, fallback string) string {
	if len(steps) > 0 {
		return steps[0].from
	}
	return filepath.Join(fallback, "x")
}
