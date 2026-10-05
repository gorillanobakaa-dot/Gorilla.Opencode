package agent

// GORILLA OVERRIDE: this file did not exist upstream. It is the safety net for
// an expensive research run.
//
// WHY IT EXISTS — measured, 2026-08-17/18. A ten-helper supervised dossier run
// on a free-tier model burned roughly 850,000 tokens over two hours, produced
// genuinely good work (three load-bearing claims verified by hand, an identity
// bridge established from an obscure tool-output fingerprint, honest grades
// throughout), announced "writing the dossier now" — and then died without
// writing a single byte. The orchestrator's context stood at 145% of the
// model's window: assembling the product is the most context-hungry moment of
// the whole run, so the failure scales with how MUCH research succeeded.
//
// The findings themselves were never the problem. Measured on that run, the
// nine lane reports totalled ~15,045 tokens — the contract (ANSWER / FINDINGS /
// SOURCES TRIED / CONFIDENCE / NOT ESTABLISHED) had already compressed two
// hours of searching into something that fits comfortably in a 32K window. What
// drowned the orchestrator was everything ELSE it was still carrying: the full
// tool results, a raw crates.io JSON dump, its own reasoning, the conversation.
//
// So this writes the graded material to disk the instant the run ends, from Go,
// before any model is asked to do anything with it. The worst case becomes an
// unpolished dossier instead of no dossier.
//
// The owner's field framing is the design constraint: on a satellite uplink at
// single-digit KB/s in Somalia, Sudan or the Lake Chad Basin, an interrupted
// run is the NORMAL case, not the contingency. Anything that cost half a
// million tokens must never exist only in a context window.

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/opencode-ai/opencode/internal/config"
	"github.com/opencode-ai/opencode/internal/logging"
)

// salvageStamp is the filename timestamp: sortable, and the same shape the
// project uses for accumulating artifacts elsewhere.
//
// GORILLA FIX (2026-10-05): audit finding S8. It stopped at the minute, and the
// name is otherwise only a four-word slug, so the same question run twice inside
// one minute wrote the second run's findings over the first's. The findings
// file is the only copy that survives a failed write-up; a save path that can
// silently replace one is the loss this file exists to prevent. Seconds are in
// the stamp now, and createFindingsFile refuses to open a name that exists.
const salvageStamp = "06-01-02-15-04-05"

// Findings are private: the question is in the first line and in the name.
//
// GORILLA FIX (2026-10-05): audit finding S8. These were 0755 and 0644, on a
// feature whose stated reason for writing outside the working folder is that a
// private question must stay private. On a shared Linux machine every other
// account could read every question asked. Owner-only, like the arsenal
// tagfile. (Windows ignores the mode; the folder is inside the user profile.)
const (
	findingsDirMode  os.FileMode = 0o700
	findingsFileMode os.FileMode = 0o600
)

// DoctrineStandard names the everyday run where it has to be written down. In
// the tool's parameters the everyday run is the empty string.
const DoctrineStandard = "standard"

// doctrineLinePrefix opens the line that records, in the findings file, which
// discipline produced it. See FindingsDoctrine.
const doctrineLinePrefix = "Doctrine: "

func doctrineName(doctrine string) string {
	if doctrine == DoctrineDossier {
		return DoctrineDossier
	}
	return DoctrineStandard
}

