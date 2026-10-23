package dialog

import (
	"fmt"
	"slices"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/opencode-ai/opencode/internal/diff"
	"github.com/opencode-ai/opencode/internal/llm/tools"
	"github.com/opencode-ai/opencode/internal/permission"
	"github.com/opencode-ai/opencode/internal/tui/layout"
	"github.com/opencode-ai/opencode/internal/tui/styles"
	"github.com/opencode-ai/opencode/internal/tui/theme"
	"github.com/opencode-ai/opencode/internal/tui/util"
)

type PermissionAction string

// Permission responses
const (
	PermissionAllow           PermissionAction = "allow"
	PermissionAllowForSession PermissionAction = "allow_session"
	// PermissionAllowFamily approves the request AND, for the session, every
	// later command that begins with the family shown on the dialog. See
	// tools/commandfamily.go for what a family may cover and what it never does.
	PermissionAllowFamily PermissionAction = "allow_family"
	PermissionDeny        PermissionAction = "deny"
)

// PermissionResponseMsg represents the user's response to a permission request
type PermissionResponseMsg struct {
	Permission permission.PermissionRequest
	Action     PermissionAction
	// Family is the prefix the person chose, set only for PermissionAllowFamily.
	Family string
}

// PermissionDialogCmp interface for permission dialog component
type PermissionDialogCmp interface {
	tea.Model
	layout.Bindings
	SetPermissions(permission permission.PermissionRequest) tea.Cmd
}

type permissionsMapping struct {
	Left         key.Binding
	Right        key.Binding
	EnterSpace   key.Binding
	Allow        key.Binding
	AllowSession key.Binding
	AllowFamily  key.Binding
	Deny         key.Binding
	Tab          key.Binding
	FamilyLess   key.Binding
	FamilyMore   key.Binding
}

var permissionsKeys = permissionsMapping{
	Left: key.NewBinding(
		key.WithKeys("left"),
		key.WithHelp("<-", "switch options"),
	),
	Right: key.NewBinding(
		key.WithKeys("right"),
		key.WithHelp("->", "switch options"),
	),
	EnterSpace: key.NewBinding(
		key.WithKeys("enter", " "),
		key.WithHelp("enter/space", "confirm"),
	),
	Allow: key.NewBinding(
		key.WithKeys("a"),
		key.WithHelp("a", "allow"),
	),
	AllowSession: key.NewBinding(
		key.WithKeys("s"),
		key.WithHelp("s", "allow for session"),
	),
	AllowFamily: key.NewBinding(
		key.WithKeys("f"),
		key.WithHelp("f", "allow family"),
	),
	Deny: key.NewBinding(
		key.WithKeys("d"),
		key.WithHelp("d", "deny"),
	),
	Tab: key.NewBinding(
		key.WithKeys("tab"),
		key.WithHelp("tab", "switch options"),
	),
	FamilyLess: key.NewBinding(
		key.WithKeys("["),
		key.WithHelp("[", "shorter family"),
	),
	FamilyMore: key.NewBinding(
		key.WithKeys("]"),
		key.WithHelp("]", "longer family"),
	),
}

// permissionDialogCmp is the implementation of PermissionDialog
type permissionDialogCmp struct {
	width           int
	height          int
	permission      permission.PermissionRequest
	windowSize      tea.WindowSizeMsg
	contentViewPort viewport.Model
	// options are the buttons offered for THIS request, left to right, and
	// selectedOption indexes them. Deny is always last and always the default.
	// "Allow family" is present only when the request carries a family
	// suggestion, so a request that has none keeps the three buttons it had.
	options        []PermissionAction
	selectedOption int

	// familyCandidates are the families the person may choose for a bash
	// request, shortest first; familyIdx is the one currently shown.
	familyCandidates []string
	familyIdx        int

	diffCache     map[string]string
	markdownCache map[string]string
}

// family is the family currently offered, or "" when there is none.
func (p *permissionDialogCmp) family() string {
	if len(p.familyCandidates) == 0 || p.familyIdx < 0 || p.familyIdx >= len(p.familyCandidates) {
		return ""
	}
	return p.familyCandidates[p.familyIdx]
}

func (p *permissionDialogCmp) hasOption(a PermissionAction) bool {
	for _, o := range p.options {
		if o == a {
			return true
		}
	}
	return false
}

