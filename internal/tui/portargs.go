// GORILLA (2026-09-02): argument parsing for /port.
//
// Separate from the dispatch switch in tui.go for the same reason
// reviewargs.go is: a nested switch inside that case reads to
// TestEveryDispatchedCommandIsDocumented as a fistful of undocumented slash
// commands. It is also the half worth testing, and testing it here does not
// require standing up a TUI.
package tui

import (
	"fmt"
	"strings"
)

// portRequest is what the user asked for, already separated into operation,
// paths and flags — so the prompt builder never has to guess whether a bare
// word was a folder or a misspelt option.
type portRequest struct {
	Op      string
	Tree    string
	Series  string
	Patch   string
	Onto    string
	Build   string
	Test    string
	Unknown []string
}

// portOps is the set /port accepts, matching patch_port.py's own --op values.
var portOps = map[string]string{
	"inspect":      "inspect",
	"forward":      "forward-port",
	"forward-port": "forward-port",
	"forwardport":  "forward-port",
	"back":         "backport",
	"backport":     "backport",
	"rebase":       "rebase",
	"refresh":      "refresh",
	"series":       "port-series",
	"port-series":  "port-series",
}

// parsePortArgs reads `/port [op] [--onto REF] [--series DIR] [--patch FILE]
// [--build CMD] [--test CMD] [folder]`.
//
// Flags that take a value accept both `--onto v6.12` and `--onto=v6.12`,
// because both get typed and silently dropping one is the flag-read-as-content
// defect /review already had once.
func parsePortArgs(raw string) portRequest {
	req := portRequest{}
	fields := strings.Fields(strings.TrimSpace(raw))

	takeValue := func(i *int, inline string) string {
		if inline != "" {
			return inline
		}
		if *i+1 < len(fields) {
			*i++
			return fields[*i]
		}
		return ""
	}

	for i := 0; i < len(fields); i++ {
		f := fields[i]

		if !strings.HasPrefix(f, "-") {
			// First bare word may name the operation; otherwise it is the tree.
			if req.Op == "" {
				if op, ok := portOps[strings.ToLower(f)]; ok {
					req.Op = op
					continue
				}
			}
			if req.Tree == "" {
				req.Tree = f
			} else {
				req.Unknown = append(req.Unknown, f)
			}
			continue
		}

		name, inline := f, ""
		if eq := strings.Index(f, "="); eq >= 0 {
			name, inline = f[:eq], f[eq+1:]
		}
		switch strings.ToLower(strings.TrimLeft(name, "-")) {
		case "onto":
			req.Onto = takeValue(&i, inline)
		case "series":
			req.Series = takeValue(&i, inline)
		case "patch":
			req.Patch = takeValue(&i, inline)
		case "build":
			req.Build = takeValue(&i, inline)
		case "test":
			req.Test = takeValue(&i, inline)
		case "inspect":
			req.Op = "inspect"
		case "forward", "forward-port":
			req.Op = "forward-port"
		case "back", "backport":
			req.Op = "backport"
		case "rebase":
			req.Op = "rebase"
		case "refresh":
			req.Op = "refresh"
		default:
			req.Unknown = append(req.Unknown, f)
		}
	}

	// Inspect is the default because it changes nothing. Someone who types
	// /port with no idea what it does should get a read-only answer, not a
	// rebase of the tree they are standing in.
	if req.Op == "" {
		req.Op = "inspect"
	}
	return req
}

func unknownPortOptionMessage(unknown []string) string {
	return fmt.Sprintf(
		"/port does not understand %s.\n\nIt takes: inspect, forward-port, backport, "+
			"rebase, refresh, series — plus --onto REF, --series DIR, --patch FILE, "+
			"--build CMD, --test CMD, and a folder.\n\nExample:\n"+
			"  /port forward-port --series ../patches --onto v6.12 --build \"make -j8\"",
		strings.Join(unknown, ", "))
}

// portPrompt turns the request into an instruction for the agent. Like
// /review it goes through the model rather than printing tool output
// directly, because whether a ported patch is CORRECT is a judgement about
// code, and a command that printed "applied" and stopped would be exactly the
// looks-complete-is-half failure the tool warns about.
func portPrompt(req portRequest) string {
	var b strings.Builder
	b.WriteString("Use the patch_port tool to ")
	switch req.Op {
	case "inspect":
		b.WriteString("INSPECT the patches and the tree without changing anything")
	case "port-series":
		b.WriteString("port a whole patch SERIES")
	default:
		b.WriteString(req.Op + " the patches")
	}

	if req.Tree != "" {
		fmt.Fprintf(&b, ", in the tree %q", req.Tree)
	}
	if req.Series != "" {
		fmt.Fprintf(&b, ", series %q", req.Series)
	}
	if req.Patch != "" {
		fmt.Fprintf(&b, ", patch %q", req.Patch)
	}
	if req.Onto != "" {
		fmt.Fprintf(&b, ", onto %q", req.Onto)
	}
	if req.Build != "" {
		fmt.Fprintf(&b, ", build command %q", req.Build)
	}
	if req.Test != "" {
		fmt.Fprintf(&b, ", test command %q", req.Test)
	}
	b.WriteString(".\n\n")

	b.WriteString("Then tell me, in plain language:\n" +
		"  - which patches applied, and HOW each one applied. applied-clean, " +
		"applied-three-way and applied-with-fuzz are NOT the same thing and must not " +
		"be reported as though they were.\n" +
		"  - for anything applied with fuzz, show me the actual diff for those files. " +
		"A fuzzed hunk was relocated by searching for context, so it can land in the " +
		"wrong place and still report success.\n" +
		"  - anything already present upstream, which should be dropped from the " +
		"series rather than forced in again.\n" +
		"  - any conflict, and what the decision actually is.\n")

	if req.Build == "" && req.Test == "" && req.Op != "inspect" {
		b.WriteString("\nI gave no build or test command, so nothing will be compiled or run. " +
			"Say so plainly rather than implying the port is known good, and tell me what " +
			"command would verify it.\n")
	}
	return b.String()
}
