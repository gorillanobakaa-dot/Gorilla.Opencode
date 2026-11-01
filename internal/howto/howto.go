// Package howto holds the how-to cards: embedded markdown the model reads ON
// DEMAND (the files tool's howto action), never in every prompt.
//
// GORILLA (2026-10-27, job 06). Long knowledge (robocopy's switches and exit
// codes, PowerShell file operations, the embedded toolbox) does not belong in
// the system prompt: it rides every turn, and a small local model's context is
// small. The prompt carries one line naming the cards that apply HERE; a card
// is a tool call away.
//
// A card is offered only where it is true: the Windows cards on Windows,
// PowerToys only when it is installed, Sysinternals only when on PATH, a
// toolbox card only when that helper is embedded in this build.
package howto

import (
	"embed"
	"sort"
	"strings"

	"github.com/opencode-ai/opencode/internal/hostprobe"
)

//go:embed cards/*.md
var cardsFS embed.FS

// windowsCards apply on every Windows machine.
var windowsCards = []string{"robocopy", "powershell-files", "attributes-acl-registry", "services-events-tasks"}

// Available lists the cards that apply to this machine, in a stable order.
func Available() []string { return availableFor(hostprobe.Get()) }

func availableFor(f hostprobe.Facts) []string {
	out := []string{"everyday"}
	if f.OS == "windows" {
		out = append(out, windowsCards...)
		if f.PowerToys != "" {
			out = append(out, "powertoys")
		}
		if len(f.Sysinternals) > 0 {
			out = append(out, "sysinternals")
		}
	}
	tb := append([]string(nil), f.Toolbox...)
	sort.Strings(tb)
	for _, name := range tb {
		if Exists(name) {
			out = append(out, name)
		}
	}
	return out
}

// Exists reports whether a card of that name is embedded at all.
func Exists(name string) bool {
	_, err := cardsFS.ReadFile("cards/" + name + ".md")
	return err == nil
}

// Get returns a card that applies here. ok is false for an unknown name or a
// card that does not apply to this machine (PowerToys when it is not
// installed): the model is told what IS available instead.
func Get(name string) (string, bool) { return getFor(hostprobe.Get(), name) }

func getFor(f hostprobe.Facts, name string) (string, bool) {
	name = strings.ToLower(strings.TrimSuffix(strings.TrimSpace(name), ".md"))
	for _, a := range availableFor(f) {
		if a == name {
			b, err := cardsFS.ReadFile("cards/" + name + ".md")
			return string(b), err == nil
		}
	}
	return "", false
}

// PromptLine is the one line of the system prompt that names the cards, or ""
// when none apply.
func PromptLine() string { return promptLineFor(hostprobe.Get()) }

func promptLineFor(f hostprobe.Facts) string {
	a := availableFor(f)
	if len(a) == 0 {
		return ""
	}
	return "How-to cards (read one with the files tool: action=howto, path=NAME): " + strings.Join(a, ", ")
}
