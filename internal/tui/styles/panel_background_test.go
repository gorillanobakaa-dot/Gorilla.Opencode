package styles

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/opencode-ai/opencode/internal/config"
)

// Outside the alternate screen the fill must be the terminal's own background, and
// on it the theme's. This is the single decision that stops the interface looking
// half-painted, so it is asserted rather than assumed.
func TestPanelBackgroundFollowsTheBuffer(t *testing.T) {
	if _, err := config.Load(t.TempDir(), false); err != nil {
		t.Fatalf("Load: %v", err)
	}

	// Default: no alternate screen, so nothing is filled.
	if got := PanelBackground(); !isTransparent(got) {
		t.Errorf("with the alternate screen off the panel fill is %#v; it must be the "+
			"terminal's own background, or every unstyled span shows as a hole in a "+
			"coloured slab", got)
	}

	if err := config.SetAlternateScreen(true); err != nil {
		t.Fatalf("SetAlternateScreen: %v", err)
	}
	if got := PanelBackground(); isTransparent(got) {
		t.Error("on the alternate screen the panel fill is transparent; there the program " +
			"owns every cell and the theme's surface is what makes it look deliberate")
	}
}

func isTransparent(c lipgloss.TerminalColor) bool {
	_, ok := c.(lipgloss.NoColor)
	return ok
}

// No component may reach for the theme's background directly as a FILL. Doing so
// bypasses the decision above, and the result is the exact defect this replaced: a
// row painted in patches, because some spans went through the decision point and
// others did not.
//
// Checked against the source, because the failure is a call that was never routed —
// invisible to any rendering test that does not happen to cover that one component.
// Semantic backgrounds (Error, Warning, Primary, Info, Success) are deliberately
// excluded: those carry meaning and must stay painted in both modes.
func TestNoComponentFillsWithTheThemeBackgroundDirectly(t *testing.T) {
	root := ".."
	var offenders []string
	checked := 0

	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() || !strings.HasSuffix(path, ".go") {
			return err
		}
		if strings.HasSuffix(path, "_test.go") || strings.HasSuffix(path, "styles/styles.go") {
			return nil
		}
		src, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		file, err := parser.ParseFile(token.NewFileSet(), path, src, 0)
		if err != nil {
			return nil // not our business to fail on unparsable files
		}
		checked++

		ast.Inspect(file, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok || len(call.Args) != 1 {
				return true
			}
			sel, ok := call.Fun.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			if sel.Sel.Name != "Background" && sel.Sel.Name != "BorderBackground" {
				return true
			}
			// Is the single argument a .Background() lookup on a theme?
			arg, ok := call.Args[0].(*ast.CallExpr)
			if !ok {
				return true
			}
			argSel, ok := arg.Fun.(*ast.SelectorExpr)
			if !ok || argSel.Sel.Name != "Background" {
				return true
			}
			offenders = append(offenders, path)
			return true
		})
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}

	if checked < 20 {
		t.Fatalf("only walked %d files; the check below would be vacuous", checked)
	}
	if len(offenders) > 0 {
		t.Errorf("these files fill with the theme background directly instead of "+
			"styles.PanelBackground(), so they stay painted when nothing else is:\n  %s",
			strings.Join(unique(offenders), "\n  "))
	}
}

func unique(in []string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(in))
	for _, s := range in {
		if !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	}
	return out
}
