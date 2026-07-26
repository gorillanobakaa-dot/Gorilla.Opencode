package chat

import (
	"context"
	"fmt"
	"sort"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/opencode-ai/opencode/internal/config"
	"github.com/opencode-ai/opencode/internal/diff"
	"github.com/opencode-ai/opencode/internal/history"
	"github.com/opencode-ai/opencode/internal/llm/models"
	"github.com/opencode-ai/opencode/internal/pubsub"
	"github.com/opencode-ai/opencode/internal/session"
	"github.com/opencode-ai/opencode/internal/tui/theme"
)

type sidebarCmp struct {
	width, height int
	session       session.Session
	history       history.Service
	modFiles      map[string]struct {
		additions int
		removals  int
	}
}

func (m *sidebarCmp) Init() tea.Cmd {
	if m.history != nil {
		ctx := context.Background()
		// Subscribe to file events
		filesCh := m.history.Subscribe(ctx)

		// Initialize the modified files map
		m.modFiles = make(map[string]struct {
			additions int
			removals  int
		})

		// Load initial files and calculate diffs
		m.loadModifiedFiles(ctx)

		// Return a command that will send file events to the Update method
		return func() tea.Msg {
			return <-filesCh
		}
	}
	return nil
}

func (m *sidebarCmp) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case SessionSelectedMsg:
		if msg.ID != m.session.ID {
			m.session = msg
			ctx := context.Background()
			m.loadModifiedFiles(ctx)
		}
	case pubsub.Event[session.Session]:
		if msg.Type == pubsub.UpdatedEvent {
			if m.session.ID == msg.Payload.ID {
				m.session = msg.Payload
			}
		}
	case pubsub.Event[history.File]:
		if msg.Payload.SessionID == m.session.ID {
			// Process the individual file change instead of reloading all files
			ctx := context.Background()
			m.processFileChanges(ctx, msg.Payload)

			// Return a command to continue receiving events
			return m, func() tea.Msg {
				ctx := context.Background()
				filesCh := m.history.Subscribe(ctx)
				return <-filesCh
			}
		}
	}
	return m, nil
}

// sidebarStyle is the panel style: a distinct (lighter) background so the
// sidebar reads as its own panel instead of blending into the chat. Uses the
// theme's BackgroundSecondary, so it adapts across themes.
func sidebarStyle() lipgloss.Style {
	t := theme.CurrentTheme()
	return lipgloss.NewStyle().
		Background(t.BackgroundSecondary()).
		Foreground(t.Text())
}

func (m *sidebarCmp) View() string {
	baseStyle := sidebarStyle()
	spacer := baseStyle.Width(m.width).Render("")

	return baseStyle.
		Width(m.width).
		PaddingLeft(4).
		PaddingRight(2).
		Height(m.height).
		Render(
			lipgloss.JoinVertical(
				lipgloss.Top,
				header(m.width, baseStyle),
				spacer,
				m.sessionSection(),
				spacer,
				m.contextSection(),
				spacer,
				m.tokenUsageSection(),
				spacer,
				m.mcpSection(),
				spacer,
				lspsConfigured(m.width, baseStyle),
				spacer,
				m.modifiedFiles(),
			),
		)
}

func (m *sidebarCmp) sessionSection() string {
	t := theme.CurrentTheme()
	baseStyle := sidebarStyle()

	sessionKey := baseStyle.
		Foreground(t.Primary()).
		Bold(true).
		Render("Session")

	sessionValue := baseStyle.
		Foreground(t.Text()).
		Width(m.width - lipgloss.Width(sessionKey)).
		Render(fmt.Sprintf(": %s", m.session.Title))

	return lipgloss.JoinHorizontal(
		lipgloss.Left,
		sessionKey,
		sessionValue,
	)
}

// formatTokens renders a token count compactly (4500 -> "4.5K", 1200000 -> "1.2M").
func formatTokens(n int64) string {
	switch {
	case n >= 1_000_000:
		return fmt.Sprintf("%.1fM", float64(n)/1_000_000)
	case n >= 1_000:
		return fmt.Sprintf("%.1fK", float64(n)/1_000)
	default:
		return fmt.Sprintf("%d", n)
	}
}

