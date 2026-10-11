// GORILLA FIX (2026-10-11): command families, so one answer can cover one
// program's many invocations without covering anything else.
//
// # WHAT WAS MEASURED
//
// Session 9af3336f (local.nvidia/nemotron-3-ultra-550b-a55b, 58 minutes, the
// owner's question was "can you work with this harness"): 31 bash calls, 31
// permission prompts, 30 distinct commands. Sixteen of them were
//
//	python -c "import sys; sys.path.insert(0, r'C:\...\Fieldkit'); from fieldkit.cli import main; main(['host'])"
//	python -c "import sys; sys.path.insert(0, r'C:\...\Fieldkit'); from fieldkit.cli import main; main(['cards', 'list'])"
//	python -c "import sys; sys.path.insert(0, r'C:\...\Fieldkit'); from fieldkit.cli import main; main(['readiness'])"
//	...
//
// "Allow for session" is keyed on the exact command text (see bash.go, and
// permission.GrantKey for why that was the right default), so it could not
// help: every variant was a new question. The owner's verdict was that the
// session "defeated the purpose of having an AI".
//
// # THE RULE
//
// A family is a PREFIX of the command, cut at a word boundary, that the person
// reads on the dialog and can lengthen or shorten word by word before saying
// yes. A later command is inside the family when
//
//  1. it begins with the family, followed by whitespace or the end;
//  2. everything AFTER the family is the plain tail of that one command: no
//     second command (`;`, `&&`, `||`, `&`, newline), no redirection, no
//     subexpression (`$(`, backtick), no script block, no bare parenthesis —
//     with two read-only exceptions a person writes constantly, `2>&1` and a
//     pipe into a viewing cmdlet such as Select-Object or Select-String;
//  3. quoting is balanced by the end of the command.
//
// Rule 2 is the one that keeps a family honest: approving `go test` must not
// also approve `go test ./... ; python evil.py`. Quotes are tracked from the
// START of the command, because a family may end inside a quoted string (the
// Nemotron one does: everything after `main;` is still inside python's `-c`
// argument). That is disclosed on the dialog in so many words, because inside
// that string the model can write anything and python will run it — the same
// is true of approving the single command, but a family makes it repeatable.
//
// What a family never covers is decided by the permission package, not here:
// a command the dangerous-pattern scan marks irreversible, an egress request,
// a tainted turn. Banned commands are refused before any prompt exists.
//
// Whitespace-splitting is deliberately the ONLY tokenisation. A real shell
// parser would be more precise and would be wrong on the shell we are not
// running; the boundary rule plus the tail scan fail closed instead.
package tools

import (
	"strings"
	"unicode"

	"github.com/opencode-ai/opencode/internal/llm/tools/shell"
	"github.com/opencode-ai/opencode/internal/permission"
)

func init() {
	permission.RegisterFamilyMatcher(BashToolName, BashFamilyCovers)
}

// familyMinWords is the shortest family the suggestion will propose on its
// own. One word is a whole program (`python`); a person may still choose that
// with the dialog's keys, but it must be chosen, not defaulted into.
const familyMinWords = 2

// familyDefaultMaxWords caps the fallback suggestion (program + bare
// subcommand words) so `git log --oneline` proposes `git log`, not more.
const familyDefaultMaxWords = 3

// familySinkCmdlets may follow a pipe inside a family's tail: they only view
// what the command printed. Anything that can write (Out-File, tee), run a
// program (xargs, ForEach-Object, Where-Object — a script block) or edit
// (sed -i, awk) is absent on purpose.
var familySinkCmdlets = map[string]bool{
	"select-object": true, "select-string": true, "out-string": true, "measure-object": true,
	"sort-object": true, "format-table": true, "format-list": true, "format-wide": true,
	"head": true, "tail": true, "grep": true, "wc": true, "sort": true, "uniq": true,
	"cut": true, "tr": true, "findstr": true, "more": true,
}

// familyHarmlessRedirects are the redirections allowed in a tail: they merge
// or discard a stream and write no file.
var familyHarmlessRedirects = []string{"2>&1", "2>$null", "2>/dev/null"}

// familyWords returns the [start, end) byte offsets of each whitespace-
// delimited word of cmd.
func familyWords(cmd string) [][2]int {
	var out [][2]int
	start := -1
	for i, r := range cmd {
		if unicode.IsSpace(r) {
			if start >= 0 {
				out = append(out, [2]int{start, i})
				start = -1
			}
			continue
		}
		if start < 0 {
			start = i
		}
	}
	if start >= 0 {
		out = append(out, [2]int{start, len(cmd)})
	}
	return out
}

