package tui

// GORILLA OVERRIDE (2026-08-23): the dialog that owns the keyboard must be the
// dialog you can see.
//
// ROADMAP item 1, reported twice by the owner: with `/tasks` open, tab and esc
// did nothing. "You have to wait for some of the tasks to finish so the tasks
// window gets narrower and narrower and it exposes the buttons underneath, and
// it is only then when TAB begins to work."
//
// Two orders in tui.go disagreed. In Update, a visible permission dialog
// swallows every KeyMsg and returns early, so it owns the keyboard ahead of
// `/tasks`. In View, it was drawn FIRST of sixteen overlays, so every one of
// them painted over it. The dialog eating the keystrokes was underneath the
// dialog on screen, and nothing in the code connected the two facts.
//
// These tests read the source. That is unusual, and it is the only way to assert
// an ORDER that exists solely as the sequence of statements in two functions,
// short of a refactor that drives both from one table. The parsing is anchored
// on `if a.showX {` at one tab of indent, which is how every one of them is
// written, and it fails loudly if it stops matching anything rather than passing
// vacuously.

import (
	"os"
	"regexp"
	"strings"
	"testing"
)

var overlayIfRe = regexp.MustCompile(`(?m)^\tif a\.(show[A-Za-z]+) \{`)

// readTUISource returns tui.go split at the start of View, so the two orders can
// be read separately. Update comes first in the file.
func readTUISource(t *testing.T) (updatePart, viewPart string) {
	t.Helper()
	b, err := os.ReadFile("tui.go")
	if err != nil {
		t.Fatalf("read tui.go: %v", err)
	}
	s := string(b)
	i := strings.Index(s, "func (a appModel) View() string {")
	if i < 0 {
		t.Fatal("cannot find View(); this test's anchors need updating")
	}
	return s[:i], s[i:]
}

// flagOrder lists the show* flags in the order their blocks appear.
func flagOrder(section string) []string {
	var out []string
	seen := map[string]bool{}
	for _, m := range overlayIfRe.FindAllStringSubmatch(section, -1) {
		if !seen[m[1]] {
			out = append(out, m[1])
			seen[m[1]] = true
		}
	}
	return out
}

func indexOf(list []string, want string) int {
	for i, v := range list {
		if v == want {
			return i
		}
	}
	return -1
}

// The three overlays that block the keyboard ahead of everything else, in
// key-handling order. Their render order must be the exact reverse, because the
// last thing drawn is the thing on top.
var blockingKeyOrder = []string{"showFilepicker", "showQuit", "showPermissions"}

func TestBlockingOverlaysAreDrawnInReverseKeyOrder(t *testing.T) {
	update, view := readTUISource(t)
	keys, renders := flagOrder(update), flagOrder(view)

	if len(keys) < 5 || len(renders) < 5 {
		t.Fatalf("parsed %d key blocks and %d render blocks; the anchors have "+
			"stopped matching and this test would pass vacuously", len(keys), len(renders))
	}

	// Precondition: the three really are the top of the key order.
	for i, flag := range blockingKeyOrder {
		if got := indexOf(keys, flag); got != i {
			t.Errorf("%s is at key position %d, expected %d. If the key order changed "+
				"deliberately, the render tail in View must be reordered to match.",
				flag, got, i)
		}
	}

	// The render order of those three must be the reverse.
	for i := 0; i < len(blockingKeyOrder)-1; i++ {
		higher, lower := blockingKeyOrder[i], blockingKeyOrder[i+1]
		hi, lo := indexOf(renders, higher), indexOf(renders, lower)
		if hi < 0 || lo < 0 {
			t.Fatalf("%s or %s has no render block", higher, lower)
		}
		if hi < lo {
			t.Errorf("%s outranks %s for keystrokes but is drawn FIRST (%d before %d), "+
				"so %s paints over the dialog that owns the keyboard.",
				higher, lower, hi, lo, lower)
		}
	}
}

// THE REPORTED BUG, stated directly. A permission prompt is the only overlay
// that arrives unbidden: every other one is opened by a keystroke, and cannot be
// opened while permissions is blocking the keyboard. So it lands on top of
// whatever was already open, and it must be DRAWN on top of it too.
func TestPermissionsIsDrawnAboveEveryDialogItOutranks(t *testing.T) {
	update, view := readTUISource(t)
	keys, renders := flagOrder(update), flagOrder(view)

	permKey := indexOf(keys, "showPermissions")
	permRender := indexOf(renders, "showPermissions")
	if permKey < 0 || permRender < 0 {
		t.Fatal("showPermissions has no key or render block")
	}

	var buried []string
	for _, flag := range renders {
		if flag == "showPermissions" {
			continue
		}
		k := indexOf(keys, flag)
		// Only dialogs that permissions OUTRANKS for keystrokes matter: those are
		// the ones whose keys it steals, so those are the ones it must cover.
		if k < 0 || k < permKey {
			continue
		}
		if indexOf(renders, flag) > permRender {
			buried = append(buried, flag)
		}
	}

	if len(buried) > 0 {
		t.Errorf("a permission prompt swallows the keystrokes of these dialogs and is "+
			"then drawn UNDERNEATH them:\n  %s\n\n"+
			"  That is ROADMAP item 1: with /tasks open, tab and esc did nothing,\n"+
			"  because the dialog eating them was hidden behind the one on screen.\n"+
			"  Move the showPermissions render block later in View().",
			strings.Join(buried, "\n  "))
	}
}

// The specific pair from the report, named so a regression says the right thing.
func TestTasksDialogCannotCoverThePermissionPrompt(t *testing.T) {
	update, view := readTUISource(t)
	keys, renders := flagOrder(update), flagOrder(view)

	if indexOf(keys, "showPermissions") > indexOf(keys, "showTasksDialog") {
		t.Fatal("/tasks now outranks permissions for keystrokes; this test's premise " +
			"has changed and the z-order rule needs rethinking, not just reordering")
	}
	if indexOf(renders, "showPermissions") < indexOf(renders, "showTasksDialog") {
		t.Error("/tasks is drawn over the permission prompt again. This is the exact " +
			"bug the owner reported twice: tab and esc do nothing until enough tasks " +
			"finish for the /tasks box to shrink and reveal the prompt underneath.")
	}
}
