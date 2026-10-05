package app

// GORILLA OVERRIDE (2026-10-04): the receipt is built from what the program
// recorded, and from nothing the model wrote.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/opencode-ai/opencode/internal/message"
)

func assistant(text string, calls ...message.ToolCall) message.Message {
	parts := []message.ContentPart{}
	if text != "" {
		parts = append(parts, message.TextContent{Text: text})
	}
	for _, c := range calls {
		parts = append(parts, c)
	}
	return message.Message{Role: message.Assistant, Parts: parts}
}

func toolMsg(results ...message.ToolResult) message.Message {
	parts := []message.ContentPart{}
	for _, r := range results {
		parts = append(parts, r)
	}
	return message.Message{Role: message.Tool, Parts: parts}
}

func call(id, name, input string) message.ToolCall {
	return message.ToolCall{ID: id, Name: name, Input: input}
}

func result(id, content string, isErr bool) message.ToolResult {
	return message.ToolResult{ToolCallID: id, Content: content, IsError: isErr}
}

// The measured failure, as recorded: one call, and an answer claiming seven.
func TestTheReceiptContradictsAClaimOfSevenAttemptsWithOneCall(t *testing.T) {
	msgs := []message.Message{
		assistant("", call("a", "bash", `{"command":"python missing_script.py"}`)),
		toolMsg(result("a", "python: can't open file 'missing_script.py': [Errno 2] No such file or directory\n\nExit code 2", false)),
		assistant("I made 7 attempts. All 7 attempts failed with the same error."),
	}
	r := BuildReceipt(msgs)
	if r.Calls != 1 {
		t.Fatalf("the receipt counts %d calls; the record holds 1", r.Calls)
	}
	text := r.Text()
	for _, want := range []string{"1 tool call", "python missing_script.py", "exit code 2", "not written by the AI"} {
		if !strings.Contains(text, want) {
			t.Errorf("the receipt does not say %q:\n%s", want, text)
		}
	}
	if strings.Contains(text, "7") {
		t.Errorf("the model's claim leaked into the receipt; it must be built from tool calls alone:\n%s", text)
	}
}

// A command that ran and failed is NOT an error result as far as the shell tool
// is concerned. Without reading the exit code every failing build would be "ok".
func TestAFailingCommandIsNotListedAsOk(t *testing.T) {
	r := BuildReceipt([]message.Message{
		assistant("", call("a", "bash", `{"command":"make"}`)),
		toolMsg(result("a", "cc: error\n\nExit code 2", false)),
	})
	if r.Problems != 1 || r.Lines[0].Outcome != "exit code 2" {
		t.Errorf("a failed command was recorded as %q with %d problems", r.Lines[0].Outcome, r.Problems)
	}
	ok := BuildReceipt([]message.Message{
		assistant("", call("a", "bash", `{"command":"make"}`)),
		toolMsg(result("a", "built\n\nExit code 0", false)),
	})
	if ok.Problems != 0 || ok.Lines[0].Outcome != "ok" {
		t.Errorf("a successful command was recorded as %q", ok.Lines[0].Outcome)
	}
}

func TestIdenticalConsecutiveCallsAreOneLineWithACount(t *testing.T) {
	var msgs []message.Message
	for _, id := range []string{"a", "b", "c", "d", "e"} {
		msgs = append(msgs, assistant("", call(id, "view", `{"file_path":"status.txt"}`)), toolMsg(result(id, "WAIT", false)))
	}
	r := BuildReceipt(msgs)
	if r.Calls != 5 || len(r.Lines) != 1 || r.Lines[0].Times != 5 {
		t.Fatalf("five identical calls became %d calls on %d lines", r.Calls, len(r.Lines))
	}
	if !strings.Contains(r.Text(), "5 tool calls") || !strings.Contains(r.Text(), "x5") {
		t.Errorf("the receipt does not show the count:\n%s", r.Text())
	}
}

func TestRefusedCancelledAndMissingResultsAreNamed(t *testing.T) {
	r := BuildReceipt([]message.Message{
		assistant("", call("a", "bash", `{"command":"git reset --hard"}`), call("b", "view", `{"file_path":"x"}`), call("c", "find", `{"query":"q","path":"p"}`)),
		toolMsg(result("a", "Permission denied: this command was NOT run, because ...", true), result("b", "Tool execution canceled by user", true)),
	})
	got := []string{r.Lines[0].Outcome, r.Lines[1].Outcome, r.Lines[2].Outcome}
	want := []string{"refused, not run", "cancelled, not run", "no result recorded"}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("call %d recorded as %q, want %q", i+1, got[i], want[i])
		}
	}
	if r.Lines[2].Detail != "q in p" {
		t.Errorf("a find call is described as %q; want the query and where", r.Lines[2].Detail)
	}
	if r.Problems != 3 {
		t.Errorf("%d problems counted, want 3", r.Problems)
	}
}

