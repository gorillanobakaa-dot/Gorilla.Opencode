package plain

// GORILLA OVERRIDE (2026-09-01): keep CommandNames honest against the switch
// that actually dispatches.
//
// CommandNames exists so the promised-commands check in internal/commands can
// tell that /exit, /extras, /show and /hide are real — plain mode has its own
// dispatcher, and the TUI registry that check consults does not know about it.
//
// A hand-maintained list that nothing verifies is exactly how the LM Studio
// picker row got added without being added to its allowlist, earlier the same
// day. So this reads the case labels out of the source rather than trusting the
// slice, and fails if the two drift in either direction.

import (
	"go/ast"
	"go/parser"
	"go/token"
	"sort"
	"strconv"
	"testing"

	"github.com/opencode-ai/opencode/internal/plaincmd"
)

// switchCaseNames returns the string case labels of the switch inside the
// `command` method.
func switchCaseNames(t *testing.T) []string {
	t.Helper()
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "plain.go", nil, 0)
	if err != nil {
		t.Fatalf("parsing plain.go: %v", err)
	}

	var names []string
	ast.Inspect(f, func(n ast.Node) bool {
		fn, ok := n.(*ast.FuncDecl)
		if !ok || fn.Name.Name != "command" || fn.Recv == nil {
			return true
		}
		ast.Inspect(fn.Body, func(m ast.Node) bool {
			cc, ok := m.(*ast.CaseClause)
			if !ok {
				return true
			}
			for _, e := range cc.List { // nil List == default:, correctly skipped
				lit, ok := e.(*ast.BasicLit)
				if !ok || lit.Kind != token.STRING {
					continue
				}
				if v, err := strconv.Unquote(lit.Value); err == nil {
					names = append(names, v)
				}
			}
			return true
		})
		return false
	})
	if len(names) == 0 {
		t.Fatal("found no case labels in command(); the parser or the function shape changed")
	}
	return names
}

func TestCommandNamesMatchesTheSwitch(t *testing.T) {
	inSwitch := switchCaseNames(t)

	declared := map[string]bool{}
	for _, n := range plaincmd.Names {
		declared[n] = true
	}
	handled := map[string]bool{}
	for _, n := range inSwitch {
		handled[n] = true
	}

	var missing, extra []string
	for _, n := range inSwitch {
		if !declared[n] {
			missing = append(missing, n)
		}
	}
	for _, n := range plaincmd.Names {
		if !handled[n] {
			extra = append(extra, n)
		}
	}
	sort.Strings(missing)
	sort.Strings(extra)

	if len(missing) > 0 {
		t.Errorf("the switch handles %v but plaincmd.Names does not list them — "+
			"instructional text naming one of those would be reported as a broken promise", missing)
	}
	if len(extra) > 0 {
		t.Errorf("plaincmd.Names lists %v but the switch does not handle them — "+
			"the promised-commands check would accept text promising a command that "+
			"prints \"not available in plain mode\"", extra)
	}
}

// The help text is a promise: every command it prints must be one plain mode
// actually handles. This is the failure the whole mechanism exists to prevent.
func TestHelpTextOnlyPromisesRealCommands(t *testing.T) {
	handled := map[string]bool{}
	for _, n := range switchCaseNames(t) {
		handled[n] = true
	}
	for _, promised := range []string{"help", "exit", "clear", "export", "extras", "show", "hide", "model"} {
		if !handled[promised] {
			t.Errorf("/%s is printed by help() but the dispatcher does not handle it", promised)
		}
	}
}
