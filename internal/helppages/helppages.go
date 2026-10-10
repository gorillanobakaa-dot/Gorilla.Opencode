// Package helppages holds the long explanation pages behind /editor, /helpers,
// /hooks and /peers (the last added 2026-10-10, see peers.go).
//
// GORILLA (2026-10-10): v0.1.144 added three capabilities that a user could not
// discover from inside the program: editor mode (`gorilla-opencode acp`), helper
// roles (the `role` parameter of the agent tool) and lifecycle hooks in
// config.json. They were described only in the README and in
// docs/EDITOR-ROLES-HOOKS.dual-track.md, which nobody running the program is
// looking at. Each now has a slash command that opens one of these pages.
//
// The pages live here, not in the TUI, because two front ends show them: the
// full interface draws them in a scrollable dialog (internal/tui/components/
// dialog/infopage.go) and plain mode prints them as ordinary text
// (internal/plain). One source, so the two cannot say different things.
//
// A page is a list of lines with a kind. Text lines are paragraphs and are
// wrapped by whoever shows them; Code lines are printed exactly as written,
// because they are meant to be copied, and every one is kept short enough to
// fit an 80-column terminal without wrapping (TestCodeLinesFitEightyColumns).
package helppages

import (
	"fmt"
	"runtime"
	"strings"

	"github.com/charmbracelet/x/ansi"
	"github.com/opencode-ai/opencode/internal/config"
)

// Kind picks how a line is shown.
type Kind int

const (
	Text    Kind = iota // a paragraph, wrapped to the window
	Heading             // a section title
	Code                // shown exactly as written, never wrapped by the page
	Note                // a quieter paragraph
)

// Line is one line of a page.
type Line struct {
	Kind Kind
	Text string
}

// Page is one explanation page.
type Page struct {
	// Name is the command that opens it, without the slash.
	Name  string
	Title string
	Lines []Line
}

// CodeWidthMax is the widest a Code line may be: an 80-column terminal less the
// dialog's padding of two columns on each side.
const CodeWidthMax = 76

// ByName returns the page a command opens, built now (the /hooks and /helpers
// pages read live settings). False for a name that has no page.
func ByName(name string) (Page, bool) {
	switch strings.TrimPrefix(strings.ToLower(strings.TrimSpace(name)), "/") {
	case "editor", "acp":
		return Editor(), true
	case "helpers", "roles":
		return Helpers(), true
	case "hooks":
		return Hooks(), true
	case "peers":
		return Peers(), true
	}
	return Page{}, false
}

// builder collects lines.
type builder struct{ lines []Line }

func (b *builder) h(s string) { b.lines = append(b.lines, Line{Heading, s}) }
func (b *builder) p(s string) { b.lines = append(b.lines, Line{Text, s}) }
func (b *builder) n(s string) { b.lines = append(b.lines, Line{Note, s}) }
func (b *builder) blank()     { b.lines = append(b.lines, Line{Text, ""}) }
func (b *builder) code(s ...string) {
	for _, l := range s {
		b.lines = append(b.lines, Line{Code, l})
	}
}

// ZedBlock is the Zed settings.json entry, one key per line so that no line is
// wider than an 80-column terminal. Whitespace aside it is exactly
//
//	"agent_servers": { "Gorilla OpenCode": { "type": "custom", "command": "gorilla-opencode", "args": ["acp"] } }
//
// which is the form the README and `gorilla-opencode acp --help` print.
var ZedBlock = []string{
	`"agent_servers": {`,
	`  "Gorilla OpenCode": {`,
	`    "type": "custom",`,
	`    "command": "gorilla-opencode",`,
	`    "args": ["acp"]`,
	`  }`,
	`}`,
}