// writeRawFindings saves every lane's graded report to disk immediately.
//
// It never returns an error to the caller: a research run that produced good
// findings must not be reported as failed because a disk write went wrong. A
// failure is logged and the path is simply absent from the report.
func writeRawFindings(question string, roles []researchRole, replies []string, audits []string, doctrine string) string {
	dir := config.DossierDir()
	if err := os.MkdirAll(dir, findingsDirMode); err != nil {
		logging.Error("could not create the dossier directory; findings stay in the session store only",
			"dir", dir, "error", err)
		return ""
	}
	// MkdirAll leaves an existing folder as it found it, and every install made
	// before this fix has one at 0755. Best-effort: a folder that cannot be
	// tightened is still a folder the findings can be saved in.
	_ = os.Chmod(dir, findingsDirMode)

	// GORILLA FIX (2026-10-05): audit finding S9. The doctrine is RECORDED. A
	// standard run's findings carry evidence TIERS; a dossier's carry two-axis
	// GRADES. Nothing in the file said which, recovery assumed dossier for
	// everything, and the write-up prompt then demanded that grades which were
	// never assigned be "carried through unchanged" — an invitation to invent
	// them.
	var b strings.Builder
	fmt.Fprintf(&b, "# Raw findings — %s\n\n", question)
	fmt.Fprintf(&b, "Saved automatically at %s, before any model was asked to assemble them.\n\n",
		time.Now().Format("2006-01-02 15:04:05"))
	fmt.Fprintf(&b, "%s%s\n\n", doctrineLinePrefix, doctrineName(doctrine))
	b.WriteString("**This is not the finished write-up.** It is every lane's report, exactly as\n")
	b.WriteString("returned, so that the work survives even if the write-up step fails — which is the\n")
	b.WriteString("normal outcome of an interrupted run on a slow link. To have it written up, run\n")
	b.WriteString("`/osint --recover` (optionally after `/model` to pick a model with a larger window).\n\n")
	if doctrine == DoctrineDossier {
		b.WriteString("Grades are two-axis: a letter for SOURCE reliability (A-F) and a digit for\n")
		b.WriteString("INFORMATION credibility (1-6). They travel with each claim and must not be\n")
		b.WriteString("altered when the dossier is written.\n\n")
	} else {
		b.WriteString("Each claim carries an evidence TIER (primary_source, config, multiple_reports,\n")
		b.WriteString("single_claim, unsourced). The tier travels with the claim and must not be\n")
		b.WriteString("altered when the answer is written. These findings carry no two-axis grades.\n\n")
	}
	b.WriteString("---\n\n")

	covered := 0
	for i, role := range roles {
		fmt.Fprintf(&b, "## %s\n\n", role.Title)
		reply := ""
		if i < len(replies) {
			reply = strings.TrimSpace(replies[i])
		}
		if reply == "" {
			b.WriteString("**LANE UNCOVERED — this helper produced nothing.** Treat the ground it was\n")
			b.WriteString("given as unexamined; do not assume the other lanes compensate.\n\n---\n\n")
			continue
		}
		covered++
		b.WriteString(reply)
		if i < len(audits) {
			if a := strings.TrimSpace(audits[i]); a != "" {
				b.WriteString("\n\n### Supervisor audit of this lane\n\n")
				b.WriteString(a)
			}
		}
		b.WriteString("\n\n---\n\n")
	}
	fmt.Fprintf(&b, "%d of %d lanes produced findings.\n", covered, len(roles))

	path, err := createFindingsFile(dir, time.Now().Format(salvageStamp), slugify(question), []byte(b.String()))
	if err != nil {
		logging.Error("could not save raw findings", "dir", dir, "error", err)
		return ""
	}
	return path
}

// createFindingsFile writes body to a NEW file and returns its path. It never
// opens a name that already exists: O_EXCL, and a numbered suffix when two runs
// of one question land in the same second.
func createFindingsFile(dir, stamp, slug string, body []byte) (string, error) {
	var lastErr error
	for n := 1; n <= 50; n++ {
		name := fmt.Sprintf("findings-%s-%s.md", stamp, slug)
		if n > 1 {
			name = fmt.Sprintf("findings-%s-%s-%d.md", stamp, slug, n)
		}
		path := filepath.Join(dir, name)
		f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, findingsFileMode)
		if err != nil {
			lastErr = err
			if os.IsExist(err) {
				continue
			}
			return "", err
		}
		_, werr := f.Write(body)
		cerr := f.Close()
		if werr != nil {
			return "", werr
		}
		if cerr != nil {
			return "", cerr
		}
		return path, nil
	}
	return "", lastErr
}

// FindingsDoctrine reads back which discipline produced a findings document.
//
// The recorded line wins. A file written before the line existed has none, and
// is judged by what it contains: a dossier helper marks every claim "GRADE:",
// a standard helper "TIER:". Guessing "dossier" for everything is what the
// recovery path did until 2026-10-05, and it was wrong for every /research run.
func FindingsDoctrine(body string) string {
	header, _, _ := strings.Cut(body, "\n---")
	for _, line := range strings.Split(header, "\n") {
		line = strings.TrimSpace(line)
		if v, ok := strings.CutPrefix(line, doctrineLinePrefix); ok {
			if strings.TrimSpace(v) == DoctrineDossier {
				return DoctrineDossier
			}
			return DoctrineStandard
		}
	}
	if strings.Contains(body, "| GRADE:") {
		return DoctrineDossier
	}
	return DoctrineStandard
}

var slugUnsafe = regexp.MustCompile(`[^a-z0-9]+`)

// slugify turns a question into a short, safe filename fragment. Bounded at
// four words so a rambling question cannot produce an unusable filename.
func slugify(q string) string {
	s := slugUnsafe.ReplaceAllString(strings.ToLower(q), "-")
	s = strings.Trim(s, "-")
	if s == "" {
		return "research"
	}
	parts := strings.Split(s, "-")
	if len(parts) > 4 {
		parts = parts[:4]
	}
	out := strings.Join(parts, "-")
	if len(out) > 48 {
		out = out[:48]
	}
	return out
}