// familyWordText is the text of the i-th word.
func familyWordText(cmd string, w [2]int) string { return cmd[w[0]:w[1]] }

// isBareWord reports whether a word looks like a subcommand (`test`, `cards`,
// `list`) rather than a flag, a path or a value.
func isBareWord(w string) bool {
	if w == "" || strings.HasPrefix(w, "-") {
		return false
	}
	for _, r := range w {
		if !(unicode.IsLetter(r) || unicode.IsDigit(r) || r == '-' || r == '_') {
			return false
		}
	}
	return true
}

// BashFamilyCovers is the FamilyMatcher registered for the bash tool: does cmd
// fall inside family? See the file comment for the rule. The shell in use
// decides the quoting rules, because the two shells disagree about what a
// backslash and a backtick mean, and guessing wrong in the permissive
// direction is how a family would come to cover a second command.
func BashFamilyCovers(cmd, family string) bool {
	return commandFamilyCovers(cmd, family, shell.IsWindowsPowerShell())
}

func commandFamilyCovers(cmd, family string, powerShell bool) bool {
	if family == "" || strings.TrimSpace(family) != family {
		return false
	}
	if !strings.HasPrefix(cmd, family) {
		return false
	}
	if len(cmd) > len(family) && !unicode.IsSpace(rune(cmd[len(family)])) {
		return false // `go testx` is not in the family `go test`
	}
	// The WHOLE command must be one plain command, not just the tail: a
	// family that itself ends in `;` (`go test ./... ;`) would otherwise turn
	// every later `go test ./... ; <anything>` into "the same command with a
	// different tail". Families exist for single commands only.
	plain, _ := familyTailIsPlain(cmd, 0, powerShell)
	return plain
}

// familyTailIsPlain scans cmd from the start, tracking quotes, and reports
// whether everything from offset `from` on is the plain tail of one command.
// The second result says whether offset `from` lies inside a quoted string,
// which the dialog discloses.
func familyTailIsPlain(cmd string, from int, powerShell bool) (plain bool, familyEndsInsideQuote bool) {
	const (
		none = iota
		single
		double
	)
	quote := none
	i := 0
	seenFrom := false
	inTail := func() bool { return i >= from }
	for i < len(cmd) {
		if !seenFrom && i >= from {
			seenFrom = true
			familyEndsInsideQuote = quote != none
		}
		c := cmd[i]
		switch quote {
		case single:
			if c == '\'' {
				if powerShell && i+1 < len(cmd) && cmd[i+1] == '\'' {
					i += 2 // '' is an escaped quote in PowerShell
					continue
				}
				quote = none
			}
			i++
			continue
		case double:
			switch {
			case c == '"':
				if powerShell && i+1 < len(cmd) && cmd[i+1] == '"' {
					i += 2
					continue
				}
				quote = none
				i++
				continue
			case !powerShell && c == '\\':
				i += 2 // bash: \" \\ \$ \` inside double quotes
				continue
			case powerShell && c == '`':
				i += 2 // PowerShell: `" `$ inside double quotes
				continue
			case c == '$' && i+1 < len(cmd) && (cmd[i+1] == '(' || cmd[i+1] == '{'):
				// Both shells expand $( ) inside double quotes; bash expands
				// ${ } too and PowerShell treats ${ } as a variable. Refused
				// for both: a tail that needs it is not a plain tail.
				if inTail() {
					return false, familyEndsInsideQuote
				}
			case !powerShell && c == '`':
				if inTail() {
					return false, familyEndsInsideQuote // bash: command substitution
				}
			}
			i++
			continue
		}
		// Outside quotes.
		switch c {
		case '\'':
			quote = single
			i++
			continue
		case '"':
			quote = double
			i++
			continue
		case '\\':
			if !powerShell {
				i += 2 // escaped character
				continue
			}
		case '`':
			if powerShell {
				i += 2 // escaped character
				continue
			}
			if inTail() {
				return false, familyEndsInsideQuote
			}
		}
		if !inTail() {
			i++
			continue
		}
		switch c {
		case ';', '&', '\n', '\r', '<', '(', '{', '}', ')':
			return false, familyEndsInsideQuote
		case '$':
			if i+1 < len(cmd) && (cmd[i+1] == '(' || cmd[i+1] == '{') {
				return false, familyEndsInsideQuote
			}
		case '>':
			return false, familyEndsInsideQuote
		case '2':
			// `2>&1` and friends are allowed only as a whole word.
			if i == 0 || unicode.IsSpace(rune(cmd[i-1])) {
				matched := false
				for _, r := range familyHarmlessRedirects {
					if strings.HasPrefix(cmd[i:], r) && (len(cmd) == i+len(r) || unicode.IsSpace(rune(cmd[i+len(r)]))) {
						i += len(r)
						matched = true
						break
					}
				}
				if matched {
					continue
				}
			}
		case '|':
			if i+1 < len(cmd) && cmd[i+1] == '|' {
				return false, familyEndsInsideQuote
			}
			// The word after the pipe must be a viewing cmdlet.
			j := i + 1
			for j < len(cmd) && unicode.IsSpace(rune(cmd[j])) {
				j++
			}
			k := j
			for k < len(cmd) && !unicode.IsSpace(rune(cmd[k])) {
				k++
			}
			sink := strings.ToLower(cmd[j:k])
			if !familySinkCmdlets[sink] {
				return false, familyEndsInsideQuote
			}
			i = k
			continue
		}
		i++
	}
	if !seenFrom {
		familyEndsInsideQuote = quote != none
	}
	if quote != none {
		return false, familyEndsInsideQuote // unbalanced: fail closed
	}
	return true, familyEndsInsideQuote
}

