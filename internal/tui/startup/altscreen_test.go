package startup

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Every interactive prompt in this package must run in the alternate screen.
//
// The reason is a bubbletea property, not a preference: its inline renderer
// erases the previous frame by walking the cursor up by the number of LOGICAL
// lines it last drew, and nothing repaints on a resize. Narrow the window while
// a prompt is on screen and those lines wrap into more physical rows than the
// count knows about, so each resize step leaves a stale, half-drawn copy behind.
//
// This is asserted against the source rather than by driving a program, because
// the failure mode is a missing option at a call site — exactly the thing a new
// picker would forget. A behavioural test cannot see an option that was never
// passed.
func TestEveryPromptRunsInTheAlternateScreen(t *testing.T) {
	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}

	checked := 0
	for _, name := range files {
		if strings.HasSuffix(name, "_test.go") {
			continue
		}
		src, err := os.ReadFile(name)
		if err != nil {
			t.Fatal(err)
		}
		fset := token.NewFileSet()
		file, err := parser.ParseFile(fset, name, src, 0)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}

		ast.Inspect(file, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok || !isSelector(call.Fun, "tea", "NewProgram") {
				return true
			}
			checked++
			for _, arg := range call.Args {
				if inner, ok := arg.(*ast.CallExpr); ok && isSelector(inner.Fun, "tea", "WithAltScreen") {
					return true
				}
			}
			t.Errorf("%s: tea.NewProgram at %s is missing tea.WithAltScreen(); an "+
				"inline prompt leaves one stale frame behind per terminal resize",
				name, fset.Position(call.Pos()))
			return true
		})
	}

	// Non-vacuity: if the walk found no programs at all, the loop above proved
	// nothing. Both known prompts (workspace, extras) must be seen.
	if checked < 2 {
		t.Fatalf("found only %d tea.NewProgram call(s) in this package; expected at "+
			"least 2 (workspace picker, extras consent). The check above was vacuous.", checked)
	}
}

func isSelector(e ast.Expr, pkg, name string) bool {
	sel, ok := e.(*ast.SelectorExpr)
	if !ok || sel.Sel.Name != name {
		return false
	}
	id, ok := sel.X.(*ast.Ident)
	return ok && id.Name == pkg
}

// The alternate screen throws away everything drawn in it, so the picker prints
// the answer afterwards. That line is the record the original inline rendering
// existed to leave behind; if it goes missing, the fix traded one regression for
// another.
func TestAnswerLineRecordsTheChoice(t *testing.T) {
	const dir = "/home/gorilla/Documents/project"

	plain := AnswerLine(dir, false)
	if !strings.Contains(plain, dir) {
		t.Errorf("answer line %q does not name the folder", plain)
	}
	if strings.Contains(plain, "remembered") {
		t.Errorf("answer line %q claims the choice was remembered when it was not", plain)
	}

	remembered := AnswerLine(dir, true)
	if !strings.Contains(remembered, dir) {
		t.Errorf("answer line %q does not name the folder", remembered)
	}
	if !strings.Contains(remembered, "remembered") {
		t.Errorf("answer line %q does not say the choice will be reused, so the user "+
			"has no way to know the question stopped being asked", remembered)
	}

	if plain == remembered {
		t.Error("both answer lines are identical; the remembered case is indistinguishable")
	}
	for _, l := range []string{plain, remembered} {
		if strings.Contains(l, "\n") {
			t.Errorf("answer line %q spans multiple lines; it is meant to be one "+
				"durable line, not a box", l)
		}
	}
}
