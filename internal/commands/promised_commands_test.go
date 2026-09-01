package commands

// GORILLA OVERRIDE (2026-08-18): the guard for the bug that produced this file.
//
// v0.1.90 shipped two pieces of user-facing text — the header of every saved
// findings file, and the research tool's own report — telling the user to run
// `/osint --recover` when a write-up died. The command did not exist. Typing it
// handed the literal string "--recover" to a ten-helper supervised dossier as
// the subject under investigation, in the most expensive configuration the
// program offers. The model refused to fabricate a dossier about a flag, which
// is the only reason it cost setup time rather than a full run.
//
// Nothing could have caught it: the promise lived in Go prose, and no compiler
// checks prose. So this test reads the prose. Any /command named in a string
// literal must resolve in the registry — instructional text is a promise, and a
// promise the program cannot keep is worse than saying nothing.

import (
	"github.com/opencode-ai/opencode/internal/plaincmd"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

// promisedCommand matches a slash command as it appears in prose: at a word
// boundary, lowercase letters and hyphens only, and NOT followed by another
// path separator — which is what keeps /usr/bin, /home/gorilla and URLs out.
// A trailing "." is excluded too, so the llms.txt convention's /llms.txt and
// /llms-full.txt do not read as commands.
var promisedCommand = regexp.MustCompile(`(^|[\s"'` + "`" + `(\[])/([a-z][a-z-]{1,20})\b([^./a-zA-Z0-9-]|$)`)

func TestEverySlashCommandNamedInProseActuallyExists(t *testing.T) {
	known := map[string]bool{}
	for _, c := range All {
		known[c.Name] = true
		for _, a := range c.Aliases {
			known[a] = true
		}
	}
	// GORILLA OVERRIDE (2026-09-01): plain mode has its OWN dispatcher.
	//
	// It is a deliberately small, separate command set (internal/plain), not a
	// subset of this registry — so /exit, /extras, /show and /hide, which plain
	// mode's own help text promises and its own switch handles, were reported
	// here as commands that do not exist. They do. The check was right to look;
	// it was looking in only one of the two places commands live.
	//
	// plaincmd.Names is verified against that switch by
	// TestCommandNamesMatchesTheSwitch, so this is not a second hand-maintained
	// list that can quietly drift.
	for _, n := range plaincmd.Names {
		known[n] = true
	}

	// Words that read as commands but are not, in contexts this walk cannot
	// distinguish. Kept short and explicit: every entry is a place the check
	// was proven to misfire, not a convenience.
	notCommands := map[string]bool{
		"dev":  true, // /dev/null
		"etc":  true, // /etc/...
		"tmp":  true,
		"usr":  true,
		"var":  true,
		"proc": true,
		"sys":  true,
		"opt":  true,
		"home": true,
		"run":  true,
		"bin":  true,
		"v":    true, // /v1/models
		"s":    true, // regex and sed fragments
	}

	root := filepath.Join("..", "..")
	var offenders []string
	files := 0

	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			switch d.Name() {
			case ".git", "node_modules", "Compiled.Builds", "vendor":
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		// Generated model catalogues are vendor prose, not this program's
		// promises, and they are enormous.
		if strings.Contains(path, "_generated.go") || strings.Contains(path, "/lsp/protocol/") {
			return nil
		}
		// Plain mode has its OWN command set (/exit, /extras, /show, /hide),
		// dispatched in internal/plain and deliberately not in this registry —
		// it is a different surface for a different terminal. Its promises are
		// real; they are just kept somewhere else.
		if strings.Contains(path, "/internal/plain/") {
			return nil
		}
		files++

		fset := token.NewFileSet()
		f, err := parser.ParseFile(fset, path, nil, 0)
		if err != nil {
			return nil
		}
		ast.Inspect(f, func(n ast.Node) bool {
			lit, ok := n.(*ast.BasicLit)
			if !ok || lit.Kind != token.STRING {
				return true
			}
			text, err := strconv.Unquote(lit.Value)
			if err != nil {
				text = lit.Value
			}
			for _, m := range promisedCommand.FindAllStringSubmatch(text, -1) {
				name := m[2]
				if notCommands[name] || known[name] {
					continue
				}
				rel, _ := filepath.Rel(root, path)
				offenders = append(offenders, rel+": /"+name+
					"  (in: "+shorten(text)+")")
			}
			return true
		})
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}

	if files < 50 {
		t.Fatalf("only walked %d Go files; the check below would be vacuous", files)
	}
	if len(offenders) > 0 {
		t.Errorf("these strings tell the user to run a command that is not in the registry.\n"+
			"Instructional text is a promise, and /osint --recover was documented for a day\n"+
			"before it existed — typing it started the most expensive run the program offers.\n  %s",
			strings.Join(offenders, "\n  "))
	}
}

func shorten(s string) string {
	s = strings.Join(strings.Fields(s), " ")
	if len(s) > 70 {
		return s[:67] + "…"
	}
	return s
}
