package agent

// GORILLA OVERRIDE (2026-08-18): `/osint --recover` — turn the findings of a
// research run that never got written up into the dossier it should have been.
//
// WHY IT EXISTS. A supervised ten-helper run on 2026-08-17 burned roughly
// 850,000 tokens over two hours, verified three load-bearing claims by hand,
// bridged an identity through an obscure tool-output fingerprint, graded it
// honestly — and then died at the write-up with the orchestrator's context at
// 145% of the model's window. Nothing reached disk. The failure scales with how
// MUCH research succeeded, which is the worst possible shape for it to have.
//
// research_salvage.go closes that hole going forward: findings are written from
// Go the instant the lanes report, before any model is asked to assemble them.
// This file closes it BACKWARDS — every dead run already in the session store
// is still recoverable, because the lane reports were always there. Last
// night's four runs are the corpus this was built and tested against.
//
// TWO THINGS MADE THIS CHEAP, and both are deliberate:
//
//   1. The extraction is pure Go. Listing the runs, pulling each lane's final
//      report, pairing supervisor audits to their lanes, writing the markdown —
//      not one token. A recovery that needed a model to find its own findings
//      would be a second chance to fail the same way.
//
//   2. The output contract had already done the compressing. Measured on that
//      run, nine lane reports totalled ~15,045 tokens — the ANSWER / FINDINGS /
//      SOURCES TRIED / CONFIDENCE / NOT ESTABLISHED shape had already squeezed
//      two hours of searching into something that fits a 32K window with room
//      to spare. What drowned the orchestrator was everything else it carried:
//      raw tool results, a crates.io JSON dump, its own reasoning, the whole
//      conversation. Assembling from the distillate in a FRESH session is a
//      different job of a different size.
//
// The owner's framing is the design constraint: on a satellite uplink at
// single-digit KB/s in Somalia, Sudan or the Lake Chad Basin, an interrupted
// run is the normal case rather than the contingency. Recovery is not a repair
// tool for a bug — it is a first-class part of how the thing is used.

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/opencode-ai/opencode/internal/config"
	"github.com/opencode-ai/opencode/internal/message"
	"github.com/opencode-ai/opencode/internal/session"
)

// RecoverableRun is one research run whose findings survived, whatever became
// of its write-up.
type RecoverableRun struct {
	// CallID identifies the run in the session store. Empty for a run that is
	// already a findings file on disk.
	CallID string
	// Path is the findings file, when one exists. A DB-sourced run has no path
	// until it is recovered.
	Path string

	Question string
	When     time.Time
	Lanes    int // helper sessions belonging to the run
	Covered  int // of those, how many actually reported
	Tokens   int64
}

// FromDisk reports whether this run was already saved as a findings file — the
// case for anything run after the salvage path shipped.
func (r RecoverableRun) FromDisk() bool { return r.Path != "" && r.CallID == "" }

// Label is the one line the picker shows.
func (r RecoverableRun) Label() string {
	q := strings.TrimSpace(r.Question)
	if q == "" {
		q = "(question not recorded)"
	}
	if len(q) > 60 {
		q = strings.TrimSpace(q[:57]) + "..."
	}
	return fmt.Sprintf("%s  %s", r.When.Format("Jan 02 15:04"), q)
}

// Detail is the second line: what survived, and what it cost to produce.
//
// A file-sourced run states how many lanes REPORTED, because writeRawFindings
// counted them against the actual reports. A store-sourced run states only how
// many lanes there were — deliberately.
//
// GORILLA FIX (2026-08-18): it used to claim "8 of 8 lanes reported" for a run
// that was cancelled after two minutes, where six lanes had emitted nothing but
// "Let me check the memory directories" and the recovered file correctly marked
// them LANE UNCOVERED. Coverage was being inferred from completion tokens,
// which narration and reasoning also produce. Reading every lane's messages to
// count properly would make the picker slower with every run ever made, so the
// listing says what it actually knows and the recovered file says the rest.
func (r RecoverableRun) Detail() string {
	if r.FromDisk() {
		if r.Lanes > 0 {
			return fmt.Sprintf("%d of %d lanes reported · from the saved findings file", r.Covered, r.Lanes)
		}
		return "from the saved findings file"
	}
	switch {
	case r.Lanes > 0 && r.Tokens > 0:
		return fmt.Sprintf("%d lanes · %s spent · from the session store", r.Lanes, humanTokens(r.Tokens))
	case r.Lanes > 0:
		return fmt.Sprintf("%d lanes · from the session store", r.Lanes)
	default:
		return "from the session store"
	}
}

func humanTokens(n int64) string {
	if n < 1000 {
		return fmt.Sprintf("%d tokens", n)
	}
	return fmt.Sprintf("%.0fK tokens", float64(n)/1000)
}

