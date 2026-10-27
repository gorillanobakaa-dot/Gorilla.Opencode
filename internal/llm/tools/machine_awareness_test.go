package tools

// GORILLA (2026-10-27, job 06): how-to cards on demand, the PowerShell edition
// in the bash tool, and the '&&' warning only for Windows PowerShell 5.1.

import (
	"context"
	"runtime"
	"strings"
	"testing"

	"github.com/opencode-ai/opencode/internal/hostprobe"
	"github.com/opencode-ai/opencode/internal/llm/tools/shell"
)

func TestFilesHowtoServesACardWithoutAPath(t *testing.T) {
	ft := NewFilesTool(nil)
	resp, err := ft.Run(context.Background(), ToolCall{Input: `{"action":"howto","path":"robocopy"}`})
	if err != nil {
		t.Fatal(err)
	}
	if runtime.GOOS != "windows" {
		if !resp.IsError {
			t.Fatalf("the robocopy card was served on %s", runtime.GOOS)
		}
		return
	}
	if resp.IsError || !strings.Contains(resp.Content, "/MIR") || !strings.Contains(resp.Content, "/MT:n") {
		t.Fatalf("robocopy card: %+v", resp)
	}
	resp, _ = ft.Run(context.Background(), ToolCall{Input: `{"action":"howto"}`})
	if resp.IsError || !strings.Contains(resp.Content, "powershell-files") {
		t.Fatalf("an empty howto must list the cards: %+v", resp)
	}
	resp, _ = ft.Run(context.Background(), ToolCall{Input: `{"action":"howto","path":"nosuchcard"}`})
	if !resp.IsError || !strings.Contains(resp.Content, "robocopy") {
		t.Fatalf("an unknown card must be an error that names the real ones: %+v", resp)
	}
}

func TestChainingAdviceMatchesTheEdition(t *testing.T) {
	ps51 := powerShellGuidance(ps51Chaining)
	pwsh7 := powerShellGuidance(pwsh7Chaining)
	if !strings.Contains(ps51, "does NOT support '&&'") {
		t.Error("the 5.1 guidance must warn about '&&'")
	}
	if strings.Contains(pwsh7, "NOT support") || !strings.Contains(pwsh7, "'&&'") {
		t.Error("the PowerShell 7 guidance must allow '&&'")
	}
	if strings.Contains(ps51, "{{") || strings.Contains(pwsh7, "{{") {
		t.Error("placeholder left in the guidance")
	}
}

func TestBashDescriptionNamesTheEdition(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("PowerShell is the Windows shell")
	}
	d := bashDescription()
	ed := hostprobe.ShellEdition(shell.ShellPath())
	if !strings.Contains(d, "Executes a given "+ed+" command") {
		t.Errorf("description does not name %q", ed)
	}
	if hostprobe.IsPowerShell7() == strings.Contains(d, "does NOT support '&&'") {
		t.Errorf("'&&' advice does not match the edition (pwsh7=%v)", hostprobe.IsPowerShell7())
	}
	if s := (&bashTool{}).Summary(); !strings.Contains(s, ed) {
		t.Errorf("tool table line %q lacks %q", s, ed)
	}
}

func TestToolTableLines(t *testing.T) {
	if got := capWords("one two three four five six", 12); got != "one two..." {
		t.Errorf("capWords: %q", got)
	}
	if got := firstSentence("Does a thing. Then more."); got != "Does a thing." {
		t.Errorf("firstSentence: %q", got)
	}
	table := ToolTableBlock([]BaseTool{NewFilesTool(nil), NewFindTool()}, false)
	if !strings.HasPrefix(table, "YOUR TOOLS") || strings.Count(table, "\n") != 3 {
		t.Errorf("table:\n%s", table)
	}
	if ToolTableBlock(nil, false) != "" {
		t.Error("an empty tool set made a header")
	}
}
