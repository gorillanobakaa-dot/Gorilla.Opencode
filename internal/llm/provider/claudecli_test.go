package provider

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/opencode-ai/opencode/internal/llm/models"
	"github.com/opencode-ai/opencode/internal/message"
)

// stubCLI writes a program that prints the given file and exits 0, so the
// provider can be driven end to end without a network, an account, or the real
// `claude`. The fixture it replays is REAL captured output from claude 2.1.296
// (testdata/claudecli_stream.jsonl), not hand-written JSON: a fixture someone
// invented would only ever prove the parser agrees with its author.
func stubCLI(t *testing.T, fixture string) string {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("stub uses a shell script")
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "claude-stub")
	script := fmt.Sprintf("#!/bin/sh\ncat %q\n", fixture)
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		t.Fatalf("writing stub: %v", err)
	}
	return path
}

func newTestCLIClient(t *testing.T, binary string) *claudeCLIClient {
	t.Helper()
	c := newClaudeCLIClient(providerClientOptions{
		model:         models.ClaudeCLIModels[models.ClaudeCLISonnet],
		systemMessage: "You are a test agent.",
		claudeCLIOptions: []ClaudeCLIOption{
			WithClaudeCLIBinary(binary),
		},
	})
	return c.(*claudeCLIClient)
}

func cliUserMsg(session, text string) message.Message {
	m := message.Message{Role: message.User, SessionID: session}
	m.Parts = append(m.Parts, message.TextContent{Text: text})
	return m
}

// The parser must turn real captured CLI output into streamed deltas, a final
// answer, and real usage numbers. Asserting that deltas ARRIVED is the point:
// a provider that returns the right text in one lump looks identical in the
// finished message and feels broken on screen.
func TestClaudeCLIStreamParsesRealOutput(t *testing.T) {
	fixture, err := filepath.Abs("testdata/claudecli_stream.jsonl")
	if err != nil {
		t.Fatal(err)
	}
	c := newTestCLIClient(t, stubCLI(t, fixture))

	var deltas int
	var content strings.Builder
	var final *ProviderResponse
	for ev := range c.stream(context.Background(), []message.Message{cliUserMsg("s1", "count")}, nil) {
		switch ev.Type {
		case EventContentDelta:
			deltas++
			content.WriteString(ev.Content)
		case EventComplete:
			final = ev.Response
		case EventError:
			t.Fatalf("stream reported an error: %v", ev.Error)
		}
	}

	if deltas == 0 {
		t.Error("no content deltas were emitted, so the answer would appear all at once instead of streaming")
	}
	if final == nil {
		t.Fatal("stream ended without an EventComplete")
	}
	if content.Len() == 0 {
		t.Error("no content was assembled from the deltas")
	}
	if final.Content != content.String() {
		t.Errorf("final content does not match the streamed deltas:\n final=%q\ndeltas=%q", final.Content, content.String())
	}
	if final.FinishReason != message.FinishReasonEndTurn {
		t.Errorf("finish reason = %q, want %q", final.FinishReason, message.FinishReasonEndTurn)
	}
	// From the captured run: input 2, output 26, cache_read 4789, cache_creation 10173.
	if final.Usage.OutputTokens == 0 {
		t.Error("output tokens were not reported, so this turn would cost nothing in the receipt")
	}
	if final.Usage.CacheReadTokens == 0 {
		t.Error("cache-read tokens were not reported")
	}
}

// Usage totals in this wire format are CUMULATIVE for the turn and arrive twice,
// on message_delta and again on result. Summing them double-counts. This pins
// the replace-don't-add rule in applyCLIUsage.
func TestClaudeCLIUsageIsNotSummed(t *testing.T) {
	var got TokenUsage
	u := &cliUsage{InputTokens: 2, OutputTokens: 26, CacheReadInputTokens: 4789}
	applyCLIUsage(&got, u)
	applyCLIUsage(&got, u) // the same cumulative totals, reported again
	if got.OutputTokens != 26 {
		t.Errorf("output tokens = %d after two reports of the same total, want 26", got.OutputTokens)
	}
	if got.CacheReadTokens != 4789 {
		t.Errorf("cache-read tokens = %d, want 4789", got.CacheReadTokens)
	}
}

// --verbose and --tools "" are not cosmetic. Without --verbose the CLI emits no
// stream_event lines at all and the answer arrives in one lump; without
// --tools "" the child silently uses Claude Code's own Read/Edit/Bash, which
// would mean the user is testing Claude Code rather than this program.
func TestClaudeCLIArgsCarryTheLoadBearingFlags(t *testing.T) {
	c := newTestCLIClient(t, "claude")
	args := c.buildArgs(&cliSession{id: "abc"}, "hello", nil)
	joined := strings.Join(args, " ")

	for _, want := range []string{"--verbose", "--include-partial-messages", "--strict-mcp-config"} {
		if !strings.Contains(joined, want) {
			t.Errorf("args are missing %s: %v", want, args)
		}
	}
	if !hasFlagValue(args, "--tools", "") {
		t.Errorf("--tools is not set to the empty string, so Claude Code's own tools stay enabled: %v", args)
	}
	if !hasFlagValue(args, "--model", "sonnet") {
		t.Errorf("--model is not the alias: %v", args)
	}
	if !hasFlagValue(args, "--system-prompt", "You are a test agent.") {
		t.Errorf("this program's system prompt was not passed: %v", args)
	}
}

