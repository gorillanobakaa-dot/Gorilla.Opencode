package tools

// GORILLA OVERRIDE (2026-08-19): tool audit, bash.
//
// The first pass of this audit concluded bash was sound and said so publicly.
// The owner asked for it to be checked properly rather than reasoned about.
// It was not sound.

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/opencode-ai/opencode/internal/permission"
	"github.com/opencode-ai/opencode/internal/pubsub"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type allowAllPerms struct {
	*pubsub.Broker[permission.PermissionRequest]
}

func (p *allowAllPerms) GrantPersistant(permission.PermissionRequest)    {}
func (p *allowAllPerms) Grant(permission.PermissionRequest)              {}
func (p *allowAllPerms) Deny(permission.PermissionRequest)               {}
func (p *allowAllPerms) Request(permission.CreatePermissionRequest) bool { return true }
func (p *allowAllPerms) AutoApproveSession(string)                       {}
func (p *allowAllPerms) RevokeAutoApprove(string)                        {}
func (p *allowAllPerms) IsAutoApproved(string) bool                      { return true }
func (p *allowAllPerms) RegisterChildSession(string, string)             {}
func (p *allowAllPerms) SetUnattended(bool)                              {}

func bashRun(t *testing.T, cmd string) (ToolResponse, time.Duration) {
	t.Helper()
	ctx := context.WithValue(context.Background(), SessionIDContextKey, "audit")
	ctx = context.WithValue(ctx, MessageIDContextKey, "m")
	b := NewBashTool(&allowAllPerms{pubsub.NewBroker[permission.PermissionRequest]()})
	esc := strings.NewReplacer(`\`, `\\`, `"`, `\"`, "\n", `\n`).Replace(cmd)
	start := time.Now()
	r, err := b.Run(ctx, ToolCall{Input: `{"command":"` + esc + `"}`})
	require.NoError(t, err)
	return r, time.Since(start)
}

// MEASURED before the fix: `echo out; exit 1` waited the FULL DEFAULT TIMEOUT
// (one minute) and then reported "Command execution timed out or was
// interrupted".
//
// Every part of that is wrong except the delay. The command runs through
// `eval`, which executes in the CURRENT shell — so `exit` ends the persistent
// shell itself, the status file is never written, and the watcher polls for a
// file that is never coming. The command did not time out, was not
// interrupted, and had already produced its output.
//
// A model told "timed out" retries, raises the timeout, or reports a hang.
// It fixes the wrong thing, having waited a minute to be misled.
func TestACommandThatCallsExitIsNotReportedAsATimeout(t *testing.T) {
	r, elapsed := bashRun(t, `echo out; exit 1`)

	assert.Less(t, elapsed, 20*time.Second,
		"it waited %s — the shell died and nothing noticed until the timeout", elapsed)
	assert.NotContains(t, r.Content, "timed out",
		"a command that called exit was reported as a timeout")
	assert.Contains(t, r.Content, "shell session ended",
		"it must name what actually happened")
	assert.Contains(t, r.Content, "`exit`",
		"it must name the cause so the model can avoid it")
	assert.Contains(t, r.Content, "out",
		"the output produced before the shell ended is real and must be kept")
}

// The next command must work — a fresh shell is started.
func TestTheShellRecoversAfterACommandCallsExit(t *testing.T) {
	bashRun(t, `exit 3`)
	r, _ := bashRun(t, `echo still-working`)
	assert.Contains(t, r.Content, "still-working")
	assert.NotContains(t, r.Content, "shell session ended")
}

// The first audit pass claimed bash was sound. This is the evidence for the
// part of that claim which WAS true: an ordinary non-zero exit reports the real
// error and the real code, and is not flagged as a tool error — because grep
// exits 1 for "no match" and test exits 1 for "false", and flagging those would
// turn correct answers into errors.
func TestOrdinaryFailuresReportTheRealErrorAndExitCode(t *testing.T) {
	for _, c := range []struct{ name, cmd, wantOut, wantCode string }{
		{"missing path", `ls /definitely-not-here-xyz`, "No such file", "Exit code 2"},
		{"grep no match", `echo hello | grep zzz`, "", "Exit code 1"},
		{"command not found", `definitely-not-a-real-cmd-xyz`, "command not found", "Exit code 127"},
	} {
		r, _ := bashRun(t, c.cmd)
		assert.False(t, r.IsError, "%s: a non-zero exit is not necessarily a tool failure", c.name)
		if c.wantOut != "" {
			assert.Contains(t, r.Content, c.wantOut, "%s: the real error was lost", c.name)
		}
		assert.Contains(t, r.Content, c.wantCode, "%s: the exit code was lost", c.name)
	}
}

// A failing build must show its diagnostics — this is the case the whole tool
// exists for, and the one the honesty rules in the prompt depend on.
func TestAFailingBuildShowsItsDiagnostics(t *testing.T) {
	dir := t.TempDir()
	r, _ := bashRun(t, `cd `+dir+` && printf 'package main\nfunc main(){ undefinedThing }\n' > b.go && go vet b.go`)
	assert.Contains(t, r.Content, "undefined", "the compiler's message was lost")
	assert.Contains(t, r.Content, "Exit code")
}
