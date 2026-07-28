package cmd

import (
	"go/ast"
	"go/parser"
	"go/token"
	"strings"
	"testing"
)

// The interactive program must take its buffer and mouse decisions from the
// settings, never by passing tea.WithAltScreen() or tea.WithMouseCellMotion()
// straight in.
//
// This is asserted against the source because the failure mode is an option at a
// call site, and an option that was never passed is invisible to a behavioural
// test. It is also the only honest check available here: driving the real
// interface from a test is not possible in this environment — bubbletea queries
// the terminal for its background colour and cursor position at startup, and any
// reply injected on a pty is consumed as keystrokes by whichever text input has
// focus, so the program never gets past its first prompt. That was tried; the
// reply ended up typed into the workspace picker's input box.
func TestInteractiveProgramTakesItsBufferFromSettings(t *testing.T) {
	const file = "root.go"
	fset := token.NewFileSet()
	parsed, err := parser.ParseFile(fset, file, nil, parser.ParseComments)
	if err != nil {
		t.Fatalf("parse %s: %v", file, err)
	}

	var (
		programs     int
		sawAltOption bool
		sawMouse     bool
	)
	ast.Inspect(parsed, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok || !isSel(call.Fun, "tea", "NewProgram") {
			return true
		}
		programs++
		for _, arg := range call.Args {
			inner, ok := arg.(*ast.CallExpr)
			if !ok {
				continue
			}
			// Bare library options are the bug: they ignore the setting entirely.
			if isSel(inner.Fun, "tea", "WithAltScreen") {
				t.Errorf("%s: tea.WithAltScreen() is passed directly at %s. The alternate "+
					"screen is a setting that defaults to OFF, because that buffer has no "+
					"scrollback — passing it unconditionally makes the conversation "+
					"unscrollable, unselectable and uncopyable again.",
					file, fset.Position(inner.Pos()))
			}
			if isSel(inner.Fun, "tea", "WithMouseCellMotion") || isSel(inner.Fun, "tea", "WithMouseAllMotion") {
				t.Errorf("%s: a mouse option is passed directly at %s. Mouse reporting must "+
					"go through mouseOption(), which only requests it on the alternate "+
					"screen — elsewhere the terminal already scrolls the conversation and "+
					"requesting events would break drag-to-select for nothing.",
					file, fset.Position(inner.Pos()))
			}
			if id, ok := inner.Fun.(*ast.Ident); ok {
				switch id.Name {
				case "altScreenOption":
					sawAltOption = true
				case "mouseOption":
					sawMouse = true
				}
			}
		}
		return true
	})

	if programs == 0 {
		t.Fatal("no tea.NewProgram call found in root.go; this check was vacuous")
	}
	if !sawAltOption {
		t.Error("the interactive program does not call altScreenOption(), so the " +
			"alternateScreen setting has no effect on which buffer is used")
	}
	if !sawMouse {
		t.Error("the interactive program does not call mouseOption(), so mouse " +
			"reporting is not gated on anything")
	}
}

// altScreenOption must consult the setting rather than deciding for itself. A
// version that returned WithAltScreen unconditionally would satisfy the test above
// while restoring exactly the behaviour being removed.
func TestAltScreenOptionConsultsTheSetting(t *testing.T) {
	src, err := parser.ParseFile(token.NewFileSet(), "root.go", nil, 0)
	if err != nil {
		t.Fatal(err)
	}

	var body string
	ast.Inspect(src, func(n ast.Node) bool {
		fn, ok := n.(*ast.FuncDecl)
		if !ok || fn.Name.Name != "altScreenOption" || fn.Body == nil {
			return true
		}
		var sb strings.Builder
		ast.Inspect(fn.Body, func(inner ast.Node) bool {
			if sel, ok := inner.(*ast.SelectorExpr); ok {
				sb.WriteString(sel.Sel.Name + " ")
			}
			return true
		})
		body = sb.String()
		return false
	})

	if body == "" {
		t.Fatal("altScreenOption not found in root.go")
	}
	if !strings.Contains(body, "AlternateScreenEnabled") {
		t.Errorf("altScreenOption does not read config.AlternateScreenEnabled; it "+
			"references only: %s", body)
	}
}

func isSel(e ast.Expr, pkg, name string) bool {
	sel, ok := e.(*ast.SelectorExpr)
	if !ok || sel.Sel.Name != name {
		return false
	}
	id, ok := sel.X.(*ast.Ident)
	return ok && id.Name == pkg
}