// currentAction is the button under the highlight.
func (p *permissionDialogCmp) currentAction() PermissionAction {
	if len(p.options) == 0 {
		return PermissionDeny
	}
	if p.selectedOption < 0 || p.selectedOption >= len(p.options) {
		return PermissionDeny
	}
	return p.options[p.selectedOption]
}

func (p *permissionDialogCmp) respond(action PermissionAction) tea.Cmd {
	msg := PermissionResponseMsg{Action: action, Permission: p.permission}
	if action == PermissionAllowFamily {
		fam := p.family()
		if fam == "" {
			// No family to grant: never widen silently. Treat the key as a
			// plain allow of this one request.
			msg.Action = PermissionAllow
		} else {
			msg.Family = fam
		}
	}
	return util.CmdHandler(msg)
}

func (p *permissionDialogCmp) Init() tea.Cmd {
	return p.contentViewPort.Init()
}

func (p *permissionDialogCmp) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		p.windowSize = msg
		cmd := p.SetSize()
		cmds = append(cmds, cmd)
		p.markdownCache = make(map[string]string)
		p.diffCache = make(map[string]string)
	case tea.KeyMsg:
		n := len(p.options)
		if n == 0 {
			n = 1
		}
		switch {
		case key.Matches(msg, permissionsKeys.Right) || key.Matches(msg, permissionsKeys.Tab):
			p.selectedOption = (p.selectedOption + 1) % n
			return p, nil
		case key.Matches(msg, permissionsKeys.Left):
			p.selectedOption = (p.selectedOption + n - 1) % n
		case key.Matches(msg, permissionsKeys.EnterSpace):
			return p, p.selectCurrentOption()
		case key.Matches(msg, permissionsKeys.Allow):
			return p, p.respond(PermissionAllow)
		case key.Matches(msg, permissionsKeys.AllowSession):
			return p, p.respond(PermissionAllowForSession)
		case key.Matches(msg, permissionsKeys.AllowFamily):
			if !p.hasOption(PermissionAllowFamily) {
				return p, nil
			}
			return p, p.respond(PermissionAllowFamily)
		case key.Matches(msg, permissionsKeys.FamilyLess):
			if p.familyIdx > 0 {
				p.familyIdx--
			}
			return p, nil
		case key.Matches(msg, permissionsKeys.FamilyMore):
			if p.familyIdx+1 < len(p.familyCandidates) {
				p.familyIdx++
			}
			return p, nil
		case key.Matches(msg, permissionsKeys.Deny):
			return p, p.respond(PermissionDeny)
		default:
			// Pass other keys to viewport
			viewPort, cmd := p.contentViewPort.Update(msg)
			p.contentViewPort = viewPort
			cmds = append(cmds, cmd)
		}
	}

	return p, tea.Batch(cmds...)
}

func (p *permissionDialogCmp) selectCurrentOption() tea.Cmd {
	return p.respond(p.currentAction())
}

// buttonLabels are the captions, in the order options lists them.
var buttonLabels = map[PermissionAction]string{
	PermissionAllow:           "Allow (a)",
	PermissionAllowForSession: "Allow for session (s)",
	PermissionAllowFamily:     "Allow family (f)",
	PermissionDeny:            "Deny (d)",
}

func (p *permissionDialogCmp) renderButtons() string {
	t := theme.CurrentTheme()
	baseStyle := styles.BaseStyle()
	spacerStyle := baseStyle.Background(styles.PanelBackground())

	parts := make([]string, 0, 2*len(p.options))
	for i, o := range p.options {
		style := baseStyle.Background(styles.PanelBackground()).Foreground(t.Primary())
		if i == p.selectedOption {
			style = baseStyle.Background(t.Primary()).Foreground(t.Background())
		}
		parts = append(parts, style.Padding(0, 1).Render(buttonLabels[o]), spacerStyle.Render("  "))
	}
	content := lipgloss.JoinHorizontal(lipgloss.Left, parts...)

	remainingWidth := p.width - lipgloss.Width(content)
	if remainingWidth > 0 {
		content = spacerStyle.Render(strings.Repeat(" ", remainingWidth)) + content
	}
	return content
}

