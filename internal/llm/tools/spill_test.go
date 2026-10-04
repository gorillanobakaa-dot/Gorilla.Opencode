package tools

// GORILLA OVERRIDE (2026-10-04): what a cap cuts off must still exist somewhere.

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

var spillPathInNotice = regexp.MustCompile(`saved at (\S+\.txt)`)

func useTempSpillDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	old := spillDirOverride
	spillDirOverride = dir
	t.Cleanup(func() { spillDirOverride = old })
	return dir
}

// The case this exists for: the line that matters is in the middle, and the
// middle is what truncation removes.
func TestTheMiddleOfALongCommandOutputIsKeptOnDisk(t *testing.T) {
	useTempSpillDir(t)
	needle := "src/geometry.h:412: error: expected ';' before '}' token"
	out := strings.Repeat("  CC  some/object/file.o\n", 2000) + needle + "\n" +
		strings.Repeat("  LD  some/other/thing\n", 2000)

	got := truncateOutput(out)
	if strings.Contains(got, needle) {
		t.Fatal("the test is not testing anything: the needle survived truncation")
	}
	m := spillPathInNotice.FindStringSubmatch(got)
	if m == nil {
		t.Fatalf("output was truncated and the notice names no file:\n%s", got[len(got)/2-200:len(got)/2+300])
	}
	saved, err := os.ReadFile(m[1])
	if err != nil {
		t.Fatalf("the notice names a file that cannot be read: %v", err)
	}
	if string(saved) != out {
		t.Errorf("the saved file is not the complete output: %d bytes saved, %d produced", len(saved), len(out))
	}
	if !strings.Contains(string(saved), needle) {
		t.Errorf("the line that was cut is not in the saved file")
	}
}

func TestTheGeneralCapAlsoKeepsWhatItCuts(t *testing.T) {
	useTempSpillDir(t)
	huge := strings.Repeat("x", MaxToolResponseBytes) + "THE-TAIL"
	got := NewTextResponse(huge).Content
	m := spillPathInNotice.FindStringSubmatch(got)
	if m == nil {
		t.Fatalf("clamped result names no file")
	}
	saved, err := os.ReadFile(m[1])
	if err != nil || !strings.HasSuffix(string(saved), "THE-TAIL") {
		t.Errorf("the tail that was cut is not in the saved file (err=%v)", err)
	}
}

// CAPABILITY GUARD. Output that fits is returned as it was and nothing is
// written: an ordinary command must not leave files behind.
func TestOutputThatFitsWritesNothing(t *testing.T) {
	dir := useTempSpillDir(t)
	small := strings.Repeat("ok\n", 100)
	if got := truncateOutput(small); got != small {
		t.Errorf("output that fits was altered")
	}
	if got := NewTextResponse(small).Content; got != small {
		t.Errorf("a result that fits was altered")
	}
	if entries, _ := os.ReadDir(dir); len(entries) != 0 {
		t.Errorf("%d file(s) written for output that was never truncated", len(entries))
	}
}

// If the file cannot be written the tool call must still succeed, with the old
// notice. A full disk is not a reason to lose the excerpt too.
func TestAnUnwritableDirectoryDoesNotFailTheCall(t *testing.T) {
	blocker := filepath.Join(t.TempDir(), "a-file-not-a-directory")
	if err := os.WriteFile(blocker, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	old := spillDirOverride
	spillDirOverride = filepath.Join(blocker, "cannot-exist")
	t.Cleanup(func() { spillDirOverride = old })

	got := truncateOutput(strings.Repeat("line\n", 20000))
	if !strings.Contains(got, "lines truncated") {
		t.Errorf("truncation notice lost when the spill failed")
	}
	if strings.Contains(got, "saved at") {
		t.Errorf("the notice claims a file was saved when it was not")
	}
}

func TestOldSpilledOutputIsPruned(t *testing.T) {
	dir := useTempSpillDir(t)
	for i := 0; i < spillKeep+15; i++ {
		if spillOutput("bash", "x") == "" {
			t.Fatal("spill failed")
		}
	}
	entries, _ := os.ReadDir(dir)
	if len(entries) > spillKeep {
		t.Errorf("%d spilled files kept; the limit is %d", len(entries), spillKeep)
	}
}

// Found with a real model (gemma-4-e2b, 2026-10-04). The notice naming the saved
// file sat in the middle of 30,000 bytes; the model never opened the file and
// reported that the line it was asked for did not exist. The LAST lines of a
// result are what a model weighs, so the file and the warning must be there.
func TestTheTruncationNoticeIsTheLastThingInTheResult(t *testing.T) {
	useTempSpillDir(t)
	out := strings.Repeat("record processed ok\n", 3000) + "NEEDLE: the one line that matters\n" +
		strings.Repeat("record processed ok\n", 3000)
	got := truncateOutput(out)

	tail := got[len(got)-1100:]
	for _, want := range []string{"OUTPUT TRUNCATED", "saved at", "Do not conclude that something is absent"} {
		if !strings.Contains(tail, want) {
			t.Errorf("the end of a truncated result does not say %q; a model that reads the start and the end never learns the middle exists:\n%s", want, tail)
		}
	}
	if !strings.HasSuffix(strings.TrimSpace(got), "]") {
		t.Errorf("the notice is not the final thing in the result")
	}
}
