package chat

import (
	"fmt"
	"sort"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/opencode-ai/opencode/internal/config"
	"github.com/opencode-ai/opencode/internal/message"
	"github.com/opencode-ai/opencode/internal/session"
	"github.com/opencode-ai/opencode/internal/tui/styles"
	"github.com/opencode-ai/opencode/internal/tui/theme"
	"github.com/opencode-ai/opencode/internal/version"
)

type SendMsg struct {
	Text        string
	Attachments []message.Attachment
}

// GORILLA OVERRIDE: emitted when the user types a slash command in the
// editor (e.g. /model, /models, /export). Handled centrally in the TUI.
type SlashCommandMsg struct {
	Name string
	// GORILLA OVERRIDE: everything after the command word, untouched. Needed so
	// `/cd /path/to/project` can narrow the workspace in one step instead of
	// opening a dialog — which is the whole point of the command: pointing the
	// agent at ONE directory instead of a home directory holding millions of
	// files across a kernel tree and a browser tree.
	Args string
}

// GORILLA OVERRIDE: request a fresh session. Handled by the chat PAGE so
// it resets the page's own session and clears the sidebar — /clear used
// to only clear the message list, leaving the editor in a broken state.
type NewSessionMsg struct{}

type SessionSelectedMsg = session.Session

type SessionClearedMsg struct{}

type EditorFocusMsg bool

// GORILLA OVERRIDE: header/logo/repo/cwd/lspsConfigured take a base style so
// the caller decides the background — the sidebar renders them on the panel
// color (BackgroundSecondary), the main-area welcome on the normal background.
func header(width int, base lipgloss.Style) string {
	return lipgloss.JoinVertical(
		lipgloss.Top,
		logo(width, base),
		repo(width, base),
		base.Width(width).Render(""),
		cwd(width, base),
	)
}

func lspsConfigured(width int, base lipgloss.Style) string {
	cfg := config.Get()
	title := "LSP Configuration"
	title = ansi.Truncate(title, width, "...")

	t := theme.CurrentTheme()
	baseStyle := base

	lsps := baseStyle.
		Width(width).
		Foreground(t.Primary()).
		Bold(true).
		Render(title)

	// GORILLA OVERRIDE: list only the servers actually RUNNING, and say how many
	// are switched off.
	//
	// This iterated cfg.LSP raw, so a server disabled in /context still appeared
	// here exactly as before — which reads as "your toggle did nothing". It did
	// work (with all nine off, zero language-server processes spawn; with them on,
	// clangd, gopls and five node servers do), but a panel that keeps listing them
	// is indistinguishable from a broken switch.
	var lspNames, disabled []string
	for name := range cfg.LSP {
		if config.LSPEnabled(name) {
			lspNames = append(lspNames, name)
		} else {
			disabled = append(disabled, name)
		}
	}
	sort.Strings(lspNames)
	sort.Strings(disabled)

	var lspViews []string
	for _, name := range lspNames {
		lsp := cfg.LSP[name]
		lspName := baseStyle.
			Foreground(t.Text()).
			Render(fmt.Sprintf("* %s", name))

		cmd := lsp.Command
		cmd = ansi.Truncate(cmd, width-lipgloss.Width(lspName)-3, "...")

		lspPath := baseStyle.
			Foreground(t.TextMuted()).
			Render(fmt.Sprintf(" (%s)", cmd))

		lspViews = append(lspViews,
			baseStyle.
				Width(width).
				Render(
					lipgloss.JoinHorizontal(
						lipgloss.Left,
						lspName,
						lspPath,
					),
				),
		)
	}

	// Say what is off, so the panel is a complete picture rather than a filtered
	// one. Silence here would make a fully-disabled setup look unconfigured.
	if len(disabled) > 0 {
		note := fmt.Sprintf("%d off (/context to change)", len(disabled))
		if len(lspNames) == 0 {
			note = fmt.Sprintf("all %d off (/context to change)", len(disabled))
		}
		lspViews = append(lspViews, baseStyle.
			Width(width).
			Foreground(t.TextMuted()).
			Render(ansi.Truncate(note, width, "...")))
	}

	return baseStyle.
		Width(width).
		Render(
			lipgloss.JoinVertical(
				lipgloss.Left,
				lsps,
				lipgloss.JoinVertical(
					lipgloss.Left,
					lspViews...,
				),
			),
		)
}

func logo(width int, base lipgloss.Style) string {
	// GORILLA OVERRIDE: user-facing product name. The Go module path
	// stays github.com/opencode-ai/opencode for provenance; only the
	// displayed branding changes.
	logo := fmt.Sprintf("%s %s", styles.OpenCodeIcon, "Gorilla OpenCode")
	t := theme.CurrentTheme()
	baseStyle := base

	versionText := baseStyle.
		Foreground(t.TextMuted()).
		Render(version.Version)

	return baseStyle.
		Bold(true).
		Width(width).
		Render(
			lipgloss.JoinHorizontal(
				lipgloss.Left,
				logo,
				" ",
				versionText,
			),
		)
}

func repo(width int, base lipgloss.Style) string {
	// GORILLA OVERRIDE: point users at the revival repo, not upstream.
	repo := "https://github.com/gorillanobakaa-dot/Gorilla.Opencode"
	t := theme.CurrentTheme()

	return base.
		Foreground(t.TextMuted()).
		Width(width).
		Render(repo)
}

func cwd(width int, base lipgloss.Style) string {
	cwd := fmt.Sprintf("cwd: %s", config.WorkingDirectory())
	t := theme.CurrentTheme()

	return base.
		Foreground(t.TextMuted()).
		Width(width).
		Render(cwd)
}
