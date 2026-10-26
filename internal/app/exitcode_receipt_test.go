package app

// robocopy exit 1-7 is success; the receipt and the claim guard must say so.
// Test names end in a lowercase token because `go test -run` compiles each
// top-level alternative of the goal's pattern on its own.

import (
	"strings"
	"testing"

	"github.com/opencode-ai/opencode/internal/message"
)

const robocopyCmd = `{"command":"robocopy \"C:\\\\SourceDir\" \"D:\\\\DestDir\" /MIR /MT:16 /Z"}`

func TestRobocopyReceiptCountsSuccess_robocopy(t *testing.T) {
	msgs := []message.Message{
		assistant("", call("a", "bash", robocopyCmd)),
		toolMsg(result("a", "  New File  a.txt\nExit code 1\nrobocopy: success - files were copied", false)),
		assistant("", call("b", "bash", robocopyCmd)),
		toolMsg(result("b", "ERROR 5 (0x00000005) Access is denied.\nExit code 8\nrobocopy: FAILED - some files or folders could NOT be copied (retries exhausted)", false)),
	}
	r := BuildReceipt(msgs)
	if r.Problems != 1 {
		t.Fatalf("problems = %d, want 1 (exit 8 only):\n%s", r.Problems, r.Text())
	}
	if got := r.Lines[0].Outcome; !strings.HasPrefix(got, "ok, exit code 1: robocopy: success") {
		t.Errorf("exit 1 outcome = %q", got)
	}
	if got := r.Lines[len(r.Lines)-1].Outcome; got != "exit code 8" {
		t.Errorf("exit 8 outcome = %q", got)
	}
}

func TestExitCodeOtherProgramsStayFailures_exitcode(t *testing.T) {
	tr := result("a", "Exit code 1", false)
	if got := outcomeForCall(call("a", "bash", `{"command":"go build ./..."}`), tr); got != "exit code 1" {
		t.Errorf("go build exit 1 = %q, want a failure", got)
	}
	if got := outcomeForCall(call("a", "bash", `{"command":"fc a.txt b.txt"}`), tr); !strings.HasPrefix(got, "ok, ") {
		t.Errorf("fc exit 1 = %q, want success (files differ)", got)
	}
	// Without the command (the old entry point) nothing is reinterpreted.
	if got := outcomeFor("bash", tr); got != "exit code 1" {
		t.Errorf("outcomeFor without a command = %q", got)
	}
}

// A mirror that worked is a call that succeeded: no "every call failed"
// warning under an answer that says files were copied.
func TestRobocopyClaimGuardSeesSuccess_robocopy(t *testing.T) {
	turn := []message.Message{
		user("mirror C:\\SourceDir to D:\\DestDir with 16 threads"),
		assistant("", call("a", "bash", robocopyCmd)),
		toolMsg(result("a", "Exit code 1\nrobocopy: success - files were copied", false)),
		assistant("I ran the mirror."),
	}
	if k := ClaimedActions("I ran the mirror."); len(k) == 0 {
		t.Fatal("the answer must carry a claim, or this test proves nothing")
	}
	if w := ClaimWarning(turn); w != "" {
		t.Errorf("a successful robocopy drew a warning: %s", w)
	}
	turn[2] = toolMsg(result("a", "Exit code 16\nrobocopy: FAILED - serious error", false))
	if w := ClaimWarning(turn); w == "" {
		t.Error("robocopy exit 16 should leave the claim unsupported")
	}
}
