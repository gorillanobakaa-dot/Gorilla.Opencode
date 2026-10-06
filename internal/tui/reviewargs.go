// GORILLA OVERRIDE (2026-08-18): argument parsing for /review.
//
// WHY THIS IS ITS OWN FILE AND ITS OWN TEST. The first version of /review took
// `msg.Args` and used the whole string as a path. `/review --deep` therefore
// asked the model to review a folder called "--deep".
//
// That is the SAME defect as `/osint --recover` earlier the same day: a flag
// read as content. That one was found by the owner running the documented
// command; this one was written hours later, after the lesson was filed and
// after a test was added to catch the class — because that test only checks
// that a command NAMED in prose exists in the registry. It has nothing to say
// about a command mishandling its own arguments.
//
// So the parser is separated from the dispatch, and the test types what a
// person actually types — flags before the path, flags after it, the American
// spelling, the abbreviations, and a bare word that is genuinely a directory
// called "security".
package tui

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/opencode-ai/opencode/internal/config"
)

// reviewRequest is what /review resolves to.
type reviewRequest struct {
	Path  string
	Focus string // "", "quick", "security", "full"
	Diff  string
	// Unknown holds anything flag-shaped that was not recognised. It is
	// reported back to the user rather than silently ignored or, worse, used as
	// a path — a mistyped flag must never become the thing under review.
	Unknown []string
}

// focusAliases maps what people type onto the three levels. Generous on
// purpose: the cost of accepting "--sec" is nothing, and the cost of rejecting
// it is a user who concludes the feature does not work.
var focusAliases = map[string]string{
	"quick": "quick", "fast": "quick", "light": "quick", "lint": "quick",
	"security": "security", "sec": "security", "secure": "security",
	"audit": "security", "vuln": "security", "vulns": "security",
	"full": "full", "deep": "full", "all": "full", "thorough": "full",
	"everything": "full",
}

// parseReviewArgs turns the raw argument string into a request.
//
// Flags may appear anywhere, with one dash or two, and `--focus=security`,
// `--focus security` and plain `--security` all work. Everything left over is
// the path.
func parseReviewArgs(raw string) reviewRequest {
	var req reviewRequest
	var positional []string

	tokens := reviewFields(raw)
	fields := make([]string, len(tokens))
	for i, tk := range tokens {
		fields[i] = tk.text
	}
	for i := 0; i < len(fields); i++ {
		f := fields[i]

		// A quoted word is content, whatever it starts with: quoting is how a
		// person says "this is the path, do not read it as anything else".
		if tokens[i].quoted || !strings.HasPrefix(f, "-") {
			positional = append(positional, f)
			continue
		}

		name := strings.TrimLeft(f, "-")
		value := ""
		if k, v, ok := strings.Cut(name, "="); ok {
			name, value = k, v
		}
		name = strings.ToLower(name)

		// A flag that takes its value as the next word.
		takesNext := func() string {
			if value != "" {
				return value
			}
			if i+1 < len(fields) && !tokens[i+1].quoted && !strings.HasPrefix(fields[i+1], "-") {
				i++
				return fields[i]
			}
			return ""
		}

		switch name {
		case "focus", "level", "depth":
			// GORILLA FIX (2026-10-05): `--focus banana` consumed "banana" and
			// then reported "Don't know the option --focus". The option is
			// known; the value is not. The value now travels with the flag so
			// the message can say which half was wrong.
			got := takesNext()
			if v, ok := focusAliases[strings.ToLower(got)]; ok {
				req.Focus = v
			} else {
				req.Unknown = append(req.Unknown, "--"+name+"="+got)
			}
		case "diff", "changes", "since":
			// GORILLA FIX (2026-10-05): the next word was taken as the ref
			// unconditionally, so `/review --diff internal/auth` became "the
			// whole folder, against a ref called internal/auth" and failed in
			// git. A bare --diff is documented as meaning HEAD, so a word that
			// is a path on disk and NOT something git can resolve is the path,
			// and the ref stays HEAD. A word that is both (a folder called
			// main, a branch called main) is the ref: that is what was typed
			// after --diff. Git is only asked when the path exists, so the
			// ordinary `--diff HEAD` never starts a process.
			if value == "" && i+1 < len(fields) && !tokens[i+1].quoted &&
				!strings.HasPrefix(fields[i+1], "-") &&
				reviewPathExists(fields[i+1]) && !reviewRefExists(fields[i+1]) {
				req.Diff = "HEAD"
				continue
			}
			if v := takesNext(); v != "" {
				req.Diff = v
			} else {
				// `--diff` with nothing after it means "what I have changed",
				// which is the overwhelmingly common intent.
				req.Diff = "HEAD"
			}
		default:
			if v, ok := focusAliases[name]; ok {
				req.Focus = v // --security, --quick, --deep
				continue
			}
			req.Unknown = append(req.Unknown, f)
		}
	}

	// A bare depth word is a depth, not a folder.
	//
	// `/review quick` used to build a prompt saying path="quick", so the model
	// went looking for a directory of that name and reviewed nothing. The dash
	// is the difference between --quick and quick, and nobody types the dash
	// reliably -- least of all for a word that reads as an adverb.
	//
	// Guarded by an existence check, because "full" and "audit" are perfectly
	// plausible directory names. A folder that is really there always wins: the
	// depth reading only applies when the alternative is reviewing something
	// that does not exist.
	if len(positional) == 1 && req.Focus == "" && !soleTokenQuoted(tokens, positional[0]) {
		word := strings.ToLower(positional[0])
		if depth, ok := focusAliases[word]; ok && !reviewPathExists(positional[0]) {
			req.Focus = depth
			positional = nil
		}
	}

	req.Path = strings.Join(positional, " ")
	return req
}