// helperIDPattern matches the session ids research helpers are given:
// call_<toolCallID>-<roleID>, where a supervisor's role id is "supervisor:x".
var helperIDPattern = regexp.MustCompile(`^(call_[A-Za-z0-9]+)-(.+)$`)

// ListRecoverableRuns finds every run that can still be written up: the saved
// findings files first, then the runs that live only in the session store.
//
// It never fails on one bad row. A run whose sessions are half-readable is
// worth listing with what it has — the alternative is telling someone that two
// hours of their money is gone because one query returned an error.
func ListRecoverableRuns(ctx context.Context, sessions session.Service, messages message.Service) []RecoverableRun {
	files := recoverableFiles()

	// A run that has already been written to a findings file is the SAME run as
	// the helper sessions it came from, and listing it twice makes the picker
	// look like the user ran everything twice. Observed live the first time this
	// screen was driven: eleven entries for six runs.
	//
	// Matched on the question because nothing links a file back to its call id —
	// the file is generated FROM those sessions, so the two are equivalent and
	// the file is the one that survives a database reset.
	saved := make(map[string]bool, len(files))
	for _, f := range files {
		if q := normaliseQuestion(f.Question); q != "" {
			saved[q] = true
		}
	}

	runs := files
	for _, r := range recoverableSessions(ctx, sessions, messages) {
		if saved[normaliseQuestion(r.Question)] {
			continue
		}
		runs = append(runs, r)
	}
	sort.Slice(runs, func(i, j int) bool { return runs[i].When.After(runs[j].When) })
	return runs
}

// normaliseQuestion folds the differences that do not make two questions
// different: case, and the whitespace a wrapped prompt picks up.
func normaliseQuestion(q string) string {
	return strings.ToLower(strings.Join(strings.Fields(q), " "))
}

// recoverableFiles lists the findings files the salvage path wrote.
func recoverableFiles() []RecoverableRun {
	dir := config.DossierDir()
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil // no directory yet is the normal state, not an error
	}
	var out []RecoverableRun
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasPrefix(name, "findings-") || !strings.HasSuffix(name, ".md") {
			continue
		}
		path := filepath.Join(dir, name)
		run := RecoverableRun{Path: path}
		if info, err := e.Info(); err == nil {
			run.When = info.ModTime()
		}
		// The question is on the first line as "# Raw findings — <question>".
		if body, err := os.ReadFile(path); err == nil {
			s := string(body)
			if first, _, ok := strings.Cut(s, "\n"); ok {
				run.Question = strings.TrimSpace(strings.TrimPrefix(strings.TrimPrefix(first, "# Raw findings"), " —"))
			}
			// Coverage comes from the line writeRawFindings puts at the foot,
			// not from counting headings. The lane REPORTS carry their own
			// "## ANSWER" and "## FINDINGS" headings, so counting them read a
			// six-lane run as thirty — measured on a recovered file.
			run.Covered, run.Lanes = parseCoverage(s)
		}
		out = append(out, run)
	}
	return out
}

// recoverableSessions groups helper sessions back into the runs they came from.
func recoverableSessions(ctx context.Context, sessions session.Service, messages message.Service) []RecoverableRun {
	// A missing store is a state, not a failure: the findings files listed
	// beside this are still recoverable without it.
	if sessions == nil || messages == nil {
		return nil
	}
	all, err := sessions.ListResearchHelpers(ctx)
	if err != nil {
		return nil
	}

	type group struct {
		lanes  []session.Session
		when   time.Time
		tokens int64
	}
	groups := map[string]*group{}

	for _, s := range all {
		if !strings.HasPrefix(s.Title, "Research: ") {
			continue
		}
		m := helperIDPattern.FindStringSubmatch(s.ID)
		if m == nil {
			continue
		}
		callID := m[1]
		g := groups[callID]
		if g == nil {
			g = &group{}
			groups[callID] = g
		}
		g.lanes = append(g.lanes, s)
		g.tokens += s.PromptTokens + s.CompletionTokens
		if t := time.Unix(s.CreatedAt, 0); g.when.IsZero() || t.Before(g.when) {
			g.when = t
		}
	}

	var out []RecoverableRun
	for callID, g := range groups {
		run := RecoverableRun{
			CallID: callID,
			When:   g.when,
			Lanes:  len(g.lanes),
			Tokens: g.tokens,
		}
		// The question is taken from whichever lane answers first: every helper
		// prompt carries it verbatim under a fixed heading, so nothing had to
		// have been stored separately. Coverage is NOT counted here — see
		// Detail() for why an honest "8 lanes" beats a cheap "8 of 8 reported".
		for _, s := range g.lanes {
			if run.Question == "" {
				run.Question = questionFromLane(ctx, messages, s.ID)
			}
		}
		out = append(out, run)
	}
	return out
}