func (p *permissionDialogCmp) renderHeader() string {
	t := theme.CurrentTheme()
	baseStyle := styles.BaseStyle()

	toolKey := baseStyle.Foreground(t.TextMuted()).Bold(true).Render("Tool")
	toolValue := baseStyle.
		Foreground(t.Text()).
		Width(p.width - lipgloss.Width(toolKey)).
		Render(fmt.Sprintf(": %s", p.permission.ToolName))

	pathKey := baseStyle.Foreground(t.TextMuted()).Bold(true).Render("Path")
	pathValue := baseStyle.
		Foreground(t.Text()).
		Width(p.width - lipgloss.Width(pathKey)).
		Render(fmt.Sprintf(": %s", p.permission.Path))

	headerParts := []string{}

	// GORILLA FIX (2026-08-19): a prompt that appears while auto-approve is ON
	// must say why it appeared. Without this the user sees a dialog in the
	// mode they switched on precisely to stop seeing dialogs, concludes the
	// setting is broken, and stops reading them — which is the failure mode
	// the whole carve-out was built to avoid. See permission.mustAskAnyway.
	if reason := p.permission.AutoApproveOverridden; reason != "" {
		headerParts = append(headerParts,
			baseStyle.Foreground(t.Warning()).Bold(true).Width(p.width).
				Render("Asking even though auto-approve is on:"),
			baseStyle.Foreground(t.Warning()).Width(p.width).Render(reason),
			baseStyle.Render(strings.Repeat(" ", p.width)),
		)
	}

	headerParts = append(headerParts,
		lipgloss.JoinHorizontal(
			lipgloss.Left,
			toolKey,
			toolValue,
		),
		baseStyle.Render(strings.Repeat(" ", p.width)),
		lipgloss.JoinHorizontal(
			lipgloss.Left,
			pathKey,
			pathValue,
		),
		baseStyle.Render(strings.Repeat(" ", p.width)),
	)

	// Add tool-specific header information
	switch p.permission.ToolName {
	case tools.BashToolName:
		headerParts = append(headerParts, baseStyle.Foreground(t.TextMuted()).Width(p.width).Bold(true).Render("Command"))
	case tools.EditToolName:
		params := p.permission.Params.(tools.EditPermissionsParams)
		fileKey := baseStyle.Foreground(t.TextMuted()).Bold(true).Render("File")
		filePath := baseStyle.
			Foreground(t.Text()).
			Width(p.width - lipgloss.Width(fileKey)).
			Render(fmt.Sprintf(": %s", params.FilePath))
		headerParts = append(headerParts,
			lipgloss.JoinHorizontal(
				lipgloss.Left,
				fileKey,
				filePath,
			),
			baseStyle.Render(strings.Repeat(" ", p.width)),
		)

	case tools.WriteToolName:
		params := p.permission.Params.(tools.WritePermissionsParams)
		fileKey := baseStyle.Foreground(t.TextMuted()).Bold(true).Render("File")
		filePath := baseStyle.
			Foreground(t.Text()).
			Width(p.width - lipgloss.Width(fileKey)).
			Render(fmt.Sprintf(": %s", params.FilePath))
		headerParts = append(headerParts,
			lipgloss.JoinHorizontal(
				lipgloss.Left,
				fileKey,
				filePath,
			),
			baseStyle.Render(strings.Repeat(" ", p.width)),
		)
	case tools.FetchToolName:
		headerParts = append(headerParts, baseStyle.Foreground(t.TextMuted()).Width(p.width).Bold(true).Render("URL"))
	}

	return lipgloss.NewStyle().Background(styles.PanelBackground()).Render(lipgloss.JoinVertical(lipgloss.Left, headerParts...))
}

