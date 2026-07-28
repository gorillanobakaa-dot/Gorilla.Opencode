// GORILLA OVERRIDE: this file did not exist upstream.
//
// It renders the sidebar's information as two horizontal lines, for the footer
// used when the conversation lives in the terminal's scrollback and there is no
// right-hand panel to put it in.
//
// It is a second RENDERING of sidebarCmp rather than a second component, because
// that component already gathers everything — including the asynchronous
// modified-files scan against the history service. A parallel component would have
// to repeat that gathering, and would drift from it.
//
// Why horizontal. The sidebar stacks eight labelled sections vertically and runs
// to twenty-odd rows. The footer is redrawn in place, and outside the alternate
// screen bubbletea erases its previous frame by counting logical lines — so a tall
// footer does not merely look wrong, it makes every later erase land in the wrong
// place. Two rows of "key: value · key: value" carries the same facts within that
// budget.
package chat

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/opencode-ai/opencode/internal/config"
	"github.com/opencode-ai/opencode/internal/llm/models"
	"github.com/opencode-ai/opencode/internal/tui/styles"
	"github.com/opencode-ai/opencode/internal/tui/theme"
	"github.com/opencode-ai/opencode/internal/version"
)

// FooterInfo is the sidebar seen from outside the package, reduced to its compact
// rendering. The page holds one of these when there is no right-hand panel to put
// the sidebar in.
type FooterInfo interface {
	CompactView(width int) string
}

// footerInfoRows is how many rows CompactView may occupy. Asserted by a test
// rather than trusted: the footer's total height is what keeps the inline
// renderer's arithmetic valid.
const footerInfoRows = 2

// separator between fields on a line. A middle dot rather than a pipe because it
// reads as a gap instead of a border, and the footer already has enough furniture.
const fieldSep = "  ·  "

// CompactView renders the session's state as at most footerInfoRows lines, wrapped
// to width. It returns "" if there is nothing worth showing, so the footer does not
// reserve rows for an empty session.
func (m *sidebarCmp) CompactView(width int) string {
	if width <= 0 {
		return ""
	}
	t := theme.CurrentTheme()
	base := styles.BaseStyle()
	key := base.Foreground(t.TextMuted())
	val := base.Foreground(t.Text())

	pair := func(k, v string) string {
		if v == "" {
			return ""
		}
		return key.Render(k+" ") + val.Render(v)
	}

	name := lipgloss.NewStyle().Foreground(t.TextMuted()).
		Render("Gorilla OpenCode " + version.Version)
	first := joinWithTrailer(base, width, name,
		pair("model", m.modelName()),
		pair("in", m.folderName()),
		pair("context", m.contextSummary()),
		pair("spent", fmt.Sprintf("$%.2f", m.session.Cost)),
	)
	second := join(base, width,
		pair("tokens", fmt.Sprintf("%s in / %s out",
			formatTokens(m.session.PromptTokens), formatTokens(m.session.CompletionTokens))),
		pair("mcp", m.mcpSummary()),
		pair("lsp", m.lspSummary()),
		pair("changed", m.changedSummary()),
	)

	lines := make([]string, 0, footerInfoRows)
	for _, l := range []string{first, second} {
		if strings.TrimSpace(l) != "" {
			lines = append(lines, l)
		}
	}
	if len(lines) == 0 {
		return ""
	}
	return lipgloss.JoinVertical(lipgloss.Left, lines...)
}

// join lays fields out on one line, dropping empties, truncating to width, and
// painting the WHOLE line — separators and trailing space included.
//
// Two things here are load-bearing, both learned from the same screenshot.
//
// The separators are rendered through the style rather than concatenated raw. An
// unstyled separator inherits the terminal's own background, and outside the
// alternate screen that is not a subtle shade difference — it is a hole. The
// measured symptom was a background break at column 19 of a 100-column line, which
// on screen reads as black rectangles punched through a coloured bar.
//
// The line is then padded to the full width by the same style, because a row that
// stops at its last character leaves the rest of the row unpainted for exactly the
// same reason.
//
// Truncation rather than wrapping is the third: a wrapped line silently becomes two
// rows, which is how a two-row footer turns into a three-row one and breaks the
// height guarantee this file is built around.
func join(base lipgloss.Style, width int, fields ...string) string {
	kept := make([]string, 0, len(fields))
	for _, f := range fields {
		if strings.TrimSpace(f) != "" {
			kept = append(kept, f)
		}
	}
	if len(kept) == 0 {
		return ""
	}
	line := strings.Join(kept, base.Render(fieldSep))
	if width <= 0 {
		return line // unpadded: the caller is composing a wider line itself
	}
	return base.Width(width).MaxWidth(width).Render(ansi.Truncate(line, width, "…"))
}

