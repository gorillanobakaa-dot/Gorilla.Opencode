package helppages

// GORILLA (2026-10-10): the /editor, /helpers and /hooks pages. Each test pins
// a promise the page makes to someone who has never opened a terminal: the Zed
// block is the exact one the README prints, all three helper kinds are named
// with a sentence to type, /hooks says "none" when there are none and lists
// them when there are, and the example it offers is for the system it runs on
// and is valid JSON that the program would accept.

import (
	"encoding/json"
	"os"
	"runtime"
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
	"github.com/opencode-ai/opencode/internal/config"
	"github.com/opencode-ai/opencode/internal/config/configtest"
)

// Helpers() reads the helper limit from the settings folder; keep it off the
// developer's real one.
func TestMain(m *testing.M) { os.Exit(configtest.Isolate(m)) }

func pageText(p Page) string {
	var b strings.Builder
	b.WriteString(p.Title + "\n")
	for _, l := range p.Lines {
		b.WriteString(l.Text + "\n")
	}
	return b.String()
}

func codeLines(p Page) []string {
	var out []string
	for _, l := range p.Lines {
		if l.Kind == Code {
			out = append(out, l.Text)
		}
	}
	return out
}

func stripSpace(s string) string {
	return strings.Join(strings.Fields(s), "")
}

// The block the README and `gorilla-opencode acp --help` print, character for
// character apart from line breaks and indentation.
const zedOneLine = `"agent_servers": { "Gorilla OpenCode": { "type": "custom", "command": "gorilla-opencode", "args": ["acp"] } }`

func TestEditorPageCarriesTheExactZedBlock(t *testing.T) {
	p := Editor()
	code := strings.Join(codeLines(p), "\n")
	if !strings.Contains(code, strings.Join(ZedBlock, "\n")) {
		t.Fatalf("the Zed block is not on the page as consecutive code lines:\n%s", code)
	}
	if got := stripSpace(strings.Join(ZedBlock, "")); got != stripSpace(zedOneLine) {
		t.Errorf("Zed block differs from the README's:\n got %s\nwant %s", got, stripSpace(zedOneLine))
	}
	// Pasted into settings.json it must parse, and carry the values Zed reads.
	var v struct {
		AgentServers map[string]struct {
			Type    string   `json:"type"`
			Command string   `json:"command"`
			Args    []string `json:"args"`
		} `json:"agent_servers"`
	}
	if err := json.Unmarshal([]byte("{"+strings.Join(ZedBlock, "\n")+"}"), &v); err != nil {
		t.Fatalf("the Zed block is not valid JSON: %v", err)
	}
	s, ok := v.AgentServers["Gorilla OpenCode"]
	if !ok || s.Type != "custom" || s.Command != "gorilla-opencode" || strings.Join(s.Args, " ") != "acp" {
		t.Errorf("decoded %+v", v.AgentServers)
	}

	text := pageText(p)
	for _, want := range []string{
		"JetBrains", "provider", "permission question", "--- what actually ran",
		"old conversation", "Tools your editor offers", "Pictures",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("the /editor page does not mention %q", want)
		}
	}
}

func TestHelpersPageNamesAllThreeRolesWithASentenceToType(t *testing.T) {
	text := pageText(Helpers())
	for _, want := range []string{
		"explore:", "plan:", "coder:",
		"have an explore helper find", "use a plan helper to work out how to", "send a coder helper to",
		"asks your permission before each change", "/tasks", "[plan]", "/context", "indented under that helper",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("the /helpers page does not contain %q", want)
		}
	}
}

// The page states the limit actually set, not a typed figure.
func TestHelpersPageStatesTheLiveLimit(t *testing.T) {
	prev := config.MaxSubAgents()
	t.Cleanup(func() { config.SetMaxSubAgents(prev) })
	config.SetMaxSubAgents(3)
	if text := pageText(Helpers()); !strings.Contains(text, "the limit is 3 helpers per answer") {
		t.Errorf("limit 3 not stated:\n%s", text)
	}
	config.SetMaxSubAgents(config.SubAgentsNuclear)
	if text := pageText(Helpers()); !strings.Contains(text, "switched OFF") {
		t.Errorf("helpers off not stated:\n%s", text)
	}
}