// renderBashContent is no longer called (2026-10-23): bash requests are drawn
// by renderCommandDialog. Kept for the history in its comments.
func (p *permissionDialogCmp) renderBashContent() string {
	baseStyle := styles.BaseStyle()

	if pr, ok := p.permission.Params.(tools.BashPermissionsParams); ok {
		// GORILLA FIX (2026-08-23): WRAP the command. A fenced code block does
		// not wrap, it CLIPS, so a command longer than the dialog was silently
		// cut and the user was asked to approve something they could not read.
		//
		// Observed on a live run: "cd /home/gorilla/Documents/Debian.Kernel.Work/
		// kernel-" and "find /home/gorilla/Documents/Debian.Kernel.Work -name \".",
		// both severed mid-argument. The destination and the pattern, which are
		// the only parts that decide whether the command is safe, were the parts
		// removed.
		//
		// This is the worst instance of a class that turned up three times in
		// one day (the helper spawn notice, the /tasks label, this): text cut to
		// fit a container instead of the container fitting the text. It is worst
		// here because this dialog exists for one purpose, which is showing you
		// what you are about to allow.
		content := fmt.Sprintf("```bash\n%s\n```", wrapCommand(pr.Command, p.width-14))

		// GORILLA FIX (2026-10-11): the family, in full, under the command.
		// The sentence the person reads IS the grant: every character of the
		// prefix reaches the screen, wrapped not clipped, with the keys that
		// move it and the one warning that matters (see BashFamilyNote).
		cacheKey := p.permission.ID
		if fam := p.family(); fam != "" {
			content += fmt.Sprintf("\n**Allow family (f)** also allows, for this session, every command that begins with:\n```bash\n%s\n```\n`[` shorter, `]` longer (%d of %d)",
				wrapCommand(fam, p.width-14), p.familyIdx+1, len(p.familyCandidates))
			if note := tools.BashFamilyNote(pr.Command, fam); note != "" {
				content += "\n\n" + note
			}
			content += "\n\nNever covered by a family: a dangerous command, anything after untrusted content was read, a different path."
			cacheKey = fmt.Sprintf("%s#%d", p.permission.ID, p.familyIdx)
		}

		// Use the cache for markdown rendering
		renderedContent := p.GetOrSetMarkdown(cacheKey, func() (string, error) {
			r := styles.GetMarkdownRenderer(p.width - 10)
			s, err := r.Render(content)
			return styles.ApplyPanelBackground(s), err
		})

		finalContent := baseStyle.
			Width(p.contentViewPort.Width).
			Render(renderedContent)
		p.contentViewPort.SetContent(finalContent)
		return p.styleViewport()
	}
	return ""
}

func (p *permissionDialogCmp) renderEditContent() string {
	if pr, ok := p.permission.Params.(tools.EditPermissionsParams); ok {
		diff := p.GetOrSetDiff(p.permission.ID, func() (string, error) {
			return diff.FormatDiff(pr.Diff, diff.WithTotalWidth(p.contentViewPort.Width))
		})

		p.contentViewPort.SetContent(diff)
		return p.styleViewport()
	}
	return ""
}

func (p *permissionDialogCmp) renderPatchContent() string {
	if pr, ok := p.permission.Params.(tools.EditPermissionsParams); ok {
		diff := p.GetOrSetDiff(p.permission.ID, func() (string, error) {
			return diff.FormatDiff(pr.Diff, diff.WithTotalWidth(p.contentViewPort.Width))
		})

		p.contentViewPort.SetContent(diff)
		return p.styleViewport()
	}
	return ""
}

func (p *permissionDialogCmp) renderWriteContent() string {
	if pr, ok := p.permission.Params.(tools.WritePermissionsParams); ok {
		// Use the cache for diff rendering
		diff := p.GetOrSetDiff(p.permission.ID, func() (string, error) {
			return diff.FormatDiff(pr.Diff, diff.WithTotalWidth(p.contentViewPort.Width))
		})

		p.contentViewPort.SetContent(diff)
		return p.styleViewport()
	}
	return ""
}

// renderFetchContent is no longer called (2026-10-23): web_fetch requests are
// drawn by renderCommandDialog. Kept for the history in its comments.
func (p *permissionDialogCmp) renderFetchContent() string {
	baseStyle := styles.BaseStyle()

	if pr, ok := p.permission.Params.(tools.FetchPermissionsParams); ok {
		// GORILLA FIX (2026-08-23): wrap the URL, same as the command.
		//
		// FOURTH instance of one fault, caught on the owner's screen after the
		// bash dialog had already been fixed and this one was not looked at:
		//
		//   https://www.forbes.com/sites/forbeswealthteam/article/the-
		//
		// A truncated URL is worse here than a truncated command is next door.
		// For web_fetch the HOST is the grant key: approving one authorises
		// every later page on that host for the session. So the string the
		// user is asked to judge is the exact string the grant is built from,
		// and it was being cut before they could read it.
		content := fmt.Sprintf("```bash\n%s\n```", wrapCommand(pr.URL, p.width-14))

		// Use the cache for markdown rendering
		renderedContent := p.GetOrSetMarkdown(p.permission.ID, func() (string, error) {
			r := styles.GetMarkdownRenderer(p.width - 10)
			s, err := r.Render(content)
			return styles.ApplyPanelBackground(s), err
		})

		finalContent := baseStyle.
			Width(p.contentViewPort.Width).
			Render(renderedContent)
		p.contentViewPort.SetContent(finalContent)
		return p.styleViewport()
	}
	return ""
}

