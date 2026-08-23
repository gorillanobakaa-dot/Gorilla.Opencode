package dialog

// GORILLA OVERRIDE (2026-08-23): you cannot approve what you cannot read.
//
// Reported live, three times in four minutes, from the owner's own screen:
//
//   cd /home/gorilla/Documents/Debian.Kernel.Work/Kernel.
//   cd /home/gorilla/Documents/Debian.Kernel.Work/kernel-
//   find /home/gorilla/Documents/Debian.Kernel.Work -name ".
//
// Each severed mid-argument. A fenced code block CLIPS rather than wraps, and
// the dialog was 0.4 of the window, so the destination and the pattern went
// off the edge. Those are precisely the parts that decide whether a command is
// safe to run.
//
// This is the third instance in one day of one class: text cut to fit a
// container instead of the container fitting the text. It is the worst instance
// because this dialog has exactly one job.

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
)

// NOTHING MAY BE LOST. Every character of the command reaches the screen, or
// the dialog is lying about what it is asking for.
func TestWrappingLosesNoCharacterOfTheCommand(t *testing.T) {
	commands := []string{
		"cd /home/gorilla/Documents/Debian.Kernel.Work/Kernel.7.1.2.Vault.Do.Not.Delete && grep -rn 'Holmes' CREDITS",
		`find /home/gorilla/Documents/Debian.Kernel.Work -name "*.patch" -newer /tmp/x -print0 | xargs -0 grep -l Holmes`,
		"rm -rf /home/gorilla/Documents/Debian.Kernel.Work/Kernel.Vault.Do.Not.Delete",
		"echo hi",
	}
	for _, cmd := range commands {
		wrapped := wrapCommand(cmd, 40)
		// Compare the sequence of NON-WHITESPACE characters.
		//
		// The first version of this test joined the lines with a space and
		// compared collapsed whitespace, and it failed against correct output:
		// a mid-token break makes "...Kernel.Wo" + "rk/..." and rejoining with
		// a space invents one inside the path. The wrapper was right and the
		// assertion was wrong, which is its own small lesson about writing the
		// check before looking at what the code does.
		//
		// What must hold is that no character of the COMMAND is lost. Where the
		// line breaks fall is a layout decision, not a correctness one.
		if squash(wrapped) != squash(cmd) {
			t.Errorf("characters were lost or altered.\n  in:  %q\n  out: %q", cmd, wrapped)
		}
	}
}

// No line may exceed the width, or it clips again and the wrapping bought
// nothing. A single long token still has to be broken.
func TestNoWrappedLineOverflowsTheDialog(t *testing.T) {
	const width = 40
	long := "cd /home/gorilla/Documents/Debian.Kernel.Work/Kernel.7.1.2.Vault.Do.Not.Delete.Very.Long.Path.Indeed"
	for _, line := range strings.Split(wrapCommand(long, width), "\n") {
		if lipgloss.Width(line) > width {
			t.Errorf("line is %d columns wide, limit is %d: %q\n\n"+
				"  A path with no spaces in it must be broken mid-token. Refusing to "+
				"break it is how the destination ends up off the edge of the dialog.",
				lipgloss.Width(line), width, line)
		}
	}
}

// THE REPORTED CASE, named so a regression says the right thing. The tail of
// the path is the part that decides whether this is safe.
func TestTheDestinationOfACdIsAlwaysVisible(t *testing.T) {
	cmd := "cd /home/gorilla/Documents/Debian.Kernel.Work/Kernel.7.1.2.Vault"
	out := wrapCommand(cmd, 40)
	if !strings.Contains(strings.ReplaceAll(out, "\n", ""), "Kernel.7.1.2.Vault") {
		t.Errorf("the end of the path is missing:\n%s\n\n"+
			"  This is the live report: the dialog showed "+
			"'cd /home/gorilla/Documents/Debian.Kernel.Work/Kernel.' and the user "+
			"was asked to approve the part that had been cut off.", out)
	}
}

// A silly width must not spin or drop text. Terminals get resized to absurd
// sizes and a permission prompt is the last place that may misbehave.
func TestAnAbsurdWidthStillShowsTheWholeCommand(t *testing.T) {
	cmd := "grep -rn Holmes CREDITS"
	for _, w := range []int{-5, 0, 1, 3, 10} {
		if squash(wrapCommand(cmd, w)) != squash(cmd) {
			t.Errorf("width %d lost text: %q", w, wrapCommand(cmd, w))
		}
	}
}

// squash reduces a string to its non-whitespace characters, so two renderings
// that differ only in where lines were broken compare equal.
func squash(s string) string { return strings.Join(strings.Fields(s), "") }

// THE FETCH DIALOG, caught one fix too late. The bash dialog was widened and
// wrapped; this one was not looked at, and the owner's next screenshot showed:
//
//	https://www.forbes.com/sites/forbeswealthteam/article/the-
//
// It matters MORE here than next door. For web_fetch the HOST is the grant key,
// so approving one URL authorises every later page on that host for the whole
// session. The string being judged is the string the grant is built from.
func TestALongURLSurvivesTheFetchDialog(t *testing.T) {
	url := "https://www.forbes.com/sites/forbeswealthteam/article/the-worlds-richest-people-august-2026/"
	out := wrapCommand(url, 60)

	if squash(out) != squash(url) {
		t.Errorf("the URL lost characters:\n  in:  %s\n  out: %s", url, out)
	}
	// The tail is what says which page, and the host is what the grant keys on.
	joined := strings.ReplaceAll(out, "\n", "")
	if !strings.Contains(joined, "forbes.com") {
		t.Error("the HOST is missing, and the host is the grant key")
	}
	if !strings.Contains(joined, "august-2026") {
		t.Error("the end of the URL was cut, so the user cannot see which page " +
			"they are approving")
	}
	for _, line := range strings.Split(out, "\n") {
		if lipgloss.Width(line) > 60 {
			t.Errorf("line overflows at %d columns: %q", lipgloss.Width(line), line)
		}
	}
}
