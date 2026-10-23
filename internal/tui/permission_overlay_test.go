package tui

// GORILLA FIX (2026-10-23): rows 5 and 6 of MACOS-AIR-LAYOUT-PLAN.md. On the
// MacBook Air (Terminal 169x33, inline layout) the permission dialog cut the
// rows under it to "Waiting for to" with a grey block beside and below the box,
// and its rounded border was misaligned. The block was the drop shadow: one row
// taller and one column wider than the dialog, landing on the first real row.

import (
	"fmt"
	"regexp"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/opencode-ai/opencode/internal/llm/tools"
	"github.com/opencode-ai/opencode/internal/permission"
	"github.com/opencode-ai/opencode/internal/tui/components/dialog"
)

var ansiSeq = regexp.MustCompile("\x1b\\[[0-9;]*m")

func TestThePermissionDialogLeavesTheRowsAroundItAlone(t *testing.T) {
	for _, scrollback := range []bool{true, false} {
		const w, h = 169, 33
		d := dialog.NewPermissionDialogCmp()
		d.Update(tea.WindowSizeMsg{Width: w, Height: h})
		d.SetPermissions(permission.PermissionRequest{ID: "x", ToolName: "bash", Path: "/Users/gorilla/Stranger-Test",
			AutoApproveOverridden: "this turn has already read untrusted content (web page fetched from https://raw.githubusercontent.com/gorillanobakaa-dot/Gorilla.Fieldkit/feat/stranger-install/install.sh)",
			Params:                tools.BashPermissionsParams{Command: "python3 --version"}})
		overlay := d.View()
		var bg []string
		for i := 0; i < h; i++ {
			bg = append(bg, fmt.Sprintf("%-169s", fmt.Sprintf(":: Waiting for tool response... row %d", i)))
		}
		a := appModel{width: w, scrollback: scrollback}
		out := strings.Split(ansiSeq.ReplaceAllString(a.placePermissionDialog(overlay, strings.Join(bg, "\n")), ""), "\n")
		box := strings.Split(ansiSeq.ReplaceAllString(overlay, ""), "\n")

		// Find the box in the frame and check every row outside it is untouched.
		top := -1
		for i, l := range out {
			if strings.Contains(l, "+--") {
				top = i
				break
			}
		}
		if top < 0 {
			t.Fatalf("scrollback=%v: no ASCII border in the frame", scrollback)
		}
		col := strings.Index(out[top], "+")
		for i, l := range out {
			inBox := i >= top && i < top+len(box)
			if !inBox {
				if strings.TrimSpace(l) != "" && !strings.HasPrefix(l, ":: Waiting for tool response... row") {
					t.Errorf("scrollback=%v row %d outside the dialog was cut: %q", scrollback, i, l)
				}
				continue
			}
			// Every box row starts with its border glyph in the same column and
			// ends with one in the same column: no offset top, no stray bar.
			r := []rune(l)
			bw := len([]rune(box[0]))
			if col+bw > len(r) {
				t.Fatalf("scrollback=%v row %d shorter than the box", scrollback, i)
			}
			first, last := r[col], r[col+bw-1]
			edge := i == top || i == top+len(box)-1
			if edge && (first != '+' || last != '+') || !edge && (first != '|' || last != '|') {
				t.Errorf("scrollback=%v row %d border misaligned: %q ... %q", scrollback, i, string(first), string(last))
			}
		}
	}
}
