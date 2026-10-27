package tools

import (
	"fmt"
	"strings"
)

// GORILLA (2026-10-27, job 06): "Your tools", one line per tool, in the system
// prompt. The owner's local models did not know Gorilla's OWN tools: a schema
// list is not the same as being told, in one place, what each tool is for. It is
// generated from the tool set the agent really built (internal/llm/agent/tools.go),
// so a tool added or switched off changes the table with it and it cannot drift.
//
// When deferral is on, the withheld tools are NOT repeated here: they are in the
// deferred catalogue right below it (DeferredCatalogueBlock), and the two
// together are every tool. Each line costs tokens on every turn, so it is the
// first sentence only, capped.

// toolTableLineMax caps one line's summary, in characters.
const toolTableLineMax = 72

// Summarizer is a tool whose one-line summary is not the first sentence of its
// description (bash: the edition of PowerShell it drives matters more than the
// generic wording).
type Summarizer interface {
	Summary() string
}

// ToolSummary is the one-line summary of a tool.
func ToolSummary(t BaseTool) string {
	if s, ok := t.(Summarizer); ok {
		return capWords(s.Summary(), toolTableLineMax)
	}
	return capWords(firstSentence(firstParagraph(t.Info().Description)), toolTableLineMax)
}

// capWords is s on one line, cut at a word boundary to at most max runes, with
// "..." when it was cut.
func capWords(s string, max int) string {
	s = strings.Join(strings.Fields(s), " ")
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	cut := string(r[:max])
	if i := strings.LastIndex(cut, " "); i > max/2 {
		cut = cut[:i]
	}
	return strings.TrimRight(cut, " ,;:") + "..."
}

// firstSentence cuts at the first ". " (or ".\n"); a description with no full
// stop is returned whole.
func firstSentence(s string) string {
	for i := 0; i+1 < len(s); i++ {
		if s[i] == '.' && (s[i+1] == ' ' || s[i+1] == '\n') {
			return s[:i+1]
		}
	}
	return s
}

// ToolTableBlock is the "Your tools" block. deferral says whether withheld
// tools are left to the deferred catalogue.
func ToolTableBlock(all []BaseTool, deferral bool) string {
	var b strings.Builder
	n := 0
	for _, t := range all {
		name := t.Info().Name
		if deferral && IsDeferrable(name) {
			continue
		}
		if n == 0 {
			b.WriteString("YOUR TOOLS (loaded now; call them by these names)\n")
		}
		n++
		fmt.Fprintf(&b, "  %-14s %s\n", name, ToolSummary(t))
	}
	return b.String()
}
