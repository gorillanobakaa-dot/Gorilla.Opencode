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
const salvageStamp = "06-01-02-15-04"

// writeRawFindings saves every lane's graded report to disk immediately.
//
// It never returns an error to the caller: a research run that produced good
// findings must not be reported as failed because a disk write went wrong. A
// failure is logged and the path is simply absent from the report.
func writeRawFindings(question string, roles []researchRole, replies []string, audits []string, doctrine string) string {
	dir := config.DossierDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		logging.Error("could not create the dossier directory; findings stay in the session store only",
			"dir", dir, "error", err)
		return ""
	}

	name := fmt.Sprintf("findings-%s-%s.md", time.Now().Format(salvageStamp), slugify(question))
	path := filepath.Join(dir, name)

	var b strings.Builder
	fmt.Fprintf(&b, "# Raw findings — %s\n\n", question)
	fmt.Fprintf(&b, "Saved automatically at %s, before any model was asked to assemble them.\n\n",
		time.Now().Format("2006-01-02 15:04:05"))
	b.WriteString("**This is not the finished assessment.** It is every lane's graded report, exactly as\n")
	b.WriteString("returned, so that the work survives even if the write-up step fails — which is the\n")
	b.WriteString("normal outcome of an interrupted run on a slow link. To turn it into a dossier, run\n")
	b.WriteString("`/osint --recover` (optionally after `/model` to pick a model with a larger window).\n\n")
	if doctrine == "dossier" {
		b.WriteString("Grades are two-axis: a letter for SOURCE reliability (A-F) and a digit for\n")
		b.WriteString("INFORMATION credibility (1-6). They travel with each claim and must not be\n")
		b.WriteString("altered when the dossier is written.\n\n")
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

	if err := os.WriteFile(path, []byte(b.String()), 0o644); err != nil {
		logging.Error("could not save raw findings", "path", path, "error", err)
		return ""
	}
	return path
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