// reviewToken is one word of the argument string.
type reviewToken struct {
	text   string
	quoted bool
}

// reviewFields splits the arguments on spaces, keeping a quoted run together
// and dropping the quotes.
//
// GORILLA FIX (2026-10-05): this was strings.Fields. `/review "my project"`
// became the two words `"my` and `project"`, rejoined with the quote characters
// still in them, and the model was told to review a path that began with a
// double quote. No backslash escapes are interpreted: a Windows path is full of
// backslashes that mean nothing of the kind.
func reviewFields(raw string) []reviewToken {
	var out []reviewToken
	var cur strings.Builder
	var quote rune
	inWord, wasQuoted := false, false
	flush := func() {
		if inWord {
			out = append(out, reviewToken{text: cur.String(), quoted: wasQuoted})
		}
		cur.Reset()
		inWord, wasQuoted = false, false
	}
	for _, r := range raw {
		switch {
		case quote != 0:
			if r == quote {
				quote = 0
			} else {
				cur.WriteRune(r)
			}
		case r == '"' || r == '\'':
			// Only at the start of a word. An apostrophe in the middle of one
			// (O'Brien, don't) is a character, not a delimiter.
			if !inWord {
				quote, inWord, wasQuoted = r, true, true
			} else {
				cur.WriteRune(r)
			}
		case r == ' ' || r == '\t' || r == '\n' || r == '\r':
			flush()
		default:
			inWord = true
			cur.WriteRune(r)
		}
	}
	flush()
	return out
}

// soleTokenQuoted reports whether the one positional word was typed in quotes,
// in which case it is a path even if it spells a depth.
func soleTokenQuoted(tokens []reviewToken, word string) bool {
	for _, tk := range tokens {
		if tk.quoted && tk.text == word {
			return true
		}
	}
	return false
}

// reviewRefExists reports whether git, in the working directory, can resolve
// the word to a commit. A variable for the same reason as reviewPathExists.
// Anything that stops git answering — not installed, not a repository, slow —
// is "no": the caller only asks about a word that IS a path on disk, so "no"
// means it is treated as the path it is.
var reviewRefExists = func(ref string) bool {
	wd := workingDirOrEmpty()
	if wd == "" {
		return false
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "git", "-C", wd, "rev-parse", "--verify", "--quiet", ref+"^{commit}")
	return cmd.Run() == nil
}

// reviewPathExists is a variable so tests can decide what is on disk instead of
// creating directories named "quick" and "full" to find out.
var reviewPathExists = func(p string) bool {
	if _, err := os.Stat(p); err == nil {
		return true
	}
	// Relative to the working directory too: /review quick is typed from
	// wherever the user happens to be.
	//
	// config.WorkingDirectory panics when no config is loaded, which is the
	// normal state in a parser unit test. Recovering is right here rather than
	// making the parser require a loaded config: not knowing the working
	// directory means "cannot confirm this path exists", which is exactly the
	// false this returns.
	if wd := workingDirOrEmpty(); wd != "" {
		if _, err := os.Stat(filepath.Join(wd, p)); err == nil {
			return true
		}
	}
	return false
}

