package styles

// GORILLA OVERRIDE (2026-08-19): the guard that stops box-drawing creeping back
// into anything that draws.
//
// Recording the individual mistakes did not work. Three of them were already
// written up in this project's CLAUDE.md and got re-derived anyway — the owner
// reports having corrected Opus models on it several times, and it recurred
// twice in one hour while /arsenal was being built. So this is mechanical now.

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mattn/go-runewidth"
)

// ambiguousWidth reports a rune whose width depends on the terminal's
// East-Asian setting: one column normally, TWO in a CJK-configured terminal.
// lipgloss measures with the default (1) while the terminal may draw 2, so the
// arithmetic silently goes wrong by a column per character — and lipgloss
// responds by WRAPPING, which shows up as height, elsewhere.
func ambiguousWidth(r rune) bool {
	runewidth.DefaultCondition.EastAsianWidth = false
	a := runewidth.RuneWidth(r)
	runewidth.DefaultCondition.EastAsianWidth = true
	b := runewidth.RuneWidth(r)
	runewidth.DefaultCondition.EastAsianWidth = false
	return a != b
}

// drawing characters — the ones that build boxes, rules, bars and markers.
// Prose punctuation (em-dash, curly quotes) is deliberately NOT here: it sits
// inside sentences that get wrapped, where a one-column drift is absorbed
// rather than accumulated into a broken frame. The rule is "never in anything
// that draws or separates", not "never".
var bannedDrawing = []rune{
	'─', '│', '┌', '┐', '└', '┘', '├', '┤', '┬', '┴', '┼', // light box
	'━', '┃', '┏', '┓', '┗', '┛', '╋', // heavy box
	'═', '║', '╔', '╗', '╚', '╝', '╬', // double box
	'╭', '╮', '╰', '╯', // rounded box
	'█', '▓', '▒', '░', '▌', '▐', '▏', // blocks and bars
	'…', '•', '·', '★', '▲', '▼', '◆', // markers
	'↑', '↓', '←', '→', '⇧', '⇩', // arrows
	'≈', '×', '÷', '≠', '½', // symbols used in aligned output
}

// exempt files, each for a stated reason. An exemption without a reason is how
// a rule becomes decoration.
var exemptFiles = map[string]string{
	"image/images.go": "U+2580 IS the image renderer — one cell shows two pixels " +
		"by colouring its halves. There is no ASCII substitute; replacing it removes the technique.",
	"util/util.go": "NoticeDeco's middle dots were chosen deliberately (2026-08-18) because " +
		"they have no space to break on and absorb the U+FE0F width mismatch the warning emoji carries.",
	"styles/ascii_test.go": "this file names the banned characters in order to ban them",
	"quota_panel.go": "the quota meter is a THERMOMETER and needs a solid body — U+2588 filled, " +
		"U+2591 trough — for the red-to-green gradient to sit in. A '#' reads as text, not a meter. " +
		"Safe here because this panel prints to SCROLLBACK via tea.Println, NOT into the one-row " +
		"inline frame this rule protects: a mis-measured cell can at worst wrap one line that then " +
		"scrolls away, and cannot strand debris in a frame that must be exactly its window height. " +
		"Restored 2026-08-20 after a codebase-wide sweep (5e4cd97, 81 files) flattened it while " +
		"fixing something else entirely. Locked by tui/quota_locked_test.go — change both or neither.",
}

func TestNothingThatDrawsUsesAmbiguousWidthCharacters(t *testing.T) {
	banned := map[rune]bool{}
	for _, r := range bannedDrawing {
		banned[r] = true
		if !ambiguousWidth(r) && r != '·' {
			t.Logf("note: %q is on the list but is not ambiguous-width; it stays for consistency", string(r))
		}
	}

	root := "../.." // internal/
	fset := token.NewFileSet()
	var problems []string

	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() || !strings.HasSuffix(path, ".go") {
			return nil
		}
		rel, _ := filepath.Rel(root, path)
		rel = filepath.ToSlash(rel)
		for f := range exemptFiles {
			if strings.HasSuffix(rel, f) {
				return nil
			}
		}
		// Only the TUI draws. Tool output and prose elsewhere are not laid out
		// against a terminal width by this program.
		if !strings.HasPrefix(rel, "tui/") {
			return nil
		}
		// Tests may NAME a character — usually to assert it is absent, which
		// is the opposite of the problem. Guarding them would forbid writing
		// the assertion that proves the guard works.
		if strings.HasSuffix(rel, "_test.go") {
			return nil
		}
		f, err := parser.ParseFile(fset, path, nil, 0) // comments dropped
		if err != nil {
			return nil
		}
		ast.Inspect(f, func(n ast.Node) bool {
			bl, ok := n.(*ast.BasicLit)
			if !ok || bl.Kind != token.STRING {
				return true
			}
			for _, r := range bl.Value {
				if banned[r] {
					problems = append(problems, rel+": "+fset.Position(bl.Pos()).String()+" contains "+string(r))
					return false
				}
			}
			return true
		})
		return nil
	})
	if err != nil {
		t.Fatalf("walk: %v", err)
	}

	for _, p := range problems {
		t.Errorf("%s\n"+
			"  That character's width depends on the reader's terminal (1 column normally, 2 in a\n"+
			"  CJK-configured one), lipgloss measures it as 1, and when it is really 2 the line\n"+
			"  overflows and lipgloss WRAPS — so the frame grows in height, somewhere else, with\n"+
			"  nothing pointing at the cause. Use internal/tui/styles/ascii.go, or lipgloss.ASCIIBorder().", p)
	}
}

func TestRuleIsSizedNotTyped(t *testing.T) {
	if got := Rule(10); got != "----------" {
		t.Errorf("Rule(10) = %q", got)
	}
	if Rule(0) != "" || Rule(-5) != "" {
		t.Error("Rule invented characters for a non-positive width")
	}
	for _, w := range []int{8, 20, 40, 79, 120} {
		got := RuleLabel("WHAT THIS COSTS", w)
		if len(got) != w {
			t.Errorf("RuleLabel at width %d produced %d columns: %q", w, len(got), got)
		}
	}
}

// When there is not room for both, the words win. Decoration is what gets
// dropped when space runs out.
func TestALabelTooWideForTheRuleKeepsTheWords(t *testing.T) {
	got := RuleLabel("A VERY LONG HEADING INDEED", 10)
	if len(got) > 10 {
		t.Fatalf("overflowed: %q", got)
	}
	if !strings.HasPrefix(got, "A VERY") {
		t.Errorf("dropped the words instead of the dashes: %q", got)
	}
}

// Anything that reserves room for the truncation marker must reserve its REAL
// width. This caught a live bug the moment the marker grew from 1 to 3.
func TestTheEllipsisIsThreeColumnsAndSaysSo(t *testing.T) {
	if runewidth.StringWidth(Ellipsis) != 3 {
		t.Fatalf("Ellipsis measures %d columns; code reserving room for it will be wrong",
			runewidth.StringWidth(Ellipsis))
	}
	for _, s := range []string{Ellipsis, Bullet, Sep, VLine, HChar} {
		for _, r := range s {
			if r > 0x7F {
				t.Errorf("%q is not ASCII", s)
			}
		}
	}
}