// Editor is the /editor page.
func Editor() Page {
	var b builder
	b.p("Besides this terminal window, Gorilla OpenCode can work inside a code editor that " +
		"speaks the Agent Client Protocol (ACP), such as Zed or a JetBrains editor. The editor " +
		"starts the program itself, in the background, as gorilla-opencode acp, and you type your " +
		"questions in the editor's agent panel instead of here.")
	b.blank()

	b.h("First, choose a provider here")
	b.p("Editor mode has no setup screen of its own. It uses the provider, model, keys and " +
		"settings you chose in this terminal program (/providers or /connect). If none was ever " +
		"chosen, the editor reports: no AI provider is configured.")
	b.blank()

	b.h("Zed")
	b.p("Open Zed's settings.json and add this block inside its outermost { }, with a comma " +
		"after the entry before it:")
	b.code(ZedBlock...)
	b.p("Then open Zed's agent panel and choose Gorilla OpenCode. If the program is not on your " +
		"PATH (the list of folders your computer searches for programs), put its full path after " +
		"\"command\" instead; on Windows write every backslash twice, as in C:\\\\Tools\\\\gorilla-opencode.exe.")
	b.blank()

	b.h("JetBrains editors")
	b.p("A JetBrains editor that supports ACP agents asks for the same two things when you add a " +
		"custom agent: the command gorilla-opencode and the argument acp. Where that setting is " +
		"depends on the editor and its version; look for agents in its AI settings.")
	b.blank()

	b.h("What the editor shows you")
	b.p("  * Each action as it happens: the file being read, the command being run, the change being made.")
	b.p("  * Every permission question this program would ask you here. No answer, a cancelled " +
		"answer or a closed editor counts as no.")
	b.p("  * At the end of each answer, the program's own record of what really ran, beginning " +
		"\"--- what actually ran\". The program writes it, not the AI, and a helper's actions are " +
		"listed under that helper.")
	b.blank()

	b.h("Not supported yet")
	b.p("  * Picking up an old conversation: each time the editor starts the program, a new conversation begins.")
	b.p("  * Tools your editor offers to assistants: this program uses only its own.")
	b.p("  * Pictures, such as a pasted screenshot, in a question.")
	b.p("The editor is told plainly that these are not supported.")
	b.blank()

	b.h("What leaves your machine")
	b.p("Nothing new. Editor mode adds no account and sends nothing anywhere this terminal " +
		"program would not send it. Your own checks (/hooks) run in editor mode too.")
	return Page{Name: "editor", Title: "Using Gorilla OpenCode inside your code editor", Lines: b.lines}
}

// helperLimitNow is the helper limit from /context, in words.
func helperLimitNow() string {
	switch n := config.MaxSubAgents(); {
	case n == config.SubAgentsNuclear:
		return "Right now helpers are switched OFF there, so the AI does every job itself."
	case n == config.SubAgentsUnlimited:
		return "Right now there is no limit."
	case n == 1:
		return "Right now the limit is 1 helper per answer."
	default:
		return fmt.Sprintf("Right now the limit is %d helpers per answer.", n)
	}
}

// Helpers is the /helpers page.
func Helpers() Page {
	var b builder
	b.p("The AI can send out a helper: a second, short conversation that does one job and " +
		"reports back to it. Every helper has one of three jobs. There is no command to start " +
		"one: ask in ordinary words and name the kind you want. If you name none, the AI decides " +
		"whether to use a helper, and an unnamed helper is an explore helper.")
	b.blank()

	b.h("explore: looks, changes nothing")
	b.p("Searches and reads files to find something out. It cannot change anything. Type, for example:")
	b.code("  have an explore helper find where the settings file is read")
	b.blank()

	b.h("plan: writes a plan, changes nothing")
	b.p("Reads the project and comes back with a numbered, file-by-file plan: what to change, " +
		"where, and how to check it worked. It cannot change anything. Type, for example:")
	b.code("  use a plan helper to work out how to add a dark theme")
	b.blank()

	b.h("coder: changes files and runs commands")
	b.p("Does the work: edits files and runs commands, and reports every file it changed and " +
		"every command it ran. It asks your permission before each change, exactly as the main " +
		"AI does, and it cannot send out helpers of its own. Type, for example:")
	b.code("  send a coder helper to rename load_config to read_config")
	b.blank()

	b.h("What stays the same")
	b.p("  * Every change still asks you first. If you switched the questions off for this " +
		"conversation with /yolo, that covers helpers too.")
	b.p("  * /tasks shows the helpers that are running, each with its kind in brackets, such as " +
		"[plan], and lets you stop one.")
	b.p("  * The record printed under each answer lists every action a helper took, indented under that helper.")
	b.p("  * explore and plan run on the model the program uses for helpers; coder runs on the " +
		"main conversation's model, so it costs what the main conversation costs.")
	b.blank()

	b.h("How many helpers")
	b.p("Each helper is a model conversation of its own, so it costs quota or money. /context " +
		"sets how many one answer may use (the GORILLA AGENTS/SUBAGENTS row; [ and ] change it). " +
		helperLimitNow())
	return Page{Name: "helpers", Title: "Helpers: three kinds, and how to ask for one", Lines: b.lines}
}

// Hooks is the /hooks page for this machine: the hooks loaded now, the real
// settings-file paths, and the example for the running operating system.
func Hooks() Page {
	return HooksPage(config.Hooks(), config.GorillaConfigFile(), config.LocalConfigFile(), runtime.GOOS)
}