func workingDirOrEmpty() (dir string) {
	defer func() {
		if recover() != nil {
			dir = ""
		}
	}()
	return config.WorkingDirectory()
}

// unknownReviewOptionMessage names the typo and the options that do exist.
// Guessing at what was meant would be worse: a review is expensive enough that
// running the wrong one wastes real time.
func unknownReviewOptionMessage(unknown []string) string {
	// A depth flag with a value that is not a depth is a different mistake from
	// a flag that does not exist, and "don't know the option --focus" sends the
	// user looking for the wrong one.
	var notOptions []string
	for _, u := range unknown {
		name, value, hasValue := strings.Cut(strings.TrimLeft(u, "-"), "=")
		switch strings.ToLower(name) {
		case "focus", "level", "depth":
			if !hasValue || value == "" {
				return "--" + name + " needs a depth after it: quick, security or full." +
					"  Full explanation: /review help"
			}
			return "--" + name + " takes quick, security or full; \"" + value + "\" is none of them." +
				"  Full explanation: /review help"
		}
		notOptions = append(notOptions, u)
	}
	unknown = notOptions
	return "Don't know the option " + strings.Join(unknown, " ") +
		". Try: /review --quick, --security, --full, --diff HEAD, or a path." +
		"  Full explanation: /review help"
}

// reviewPrompt turns the parsed request into the instruction the agent runs.
//
// It instructs rather than calls, because analysers are half a review: the
// model must also read the changed code and say that it did. A command that
// printed findings and stopped would be the "looks complete, is half" failure
// the tool's own description warns about.
func reviewPrompt(req reviewRequest) string {
	where := "the current folder"
	if req.Path != "" {
		where = req.Path
	}

	var b strings.Builder
	b.WriteString("Review the code in " + where + " using the `review` tool.\n\n")
	if req.Path != "" {
		b.WriteString("Pass path=\"" + req.Path + "\".\n")
	}

	switch req.Focus {
	case "quick":
		b.WriteString("The user asked for a QUICK pass: pass focus=\"quick\". Tell them plainly " +
			"that this runs linters and formatters only — no static analysis, no security " +
			"tools, no secret scan — and name the analysers the result says were left out.\n")
	// GORILLA FIX (2026-10-06): security and full used to be the same run under
	// two names, so the prompt had nothing to say about either. Each now says
	// what its pass runs, in the same terms as the quick one, so the model can
	// tell the user — and can see that security is NOT the full review plus a
	// filter.
	case "security":
		b.WriteString("The user asked for a SECURITY review: pass focus=\"security\". Tell them plainly " +
			"that this runs only the secret scanners, the security analysers and the static " +
			"analysers, with the deep pass forced over every file — no linters, no formatters, " +
			"so it says nothing about style or dead code — and name the analysers the result " +
			"says were left out.\n")
	case "full":
		b.WriteString("The user asked for a FULL review: pass focus=\"full\". Tell them plainly that " +
			"this runs every analyser of every kind — linters, formatters, static analysis, " +
			"security tools and the secret scan — with the deep pass forced over every file. " +
			"It is the slowest pass and the only one that leaves nothing out by depth.\n")
	}

	switch {
	case req.Diff != "":
		b.WriteString("Scope it to changes: pass diff=\"" + req.Diff + "\".\n")
	case req.Path == "":
		b.WriteString("If this is a git repository with uncommitted or recent changes, pass " +
			"diff=\"HEAD\" so the review is scoped to what changed rather than every tracked " +
			"file. If the tool answers that nothing was in scope, nothing has changed against " +
			"that ref: say so and run it once more without diff.\n")
	}

	b.WriteString("\nWhen it returns: read the trust block FIRST and tell the user plainly which " +
		"analysers did not run and what the chosen depth skipped, because an empty findings " +
		"list is not the same as clean code. Start from the corroborated findings. Then READ " +
		"the code yourself for the things static analysis cannot see — wrong logic, broken " +
		"invariants, swallowed errors — and say explicitly that you did, and what you found.")
	return b.String()
}
