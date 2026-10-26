package app

// GORILLA OVERRIDE (2026-10-25): flag an answer that says it did something in
// a turn where nothing ran.
//
// # THE FAILURE
//
// DeepSeek-R1-0528-Qwen3-8B, local, a 41-minute session. Asked to delete the
// screenshots in a folder, it answered, twice:
//
//	Executed `del /Q /F Screenshot*.png` ... removed all 52 PNG files
//
// with zero tool calls. The receipt (receipt.go) said "No tool was called" right
// under it, and the owner still had to read both and compare them. A claim of a
// deletion beside a record of nothing is the one contradiction that must not
// depend on a person noticing it.
//
// # WHAT THIS DOES
//
// When the turn's text claims a completed action (deleted, removed, moved, ran,
// executed, installed, saved) and the turn made no tool call, or every call it
// made failed, one warning line is shown: red in the TUI, after the answer in a
// -p run, and as a last chunk over ACP. The model's text is never blocked or
// changed; the line is added beside it.
//
// # WHAT IT IS NOT
//
// It is a phrase rule, not a judge. English only: a claim in another language
// is not flagged. It reads the words, not the files, so "nothing was deleted"
// means the record holds no call that could have done it. A turn that made a
// successful call is never flagged, whatever it says: checking the claim
// against what that call did is the receipt's job and the reader's.
//
// Known false positive: an explanation in the past tense ("in your example,
// `git rm` removed the file") reads exactly like DeepSeek's "This command
// removed all 52 PNG files" and is flagged. The line says what the answer
// says and what the record holds, both true, so a wrong flag costs a glance.
//
// The rules come from Fieldkit's run_harness_scenarios.py claims_done(): bare
// verbs were too loose (Gemma's honest "close the programs before they can be
// moved" matched "moved"), so a verb counts only in the shape of a finished
// claim, and not under a negation, a modal, a condition or a question.

import (
	"context"
	"fmt"
	"io"
	"os"
	"regexp"
	"strings"

	"github.com/opencode-ai/opencode/internal/message"
	"golang.org/x/term"
)

// claimKind is one kind of action an answer can claim.
type claimKind struct {
	claim string // what the answer says happened
	past  string // the word in "nothing was ..."
}

var (
	kindDelete  = claimKind{"files were deleted", "deleted"}
	kindMove    = claimKind{"files were moved", "moved"}
	kindRun     = claimKind{"a command was run", "run"}
	kindInstall = claimKind{"something was installed", "installed"}
	kindSave    = claimKind{"a file was saved", "saved"}
)

// claimVerbs are the past forms that can claim a finished action. "run" is not
// here: it is also the imperative ("Run `npm test`"), so it counts only after
// have/has/been (see participleOnly).
var claimVerbs = map[string]claimKind{
	"deleted": kindDelete, "removed": kindDelete, "erased": kindDelete, "wiped": kindDelete,
	"moved": kindMove, "renamed": kindMove,
	"ran": kindRun, "executed": kindRun,
	"installed": kindInstall, "uninstalled": kindInstall,
	"saved": kindSave,
}

// participleOnly are verbs whose past participle is also the bare form.
var participleOnly = map[string]claimKind{"run": kindRun}

// claimSubjects, read right before the verb (adverbs skipped), make it a claim:
// "I deleted", "have moved", "were removed", "has been run".
var claimSubjects = map[string]bool{
	"i": true, "we": true, "i've": true, "we've": true,
	"have": true, "has": true, "been": true, "was": true, "were": true,
}

// auxOnly is the subset that a participle-only verb needs: "I run" is not a claim.
var auxOnly = map[string]bool{"i've": true, "we've": true, "have": true, "has": true, "been": true}

// claimAdverbs may stand between the subject and the verb.
var claimAdverbs = map[string]bool{
	"successfully": true, "also": true, "just": true, "now": true, "then": true, "already": true,
	"finally": true, "all": true, "both": true, "completely": true, "permanently": true,
	"automatically": true, "simply": true, "safely": true,
}

// claimObjects, right after the verb, make it a claim on their own: "removed all
// 52 files", "deleted the folder", "which moved them".
var claimObjects = map[string]bool{
	"all": true, "every": true, "the": true, "them": true, "it": true, "those": true,
	"these": true, "both": true, "your": true,
}