// HooksPage builds the /hooks page from its inputs, so a test can give it any
// set of hooks and any operating system.
func HooksPage(hooks []config.Hook, settingsFile, folderFile, goos string) Page {
	var b builder
	b.p("A hook is a command you write yourself, which the program runs at one of three moments: " +
		"before the AI uses a tool (and able to stop it), after a tool has run, and when an " +
		"answer is finished. None ships with the program; one exists only if you write it.")
	b.blank()

	b.h("Loaded now")
	if len(hooks) == 0 {
		b.p("None. No hook is configured, so nothing extra runs before or after the AI's actions.")
	} else {
		for i, h := range hooks {
			b.p(fmt.Sprintf("%d. %s", i+1, describeHook(h)))
			b.code("   " + h.Command)
		}
	}
	b.n("These are the hooks loaded when the program started. A change to the file takes effect the next time it starts.")
	b.blank()

	b.h("Where to write them")
	b.p("In the settings file on this computer:")
	b.code("  " + settingsFile)
	folder := "a file named .gorilla-opencode.json in the folder the program starts in"
	if folderFile != "" {
		folder = "the file " + folderFile
	}
	b.p("Only that file counts. Hooks written in " + folder + " are ignored, and the program " +
		"says so when it starts: a folder you cloned from someone else must not be able to run " +
		"commands as you, or remove your own checks.")
	b.blank()

	windows := goos == "windows"
	if windows {
		b.h("Example for Windows (the commands are PowerShell)")
	} else {
		b.h("Example for this system (the commands are sh)")
	}
	b.p("Put this inside the outermost { } of the settings file, with a comma after the entry " +
		"before it. If there is already a \"hooks\" list, add the three entries to it instead.")
	b.code(exampleFor(windows)...)
	b.p("The first stops any shell command containing the words git push. The second adds the " +
		"name of every tool that ran to gorilla-hooks.log in your home folder. The third beeps " +
		"when an answer is finished.")
	b.blank()

	b.h("The rules")
	b.p("  * before_tool: if the command ends with any exit code other than 0, the tool is NOT " +
		"run, the AI is told why, and the record under the answer says \"refused by hook, not run\". " +
		"A hook that takes longer than its timeout, or cannot be started, refuses too.")
	b.p("  * after_tool and turn_end only notify. They cannot stop or change anything.")
	b.p("  * \"tools\" limits a hook to the tools named, such as bash, edit or write. Leave it out " +
		"to cover every tool. turn_end ignores it.")
	b.p(fmt.Sprintf("  * \"timeout_seconds\" is %d if left out, and at most %d.",
		config.HookDefaultTimeoutSeconds, config.HookMaxTimeoutSeconds))
	if windows {
		b.p("  * The command runs through powershell.exe -NoProfile -NonInteractive -Command, in the " +
			"working folder, as you, with your permissions.")
	} else {
		b.p("  * The command runs through sh -c, in the working folder, as you, with your permissions.")
	}
	b.p("  * A hook with a mistake in it (an unknown event, an empty command, a timeout out of " +
		"range) stops the program at start, with a message naming that hook.")
	b.p("  * A hook that runs but tests the wrong thing lets the action through, and nothing " +
		"tells you. A hook that sends tool output somewhere sends it there.")
	b.blank()

	b.h("What a hook is told")
	b.p("Every hook gets GORILLA_HOOK_EVENT and GORILLA_HOOK_SESSION.")
	b.p("before_tool and after_tool also get GORILLA_HOOK_TOOL (the tool's name) and " +
		"GORILLA_HOOK_TOOL_INPUT (what the AI asked the tool to do). The input is also written " +
		"in full to the hook's standard input; the variable is cut at 24 KB, and then " +
		"GORILLA_HOOK_TOOL_INPUT_TRUNCATED is 1.")
	b.p("after_tool also gets GORILLA_HOOK_TOOL_RESULT (the first 8 KB of the result, with " +
		"passwords and keys masked) and GORILLA_HOOK_TOOL_ERROR (1 if the tool failed, else 0).")
	b.p("turn_end also gets GORILLA_HOOK_FINISH_REASON (how the answer ended).")
	return Page{Name: "hooks", Title: "Your own checks around the AI's actions (hooks)", Lines: b.lines}
}

// describeHook is one loaded hook in words.
func describeHook(h config.Hook) string {
	tools := "every tool"
	switch {
	case h.Event == config.HookTurnEnd:
		tools = ""
	case len(h.Tools) > 0:
		tools = strings.Join(h.Tools, ", ")
	}
	s := h.Event
	if tools != "" {
		s += ", tools: " + tools
	}
	return fmt.Sprintf("%s, timeout: %d seconds, command:", s, h.Timeout())
}

