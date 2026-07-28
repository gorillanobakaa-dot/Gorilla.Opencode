// GORILLA OVERRIDE: this file did not exist upstream.
//
// It prints the session's identity ONCE, into the terminal's own scrollback, the
// way agy and Gemini CLI announce themselves when they start.
//
// It exists because that information used to live in the right-hand sidebar, and
// the sidebar is not drawn when the conversation lives in the scrollback — so the
// program's name, its version and the folder it was pointed at simply vanished.
// Nothing was broken; there was nowhere left for them to be. Searching the frame
// for them, including at a smaller font, finds nothing, which is exactly what was
// reported.
//
// Printed rather than kept in the footer because it never changes. A fixed footer
// is expensive real estate redrawn on every keystroke, and an identity banner has
// no reason to be redrawn at all — printing it once puts it at the top of the
// session, where it is also copied out with everything else.
package chat

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/opencode-ai/opencode/internal/config"
	"github.com/opencode-ai/opencode/internal/llm/models"
	"github.com/opencode-ai/opencode/internal/tui/theme"
	"github.com/opencode-ai/opencode/internal/version"
)

// SessionBanner is the block printed at the start of a session: what this program
// is, which version, where it is pointed, and which model will answer.
//
// Returns "" when the width is unknown, for the same reason nothing else prints
// then: the terminal would wrap it at a width we did not choose, and printed
// output cannot be withdrawn.
func SessionBanner(width int) string {
	if width <= 0 {
		return ""
	}
	t := theme.CurrentTheme()
	name := lipgloss.NewStyle().Bold(true).Foreground(t.Primary())
	dim := lipgloss.NewStyle().Foreground(t.TextMuted())

	lines := []string{
		name.Render("Gorilla OpenCode") + dim.Render("  "+version.Version),
		dim.Render("https://github.com/gorillanobakaa-dot/Gorilla.Opencode"),
	}

	if cfg := config.Get(); cfg != nil {
		if cfg.WorkingDir != "" {
			lines = append(lines, dim.Render("folder  ")+lipgloss.NewStyle().Foreground(t.Text()).Render(cfg.WorkingDir))
		}
		if agent, ok := cfg.Agents[config.AgentCoder]; ok {
			label := string(agent.Model)
			if m, ok := models.SupportedModels[agent.Model]; ok && m.Name != "" {
				label = m.Name
			}
			lines = append(lines, dim.Render("model   ")+lipgloss.NewStyle().Foreground(t.Text()).Render(label))
		}
	}
	// One plain sentence about the thing that is genuinely new, because a user who
	// upgraded has no other way to discover it.
	lines = append(lines, "", dim.Render("This conversation is ordinary terminal output: scroll it, select it, copy it."))

	out := make([]string, 0, len(lines))
	for _, l := range lines {
		out = append(out, truncateToWidth(l, width))
	}
	return strings.Join(out, "\n")
}

func truncateToWidth(s string, width int) string {
	if lipgloss.Width(s) <= width {
		return s
	}
	return fmt.Sprintf("%s", lipgloss.NewStyle().MaxWidth(width).Render(s))
}