func (p *permissionDialogCmp) renderDefaultContent() string {
	baseStyle := styles.BaseStyle()

	content := p.permission.Description

	// Use the cache for markdown rendering
	renderedContent := p.GetOrSetMarkdown(p.permission.ID, func() (string, error) {
		r := styles.GetMarkdownRenderer(p.width - 10)
		s, err := r.Render(content)
		return styles.ApplyPanelBackground(s), err
	})

	finalContent := baseStyle.
		Width(p.contentViewPort.Width).
		Render(renderedContent)
	p.contentViewPort.SetContent(finalContent)

	if renderedContent == "" {
		return ""
	}

	return p.styleViewport()
}

func (p *permissionDialogCmp) styleViewport() string {
	contentStyle := lipgloss.NewStyle().
		Background(styles.PanelBackground())

	return contentStyle.Render(p.contentViewPort.View())
}

// renderCommandDialog draws a bash or web_fetch request: the command or URL
// FIRST and in full, then one plain line of what it does, why it asks, where,
// and what "Allow for session" will cover, then the buttons.
//
// GORILLA FIX (2026-10-23): the Command and URL fields rendered EMPTY. Measured
// on the MacBook Air (80x24, auto-approve on) and reproduced headlessly in
// permission_content_test.go: the box was 40% of the window high, the
// auto-approve banner plus the Tool/Path header filled all of it, and the
// viewport holding the command was given a height of zero or less. At 160x45
// a URL was still empty. A person was asked to judge a request with the one
// thing worth judging missing, which trains Allow by reflex.
//
// The box is now sized to its content and capped at the window; when rows run
// short the optional lines go first and the command or URL is the last thing
// to be cut, with a line saying how much was not shown.
func (p *permissionDialogCmp) renderCommandDialog() string {
	t := theme.CurrentTheme()
	base := styles.BaseStyle()
	// Border = 2 columns OUTSIDE Width; Padding(1,0,0,1) = 1 column INSIDE.
	// One more column of slack on the right keeps wrapped text off the border.
	tw := p.width - 2
	if tw < 20 {
		tw = 20
	}
	text := func(fg lipgloss.TerminalColor, bold bool, s string) []string {
		st := base.Foreground(fg).Bold(bold)
		var out []string
		for _, l := range strings.Split(wrapCommand(s, tw), "\n") {
			out = append(out, st.Render(l))
		}
		return out
	}
	blank := []string{""}

	label, subject, purpose := "Command", "", ""
	switch pr := p.permission.Params.(type) {
	case tools.BashPermissionsParams:
		subject = pr.Command
		purpose = tools.CommandPurpose(pr.Command)
	case tools.FetchPermissionsParams:
		label, subject = "URL", pr.URL
		purpose = "Downloads this page and gives its text to the model."
	}
	if subject == "" {
		// Never an empty field: fall back to the request's own sentence.
		subject = p.permission.Description
	}
	subjectLines := text(t.Text(), true, subject)

	title := text(t.Primary(), true, "Permission Required")
	head := append(text(t.TextMuted(), true, label), subjectLines...)

	// Optional lines, each with a keep-priority (lower kept first) and a
	// display order. The family text is the one whose loss also takes its
	// button away: a grant must never be offered without its sentence.
	type part struct {
		prio  int
		lines []string
	}
	var reason, scope, fam, what, where part
	if r := p.permission.AutoApproveOverridden; r != "" {
		reason = part{1, text(t.Warning(), false, "Asks even with auto-approve on: "+r)}
	}
	scope = part{2, text(t.TextMuted(), false, "Allow for session (s) covers: "+permission.SessionScope(p.permission))}
	if f := p.family(); f != "" {
		pr, _ := p.permission.Params.(tools.BashPermissionsParams)
		block := text(t.Text(), false, fmt.Sprintf("Allow family (f) also allows, this session, every command beginning with ([ shorter, ] longer, %d of %d):",
			p.familyIdx+1, len(p.familyCandidates)))
		block = append(block, text(t.Text(), true, f)...)
		if note := tools.BashFamilyNote(pr.Command, f); note != "" {
			block = append(block, text(t.Warning(), false, note)...)
		}
		block = append(block, text(t.TextMuted(), false, "Never covered by a family: a dangerous command, anything after untrusted content was read, a different path.")...)
		fam = part{3, block}
	}
	if purpose != "" {
		what = part{4, text(t.TextMuted(), false, purpose)}
	}
	where = part{5, text(t.TextMuted(), false, "Tool: "+p.permission.ToolName+"   Path: "+p.permission.Path)}

	// Rows available inside the window: border 2 + top padding 1, and one row
	// of the window left free so the box never touches both edges.
	budget := p.windowSize.Height - 4
	if budget < 8 {
		budget = 8
	}
	buttonsFor := func() []string { return strings.Split(p.renderButtonsWithin(tw), "\n") }
	buttons := buttonsFor()
	fixed := len(title) + 1 + len(head) + 1 + len(buttons)
	if fixed > budget {
		// Even the command does not fit: keep as much of it as there is room
		// for and say how much was not shown.
		keep := budget - (len(title) + 1 + 1 + 1 + 1 + len(buttons)) // title, blank, label, note, blank
		if keep < 1 {
			keep = 1
		}
		hidden := len(subjectLines) - keep
		head = append(head[:1], subjectLines[:keep]...)
		head = append(head, text(t.Warning(), false, fmt.Sprintf("... %d more line(s) not shown. Deny and ask the model to explain if you cannot judge it.", hidden))...)
		reason, scope, fam, what, where = part{}, part{}, part{}, part{}, part{}
		fixed = len(title) + 1 + len(head) + 1 + len(buttons)
	}
	room := budget - fixed - 1 // one blank row between the command and the rest
	kept := map[int]bool{}
	for _, c := range []part{reason, scope, fam, what, where} { // priority order
		if len(c.lines) > 0 && len(c.lines) <= room {
			kept[c.prio] = true
			room -= len(c.lines)
		}
	}
	if fam.lines != nil && !kept[fam.prio] && p.hasOption(PermissionAllowFamily) {
		// No room to say what a family covers, so it is not offered.
		p.options = slices.DeleteFunc(p.options, func(a PermissionAction) bool { return a == PermissionAllowFamily })
		p.selectedOption = len(p.options) - 1
		buttons = buttonsFor()
	}
	body := append(append(append([]string{}, title...), blank...), head...)
	body = append(body, "")
	for _, c := range []part{what, reason, where, fam, scope} { // display order
		if kept[c.prio] {
			body = append(body, c.lines...)
		}
	}
	body = append(body, "")
	body = append(body, buttons...)

	return base.
		Padding(1, 0, 0, 1).
		Border(lipgloss.RoundedBorder()).
		BorderBackground(styles.PanelBackground()).
		BorderForeground(t.TextMuted()).
		Width(p.width).
		Render(strings.Join(body, "\n"))
}

