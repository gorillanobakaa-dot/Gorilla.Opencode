package plain

// GORILLA FIX (2026-08-18): plain mode must show WHAT is being approved.
//
// The TUI renders the diff and the command in a fenced block. This mode showed
// only the tool name and a one-line description, so a file rewrite was approved
// blind — in the mode this project steers weak terminals toward, and against
// PHILOSOPHY.md's central claim that the user can CHECK what was done.

import (
	"strings"
	"testing"
)

func TestTheCommandIsShownBeforeApproval(t *testing.T) {
	out := describePermissionParams(map[string]any{
		"command": "rm -rf /home/gorilla/Documents",
	})
	if !strings.Contains(out, "rm -rf /home/gorilla/Documents") {
		t.Errorf("the command being approved is not shown: %q", out)
	}
}

func TestTheDiffIsShownBeforeApproval(t *testing.T) {
	diff := "--- a/main.go\n+++ b/main.go\n-  safe()\n+  rm_everything()"
	out := describePermissionParams(map[string]any{
		"file_path": "/work/main.go",
		"diff":      diff,
	})
	for _, want := range []string{"rm_everything()", "safe()"} {
		if !strings.Contains(out, want) {
			t.Errorf("the diff line %q is missing — the change was approved blind:\n%s", want, out)
		}
	}
}

// Multi-line content must stay readable, not JSON-escaped into one line.
func TestOutputIsReadableNotEscaped(t *testing.T) {
	out := describePermissionParams(map[string]any{"diff": "line one\nline two"})
	if strings.Contains(out, "\\n") {
		t.Errorf("newlines were escaped rather than rendered: %q", out)
	}
	if len(strings.Split(out, "\n")) < 2 {
		t.Errorf("multi-line content collapsed to one line: %q", out)
	}
}

// No params, or params with nothing meaningful, must not print an empty header.
func TestNothingToShowPrintsNothing(t *testing.T) {
	if got := describePermissionParams(nil); got != "" {
		t.Errorf("nil params produced output: %q", got)
	}
	if got := describePermissionParams(map[string]any{"unrelated": 5}); got != "" {
		t.Errorf("params with nothing showable produced output: %q", got)
	}
}
