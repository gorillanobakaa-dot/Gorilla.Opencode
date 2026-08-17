// GORILLA OVERRIDE: generates docs/COMMANDS.md from internal/commands.
//
// The doc is generated rather than written because a hand-maintained reference
// drifts on the first change — and this one drifted immediately. v0.1.90 added
// /osint, /yolo, /goal, /compact and /init, all described inside the program in
// `/help` and NOWHERE a reader could find them without launching it first. The
// owner noticed within hours of the release: "not documented and detailed".
//
// That is the closed door PHILOSOPHY.md exists to argue against, in miniature:
// someone deciding whether to download this cannot see what it does, and
// someone on a slow link should not have to install it to find out.
//
// A test compares the checked-in file against this output, so a new command
// cannot land undocumented — the same discipline as cmd/settings-doc, and the
// same reason: the registry is the single source of truth and the page must be
// derived from it, never maintained alongside it.
//
//	go run ./cmd/commands-doc > docs/COMMANDS.md
package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/opencode-ai/opencode/internal/commands"
)

func main() {
	var b strings.Builder

	b.WriteString("# Every command, and what it does\n\n")
	b.WriteString("Generated from `internal/commands/registry.go` by `go run ./cmd/commands-doc`.\n")
	b.WriteString("Do not edit by hand — a test compares this file against the registry, so a\n")
	b.WriteString("command cannot be added without appearing here.\n\n")
	b.WriteString("You can read the same list inside the program with **`/help`**, where the\n")
	b.WriteString("explanation for whichever command you have highlighted appears underneath it.\n")
	b.WriteString("Type a command by starting a message with `/`.\n\n")
	b.WriteString("**Nothing here costs money except where it says so.** Two commands spend real\n")
	b.WriteString("tokens on your behalf — `/research` and `/osint` — and both say what they will\n")
	b.WriteString("cost before they start. Everything else is local.\n\n")

	// The quick table first: someone scanning for "what can I type" should not
	// have to read the prose.
	b.WriteString("## At a glance\n\n")
	b.WriteString("| Type this | What happens |\n|---|---|\n")
	for _, g := range commands.GroupOrder {
		for _, c := range commands.InGroup(g) {
			name := "`/" + c.Name + "`"
			if c.Args != "" {
				name = "`/" + c.Name + " " + c.Args + "`"
			}
			b.WriteString(fmt.Sprintf("| %s | %s |\n", name, c.Summary))
		}
	}
	b.WriteString("\n")

	// Then the full explanation, grouped the way the in-app reference groups
	// them: by what the user is trying to DO, not by subsystem.
	for _, g := range commands.GroupOrder {
		rows := commands.InGroup(g)
		if len(rows) == 0 {
			continue
		}
		fmt.Fprintf(&b, "## %s\n\n", string(g))
		for _, c := range rows {
			heading := "/" + c.Name
			if c.Args != "" {
				heading += " " + c.Args
			}
			fmt.Fprintf(&b, "### `%s`\n\n", heading)
			if len(c.Aliases) > 0 {
				fmt.Fprintf(&b, "*Also: %s*\n\n", "`/"+strings.Join(c.Aliases, "`, `/")+"`")
			}
			fmt.Fprintf(&b, "**%s**\n\n", c.Summary)
			// Detail carries its own paragraph breaks; emit them as written so
			// the reference reads the same on the page as it does on screen.
			for _, para := range strings.Split(c.Detail, "\n\n") {
				para = strings.TrimSpace(para)
				if para != "" {
					fmt.Fprintf(&b, "%s\n\n", para)
				}
			}
		}
	}

	b.WriteString("---\n\n")
	b.WriteString("*This page is generated. If a command is missing from it, that is a bug in\n")
	b.WriteString("the program rather than in the documentation — the registry and this file are\n")
	b.WriteString("held together by a test.*\n")

	if _, err := os.Stdout.WriteString(b.String()); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