// contextSection shows the session's total context size, how much of the
// model's window it fills, and the running cost — like OpenCode/Kilo.
func (m *sidebarCmp) contextSection() string {
	t := theme.CurrentTheme()
	baseStyle := sidebarStyle()

	used := m.session.PromptTokens + m.session.CompletionTokens

	var ctxWindow int64
	if cfg := config.Get(); cfg != nil {
		if a, ok := cfg.Agents[config.AgentCoder]; ok {
			if mdl, ok := models.SupportedModels[a.Model]; ok {
				ctxWindow = mdl.ContextWindow
			}
		}
	}

	lines := []string{
		baseStyle.Foreground(t.Primary()).Bold(true).Width(m.width).Render("Context"),
		baseStyle.Foreground(t.Text()).Width(m.width).Render(fmt.Sprintf("%s tokens", formatTokens(used))),
	}
	if ctxWindow > 0 {
		pct := float64(used) / float64(ctxWindow) * 100
		lines = append(lines, baseStyle.Foreground(t.TextMuted()).Width(m.width).Render(fmt.Sprintf("%.0f%% used", pct)))
	}
	lines = append(lines, baseStyle.Foreground(t.TextMuted()).Width(m.width).Render(fmt.Sprintf("$%.2f spent", m.session.Cost)))

	return lipgloss.JoinVertical(lipgloss.Left, lines...)
}

// tokenUsageSection breaks the session tokens into input (prompt) and output
// (completion). Cache metrics aren't stored per-session, so they're omitted.
func (m *sidebarCmp) tokenUsageSection() string {
	t := theme.CurrentTheme()
	baseStyle := sidebarStyle()

	row := func(label string, v int64) string {
		return baseStyle.Foreground(t.Text()).Width(m.width).Render(
			fmt.Sprintf("%-8s%s", label, formatTokens(v)))
	}

	return lipgloss.JoinVertical(lipgloss.Left,
		baseStyle.Foreground(t.Primary()).Bold(true).Width(m.width).Render("Token Usage"),
		row("Input", m.session.PromptTokens),
		row("Output", m.session.CompletionTokens),
	)
}

// mcpSection lists configured MCP servers. Live connection status isn't
// available to the sidebar, so we list what's configured rather than fake it.
func (m *sidebarCmp) mcpSection() string {
	t := theme.CurrentTheme()
	baseStyle := sidebarStyle()

	var servers []string
	if cfg := config.Get(); cfg != nil {
		for name := range cfg.MCPServers {
			servers = append(servers, name)
		}
	}
	sort.Strings(servers)

	lines := []string{baseStyle.Foreground(t.Primary()).Bold(true).Width(m.width).Render("MCP")}
	if len(servers) == 0 {
		lines = append(lines, baseStyle.Foreground(t.TextMuted()).Width(m.width).Render("No MCP servers"))
	} else {
		for _, s := range servers {
			lines = append(lines, baseStyle.Foreground(t.Text()).Width(m.width).Render("• "+s))
		}
	}

	return lipgloss.JoinVertical(lipgloss.Left, lines...)
}

func (m *sidebarCmp) modifiedFile(filePath string, additions, removals int) string {
	t := theme.CurrentTheme()
	baseStyle := sidebarStyle()

	stats := ""
	if additions > 0 && removals > 0 {
		additionsStr := baseStyle.
			Foreground(t.Success()).
			PaddingLeft(1).
			Render(fmt.Sprintf("+%d", additions))

		removalsStr := baseStyle.
			Foreground(t.Error()).
			PaddingLeft(1).
			Render(fmt.Sprintf("-%d", removals))

		content := lipgloss.JoinHorizontal(lipgloss.Left, additionsStr, removalsStr)
		stats = baseStyle.Width(lipgloss.Width(content)).Render(content)
	} else if additions > 0 {
		additionsStr := fmt.Sprintf(" %s", baseStyle.
			PaddingLeft(1).
			Foreground(t.Success()).
			Render(fmt.Sprintf("+%d", additions)))
		stats = baseStyle.Width(lipgloss.Width(additionsStr)).Render(additionsStr)
	} else if removals > 0 {
		removalsStr := fmt.Sprintf(" %s", baseStyle.
			PaddingLeft(1).
			Foreground(t.Error()).
			Render(fmt.Sprintf("-%d", removals)))
		stats = baseStyle.Width(lipgloss.Width(removalsStr)).Render(removalsStr)
	}

	filePathStr := baseStyle.Render(filePath)

	return baseStyle.
		Width(m.width).
		Render(
			lipgloss.JoinHorizontal(
				lipgloss.Left,
				filePathStr,
				stats,
			),
		)
}

func (m *sidebarCmp) modifiedFiles() string {
	t := theme.CurrentTheme()
	baseStyle := sidebarStyle()

	modifiedFiles := baseStyle.
		Width(m.width).
		Foreground(t.Primary()).
		Bold(true).
		Render("Modified Files:")

	// If no modified files, show a placeholder message
	if m.modFiles == nil || len(m.modFiles) == 0 {
		message := "No modified files"
		remainingWidth := m.width - lipgloss.Width(message)
		if remainingWidth > 0 {
			message += strings.Repeat(" ", remainingWidth)
		}
		return baseStyle.
			Width(m.width).
			Render(
				lipgloss.JoinVertical(
					lipgloss.Top,
					modifiedFiles,
					baseStyle.Foreground(t.TextMuted()).Render(message),
				),
			)
	}

	// Sort file paths alphabetically for consistent ordering
	var paths []string
	for path := range m.modFiles {
		paths = append(paths, path)
	}
	sort.Strings(paths)

	// Create views for each file in sorted order
	var fileViews []string
	for _, path := range paths {
		stats := m.modFiles[path]
		fileViews = append(fileViews, m.modifiedFile(path, stats.additions, stats.removals))
	}

	return baseStyle.
		Width(m.width).
		Render(
			lipgloss.JoinVertical(
				lipgloss.Top,
				modifiedFiles,
				lipgloss.JoinVertical(
					lipgloss.Left,
					fileViews...,
				),
			),
		)
}

