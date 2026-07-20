// GORILLA OVERRIDE: this file did not exist upstream. It implements the
// `/export` slash command, mirroring the feature from the current
// OpenCode: write the active session's transcript to a Markdown file in
// the current working directory, so a conversation can be saved,
// shared, or committed alongside the work it produced.
package tui

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/opencode-ai/opencode/internal/config"
	"github.com/opencode-ai/opencode/internal/message"
	"github.com/opencode-ai/opencode/internal/tui/util"
)

func (a *appModel) exportSession() tea.Cmd {
	sess := a.selectedSession
	if sess.ID == "" {
		return util.ReportWarn("No active session to export")
	}

	msgs, err := a.app.Messages.List(context.Background(), sess.ID)
	if err != nil {
		return util.ReportError(err)
	}

	var b strings.Builder
	title := sess.Title
	if title == "" {
		title = "OpenCode Session"
	}
	fmt.Fprintf(&b, "# %s\n\n", title)
	fmt.Fprintf(&b, "*Exported %s — Gorilla OpenCode*\n\n---\n\n", time.Now().Format(time.RFC1123))

	for _, m := range msgs {
		switch m.Role {
		case message.User:
			fmt.Fprintf(&b, "## User\n\n%s\n\n", strings.TrimSpace(m.Content().String()))
		case message.Assistant:
			fmt.Fprintf(&b, "## Assistant\n\n")
			if r := strings.TrimSpace(m.ReasoningContent().String()); r != "" {
				fmt.Fprintf(&b, "> _thinking_\n>\n> %s\n\n", strings.ReplaceAll(r, "\n", "\n> "))
			}
			if c := strings.TrimSpace(m.Content().String()); c != "" {
				fmt.Fprintf(&b, "%s\n\n", c)
			}
			for _, tc := range m.ToolCalls() {
				fmt.Fprintf(&b, "**tool: %s**\n\n```json\n%s\n```\n\n", tc.Name, strings.TrimSpace(tc.Input))
			}
		}
	}

	// A short, filesystem-safe name derived from the session title.
	slug := strings.Map(func(r rune) rune {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
			return r
		case r == ' ', r == '-', r == '_':
			return '-'
		default:
			return -1
		}
	}, title)
	slug = strings.Trim(slug, "-")
	if slug == "" {
		slug = "session"
	}
	name := fmt.Sprintf("opencode-%s-%s.md", strings.ToLower(slug), time.Now().Format("20060102-150405"))
	dst := filepath.Join(config.WorkingDirectory(), name)

	if err := os.WriteFile(dst, []byte(b.String()), 0o644); err != nil {
		return util.ReportError(err)
	}
	return util.ReportInfo("Exported session to " + dst)
}
