// GORILLA OVERRIDE: this file did not exist upstream. It implements the
// `/export` slash command: write the active session's transcript to a file.
//
// It used to build the Markdown here, inline, and that renderer had three gaps
// that made an exported session close to useless for working out what had
// happened — no timestamps, no tool results, and no reasoning. The rendering now
// lives in internal/export as a pure function over stored data, so it can be
// tested against real sessions without a terminal. This file is only plumbing:
// gather the session, ask where to put it, write the bytes.
package tui

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/atotto/clipboard"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/opencode-ai/opencode/internal/config"
	"github.com/opencode-ai/opencode/internal/export"
	"github.com/opencode-ai/opencode/internal/tui/util"
)

// openExportDialog seeds the destination fields and shows the prompt. The
// defaults are the old behaviour — working directory, generated name — so the
// fastest path is still enter, but every part of it can now be changed.
func (a *appModel) openExportDialog() tea.Cmd {
	if a.selectedSession.ID == "" {
		return util.ReportWarn("No active session to export")
	}
	a.exportDialog.SetDefaults(
		config.WorkingDirectory(),
		suggestedExportName(a.selectedSession.Title, time.Now()),
	)
	a.exportDialog.Init()
	a.showExportDialog = true
	return nil
}

// writeExport renders the session and writes it to the chosen destination.
func (a *appModel) writeExport(dir, name string) tea.Cmd {
	sess := a.selectedSession
	if sess.ID == "" {
		return util.ReportWarn("No active session to export")
	}

	msgs, err := a.app.Messages.List(context.Background(), sess.ID)
	if err != nil {
		return util.ReportError(err)
	}

	dst := filepath.Join(dir, name)

	// Refuse to clobber. An export is a record; silently overwriting one is
	// exactly the kind of quiet data loss this change exists to end.
	if _, err := os.Stat(dst); err == nil {
		return util.ReportWarn(fmt.Sprintf("%s already exists — choose another name", dst))
	}

	body := export.Render(sess, msgs, time.Now())

	// 0o600: a transcript holds whatever was discussed, including file contents
	// and command output. It is not world-readable by default.
	if err := os.WriteFile(dst, []byte(body), 0o600); err != nil {
		return util.ReportError(err)
	}
	return util.ReportInfo(fmt.Sprintf("Exported %d messages to %s", len(msgs), dst))
}

// suggestedExportName delegates to the export package, where the naming now
// lives so the sessions manager can use it for a session that is not on screen.
func suggestedExportName(title string, now time.Time) string {
	return export.SuggestedName(title, now)
}

// copyTranscript puts the whole conversation on the system clipboard.
//
// GORILLA FIX (2026-09-01), from the owner: "i can not CTRL A select and copy
// the way i can do on linux".
//
// The reason is structural, not a bug we introduced. On Linux the terminal
// EMULATOR is a separate process and intercepts Ctrl+Shift+A / Ctrl+Shift+C
// before this program ever sees them. On Windows conhost hands raw input
// straight to the TUI, so Ctrl+A is DELIVERED HERE — no console setting can
// change that while a full-screen program is reading input. Select-all on
// Windows therefore has to be implemented by the application or not exist.
//
// Doing it in the application is the better answer on both platforms anyway: it
// copies the TRANSCRIPT rather than whatever rectangle of glyphs happens to be
// on screen, so it survives scrolling, wrapping and the parts already scrolled
// out of the terminal's buffer entirely.
//
// Same renderer as /export, so what you paste is what the file would have
// contained: timestamps, models, reasoning, and every tool call with its result.
func (a *appModel) copyTranscript() tea.Cmd {
	sess := a.selectedSession
	if sess.ID == "" {
		return util.ReportWarn("Nothing to copy yet — this conversation has not started.")
	}

	msgs, err := a.app.Messages.List(context.Background(), sess.ID)
	if err != nil {
		return util.ReportError(err)
	}
	if len(msgs) == 0 {
		return util.ReportWarn("Nothing to copy yet — this conversation has no messages.")
	}

	text := export.Render(sess, msgs, time.Now())
	if err := clipboard.WriteAll(text); err != nil {
		// The usual cause on Linux is a headless box with no clipboard tool.
		// Say which programs would fix it rather than reporting the raw error,
		// which is typically just "exec: xclip: executable file not found".
		return util.ReportError(fmt.Errorf(
			"could not reach the clipboard (%w). On Linux this needs xclip or "+
				"xsel installed; over SSH there may be no clipboard at all. "+
				"/export writes the same text to a file instead", err))
	}

	return util.ReportInfo(fmt.Sprintf(
		"Copied the whole conversation to the clipboard — %d messages, %d characters.",
		len(msgs), len([]rune(text))))
}
