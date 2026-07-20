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
	Command string `json:"command"`
	Timeout int    `json:"timeout"`
}

type BashPermissionsParams struct {
	Command string `json:"command"`
	Timeout int    `json:"timeout"`
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

var safeReadOnlyCommands = []string{
	"ls", "echo", "pwd", "date", "cal", "uptime", "whoami", "id", "groups", "env", "printenv", "set", "unset", "which", "type", "whereis",
	"whatis", "uname", "hostname", "df", "du", "free", "top", "ps", "kill", "killall", "nice", "nohup", "time", "timeout",

	"git status", "git log", "git diff", "git show", "git branch", "git tag", "git remote", "git ls-files", "git ls-remote",
	"git rev-parse", "git config --get", "git config --list", "git describe", "git blame", "git grep", "git shortlog",

	"go version", "go help", "go list", "go env", "go doc", "go vet", "go fmt", "go mod", "go test", "go build", "go run", "go install", "go clean",
}

func bashDescription() string {
	bannedCommandsStr := strings.Join(bannedCommands, ", ")
	return fmt.Sprintf(`Executes a given bash command in a persistent shell session with optional timeout, ensuring proper handling and security measures.

Before executing the command, please follow these steps:

1. Directory Verification:
 - If the command will create new directories or files, first use the LS tool to verify the parent directory exists and is the correct location
 - For example, before running "mkdir foo/bar", first use LS to check that "foo" exists and is the intended parent directory

2. Security Check:
 - For security and to limit the threat of a prompt injection attack, some commands are limited or banned. If you use a disallowed command, you will receive an error message explaining the restriction. Explain the error to the User.
 - Verify that the command is not one of the banned commands: %s.

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
- VERY IMPORTANT: You MUST avoid using search commands like 'find' and 'grep'. Instead use Grep, Glob, or Agent tools to search. You MUST avoid read tools like 'cat', 'head', 'tail', and 'ls', and use FileRead and LS tools to read files.
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
		params.Timeout = MaxTimeout
	} else if params.Timeout <= 0 {
		params.Timeout = DefaultTimeout
	}

	if params.Command == "" {
		return NewTextErrorResponse("missing command"), nil
	}

	baseCmd := strings.Fields(params.Command)[0]
	for _, banned := range bannedCommands {
		if strings.EqualFold(baseCmd, banned) {
			return NewTextErrorResponse(fmt.Sprintf("command '%s' is not allowed", baseCmd)), nil
		}
	}

	isSafeReadOnly := false
	cmdLower := strings.ToLower(params.Command)

	for _, safe := range safeReadOnlyCommands {
		if strings.HasPrefix(cmdLower, strings.ToLower(safe)) {
			if len(cmdLower) == len(safe) || cmdLower[len(safe)] == ' ' || cmdLower[len(safe)] == '-' {
				isSafeReadOnly = true
				break
			}
		}
	}

	sessionID, messageID := GetContextValues(ctx)
	if sessionID == "" || messageID == "" {
		return ToolResponse{}, fmt.Errorf("session ID and message ID are required for creating a new file")
	}
	if !isSafeReadOnly {
		p := b.permissions.Request(
			permission.CreatePermissionRequest{
				SessionID:   sessionID,
				Path:        config.WorkingDirectory(),
				ToolName:    BashToolName,
				Action:      "execute",
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
	stdout, stderr, exitCode, interrupted, err := shell.Exec(ctx, params.Command, params.Timeout)
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
// progress lines (CC/CXX/AR/LD/…) that saturate the model's context and
// bury the one line that matters. When output is long AND looks like a
// build/compile log, we keep only the lines that carry signal — errors,
// warnings, linker failures, and the file:line they point at — plus a
// little context, and note how much was dropped. Opt out with
// OPENCODE_NO_LOG_FILTER=1. This is the SWE-agent finding: bounded,
// filtered tool output is the single biggest lever for build agents.
var (
	buildSignalRe = regexp.MustCompile(`(?i)(\berror\b:|fatal error:|undefined reference|undefined symbol|multiple definition|recipe for target .* failed|make(\[\d+\])?: \*\*\*|ld: |ld\.lld: |collect2:|linker command failed|cannot find|no such file|: warning:|warning generated|note: |panic:|Segmentation fault|failed with exit|Error \d)`)
	buildNoiseRe  = regexp.MustCompile(`(?i)^\s*(cc|cxx|ar|ld|as|ranlib|cpp|gen|host cc|host cxx|copy|install|strip|objcopy|compiling|building|checking|make(\[\d+\])?:\s+(entering|leaving|nothing to be done)|\[\s*\d+%\]|\d+/\d+\s)`)
	buildMarkerRe = regexp.MustCompile(`(?i)(\bgcc\b|\bclang\b|\bmake\b|\bmach\b|\bcargo\b|\bcmake\b|\bninja\b|\bmozconfig\b|CC\s|CXX\s|\.o\b|\.rlib\b)`)
	fileLineRe    = regexp.MustCompile(`^[^\s:][^:]*:\d+(:\d+)?:`)
)

func filterBuildLog(content string) string {
	if on, _ := strconv.ParseBool(os.Getenv("OPENCODE_NO_LOG_FILTER")); on {
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
			// include one line of context on each side
			for j := i - 1; j <= i+1; j++ {
				if j >= 0 && j < len(lines) && !keptIdx[j] && !buildNoiseRe.MatchString(lines[j]) {
					keptIdx[j] = true
					kept = append(kept, lines[j])
				}
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
	return fmt.Sprintf("[build log filtered: %d of %d lines were compile/progress noise; showing the %d signal lines. Set OPENCODE_NO_LOG_FILTER=1 for raw output.]\n%s",
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