// An answer written without touching anything is the case most worth flagging.
func TestNoCallsIsSaidOutright(t *testing.T) {
	text := BuildReceipt([]message.Message{assistant("The build passes and all tests are green.")}).Text()
	if !strings.Contains(text, "No tool was called") || !strings.Contains(text, "without running, reading or checking anything") {
		t.Errorf("an answer with no tool call behind it is not called what it is:\n%s", text)
	}
}

func TestALongRunKeepsTheCountCompleteAndSaysWhatIsNotListed(t *testing.T) {
	var msgs []message.Message
	for i := 0; i < receiptMaxLines+7; i++ {
		id := string(rune('a' + i))
		msgs = append(msgs, assistant("", call(id, "view", `{"file_path":"f`+id+`.go"}`)), toolMsg(result(id, "ok", false)))
	}
	text := BuildReceipt(msgs).Text()
	if !strings.Contains(text, "27 tool calls") {
		t.Errorf("the total is not complete:\n%s", text)
	}
	if !strings.Contains(text, "7 more line(s) not listed; the count above is complete") {
		t.Errorf("lines were dropped without saying so:\n%s", text)
	}
}

func TestLongArgumentsAreCutAndWhitespaceFlattened(t *testing.T) {
	long := strings.Repeat("x", 300)
	r := BuildReceipt([]message.Message{assistant("", call("a", "bash", `{"command":"echo `+long+`\n  second line"}`))})
	if len(r.Lines[0].Detail) > receiptMaxDetail+3 || strings.Contains(r.Lines[0].Detail, "\n") {
		t.Errorf("detail not bounded to one line: %d chars", len(r.Lines[0].Detail))
	}
}

// Found on the first run against a real model: `review calc.py -> ok` for a
// review that had refused to run. The receipt must use the review's own verdict.
func TestAReviewThatDidNotReviewIsNotListedAsOk(t *testing.T) {
	cases := map[string]string{
		"The review did NOT run, because no analyser for this code is installed on this machine.":      "did not run: no analyser installed",
		"# Code review\n\n## Trust\n\n- **NOTHING WAS REVIEWED.** No analyser completed a single job.": "ran, but reviewed nothing",
		"# Code review\n\n## Trust\n\n- **PARTIAL REVIEW.** 1 of 7 scheduled jobs completed.":          "partial: 1 of 7 jobs completed",
		"# Code review\n\n## Trust\n\n- Coverage: all 4 scheduled jobs completed":                      "ok",
	}
	for content, want := range cases {
		r := BuildReceipt([]message.Message{
			assistant("", call("a", "review", `{"path":"calc.py"}`)),
			toolMsg(result("a", content, false)),
		})
		if r.Lines[0].Outcome != want {
			t.Errorf("review recorded as %q, want %q", r.Lines[0].Outcome, want)
		}
		if (want != "ok") != (r.Problems == 1) {
			t.Errorf("%q: counted %d problem(s)", want, r.Problems)
		}
	}
}

// The review tool and the receipt are two files that must agree on wording. If
// the verdict lines in tools/review.go are reworded, this fails and says where.
func TestTheReceiptStillRecognisesTheReviewToolsOwnWords(t *testing.T) {
	src, err := os.ReadFile(filepath.Join("..", "llm", "tools", "review.go"))
	if err != nil {
		t.Skipf("review.go not readable: %v", err)
	}
	for _, phrase := range []string{`"The review did NOT run`, `**NOTHING WAS REVIEWED.**`, `**PARTIAL REVIEW.** %d of %d scheduled jobs completed`} {
		if !strings.Contains(string(src), phrase) {
			t.Errorf("tools/review.go no longer contains %s; receipt.go matches on it and would report such a review as ok", phrase)
		}
	}
}

// GORILLA FIX (2026-10-05): the shell's phrases are only the shell's. Viewing a
// file that CONTAINS "Exit code 1" is not a failed call, and a command stopped
// by a timeout is not "ok".
func TestOutcomesDependOnWhichToolProducedThem(t *testing.T) {
	text := "some log\nExit code 1\n"
	if got := outcomeFor("view", message.ToolResult{Content: text}); got != "ok" {
		t.Errorf("a view of a file containing the words was listed as %q", got)
	}
	if got := outcomeFor("bash", message.ToolResult{Content: text}); got != "exit code 1" {
		t.Errorf("a failing command was listed as %q", got)
	}
	stopped := message.ToolResult{Content: "partial output\nCommand was aborted before completion"}
	if got := outcomeFor("bash", stopped); got == "ok" || !strings.Contains(got, "STOPPED") {
		t.Errorf("a command killed by a timeout was listed as %q", got)
	}
	if got := outcomeFor("view", stopped); got != "ok" {
		t.Errorf("a file that quotes the phrase was listed as %q", got)
	}
	orphan := message.ToolResult{Content: "This call never returned a result: the program stopped while it was running.", IsError: true}
	if got := outcomeFor("write", orphan); !strings.Contains(got, "never returned") {
		t.Errorf("an unanswered call was listed as %q", got)
	}
}