func TestHooksPageSaysNoneWhenThereAreNone(t *testing.T) {
	text := pageText(HooksPage(nil, "/home/u/.config/gorilla-opencode/config.json", "", "linux"))
	if !strings.Contains(text, "None. No hook is configured") {
		t.Errorf("no plain statement that none is configured:\n%s", text)
	}
	if !strings.Contains(text, "/home/u/.config/gorilla-opencode/config.json") {
		t.Error("the settings-file path is not shown")
	}
	if !strings.Contains(text, ".gorilla-opencode.json") {
		t.Error("the per-folder file, whose hooks are ignored, is not mentioned")
	}
	if !strings.Contains(text, "are ignored") {
		t.Error("the page must say hooks in a project folder are ignored")
	}
}

func TestHooksPageListsTheLoadedHooks(t *testing.T) {
	hooks := []config.Hook{
		{Event: config.HookBeforeTool, Tools: []string{"bash", "edit"}, Command: "gate.ps1", TimeoutSeconds: 45},
		{Event: config.HookAfterTool, Command: "log-it"},
		{Event: config.HookTurnEnd, Command: "notify-send done", TimeoutSeconds: 5},
	}
	p := HooksPage(hooks, `C:\Users\u\.config\gorilla-opencode\config.json`, `C:\work\.gorilla-opencode.json`, "windows")
	text := pageText(p)
	for _, want := range []string{
		"1. before_tool, tools: bash, edit, timeout: 45 seconds",
		"   gate.ps1",
		"2. after_tool, tools: every tool, timeout: 30 seconds",
		"   log-it",
		"3. turn_end, timeout: 5 seconds",
		"   notify-send done",
		`C:\work\.gorilla-opencode.json`,
	} {
		if !strings.Contains(text, want) {
			t.Errorf("the /hooks page does not contain %q:\n%s", want, text)
		}
	}
	if strings.Contains(text, "None. No hook") {
		t.Error("says none while three are loaded")
	}
}

// exampleHooks decodes the example the page offers, exactly as a user would
// paste it into config.json.
func exampleHooks(t *testing.T, goos string) ([]config.Hook, string) {
	t.Helper()
	p := HooksPage(nil, "x", "", goos)
	code := codeLines(p)
	start := -1
	for i, l := range code {
		if l == `"hooks": [` {
			start = i
		}
	}
	if start < 0 {
		t.Fatalf("%s: no hooks example on the page", goos)
	}
	end := start
	for end < len(code) && code[end] != `]` {
		end++
	}
	block := strings.Join(code[start:end+1], "\n")
	var v struct {
		Hooks []config.Hook `json:"hooks"`
	}
	if err := json.Unmarshal([]byte("{"+block+"}"), &v); err != nil {
		t.Fatalf("%s example is not valid JSON: %v\n%s", goos, err, block)
	}
	return v.Hooks, block
}

func TestHooksExampleIsForTheSystemAndValid(t *testing.T) {
	for _, goos := range []string{"windows", "linux", "darwin"} {
		hooks, block := exampleHooks(t, goos)
		if len(hooks) != 3 {
			t.Fatalf("%s: %d hooks in the example, want 3", goos, len(hooks))
		}
		events := []string{config.HookBeforeTool, config.HookAfterTool, config.HookTurnEnd}
		for i, h := range hooks {
			if h.Event != events[i] || strings.TrimSpace(h.Command) == "" {
				t.Errorf("%s: hook %d is %+v", goos, i, h)
			}
			if h.TimeoutSeconds < 0 || h.TimeoutSeconds > config.HookMaxTimeoutSeconds {
				t.Errorf("%s: hook %d timeout %d out of range", goos, i, h.TimeoutSeconds)
			}
		}
		if !hooks[0].AppliesToTool("bash") || hooks[0].AppliesToTool("edit") {
			t.Errorf("%s: the gate must cover bash only: %+v", goos, hooks[0].Tools)
		}
		if !strings.Contains(hooks[0].Command, "git push") || !strings.Contains(hooks[0].Command, "exit 1") {
			t.Errorf("%s: the gate does not refuse git push: %q", goos, hooks[0].Command)
		}
		ps := strings.Contains(block, "$env:")
		if goos == "windows" && !ps {
			t.Errorf("windows example is not PowerShell:\n%s", block)
		}
		if goos != "windows" && (ps || !strings.Contains(hooks[0].Command, "case $GORILLA_HOOK_TOOL_INPUT in")) {
			t.Errorf("%s example is not sh:\n%s", goos, block)
		}
	}
}