// BashFamilyCandidates lists every family of cmd a person could choose, from
// one word up to all but the last, keeping only those that cover cmd itself
// (a tail with a chain operator rules out every family of that command). The
// dialog's [ and ] keys move through this list.
func BashFamilyCandidates(cmd string) []string {
	words := familyWords(cmd)
	var out []string
	for n := 1; n < len(words); n++ {
		f := cmd[:words[n-1][1]]
		if BashFamilyCovers(cmd, f) {
			out = append(out, f)
		}
	}
	return out
}

// BashFamilyNote is the sentence the dialog shows under a family when the
// family ends inside a quoted string, or "" when it does not.
func BashFamilyNote(cmd, family string) string {
	if !strings.HasPrefix(cmd, family) {
		return ""
	}
	_, inside := familyTailIsPlain(cmd, len(family), shell.IsWindowsPowerShell())
	if !inside {
		return ""
	}
	return "This family ends inside a quoted string: whatever the model writes there is handed to the program as its code or argument."
}

// SuggestCommandFamily proposes the family for cmd, given the commands this
// session has already prompted for (oldest first), or "" when none is worth
// proposing.
//
// First choice: the longest run of leading words cmd shares with an earlier
// prompt, if it is at least familyMinWords long — on the Nemotron session that
// is exactly the fieldkit prefix, found on the second prompt. Fallback: the
// program plus its bare subcommand words (`go test`, `git log`, `python -m
// pytest`), stretched to two words so a bare program name is never proposed
// unasked. A family that equals the whole command is not proposed: that is
// what "Allow for session" already does.
func SuggestCommandFamily(cmd string, earlier []string) string {
	words := familyWords(cmd)
	if len(words) < 2 {
		return ""
	}
	best := 0
	for i := len(earlier) - 1; i >= 0; i-- {
		e := earlier[i]
		ew := familyWords(e)
		n := 0
		for n < len(words) && n < len(ew) && familyWordText(cmd, words[n]) == familyWordText(e, ew[n]) {
			n++
		}
		if n == len(words) && n == len(ew) {
			continue // the identical command: a session grant's job
		}
		if n > best {
			best = n
		}
	}
	n := best
	if n < familyMinWords {
		// Fallback: the program and its bare subcommand words. A flag stops
		// the walk — `python -c <code>` must NOT fall back to `python -c`,
		// which would be "any code at all"; the only flag followed is `-m
		// <module>`, which names a program. When the walk yields a single
		// word there is no family worth proposing unasked.
		n = 1
		for i := 1; i < len(words) && n < familyDefaultMaxWords; i++ {
			w := familyWordText(cmd, words[i])
			if w == "-m" && i+1 < len(words) && isBareWord(familyWordText(cmd, words[i+1])) {
				n = i + 2
				i++
				continue
			}
			if !isBareWord(w) {
				break
			}
			n = i + 1
		}
	}
	if n >= len(words) {
		// The whole command, or all of it bar nothing: a family that equals
		// the command is a session grant. Back off one word if that still
		// leaves a family worth proposing.
		n = len(words) - 1
	}
	if n < familyMinWords {
		return ""
	}
	family := cmd[:words[n-1][1]]
	if !BashFamilyCovers(cmd, family) {
		return ""
	}
	return family
}
