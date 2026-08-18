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

import "strings"

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

	fields := strings.Fields(raw)
	for i := 0; i < len(fields); i++ {
		f := fields[i]

		if !strings.HasPrefix(f, "-") {
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
			if i+1 < len(fields) && !strings.HasPrefix(fields[i+1], "-") {
				i++
				return fields[i]
			}
			return ""
		}

		switch name {
		case "focus", "level", "depth":
			if v, ok := focusAliases[strings.ToLower(takesNext())]; ok {
				req.Focus = v
			} else {
				req.Unknown = append(req.Unknown, f)
			}
		case "diff", "changes", "since":
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

	req.Path = strings.Join(positional, " ")
	return req
}

// unknownReviewOptionMessage names the typo and the options that do exist.
// Guessing at what was meant would be worse: a review is expensive enough that
// running the wrong one wastes real time.
func unknownReviewOptionMessage(unknown []string) string {
	return "Don't know the option " + strings.Join(unknown, " ") +
		". Try: /review --quick, --security, --full, --diff HEAD, or a path."
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
			"that this skips the security stages entirely.\n")
	case "security":
		b.WriteString("The user asked for a SECURITY review: pass focus=\"security\".\n")
	case "full":
		b.WriteString("The user asked for a FULL review: pass focus=\"full\".\n")
	}

	switch {
	case req.Diff != "":
		b.WriteString("Scope it to changes: pass diff=\"" + req.Diff + "\".\n")
	case req.Path == "":
		b.WriteString("If this is a git repository with uncommitted or recent changes, pass " +
			"diff=\"HEAD\" so the review is scoped to what changed rather than every tracked " +
			"file.\n")
	}

	b.WriteString("\nWhen it returns: read the trust block FIRST and tell the user plainly which " +
		"analysers did not run and what the chosen depth skipped, because an empty findings " +
		"list is not the same as clean code. Start from the corroborated findings. Then READ " +
		"the code yourself for the things static analysis cannot see — wrong logic, broken " +
		"invariants, swallowed errors — and say explicitly that you did, and what you found.")
	return b.String()
}