// claimBlockers, within the few words before the verb in the same clause, mean
// it is not a claim: a negation, a modal, a condition, a plan, or the user.
var claimBlockers = map[string]bool{
	"not": true, "never": true, "no": true, "nothing": true, "none": true,
	"cannot": true, "can't": true, "won't": true, "wouldn't": true, "couldn't": true,
	"shouldn't": true, "didn't": true, "don't": true, "doesn't": true, "haven't": true,
	"hasn't": true, "wasn't": true, "weren't": true, "isn't": true, "aren't": true,
	"will": true, "would": true, "could": true, "should": true, "can": true, "may": true,
	"might": true, "must": true, "shall": true, "be": true, "being": true, "to": true,
	"if": true, "once": true, "before": true, "until": true, "unless": true, "when": true,
	"whether": true, "you": true, "need": true, "needs": true, "plan": true, "going": true,
	"let": true, "let's": true, "i'll": true, "we'll": true, "i'd": true,
}

// claimWindow is how many words before the verb are searched for a blocker.
const claimWindow = 4

var (
	fencedCodeRe = regexp.MustCompile("(?s)```.*?```")
	inlineCodeRe = regexp.MustCompile("`[^`\n]*`")
	clauseSepRe  = regexp.MustCompile(`[,;:()\[\]]|\s[-–—]\s`)
	claimWordRe  = regexp.MustCompile(`[a-z0-9']+`)
)

// ClaimedActions returns the kinds of finished action the text claims, in the
// order they first appear. Empty when it claims none.
func ClaimedActions(text string) []claimKind {
	// Code is quoted, not claimed: a script's `echo deleted` says nothing.
	text = fencedCodeRe.ReplaceAllString(text, "\n")
	text = inlineCodeRe.ReplaceAllString(text, " code ")
	text = strings.NewReplacer("’", "'", "‘", "'").Replace(strings.ToLower(text))

	var kinds []claimKind
	seen := map[claimKind]bool{}
	for _, sentence := range claimSentences(text) {
		if strings.HasSuffix(sentence, "?") {
			continue // "Should I delete them?" asks; it does not claim
		}
		for _, clause := range clauseSepRe.Split(sentence, -1) {
			for _, k := range clauseClaims(claimWordRe.FindAllString(clause, -1)) {
				if !seen[k] {
					seen[k] = true
					kinds = append(kinds, k)
				}
			}
		}
	}
	return kinds
}

// claimSentences splits at . ! ? and line ends. A dot inside a word ("a.png")
// does not end a sentence; the mark stays on the sentence so a question is seen.
func claimSentences(text string) []string {
	var out []string
	start := 0
	for i := 0; i < len(text); i++ {
		c := text[i]
		end := c == '\n' || ((c == '.' || c == '!' || c == '?') && (i+1 == len(text) || text[i+1] == ' ' || text[i+1] == '\n' || text[i+1] == '\t'))
		if end {
			if s := strings.TrimSpace(text[start : i+1]); s != "" {
				out = append(out, s)
			}
			start = i + 1
		}
	}
	if s := strings.TrimSpace(text[start:]); s != "" {
		out = append(out, s)
	}
	return out
}

func clauseClaims(words []string) []claimKind {
	var kinds []claimKind
	for i, w := range words {
		k, past := claimVerbs[w]
		pk, participle := participleOnly[w]
		if !past && !participle {
			continue
		}
		if blocked(words, i) {
			continue
		}
		// The word before the verb, past any adverbs; "" at the clause start.
		j := i - 1
		for j >= 0 && claimAdverbs[words[j]] {
			j--
		}
		before := ""
		if j >= 0 {
			before = words[j]
		}
		after := ""
		if i+1 < len(words) {
			after = words[i+1]
		}
		switch {
		case participle:
			if auxOnly[before] {
				kinds = append(kinds, pk)
			}
		case claimSubjects[before], claimObjects[after], isNumber(after):
			kinds = append(kinds, k)
		case j < 0 && (i > 0 || after == "code" || after == ""):
			// The clause opens with the verb: "Successfully installed requests"
			// (an adverb first), "Executed `del ...`" (code next, which reads
			// "code" here), or the verb alone ("Deleted."). A noun next is
			// left alone: "Deleted files go to the Recycle Bin" is not a claim.
			kinds = append(kinds, k)
		}
	}
	return kinds
}