// A fresh conversation opens a named session; the next turn resumes it rather
// than starting again. Without this the CLI would lose all history between
// turns and every reply would answer the newest message with no context.
func TestClaudeCLIResumesRatherThanRestarting(t *testing.T) {
	c := newTestCLIClient(t, "claude")
	sess := c.sessionFor([]message.Message{cliUserMsg("s1", "first")})

	first := c.buildArgs(sess, "first", nil)
	if !hasFlagValue(first, "--session-id", sess.id) {
		t.Fatalf("the first turn did not name a session: %v", first)
	}
	if hasFlag(first, "--resume") {
		t.Errorf("the first turn tried to resume a session that does not exist yet: %v", first)
	}

	sess.sent = 1
	second := c.buildArgs(sess, "second", nil)
	if !hasFlagValue(second, "--resume", sess.id) {
		t.Errorf("the second turn did not resume the session: %v", second)
	}
	if hasFlag(second, "--session-id") {
		t.Errorf("the second turn tried to create the session again: %v", second)
	}
}

// Only the messages the CLI has not already seen are sent. Re-sending the whole
// transcript every turn would spend the subscription's five-hour window on text
// the CLI already holds, which is the scarce resource on this route.
func TestClaudeCLISendsOnlyTheNewMessages(t *testing.T) {
	msgs := []message.Message{
		cliUserMsg("s1", "first question"),
		cliUserMsg("s1", "second question"),
		cliUserMsg("s1", "third question"),
	}
	if got := renderTranscript(msgs[2:]); got != "third question" {
		t.Errorf("pending transcript = %q, want only the newest turn", got)
	}
	if got := renderTranscript(msgs); !strings.Contains(got, "first question") {
		t.Errorf("a fresh session must be seeded with the whole transcript, got %q", got)
	}
}

// A signed-out CLI must say what to do about it. "exit status 1" is the thing
// this replaces.
func TestClaudeCLIExplainsBeingSignedOut(t *testing.T) {
	err := describeCLIError(cliEnvelope{IsError: true, Result: "Not logged in · Please run /login"}, "")
	if err == nil {
		t.Fatal("a signed-out result was reported as success")
	}
	if !strings.Contains(err.Error(), "claude login") {
		t.Errorf("the error does not tell the user how to fix it: %q", err)
	}
}

// A full subscription window is not a billing problem and must not be described
// as one, because on this route there is no bill to pay to make it go away.
func TestClaudeCLIExplainsAFullUsageWindow(t *testing.T) {
	err := describeCLIError(cliEnvelope{IsError: true, APIErrorStatus: 429, Result: "rate limit"}, "")
	if err == nil {
		t.Fatal("a rate-limited result was reported as success")
	}
	low := strings.ToLower(err.Error())
	if !strings.Contains(low, "window") {
		t.Errorf("the error does not mention the usage window: %q", err)
	}
	if strings.Contains(low, "billing") || strings.Contains(low, "credit card") {
		t.Errorf("the error blames money, which this route never charges: %q", err)
	}
}

// Every cost field must stay zero. Populating them with API prices would show
// the user money they are not being charged, because the subscription already
// covers this.
func TestClaudeCLIModelsAreNotPricedPerToken(t *testing.T) {
	if len(models.ClaudeCLIModels) == 0 {
		t.Fatal("no Claude CLI models are registered")
	}
	for id, m := range models.ClaudeCLIModels {
		if m.CostPer1MIn != 0 || m.CostPer1MOut != 0 || m.CostPer1MInCached != 0 || m.CostPer1MOutCached != 0 {
			t.Errorf("%s carries a per-token price; this route bills nothing per token", id)
		}
		if m.Provider != models.ProviderClaudeCLI {
			t.Errorf("%s has provider %q", id, m.Provider)
		}
		if got, ok := models.SupportedModels[id]; !ok {
			t.Errorf("%s is not in the global registry, so the picker cannot show it", id)
		} else if got.APIModel != m.APIModel {
			t.Errorf("%s registered with APIModel %q, want %q", id, got.APIModel, m.APIModel)
		}
	}
}

// The alias, not the resolved model id, is what reaches --model. An alias always
// points at the current model of its family, so the list cannot rot when
// Anthropic ships the next one.
func TestClaudeCLIUsesAliasesNotPinnedModelIDs(t *testing.T) {
	aliases := map[string]bool{"opus": true, "fable": true, "sonnet": true, "haiku": true}
	for id, m := range models.ClaudeCLIModels {
		if !aliases[m.APIModel] {
			t.Errorf("%s sends APIModel %q, which is not one of the CLI's aliases; "+
				"a pinned model id goes stale the moment Anthropic ships a new one", id, m.APIModel)
		}
	}
}

func hasFlag(args []string, flag string) bool {
	for _, a := range args {
		if a == flag {
			return true
		}
	}
	return false
}

func hasFlagValue(args []string, flag, value string) bool {
	for i, a := range args {
		if a == flag && i+1 < len(args) {
			return args[i+1] == value
		}
	}
	return false
}