// renderButtonsWithin lays the buttons out on one row when they fit in width
// columns and on as many rows as they need when they do not. Four buttons
// (with "Allow family") are 70 columns, wider than the 62-column text area of
// an 80-column window.
func (p *permissionDialogCmp) renderButtonsWithin(width int) string {
	if row := p.renderButtons(); lipgloss.Width(row) <= width {
		return row
	}
	t := theme.CurrentTheme()
	base := styles.BaseStyle()
	var rows []string
	line := ""
	for i, o := range p.options {
		style := base.Background(styles.PanelBackground()).Foreground(t.Primary())
		if i == p.selectedOption {
			style = base.Background(t.Primary()).Foreground(t.Background())
		}
		b := style.Padding(0, 1).Render(buttonLabels[o])
		if line != "" && lipgloss.Width(line)+2+lipgloss.Width(b) > width {
			rows = append(rows, line)
			line = ""
		}
		if line != "" {
			line += "  "
		}
		line += b
	}
	return strings.Join(append(rows, line), "\n")
}

func (p *permissionDialogCmp) render() string {
	switch p.permission.ToolName {
	case tools.BashToolName, tools.FetchToolName:
		return p.renderCommandDialog()
	}
	t := theme.CurrentTheme()
	baseStyle := styles.BaseStyle()

	title := baseStyle.
		Bold(true).
		Width(p.width - 4).
		Foreground(t.Primary()).
		Render("Permission Required")
	// Render header
	headerContent := p.renderHeader()
	// Render buttons
	buttons := p.renderButtons()

	// Calculate content height dynamically based on window size
	p.contentViewPort.Height = p.height - lipgloss.Height(headerContent) - lipgloss.Height(buttons) - 2 - lipgloss.Height(title)
	p.contentViewPort.Width = p.width - 4

	// Render content based on tool type
	var contentFinal string
	switch p.permission.ToolName {
	case tools.BashToolName:
		contentFinal = p.renderBashContent()
	case tools.EditToolName:
		contentFinal = p.renderEditContent()
	case tools.PatchToolName:
		contentFinal = p.renderPatchContent()
	case tools.WriteToolName:
		contentFinal = p.renderWriteContent()
	case tools.FetchToolName:
		contentFinal = p.renderFetchContent()
	default:
		contentFinal = p.renderDefaultContent()
	}

	content := lipgloss.JoinVertical(
		lipgloss.Top,
		title,
		baseStyle.Render(strings.Repeat(" ", lipgloss.Width(title))),
		headerContent,
		contentFinal,
		buttons,
		baseStyle.Render(strings.Repeat(" ", p.width-4)),
	)

	return baseStyle.
		Padding(1, 0, 0, 1).
		Border(lipgloss.RoundedBorder()).
		BorderBackground(styles.PanelBackground()).
		BorderForeground(t.TextMuted()).
		Width(p.width).
		Height(p.height).
		Render(
			content,
		)
}