// exampleFor is the ready-to-copy hooks list. Each command is valid JSON as
// written (no backslashes, no inner double quotes), so it can be pasted as is.
// The before_tool command goes on its own line because "command": and the
// value side by side would not fit 80 columns; JSON allows the break.
func exampleFor(windows bool) []string {
	gate := `if ($env:GORILLA_HOOK_TOOL_INPUT -match 'git push') { exit 1 }`
	logLine := `Add-Content ~/gorilla-hooks.log $env:GORILLA_HOOK_TOOL`
	beep := `[console]::beep(880, 300)`
	if !windows {
		// No quotes round the variable: the word after case is never split
		// or globbed, and leaving them out keeps the line free of JSON escapes.
		gate = `case $GORILLA_HOOK_TOOL_INPUT in *'git push'*) exit 1;; esac`
		logLine = `echo $GORILLA_HOOK_TOOL >> ~/gorilla-hooks.log`
		beep = `tput bel > /dev/tty`
	}
	return []string{
		`"hooks": [`,
		`  {`,
		`    "event": "before_tool",`,
		`    "tools": ["bash"],`,
		`    "command":`,
		`      "` + gate + `"`,
		`  },`,
		`  {`,
		`    "event": "after_tool",`,
		`    "command": "` + logLine + `"`,
		`  },`,
		`  {`,
		`    "event": "turn_end",`,
		`    "command": "` + beep + `",`,
		`    "timeout_seconds": 5`,
		`  }`,
		`]`,
	}
}

// PlainText renders a page as ordinary text for plain mode: paragraphs wrapped
// to width, code lines exactly as written.
func (p Page) PlainText(width int) string {
	if width < 40 {
		width = 40
	}
	var out []string
	out = append(out, p.Title, strings.Repeat("=", min(len([]rune(p.Title)), width)), "")
	for _, l := range p.Lines {
		switch l.Kind {
		case Code:
			out = append(out, l.Text)
		case Heading:
			out = append(out, l.Text, strings.Repeat("-", min(len([]rune(l.Text)), width)))
		default:
			if strings.TrimSpace(l.Text) == "" {
				out = append(out, "")
				continue
			}
			out = append(out, Wrap(l.Text, width)...)
		}
	}
	return strings.Join(out, "\n")
}

// Wrap breaks a paragraph at spaces to at most width columns. A paragraph that
// opens with "  * " keeps its later lines indented under the text, so a bullet
// reads as one item.
//
// Widths are display columns, not bytes or runes: a path on this machine may
// hold a user name in a script where one character takes two columns. A word
// wider than the line (a long path) is cut into pieces that fit, because a
// line wider than the terminal breaks the TUI's frame.
func Wrap(s string, width int) []string {
	if width < 8 {
		width = 8
	}
	lead := ""
	if strings.HasPrefix(s, "  * ") {
		lead = "    "
	}
	var words []string
	for _, w := range strings.Fields(s) {
		words = append(words, splitWide(w, width-len(lead))...)
	}
	if len(words) == 0 {
		return nil
	}
	first := ""
	if lead != "" {
		first = "  "
	}
	var out []string
	cur := first + words[0]
	for _, w := range words[1:] {
		if ansi.StringWidth(cur)+1+ansi.StringWidth(w) <= width {
			cur += " " + w
			continue
		}
		out = append(out, cur)
		cur = lead + w
	}
	return append(out, cur)
}

// HardWrap cuts one line into pieces of at most width display columns, for a
// Code line wider than the window. Continuation pieces are not indented: an
// indent would be copied as part of the value.
func HardWrap(s string, width int) []string {
	if ansi.StringWidth(s) <= width {
		return []string{s}
	}
	return splitWide(s, width)
}

// splitWide cuts a string into pieces no wider than width columns.
func splitWide(s string, width int) []string {
	if width < 2 {
		width = 2
	}
	if ansi.StringWidth(s) <= width {
		return []string{s}
	}
	var out []string
	var cur strings.Builder
	cw := 0
	for _, r := range s {
		rw := ansi.StringWidth(string(r))
		if cw+rw > width && cw > 0 {
			out = append(out, cur.String())
			cur.Reset()
			cw = 0
		}
		cur.WriteRune(r)
		cw += rw
	}
	if cur.Len() > 0 {
		out = append(out, cur.String())
	}
	return out
}