func (m *sidebarCmp) SetSize(width, height int) tea.Cmd {
	m.width = width
	m.height = height
	return nil
}

func (m *sidebarCmp) GetSize() (int, int) {
	return m.width, m.height
}

func NewSidebarCmp(session session.Session, history history.Service) tea.Model {
	return &sidebarCmp{
		session: session,
		history: history,
	}
}

func (m *sidebarCmp) loadModifiedFiles(ctx context.Context) {
	if m.history == nil || m.session.ID == "" {
		return
	}

	// Get all latest files for this session
	latestFiles, err := m.history.ListLatestSessionFiles(ctx, m.session.ID)
	if err != nil {
		return
	}

	// Get all files for this session (to find initial versions)
	allFiles, err := m.history.ListBySession(ctx, m.session.ID)
	if err != nil {
		return
	}

	// Clear the existing map to rebuild it
	m.modFiles = make(map[string]struct {
		additions int
		removals  int
	})

	// Process each latest file
	for _, file := range latestFiles {
		// Skip if this is the initial version (no changes to show)
		if file.Version == history.InitialVersion {
			continue
		}

		// Find the initial version for this specific file
		var initialVersion history.File
		for _, v := range allFiles {
			if v.Path == file.Path && v.Version == history.InitialVersion {
				initialVersion = v
				break
			}
		}

		// Skip if we can't find the initial version
		if initialVersion.ID == "" {
			continue
		}
		if initialVersion.Content == file.Content {
			continue
		}

		// Calculate diff between initial and latest version
		_, additions, removals := diff.GenerateDiff(initialVersion.Content, file.Content, file.Path)

		// Only add to modified files if there are changes
		if additions > 0 || removals > 0 {
			// Remove working directory prefix from file path
			displayPath := file.Path
			workingDir := config.WorkingDirectory()
			displayPath = strings.TrimPrefix(displayPath, workingDir)
			displayPath = strings.TrimPrefix(displayPath, "/")

			m.modFiles[displayPath] = struct {
				additions int
				removals  int
			}{
				additions: additions,
				removals:  removals,
			}
		}
	}
}

func (m *sidebarCmp) processFileChanges(ctx context.Context, file history.File) {
	// Skip if this is the initial version (no changes to show)
	if file.Version == history.InitialVersion {
		return
	}

	// Find the initial version for this file
	initialVersion, err := m.findInitialVersion(ctx, file.Path)
	if err != nil || initialVersion.ID == "" {
		return
	}

	// Skip if content hasn't changed
	if initialVersion.Content == file.Content {
		// If this file was previously modified but now matches the initial version,
		// remove it from the modified files list
		displayPath := getDisplayPath(file.Path)
		delete(m.modFiles, displayPath)
		return
	}

	// Calculate diff between initial and latest version
	_, additions, removals := diff.GenerateDiff(initialVersion.Content, file.Content, file.Path)

	// Only add to modified files if there are changes
	if additions > 0 || removals > 0 {
		displayPath := getDisplayPath(file.Path)
		m.modFiles[displayPath] = struct {
			additions int
			removals  int
		}{
			additions: additions,
			removals:  removals,
		}
	} else {
		// If no changes, remove from modified files
		displayPath := getDisplayPath(file.Path)
		delete(m.modFiles, displayPath)
	}
}

// Helper function to find the initial version of a file
func (m *sidebarCmp) findInitialVersion(ctx context.Context, path string) (history.File, error) {
	// Get all versions of this file for the session
	fileVersions, err := m.history.ListBySession(ctx, m.session.ID)
	if err != nil {
		return history.File{}, err
	}

	// Find the initial version
	for _, v := range fileVersions {
		if v.Path == path && v.Version == history.InitialVersion {
			return v, nil
		}
	}

	return history.File{}, fmt.Errorf("initial version not found")
}

// Helper function to get the display path for a file
func getDisplayPath(path string) string {
	workingDir := config.WorkingDirectory()
	displayPath := strings.TrimPrefix(path, workingDir)
	return strings.TrimPrefix(displayPath, "/")
}