func (p *permissionDialogCmp) View() string {
	return p.render()
}

func (p *permissionDialogCmp) BindingKeys() []key.Binding {
	return layout.KeyMapToSlice(permissionsKeys)
}

func (p *permissionDialogCmp) SetSize() tea.Cmd {
	if p.permission.ID == "" {
		return nil
	}
	switch p.permission.ToolName {
	case tools.BashToolName:
		// GORILLA FIX (2026-08-23): 0.4 was too narrow for the thing it exists
		// to show. A command is the whole content of this dialog, exactly as a
		// diff is for edit and write, and those already use 0.8. Wrapping alone
		// would fix the truncation and leave a tall thin box wrapping every
		// path over four lines; the width is what makes it readable.
		p.width = int(float64(p.windowSize.Width) * 0.8)
		p.height = int(float64(p.windowSize.Height) * 0.4)
		if p.family() != "" {
			// The family block is a second command's worth of text plus its
			// explanation; the box grows so neither is scrolled out of sight.
			p.height = int(float64(p.windowSize.Height) * 0.7)
		}
	case tools.EditToolName:
		p.width = int(float64(p.windowSize.Width) * 0.8)
		p.height = int(float64(p.windowSize.Height) * 0.8)
	case tools.WriteToolName:
		p.width = int(float64(p.windowSize.Width) * 0.8)
		p.height = int(float64(p.windowSize.Height) * 0.8)
	case tools.FetchToolName:
		// Wider for the same reason as bash: the URL IS the content of this
		// dialog, and a wrapped URL over four lines in a narrow box is
		// readable but miserable. See renderFetchContent.
		p.width = int(float64(p.windowSize.Width) * 0.8)
		p.height = int(float64(p.windowSize.Height) * 0.4)
	default:
		p.width = int(float64(p.windowSize.Width) * 0.7)
		p.height = int(float64(p.windowSize.Height) * 0.5)
	}
	return nil
}