// joinWithTrailer lays fields out on the left and pins one field to the RIGHT edge,
// filling the gap between them.
//
// The footer's right-hand side was empty, and the program's name and version had
// nowhere to live once the sidebar stopped being drawn. If the two would collide the
// trailer is dropped rather than truncated: half a version string is worse than
// none, and the left-hand fields are the ones that change.
func joinWithTrailer(base lipgloss.Style, width int, trailer string, fields ...string) string {
	left := join(base, 0, fields...)
	if trailer == "" {
		return base.Width(width).MaxWidth(width).Render(ansi.Truncate(left, width, "…"))
	}
	gap := width - lipgloss.Width(left) - lipgloss.Width(trailer)
	if gap < 2 {
		return base.Width(width).MaxWidth(width).Render(ansi.Truncate(left, width, "…"))
	}
	return base.MaxWidth(width).Render(left + base.Render(strings.Repeat(" ", gap)) + trailer)
}

// modelName is the model in use, by its display name rather than its ID.
func (m *sidebarCmp) modelName() string {
	cfg := config.Get()
	if cfg == nil {
		return ""
	}
	agent, ok := cfg.Agents[config.AgentCoder]
	if !ok {
		return ""
	}
	if mdl, ok := models.SupportedModels[agent.Model]; ok && mdl.Name != "" {
		return mdl.Name
	}
	return string(agent.Model)
}

// folderName is the working directory's last component. The full path is what the
// startup picker printed into the scrollback; repeating it on every frame would
// spend most of the line on a prefix the user already knows.
func (m *sidebarCmp) folderName() string {
	cfg := config.Get()
	if cfg == nil || cfg.WorkingDir == "" {
		return ""
	}
	if base := cfg.WorkingDir[strings.LastIndex(cfg.WorkingDir, "/")+1:]; base != "" {
		return base
	}
	return cfg.WorkingDir
}

// contextSummary is the token count and, when the window is known, the percentage
// of it used — the two numbers that tell someone whether they are about to be
// summarised.
func (m *sidebarCmp) contextSummary() string {
	used := m.session.PromptTokens + m.session.CompletionTokens

	var window int64
	if cfg := config.Get(); cfg != nil {
		if a, ok := cfg.Agents[config.AgentCoder]; ok {
			if mdl, ok := models.SupportedModels[a.Model]; ok {
				window = mdl.ContextWindow
			}
		}
	}
	if window <= 0 {
		return fmt.Sprintf("%s tokens", formatTokens(used))
	}
	return fmt.Sprintf("%s (%.0f%%)", formatTokens(used), float64(used)/float64(window)*100)
}

func (m *sidebarCmp) mcpSummary() string {
	cfg := config.Get()
	if cfg == nil || len(cfg.MCPServers) == 0 {
		return "none"
	}
	return fmt.Sprintf("%d", len(cfg.MCPServers))
}

// lspSummary counts what is RUNNING against what is configured, matching the
// sidebar's rule that a server switched off must not be reported as present.
func (m *sidebarCmp) lspSummary() string {
	cfg := config.Get()
	if cfg == nil || len(cfg.LSP) == 0 {
		return "none"
	}
	on := 0
	for name := range cfg.LSP {
		if config.LSPEnabled(name) {
			on++
		}
	}
	if on == 0 {
		return fmt.Sprintf("all %d off", len(cfg.LSP))
	}
	return fmt.Sprintf("%d of %d on", on, len(cfg.LSP))
}

func (m *sidebarCmp) changedSummary() string {
	if len(m.modFiles) == 0 {
		return "no files"
	}
	adds, dels := 0, 0
	for _, c := range m.modFiles {
		adds += c.additions
		dels += c.removals
	}
	return fmt.Sprintf("%d files +%d/-%d", len(m.modFiles), adds, dels)
}