// The live page follows the system the program is running on.
func TestLiveHooksPageMatchesTheRunningSystem(t *testing.T) {
	text := pageText(Hooks())
	isPS := strings.Contains(text, "$env:GORILLA_HOOK_TOOL_INPUT")
	if (runtime.GOOS == "windows") != isPS {
		t.Errorf("running on %s, but PowerShell example = %v", runtime.GOOS, isPS)
	}
	if !strings.Contains(text, config.GorillaConfigFile()) {
		t.Errorf("the live page does not show this machine's settings file %s", config.GorillaConfigFile())
	}
}

func TestHooksPageStatesTheRules(t *testing.T) {
	text := pageText(HooksPage(nil, "x", "", "windows"))
	for _, want := range []string{
		"exit code other than 0", "NOT run", "refused by hook, not run", "cannot be started",
		"only notify", "next time it starts", "timeout_seconds", "30", "600",
		"GORILLA_HOOK_EVENT", "GORILLA_HOOK_TOOL", "GORILLA_HOOK_TOOL_INPUT", "GORILLA_HOOK_SESSION",
		"GORILLA_HOOK_TOOL_RESULT", "GORILLA_HOOK_TOOL_ERROR", "GORILLA_HOOK_FINISH_REASON",
		"powershell.exe",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("the /hooks page does not state %q", want)
		}
	}
}

// Code lines are copied, so the page never wraps them; they must therefore fit
// an 80-column terminal as written. Live paths are excepted: the dialog cuts
// those where it must.
func TestCodeLinesFitEightyColumns(t *testing.T) {
	pages := []Page{Editor(), Helpers(), HooksPage(nil, "p", "", "windows"), HooksPage(nil, "p", "", "linux")}
	for _, p := range pages {
		for _, l := range codeLines(p) {
			if w := ansi.StringWidth(l); w > CodeWidthMax {
				t.Errorf("/%s code line is %d columns, over %d: %q", p.Name, w, CodeWidthMax, l)
			}
		}
	}
}

func TestByNameReachesEveryPage(t *testing.T) {
	for name, want := range map[string]string{
		"editor": "editor", "/acp": "editor", "helpers": "helpers", "ROLES": "helpers", "hooks": "hooks",
	} {
		p, ok := ByName(name)
		if !ok || p.Name != want {
			t.Errorf("ByName(%q) = %q, %v; want %q", name, p.Name, ok, want)
		}
	}
	if _, ok := ByName("help"); ok {
		t.Error("ByName(help) found a page")
	}
}

func TestWrapKeepsEveryLineWithinTheWidth(t *testing.T) {
	long := "see the file C:\\Users\\someone-with-a-long-name\\AppData\\Roaming\\deep\\deeper\\gorilla-opencode\\config.json for details"
	for _, w := range []int{20, 36, 56} {
		for _, l := range append(Wrap(long, w), Wrap("  * "+long, w)...) {
			if ansi.StringWidth(l) > w {
				t.Errorf("width %d: line %q is %d columns", w, l, ansi.StringWidth(l))
			}
		}
		for _, l := range HardWrap(long, w) {
			if ansi.StringWidth(l) > w {
				t.Errorf("HardWrap width %d: %q", w, l)
			}
		}
	}
	if got := strings.Join(HardWrap(long, 20), ""); got != long {
		t.Error("HardWrap lost characters")
	}
}

func TestPlainTextPrintsCodeExactly(t *testing.T) {
	out := Editor().PlainText(80)
	if !strings.Contains(out, strings.Join(ZedBlock, "\n")) {
		t.Errorf("plain text does not carry the Zed block verbatim:\n%s", out)
	}
	for _, l := range strings.Split(out, "\n") {
		if ansi.StringWidth(l) > 80 {
			t.Errorf("plain line over 80 columns: %q", l)
		}
	}
}