func blocked(words []string, i int) bool {
	for j := max(0, i-claimWindow); j < i; j++ {
		if claimBlockers[words[j]] {
			return true
		}
	}
	return false
}

func isNumber(w string) bool {
	if w == "" {
		return false
	}
	for _, c := range w {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}

// LastTurn returns the messages of the turn that ends at index upTo: everything
// after the last user message before it. upTo < 0 means the end of msgs.
func LastTurn(msgs []message.Message, upTo int) []message.Message {
	if upTo < 0 || upTo >= len(msgs) {
		upTo = len(msgs) - 1
	}
	start := 0
	for i := upTo; i >= 0; i-- {
		if msgs[i].Role == message.User {
			start = i + 1
			break
		}
	}
	return msgs[start : upTo+1]
}

// ClaimWarning reads one turn and returns the warning line, or "" when there is
// nothing to flag. The text checked is everything the model wrote in the turn;
// the calls are the turn's own, judged by the receipt's outcome rule.
func ClaimWarning(turn []message.Message) string {
	var text strings.Builder
	results := map[string]message.ToolResult{}
	for i := range turn {
		if turn[i].Role == message.Assistant {
			text.WriteString(turn[i].Content().String())
			text.WriteString("\n")
		}
		for _, tr := range turn[i].ToolResults() {
			results[tr.ToolCallID] = tr
		}
	}
	kinds := ClaimedActions(text.String())
	if len(kinds) == 0 {
		return ""
	}
	calls, succeeded := 0, 0
	for i := range turn {
		for _, tc := range turn[i].ToolCalls() {
			calls++
			if tr, ok := results[tc.ID]; ok {
				if o := outcomeForCall(tc, tr); o == "ok" || strings.HasPrefix(o, "ok, ") {
					succeeded++
				}
			}
		}
	}
	if succeeded > 0 {
		return ""
	}
	claims := make([]string, len(kinds))
	pasts := make([]string, len(kinds))
	for i, k := range kinds {
		claims[i], pasts[i] = k.claim, k.past
	}
	said := strings.Join(claims, " and ")
	nothing := strings.Join(pasts, " or ")
	if calls == 0 {
		return fmt.Sprintf("The answer says %s, but no tool ran this turn: nothing was %s.", said, nothing)
	}
	return fmt.Sprintf("The answer says %s, but every tool call this turn failed (%d of %d): no call succeeded, so nothing was %s by this turn.",
		said, calls, calls, nothing)
}

// ClaimWarningFor is ClaimWarning for the last turn of a stored session, for
// front ends outside this package (the ACP server). "" when the session cannot
// be read: a warning built from no record would be a guess.
func (a *App) ClaimWarningFor(sessionID string) string {
	msgs, err := a.Messages.List(context.Background(), sessionID)
	if err != nil || len(msgs) == 0 {
		return ""
	}
	return ClaimWarning(LastTurn(msgs, -1))
}

// ClaimWarningPrefix starts every warning line, so a script or a reader can
// find it the way the receipt is found by "--- what actually ran".
const ClaimWarningPrefix = "!! CLAIM NOT BACKED BY ANY TOOL CALL: "

// ClaimWarningLine is the warning as one printed line, "" for no warning.
func ClaimWarningLine(warning string) string {
	if warning == "" {
		return ""
	}
	return ClaimWarningPrefix + warning
}

// printClaimWarning writes the line, red and bold on a terminal that allows
// colour, plain otherwise (a pipe, a log, NO_COLOR).
func printClaimWarning(w io.Writer, warning string) {
	line := ClaimWarningLine(warning)
	if line == "" {
		return
	}
	if f, ok := w.(*os.File); ok && os.Getenv("NO_COLOR") == "" && term.IsTerminal(int(f.Fd())) {
		line = "\x1b[1;31m" + line + "\x1b[0m"
	}
	fmt.Fprintln(w, line)
	fmt.Fprintln(w)
}
