package tui

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

// printedText extracts the body of a bubbletea print command, or "", false if the
// command is not a print. bubbletea's printLineMessage is unexported, so the body
// is read reflectively — the alternative is not asserting on printed output at all,
// and printed output is the entire subject of this file.
func printedText(cmd tea.Cmd) (string, bool) {
	if cmd == nil {
		return "", false
	}
	v := reflect.ValueOf(cmd())
	if v.Kind() != reflect.Struct || !strings.Contains(v.Type().Name(), "printLineMessage") {
		return "", false
	}
	f := v.Field(0)
	if f.Kind() != reflect.String {
		return "", false
	}
	return f.String(), true
}

// THE regression guard, and it is a SOURCE check rather than a behavioural one.
//
// The padding came from pin_bottom.go doing tea.Println(strings.Repeat("\n", …))
// on every resize — about 44 blank lines on a 900px window, and more each time the
// window grew. They are real scrollback: they put a screen-tall gap between the
// banner and the first prompt and made the interface look like it started at the
// bottom then jumped to the top.
//
// The fix was a deletion, so there is no unit left to call. Driving the real
// resize path needs the whole dialog stack, and a test that fakes all of it tests
// the fakes. What is actually worth defending is the rule "this package never
// prints blank padding", and that is a property of the source. Stated plainly so
// nobody mistakes it for proof of runtime behaviour.
func TestNoSourceInThisPackagePrintsBlankPadding(t *testing.T) {
	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range files {
		if strings.HasSuffix(f, "_test.go") {
			continue
		}
		src, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		for i, line := range strings.Split(string(src), "\n") {
			if !strings.Contains(line, "Println") {
				continue
			}
			if strings.Contains(line, `Repeat("\n"`) || strings.Contains(line, `Repeat("\r\n"`) {
				t.Errorf("%s:%d prints repeated newlines into the scrollback:\n\t%s\n"+
					"That is the bottom-pinning padding. It is real history and it "+
					"leaves a screen-tall gap above the conversation. See "+
					"session_banner.go for why it was removed.", f, i+1, strings.TrimSpace(line))
			}
		}
	}
}

// The banner must still print, and exactly once — it no longer waits on a pin.
func TestBannerPrintsOnceWithoutAnyPinning(t *testing.T) {
	a := &appModel{scrollback: true, width: 100}

	text, ok := printedText(a.bannerCmd())
	if !ok || strings.TrimSpace(text) == "" {
		t.Fatalf("no banner printed (ok=%v, text=%q); it is what identifies the session", ok, text)
	}
	if a.bannerCmd() != nil {
		t.Error("the banner printed twice; it is once per session")
	}
}