// questionFromLane pulls the investigated question back out of a helper's
// opening instructions. Every research prompt carries it verbatim under a fixed
// heading, so nothing has to have been stored separately for this to work.
func questionFromLane(ctx context.Context, messages message.Service, sessionID string) string {
	msgs, err := messages.List(ctx, sessionID)
	if err != nil {
		return ""
	}
	for _, m := range msgs {
		if m.Role != message.User {
			continue
		}
		text := m.Content().Text
		_, after, ok := strings.Cut(text, "THE QUESTION UNDER INVESTIGATION:\n")
		if !ok {
			continue
		}
		q, _, _ := strings.Cut(after, "\n\n")
		return strings.TrimSpace(q)
	}
	return ""
}

// laneReport returns a helper's final written report — the last assistant
// message that carried prose.
//
// The last one, not the first: helpers narrate between tool calls ("Let me
// search more broadly"), and the contract-shaped report is what they write at
// the end. Taking the first non-empty text would recover the narration and
// throw away the findings.
func laneReport(ctx context.Context, messages message.Service, sessionID string) string {
	msgs, err := messages.List(ctx, sessionID)
	if err != nil {
		return ""
	}
	best := ""
	for _, m := range msgs {
		if m.Role != message.Assistant {
			continue
		}
		if text := strings.TrimSpace(m.Content().Text); text != "" {
			best = text
		}
	}
	return best
}

// RecoverFindings rebuilds a run's findings document and writes it to the
// dossier directory. It returns the path and the document body.
//
// For a run already on disk it reads that file back rather than regenerating
// it: the saved copy is the record, and rewriting a record is how records get
// quietly altered.
func RecoverFindings(ctx context.Context, run RecoverableRun, sessions session.Service, messages message.Service) (string, string, error) {
	if run.FromDisk() {
		body, err := os.ReadFile(run.Path)
		if err != nil {
			return "", "", fmt.Errorf("cannot read the saved findings at %s: %w", run.Path, err)
		}
		return run.Path, string(body), nil
	}

	if sessions == nil || messages == nil {
		return "", "", fmt.Errorf("no session store available to recover %s from", run.CallID)
	}
	all, err := sessions.ListResearchHelpers(ctx)
	if err != nil {
		return "", "", fmt.Errorf("cannot read the session store: %w", err)
	}

	// Blind lanes and their supervisor audits, kept apart so each audit lands
	// under the lane it judged rather than as a section of its own.
	type lane struct {
		roleID string
		report string
	}
	var lanes []lane
	audits := map[string]string{}

	for _, s := range all {
		m := helperIDPattern.FindStringSubmatch(s.ID)
		if m == nil || m[1] != run.CallID {
			continue
		}
		roleID := m[2]
		report := laneReport(ctx, messages, s.ID)
		if strings.HasPrefix(roleID, "supervisor:") {
			audits[strings.TrimPrefix(roleID, "supervisor:")] = report
			continue
		}
		lanes = append(lanes, lane{roleID: roleID, report: report})
	}
	if len(lanes) == 0 {
		return "", "", fmt.Errorf("no helper sessions found for %s — nothing to recover", run.CallID)
	}

	// Present them in the order the roles are defined, so a recovered document
	// reads like a fresh one rather than in whatever order SQLite returned.
	order := map[string]int{}
	for i, r := range researchRoles {
		order[r.ID] = i
	}
	sort.SliceStable(lanes, func(i, j int) bool {
		oi, ok1 := order[lanes[i].roleID]
		oj, ok2 := order[lanes[j].roleID]
		if ok1 && ok2 {
			return oi < oj
		}
		return ok1 && !ok2
	})

	roles := make([]researchRole, len(lanes))
	replies := make([]string, len(lanes))
	auditList := make([]string, len(lanes))
	for i, l := range lanes {
		roles[i] = researchRole{ID: l.roleID, Title: roleTitle(l.roleID)}
		replies[i] = l.report
		auditList[i] = audits[l.roleID]
	}

	question := run.Question
	if question == "" {
		question = "(question not recorded)"
	}
	path := writeRawFindings(question, roles, replies, auditList, "dossier")
	if path == "" {
		return "", "", fmt.Errorf("recovered the findings but could not write them to %s", config.DossierDir())
	}
	body, err := os.ReadFile(path)
	if err != nil {
		return path, "", err
	}
	return path, string(body), nil
}

