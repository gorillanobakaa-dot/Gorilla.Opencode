// GORILLA (2026-09-02): "how do I use this?" answered by the command itself.
//
// The reference has always existed — every command carries a full explanation
// in internal/commands — but the only way to reach it was to open /help and
// scroll to the right row among thirty. So a user who typed /review got a
// twenty-minute analyser run instead of an explanation, and a user who typed
// /port got an operation. Neither told them what the command was for, what it
// needed, or what it would cost.
//
// Now `/review help` and `/port help` open the reference already showing that
// command. The commands themselves are unchanged: /review with no arguments
// still reviews the current folder, /port with none still inspects. Only an
// explicit request for help is intercepted, so nothing anyone already relies
// on changes behaviour.
package tui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

// wantsCommandHelp reports whether the argument string is a request for an
// explanation rather than work.
//
// The spellings are the ones people actually type. `-h` and `--help` come from
// every CLI; `help` and `?` come from chat commands and from the way this
// program's own /help is reached. Getting this wrong in the permissive
// direction is cheap — the user sees a help page they can dismiss — and in the
// restrictive direction is not, because `--help` read as a path is how
// `/review --deep` came to review a folder called "--deep".
func wantsCommandHelp(args string) bool {
	switch strings.ToLower(strings.TrimSpace(args)) {
	case "help", "--help", "-h", "-help", "?", "/?":
		return true
	}
	return false
}

// explainCommand opens the reference focused on one command.
func (a *appModel) explainCommand(name string) tea.Cmd {
	a.commandHelp.Init()
	a.commandHelp.SetSize(a.width, a.height)
	a.commandHelp.FocusCommand(name)
	a.showCommandHelp = true
	return nil
}
