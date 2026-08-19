// GORILLA OVERRIDE (2026-08-18): the plumbing behind /sessions.
//
// Two operations live here rather than in the dialog, because both need the
// stores and the dialog must not: gathering the list, and erasing a session for
// real. "For real" is the load-bearing word — see eraseSession.
package tui

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/opencode-ai/opencode/internal/config"
	"github.com/opencode-ai/opencode/internal/export"
	"github.com/opencode-ai/opencode/internal/session"
	"github.com/opencode-ai/opencode/internal/tui/components/chat"
	"github.com/opencode-ai/opencode/internal/tui/components/dialog"
	"github.com/opencode-ai/opencode/internal/tui/util"
)

// openSessionsManager gathers every conversation with what it costs on disk.
//
// Sizes come from ONE query for the whole list rather than one per row: on the
// eMMC and CompactFlash devices this is built for, a query per session is felt
// when opening a screen.
func (a *appModel) openSessionsManager() tea.Cmd {
	ctx := context.Background()

	sessions, err := a.app.Sessions.List(ctx)
	if err != nil {
		return util.ReportError(err)
	}
	if len(sessions) == 0 {
		return util.ReportWarn("No past sessions yet.")
	}

	per, totals, err := a.app.Sessions.Storage(ctx)
	if err != nil {
		// A failed size query must not withhold the list. Someone looking for a
		// conversation after a power cut needs the conversation; the sizes are
		// what they need when deciding what to delete, which is a later problem.
		per = nil
	}

	rows := make([]dialog.SessionRow, 0, len(sessions))
	for _, s := range sessions {
		st := per[s.ID]
		rows = append(rows, dialog.SessionRow{Session: s, Bytes: st.Bytes, Msgs: st.Messages})
	}

	a.sessionsMgr.SetStore(dialog.SessionsStore{
		Rows:       rows,
		TotalBytes: totals.Bytes,
		FileBytes:  storeFileBytes(),
		Sessions:   totals.Sessions,
		Helpers:    totals.Helpers,
		Msg:        totals.Messages,
	}, a.selectedSession.ID)
	a.sessionsMgr.SetSize(a.width, a.height)
	a.showSessionsMgr = true
	return nil
}

// storeFileBytes is what the database actually occupies, write-ahead log
// included.
//
// This is deliberately NOT the sum of the message bodies. Measured on the
// developer's machine, 2026-08-18: 4.77 MB of message parts, a 5.4 MB database
// file, and a 4.3 MB WAL that nothing had ever truncated — 9.8 MB in total. The
// number someone compares against their free space is the one the filesystem
// reports, not the one the content adds up to.
func storeFileBytes() int64 {
	dir := config.Get().Data.Directory
	if dir == "" {
		return 0
	}
	base := filepath.Join(dir, "gorilla-opencode.db")
	total := int64(0)
	for _, p := range []string{base, base + "-wal", base + "-shm"} {
		if info, err := os.Stat(p); err == nil {
			total += info.Size()
		}
	}
	return total
}

// eraseSession deletes a conversation and its helper sessions, then returns the
// space to the disk and reports what actually came back.
//
// THE MEASUREMENT THIS IS BUILT AROUND. Deleting rows in SQLite frees pages for
// reuse; the FILE does not shrink. Proven in
// internal/db/session_storage_test.go: 1,073,152 bytes before, 1,073,152 bytes
// after deleting every message, 65,536 bytes after reclaiming. On a device with
// 1 GB free, an erase that returns nothing is not an erase, and reporting it as
// a success would be exactly the "tool reporting success having done nothing"
// this project's rules exist to catch.
//
// So the size is measured from the filesystem before and after, and the user is
// told the difference. If VACUUM cannot run — it needs temporary room roughly
// the size of the database, which is not a given on a full device — that is
// said plainly rather than swallowed.
func (a *appModel) eraseSession(sess session.Session) tea.Cmd {
	ctx := context.Background()
	before := storeFileBytes()

	removed, err := a.app.Sessions.DeleteTree(ctx, sess.ID)
	if err != nil {
		return util.ReportError(err)
	}

	reclaimErr := a.app.Sessions.Reclaim(ctx)
	after := storeFileBytes()

	// Refresh the list in place: the row is gone and every total has moved.
	if cmd := a.openSessionsManager(); cmd != nil {
		return cmd
	}

	switch {
	case reclaimErr != nil:
		a.sessionsMgr.SetNotice(fmt.Sprintf(
			"Erased %d sessions, but the space was NOT returned to the disk: %v", removed, reclaimErr))
	case after < before:
		a.sessionsMgr.SetNotice(fmt.Sprintf(
			"Erased %d sessions | %s returned to the disk", removed, humanBytes(before-after)))
	default:
		// Honest about the small case: a short conversation can be less than one
		// database page, and claiming a reclaim that did not measurably happen
		// is the failure mode this whole path exists to avoid.
		a.sessionsMgr.SetNotice(fmt.Sprintf(
			"Erased %d sessions | too small to change the file size", removed))
	}
	return nil
}

