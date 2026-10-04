package app

// GORILLA OVERRIDE (2026-10-04): a receipt for what actually ran.
//
// # THE FAILURE
//
// Measured with a real model (gemma-4-e2b), in two of four runs. The prompt:
// run a failing command, and if it fails run it again, up to 7 times. The model
// ran it ONCE and answered:
//
//	I made 7 attempts. All 7 attempts failed with the same error
//
// The session record shows one call. In the interactive window every call is on
// screen and a person can count them. A run started with -p prints only the
// model's answer, so the claim arrives with nothing beside it to check it
// against — and a headless run is exactly where nobody was watching.
//
// # WHAT THIS DOES
//
// After the answer, the headless run prints what the program itself recorded:
// how many tool calls were made, which, with what, and how each ended. It is
// built from the stored session, never from anything the model wrote. A model
// can say what it likes; the receipt is not its to write.
//
// # WHAT IT IS NOT
//
// It does not judge the answer, look for numbers in it, or compare claims. A
// checker that tried would be a second thing that can be wrong. It puts the
// record next to the claim and leaves the comparison to the reader, which is
// the one comparison that cannot be fooled by wording.

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"github.com/opencode-ai/opencode/internal/llm/tools"
	"github.com/opencode-ai/opencode/internal/message"
)

const (
	receiptMaxLines  = 20 // distinct lines listed; the count above it is always complete
	receiptMaxDetail = 90 // characters of a call's arguments shown
)

// ReceiptLine is one call, or a run of identical consecutive calls.
type ReceiptLine struct {
	Tool    string `json:"tool"`
	Detail  string `json:"detail"`
	Outcome string `json:"outcome"`
	Times   int    `json:"times"`
}

// Receipt is the program's own account of a run.
type Receipt struct {
	Calls    int            `json:"tool_calls"`
	ByTool   map[string]int `json:"by_tool"`
	Problems int            `json:"calls_that_did_not_succeed"`
	Lines    []ReceiptLine  `json:"calls"`
}

var exitCodeRe = regexp.MustCompile(`(?m)^Exit code (\d+)\s*$`)

// partialReviewRe matches the review tool's own verdict line (tools/review.go).
var partialReviewRe = regexp.MustCompile(`\*\*PARTIAL REVIEW\.\*\* (\d+) of (\d+) scheduled jobs completed`)

// BuildReceipt reads the stored messages of one session. It uses only tool
// calls and tool results, which the program writes; assistant text is ignored.
func BuildReceipt(msgs []message.Message) Receipt {
	results := map[string]message.ToolResult{}
	for i := range msgs {
		for _, tr := range msgs[i].ToolResults() {
			results[tr.ToolCallID] = tr
		}
	}

	r := Receipt{ByTool: map[string]int{}}
	for i := range msgs {
		for _, tc := range msgs[i].ToolCalls() {
			r.Calls++
			r.ByTool[tc.Name]++

			outcome := "no result recorded"
			if tr, ok := results[tc.ID]; ok {
				outcome = outcomeOf(tr)
			}
			if outcome != "ok" {
				r.Problems++
			}

			line := ReceiptLine{Tool: tc.Name, Detail: detailOf(tc), Outcome: outcome, Times: 1}
			if n := len(r.Lines); n > 0 && r.Lines[n-1].Tool == line.Tool &&
				r.Lines[n-1].Detail == line.Detail && r.Lines[n-1].Outcome == line.Outcome {
				r.Lines[n-1].Times++
				continue
			}
			r.Lines = append(r.Lines, line)
		}
	}
	return r
}

// outcomeOf says how a call ended, in a few words, from its result alone.
func outcomeOf(tr message.ToolResult) string {
	c := tr.Content
	low := strings.ToLower(c)
	switch {
	case strings.HasPrefix(low, "permission denied"):
		return "refused, not run"
	case strings.Contains(c, "Tool execution canceled"):
		return "cancelled, not run"
	case strings.HasPrefix(c, "Tool not found"):
		return "no such tool"
	}
	// GORILLA FIX (2026-10-04): a review that did not review is not "ok".
	//
	// Found on the first run against a real model: a review of a Python file
	// with no Python analyser installed returned its refusal as an ordinary
	// result, and this printed `review calc.py -> ok`. The call did return. But
	// "ok" beside a review that looked at nothing is the false comfort the
	// review tool's own first line exists to prevent, so the three verdicts it
	// can give are read here and said in its words.
	switch {
	case strings.HasPrefix(c, "The review did NOT run"):
		return "did not run: no analyser installed"
	case strings.Contains(c, "**NOTHING WAS REVIEWED.**"):
		return "ran, but reviewed nothing"
	}
	if m := partialReviewRe.FindStringSubmatch(c); m != nil {
		return "partial: " + m[1] + " of " + m[2] + " jobs completed"
	}
	// A command that ran and failed is not an "error result" as far as the
	// shell tool is concerned: it reports the exit code in the text. Read it,
	// or every failing build would be listed here as ok.
	if m := exitCodeRe.FindAllStringSubmatch(c, -1); len(m) > 0 {
		if code := m[len(m)-1][1]; code != "0" {
			return "exit code " + code
		}
	}
	if tr.IsError {
		return "failed"
	}
	return "ok"
}

// detailOf picks the one argument that says what the call was about.
func detailOf(tc message.ToolCall) string {
	var args map[string]any
	if json.Unmarshal([]byte(tc.Input), &args) != nil {
		return clip(tc.Input)
	}
	for _, key := range []string{"command", "file_path", "path", "query", "url", "pattern", "prompt"} {
		if v, ok := args[key].(string); ok && v != "" {
			// A second argument where one alone is ambiguous.
			if key == "path" {
				if q, ok := args["query"].(string); ok && q != "" {
					v = q + " in " + v
				}
			}
			return clip(v)
		}
	}
	return clip(tc.Input)
}

func clip(s string) string {
	// The receipt is printed where a log may keep it. A credential typed into a
	// command is masked here by the same rule that masks it for the model.
	s, _ = tools.MaskSecrets(tools.BashToolName, strings.Join(strings.Fields(s), " "))
	if len(s) > receiptMaxDetail {
		return s[:receiptMaxDetail] + "..."
	}
	return s
}

// Text renders the receipt for a person. The first line is the whole count,
// always complete, because it is the line a false claim is checked against.
func (r Receipt) Text() string {
	var b strings.Builder
	b.WriteString("--- what actually ran (recorded by Gorilla OpenCode, not written by the AI) ---\n")
	switch r.Calls {
	case 0:
		b.WriteString("No tool was called. The answer above was written without running, reading or checking anything.\n")
		return b.String()
	case 1:
		b.WriteString("1 tool call")
	default:
		fmt.Fprintf(&b, "%d tool calls", r.Calls)
	}
	if r.Problems > 0 {
		fmt.Fprintf(&b, ", %d did not succeed", r.Problems)
	}
	b.WriteString(":\n")
	for i, l := range r.Lines {
		if i == receiptMaxLines {
			fmt.Fprintf(&b, "  ... and %d more line(s) not listed; the count above is complete\n", len(r.Lines)-receiptMaxLines)
			break
		}
		times := ""
		if l.Times > 1 {
			times = fmt.Sprintf("  x%d", l.Times)
		}
		fmt.Fprintf(&b, "  %-6s %s  -> %s%s\n", l.Tool, l.Detail, l.Outcome, times)
	}
	return b.String()
}