func (p *permissionDialogCmp) SetPermissions(permission permission.PermissionRequest) tea.Cmd {
	p.permission = permission

	// The family on offer, if the request brought a suggestion. The person
	// can walk through every usable prefix of the command with [ and ], and
	// the suggestion is where that walk starts.
	p.familyCandidates = nil
	p.familyIdx = 0
	if permission.FamilySuggestion != "" {
		if pr, ok := permission.Params.(tools.BashPermissionsParams); ok {
			p.familyCandidates = tools.BashFamilyCandidates(pr.Command)
			found := false
			for i, c := range p.familyCandidates {
				if c == permission.FamilySuggestion {
					p.familyIdx, found = i, true
					break
				}
			}
			if !found {
				p.familyCandidates = []string{permission.FamilySuggestion}
				p.familyIdx = 0
			}
		}
	}
	p.options = []PermissionAction{PermissionAllow, PermissionAllowForSession}
	if p.family() != "" {
		p.options = append(p.options, PermissionAllowFamily)
	}
	p.options = append(p.options, PermissionDeny)

	// GORILLA FIX (2026-08-18): reset the selection for every NEW request.
	//
	// The dialog is reused, and the highlight persisted. So after approving one
	// thing, the next — different — request opened with an answer already
	// chosen, and a single Enter accepted a question the user had not read.
	// Consent has to be given per request, not inherited from the last one.
	p.selectedOption = len(p.options) - 1 // Deny
	return p.SetSize()
}

// Helper to get or set cached diff content
func (c *permissionDialogCmp) GetOrSetDiff(key string, generator func() (string, error)) string {
	if cached, ok := c.diffCache[key]; ok {
		return cached
	}

	content, err := generator()
	if err != nil {
		return fmt.Sprintf("Error formatting diff: %v", err)
	}

	c.diffCache[key] = content

	return content
}

// Helper to get or set cached markdown content
func (c *permissionDialogCmp) GetOrSetMarkdown(key string, generator func() (string, error)) string {
	if cached, ok := c.markdownCache[key]; ok {
		return cached
	}

	content, err := generator()
	if err != nil {
		return fmt.Sprintf("Error rendering markdown: %v", err)
	}

	c.markdownCache[key] = content

	return content
}

func NewPermissionDialogCmp() PermissionDialogCmp {
	// Create viewport for content
	contentViewport := viewport.New(0, 0)

	return &permissionDialogCmp{
		contentViewPort: contentViewport,
		// GORILLA FIX (2026-08-18): default to DENY, not Allow.
		//
		// This dialog is the only security boundary in the program. Landing on
		// "Allow" means the safe answer costs two keystrokes and the dangerous
		// one costs a reflex — and a prompt that appears mid-typing can be
		// answered by an Enter the user meant for something else. Defaulting to
		// the refusal makes the accident harmless.
		options:        []PermissionAction{PermissionAllow, PermissionAllowForSession, PermissionDeny},
		selectedOption: 2, // Default to "Deny"
		diffCache:      make(map[string]string),
		markdownCache:  make(map[string]string),
	}
}

// wrapCommand hard-wraps a shell command to width columns.
//
// Wrapped rather than truncated, and wrapped BY US rather than left to the
// markdown renderer, because a fenced code block clips instead of wrapping.
// Every character of the command reaches the screen or the dialog is lying
// about what it is asking for.
//
// Breaks at whitespace where it can, so arguments stay whole and readable, and
// mid-token only when a single token is genuinely longer than the line, which a
// long path can be.
func wrapCommand(cmd string, width int) string {
	if width < 20 {
		width = 20
	}
	var out []string
	for _, line := range strings.Split(cmd, "\n") {
		for {
			runes := []rune(line)
			if lipgloss.Width(line) <= width {
				break
			}
			// Walk runes accumulating DISPLAY columns, and stop at the last
			// index that still fits. Measuring "the first index that does not
			// fit" and cutting there is off by one, which is how the first
			// version of this produced 41-column lines in a 40-column box.
			fits, lastSpace, w := 0, -1, 0
			for i, r := range runes {
				rw := lipgloss.Width(string(r))
				if w+rw > width {
					break
				}
				w += rw
				fits = i + 1
				if r == ' ' {
					lastSpace = i
				}
			}
			if fits == 0 {
				fits = 1 // a single rune wider than the box; take it anyway
			}
			cut := fits
			trim := false
			// Break at whitespace where there is one, so arguments stay whole.
			// Mid-token only when a single token really is longer than the
			// line, which a long path often is.
			if lastSpace > 0 {
				cut, trim = lastSpace, true
			}
			out = append(out, string(runes[:cut]))
			line = string(runes[cut:])
			if trim {
				line = strings.TrimPrefix(line, " ")
			}
		}
		out = append(out, line)
	}
	return strings.Join(out, "\n")
}