// humanBytes matches the dialog's formatting so the two never disagree about
// the same number.
func humanBytes(n int64) string {
	switch {
	case n >= 1<<20:
		return fmt.Sprintf("%.1f MB", float64(n)/(1<<20))
	case n >= 1<<10:
		return fmt.Sprintf("%.0f KB", float64(n)/(1<<10))
	default:
		return fmt.Sprintf("%d B", n)
	}
}

// resumeSession hands stalled work to the currently selected model, in a FRESH
// conversation carrying a brief instead of the whole history.
//
// WHY NOT JUST REOPEN IT. Reopening is the right answer for a short
// conversation and the wrong one for a long-running job — and a long-running
// job is exactly what gets interrupted. A real working session on this machine
// held 275 messages and 2.6 MB; putting that back into a 32K model reproduces
// the context blow-out that stopped the work in the first place. The bigger the
// session, the more certainly reopening it fails, which is backwards.
//
// The brief is built in pure Go (internal/export/handoff.go). No model is asked
// to work out what happened, because that step is precisely the one that
// already failed once.
//
// It also carries the HELPER sessions' instructions and failures where there
// are any: a research run's lanes are where the work happened, and a brief that
// reports the orchestrator's twelve messages as the whole story is describing
// the wrapper rather than the job.
func (a *appModel) resumeSession(sess session.Session) tea.Cmd {
	ctx := context.Background()

	msgs, err := a.app.Messages.List(ctx, sess.ID)
	if err != nil {
		return util.ReportError(err)
	}
	if len(msgs) == 0 {
		return util.ReportWarn("That conversation has no messages to pick up from.")
	}

	// Budget the brief against the model that will read it, leaving room for the
	// work itself. Two thirds is the same split /osint --recover uses: a prompt
	// that exactly fills the window leaves nothing to answer with.
	budgetChars := 0
	if a.app.CoderAgent != nil {
		if w := a.app.CoderAgent.Model().ContextWindow; w > 0 {
			budgetChars = int(float64(w) * 0.5 * 4) // ~4 chars per token, half the window
		}
	}

	// Helper sessions travel too — a research run's work is in its lanes, and a
	// brief built from the orchestrator alone describes the wrapper, not the job.
	var branches []export.Branch
	if kids, err := a.app.Sessions.Children(ctx, sess.ID); err == nil {
		for _, kid := range kids {
			km, err := a.app.Messages.List(ctx, kid.ID)
			if err != nil {
				continue
			}
			branches = append(branches, export.Branch{Session: kid, Messages: km})
		}
	}

	brief, stats := export.Handoff(sess, msgs, branches, budgetChars)

	// A fresh session, then the brief. Sequenced rather than batched: the chat
	// page clears its session on the first message and creates a new one on the
	// second, and the order is what makes the brief land in the new
	// conversation rather than the old one.
	a.selectedSession = session.Session{}

	note := fmt.Sprintf("Picking up %q — %d instructions, %d files touched, %d failures, ~%dK tokens",
		strings.TrimSpace(sess.Title), stats.Instructions, stats.FilesTouched,
		stats.Failures, stats.EstimatedTokens()/1000)
	if stats.Truncated {
		note += " (trimmed to fit this model — the full session is still stored)"
	}

	return tea.Sequence(
		util.CmdHandler(chat.NewSessionMsg{}),
		util.CmdHandler(chat.SendMsg{Text: brief}),
		util.ReportInfo(note),
	)
}