// roleTitle maps a stored role id back to its heading. An id that is no longer
// a defined role still gets a usable heading rather than being dropped — the
// roles may change, and a recovered run predates whatever they are now.
func roleTitle(id string) string {
	for _, r := range researchRoles {
		if r.ID == id {
			return r.Title
		}
	}
	return strings.ToUpper(strings.ReplaceAll(id, "_", " "))
}

// AssemblyPrompt is the instruction that turns a findings document into the
// dossier. It is sent into a conversation carrying nothing else.
//
// The findings travel INLINE when they fit. Handing the model a path and asking
// it to read the file would work, but it would also put the file through the
// tool-result path — and a tool result is exactly the kind of bulk that put the
// original run at 145% of its window. Inline, once, is the smaller thing.
func AssemblyPrompt(question, findings, path string) string {
	var b strings.Builder

	b.WriteString("Assemble the finished OSINT dossier from findings that have ALREADY been collected. ")
	b.WriteString("Do NOT research anything. Do not call the research tool. Do not search the web. ")
	b.WriteString("The collection phase is over and was paid for; your entire job is the write-up.\n\n")

	fmt.Fprintf(&b, "THE QUESTION: %s\n\n", question)
	fmt.Fprintf(&b, "The graded lane reports are below, exactly as the helpers returned them. "+
		"They were saved to %s.\n\n", path)

	b.WriteString("RULES FOR THE WRITE-UP:\n")
	b.WriteString("1. Carry every two-axis grade through UNCHANGED. A claim that arrived as C3 leaves as C3. " +
		"You did not do this research and you are not entitled to upgrade its confidence.\n")
	b.WriteString("2. Where a lane is marked LANE UNCOVERED, say so in NOT ESTABLISHED. " +
		"A gap reported is cheaper than a gap discovered later.\n")
	b.WriteString("3. Use the PHIA probability yardstick and an analytical confidence rating, " +
		"as the findings themselves do.\n")
	b.WriteString("4. Where the lanes disagree, say they disagree and give both. " +
		"Do not average two accounts into one that nobody reported.\n")
	b.WriteString("5. Add nothing from your own knowledge. If it is not in the findings, it is not in the dossier.\n\n")

	b.WriteString("THE PRODUCT — write it as markdown in this order: BLUF (the answer in three sentences, " +
		"with its probability band and confidence rating), KEY JUDGEMENTS (each with its grade), " +
		"the detail organised by theme rather than by lane, SOURCES TRIED, " +
		"and NOT ESTABLISHED (what nobody covered, and what it would take).\n\n")

	fmt.Fprintf(&b, "Write the finished dossier to a NEW timestamped file under %s using the write tool, "+
		"then tell the user the exact path and give them the BLUF in the conversation. "+
		"Never write it into the working folder: it may be a git repository, and a private question "+
		"must not end up in a commit.\n\n", config.DossierDir())

	b.WriteString("--- FINDINGS BEGIN ---\n\n")
	b.WriteString(findings)
	b.WriteString("\n\n--- FINDINGS END ---\n")

	return b.String()
}

// ChunkedAssemblyNote is prepended when the findings are too large for the
// model that will assemble them. It is a warning to the user, not to the model:
// the honest move is to say the window is too small and name the fix, rather
// than to truncate and produce a dossier that quietly omits three lanes.
func ChunkedAssemblyNote(findingsTokens, windowTokens int) string {
	return fmt.Sprintf(
		"These findings are roughly %s and the selected model's window is about %s. "+
			"Assembling them in one pass is the failure this command exists to recover from. "+
			"Switch to a model with a larger window (/model) and run /osint --recover again, "+
			"or assemble the lanes in batches — the findings file is on disk either way and "+
			"cannot be lost by trying.",
		humanTokens(int64(findingsTokens)), humanTokens(int64(windowTokens)))
}

// EstimateTokens is the same rough four-characters-per-token rule used
// elsewhere in this program. It is deliberately crude: it decides whether to
// warn, and a warning that is 20% out is still the right warning.
func EstimateTokens(s string) int { return len(s) / 4 }

// coveragePattern matches the summary line writeRawFindings writes at the foot
// of every findings file: "6 of 9 lanes produced findings."
var coveragePattern = regexp.MustCompile(`(\d+) of (\d+) lanes produced findings`)

// parseCoverage reads that line back. A file without one — hand-written, or
// from a future format — reports zero, and the picker simply omits the counts
// rather than inventing them.
func parseCoverage(body string) (covered, lanes int) {
	m := coveragePattern.FindStringSubmatch(body)
	if m == nil {
		return 0, 0
	}
	fmt.Sscanf(m[1], "%d", &covered)
	fmt.Sscanf(m[2], "%d", &lanes)
	return covered, lanes
}
