// GORILLA OVERRIDE: this file did not exist upstream.
//
// Every other provider in this package is an HTTP client. This one is not: it
// drives the user's own `claude` binary as a child process and reads its
// newline-delimited JSON. The reason is in internal/llm/models/claudecli.go and
// is not a stylistic preference: a Claude Pro or Max subscription is not an API
// credential, and the only way for a third-party program to call Anthropic with
// one is to claim on the wire to be Claude Code. Anthropic blocks that, and
// OpenCode (this project's upstream) was one of the tools broken when they
// started on 9 January 2026. Spawning the real, unmodified, already signed-in
// binary is the route their own terms describe, and it means this program never
// handles a credential at all.
//
// # WHAT THE WIRE LOOKS LIKE
//
// `claude -p --output-format stream-json --include-partial-messages --verbose`
// writes one JSON object per line. Measured against claude 2.1.296 on 2026-10-12:
//
//	system/hook_started, system/hook_response   housekeeping, ignored
//	system/init                                 session id, resolved model, apiKeySource
//	stream_event/message_start
//	stream_event/content_block_start            content_block.type: text | thinking | tool_use
//	stream_event/content_block_delta            delta.type: text_delta | thinking_delta | input_json_delta
//	stream_event/content_block_stop
//	stream_event/message_delta                  delta.stop_reason, usage
//	stream_event/message_stop
//	assistant                                   the whole assembled message, after the deltas
//	rate_limit_event                            subscription window utilisation
//	system/post_turn_summary
//	result/success                              is_error, result, usage, total_cost_usd
//
// The objects under "stream_event" are VERBATIM Anthropic Messages API SSE
// events, which is why the parsing below looks like an SSE reader with no SSE in
// sight. `--verbose` is mandatory: without it the stream_event lines are not
// emitted at all, and the first version of this file produced a working but
// completely un-streamed answer because of it.
//
// # WHAT IS DELIBERATELY NOT SENT
//
// No max-tokens. The CLI has no flag for it and the plan governs output length,
// so providerOptions.maxTokens is unused here. That mirrors the ChatGPT
// provider, where the backend rejects max_output_tokens outright.
//
// No temperature, no top_p. The CLI exposes neither.
package provider

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/opencode-ai/opencode/internal/llm/models"
	"github.com/opencode-ai/opencode/internal/llm/tools"
	"github.com/opencode-ai/opencode/internal/logging"
	"github.com/opencode-ai/opencode/internal/message"
)

// ClaudeCLIBinary is the program spawned. Resolved on PATH so a user who
// installed it anywhere sensible is found without configuration.
const ClaudeCLIBinary = "claude"

type claudeCLIOptions struct {
	// binary overrides the program name. Tests set it to a stub; nothing in
	// the application does.
	binary string
	// extraArgs are appended last. Tests use it; nothing in the application
	// does.
	extraArgs []string
}

type ClaudeCLIOption func(*claudeCLIOptions)

// WithClaudeCLIBinary points the provider at a different executable. This
// exists for tests: the live path must always be the user's real `claude`.
func WithClaudeCLIBinary(path string) ClaudeCLIOption {
	return func(o *claudeCLIOptions) { o.binary = path }
}

// cliSession is the bridge between one gorilla-opencode conversation and one
// `claude` session.
//
// WHY THIS STATE EXISTS. The CLI cannot be seeded with history: its
// --input-format=stream-json REPLAYS every user message as a real turn, which
// was measured on 2026-10-12 (three input messages produced two separate
// `result` events, the first of which actually answered a question from the
// middle of the transcript). So history is carried the way the CLI itself
// carries it, by resuming its own session, and this records how much of the
// gorilla-opencode transcript has already been handed over.
//
// The alternative, re-flattening the whole transcript into every prompt, was
// rejected: on this route the scarce resource is the subscription's five-hour
// window, not money, and re-sending the conversation every turn spends that
// window on text the CLI already has.
type cliSession struct {
	id   string // the uuid passed to --session-id, then to --resume
	sent int    // how many messages of the transcript it has been given
}

type claudeCLIClient struct {
	providerOptions providerClientOptions
	options         claudeCLIOptions

	mu   sync.Mutex
	sess map[string]*cliSession // gorilla-opencode session id -> CLI session
}

type ClaudeCLIClient ProviderClient

func newClaudeCLIClient(opts providerClientOptions) ClaudeCLIClient {
	cliOpts := claudeCLIOptions{binary: ClaudeCLIBinary}
	for _, o := range opts.claudeCLIOptions {
		o(&cliOpts)
	}
	return &claudeCLIClient{
		providerOptions: opts,
		options:         cliOpts,
		sess:            make(map[string]*cliSession),
	}
}

// ---------------------------------------------------------------------------
// wire types
// ---------------------------------------------------------------------------

type cliUsage struct {
	InputTokens              int64 `json:"input_tokens"`
	OutputTokens             int64 `json:"output_tokens"`
	CacheCreationInputTokens int64 `json:"cache_creation_input_tokens"`
	CacheReadInputTokens     int64 `json:"cache_read_input_tokens"`
}

// cliEnvelope is the outer object on every line. Fields from several different
// line types share it because the CLI does not nest them.
type cliEnvelope struct {
	Type    string `json:"type"`
	Subtype string `json:"subtype"`

	// system/init
	Model        string `json:"model"`
	APIKeySource string `json:"apiKeySource"`
	SessionID    string `json:"session_id"`

	// stream_event
	Event *cliStreamEvent `json:"event"`

	// result
	IsError        bool      `json:"is_error"`
	Result         string    `json:"result"`
	StopReason     string    `json:"stop_reason"`
	TotalCostUSD   float64   `json:"total_cost_usd"`
	Usage          *cliUsage `json:"usage"`
	APIErrorStatus int       `json:"api_error_status"`
	TerminalReason string    `json:"terminal_reason"`

	// rate_limit_event
	RateLimitInfo json.RawMessage `json:"rate_limit_info"`
}

// cliStreamEvent is a verbatim Anthropic Messages API streaming event.
type cliStreamEvent struct {
	Type  string `json:"type"`
	Index int    `json:"index"`
	Delta struct {
		Type        string `json:"type"`
		Text        string `json:"text"`
		Thinking    string `json:"thinking"`
		PartialJSON string `json:"partial_json"`
		StopReason  string `json:"stop_reason"`
	} `json:"delta"`
	ContentBlock struct {
		Type string `json:"type"`
		ID   string `json:"id"`
		Name string `json:"name"`
	} `json:"content_block"`
	Usage *cliUsage `json:"usage"`
}

// ---------------------------------------------------------------------------
// transcript rendering
// ---------------------------------------------------------------------------

// renderTranscript turns messages into the single prompt string the CLI is
// given. Only ever called for messages the CLI has not already seen, so on a
// resumed session this is usually one user turn.
func renderTranscript(msgs []message.Message) string {
	var b strings.Builder
	for _, m := range msgs {
		switch m.Role {
		case message.User:
			if txt := strings.TrimSpace(m.Content().String()); txt != "" {
				if b.Len() > 0 {
					b.WriteString("\n\n")
				}
				b.WriteString(txt)
			}
		case message.Assistant:
			// Only reached when seeding a fresh session from an existing
			// transcript. Labelled so the model can tell its own earlier words
			// from the user's; a resumed session never takes this branch
			// because the CLI already holds those turns.
			if txt := strings.TrimSpace(m.Content().String()); txt != "" {
				if b.Len() > 0 {
					b.WriteString("\n\n")
				}
				b.WriteString("[earlier assistant turn]\n")
				b.WriteString(txt)
			}
		case message.Tool:
			for _, r := range m.ToolResults() {
				if b.Len() > 0 {
					b.WriteString("\n\n")
				}
				fmt.Fprintf(&b, "[result of %s]\n%s", r.Name, r.Content)
			}
		}
	}
	return b.String()
}

// ---------------------------------------------------------------------------
// argv
// ---------------------------------------------------------------------------

// buildArgs assembles the command line. Split out from stream() so a test can
// assert the flags without spawning anything.
//
// --tools "" strips every built-in Claude Code tool. This program supplies its
// own, and a child quietly using its own Read and Bash would mean the user was
// testing Claude Code rather than Gorilla OpenCode. Stage 2 hands our tools back
// through --mcp-config; until then the model can talk and not act, which is a
// deliberate and visible limitation rather than a silent one.
func (c *claudeCLIClient) buildArgs(sess *cliSession, prompt string, bridge *toolBridge) []string {
	args := []string{
		"-p", prompt,
		"--output-format", "stream-json",
		"--include-partial-messages",
		// Mandatory: stream_event lines are not emitted without it.
		"--verbose",
		"--tools", "",
		// Only MCP servers we pass may be used. Without this the user's own
		// configured MCP servers would silently join the session.
		"--strict-mcp-config",
		// Anything that would prompt is denied rather than hanging forever on a
		// dialog nobody can see. A child process has no terminal to ask at.
		"--permission-prompts", "none",
		"--model", c.providerOptions.model.APIModel,
	}
	if sys := strings.TrimSpace(c.providerOptions.systemMessage); sys != "" {
		args = append(args, "--system-prompt", sys)
	}
	if sess.sent == 0 {
		args = append(args, "--session-id", sess.id)
	} else {
		args = append(args, "--resume", sess.id)
	}
	// This program's own tools, served over the bridge. Without the allowlist
	// the child denies every one of them: measured 2026-10-12, the call comes
	// back "the session has no approval surface" and lands in the result's
	// permission_denials. See claudecli_bridge.go.
	if bridge != nil {
		if cfg, err := bridge.mcpConfig(); err == nil {
			args = append(args, "--mcp-config", cfg)
			args = append(args, "--allowedTools", strings.Join(bridge.allowedTools(), ","))
		}
	}
	return append(args, c.options.extraArgs...)
}

// ---------------------------------------------------------------------------
// stream
// ---------------------------------------------------------------------------

func (c *claudeCLIClient) sessionFor(msgs []message.Message) *cliSession {
	key := ""
	if len(msgs) > 0 {
		key = msgs[0].SessionID
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	s, ok := c.sess[key]
	if !ok {
		s = &cliSession{id: uuid.NewString()}
		c.sess[key] = s
	}
	return s
}

func (c *claudeCLIClient) stream(ctx context.Context, messages []message.Message, toolList []tools.BaseTool) <-chan ProviderEvent {
	out := make(chan ProviderEvent)
	go func() {
		defer close(out)
		c.runOnce(ctx, messages, toolList, out)
	}()
	return out
}

func (c *claudeCLIClient) runOnce(ctx context.Context, messages []message.Message, toolList []tools.BaseTool, out chan<- ProviderEvent) {
	sess := c.sessionFor(messages)

	// One bridge per turn. It carries a fresh token and a port that exists only
	// while the child is running.
	sessionID, messageID := "", ""
	if len(messages) > 0 {
		sessionID = messages[0].SessionID
		messageID = messages[len(messages)-1].ID
	}
	bridge, err := startToolBridge(toolList, sessionID, messageID)
	if err != nil {
		out <- ProviderEvent{Type: EventError, Error: err}
		return
	}
	defer bridge.Close()

	pending := messages
	if sess.sent > 0 && sess.sent <= len(messages) {
		pending = messages[sess.sent:]
	}
	prompt := renderTranscript(pending)
	if strings.TrimSpace(prompt) == "" {
		// Nothing new to say. Returning an error here rather than spawning
		// avoids a child that would sit waiting on an empty prompt.
		out <- ProviderEvent{Type: EventError, Error: errors.New("claude CLI: nothing to send")}
		return
	}

	args := c.buildArgs(sess, prompt, bridge)
	cmd := exec.CommandContext(ctx, c.options.binary, args...)
	stdout, perr := cmd.StdoutPipe()
	if perr != nil {
		out <- ProviderEvent{Type: EventError, Error: fmt.Errorf("claude CLI: stdout pipe: %w", perr)}
		return
	}
	var stderr strings.Builder
	cmd.Stderr = &stderr

	if err := cmd.Start(); err != nil {
		out <- ProviderEvent{Type: EventError, Error: describeSpawnFailure(c.options.binary, err)}
		return
	}

	resp := &ProviderResponse{FinishReason: message.FinishReasonUnknown}
	var content strings.Builder
	started := false
	// Tool-use blocks are reported for DISPLAY only and are never put into
	// resp.ToolCalls. On this route the child executes its own tool calls, so
	// handing them to the agent loop as work to do would run everything twice.
	toolNames := map[int]string{}

	sc := bufio.NewScanner(stdout)
	// A single assistant message, or one tool result, can exceed the default
	// 64 KB line limit comfortably.
	sc.Buffer(make([]byte, 0, 64*1024), 8*1024*1024)

	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		var env cliEnvelope
		if err := json.Unmarshal([]byte(line), &env); err != nil {
			// A line we cannot parse is not fatal: the CLI adds event types
			// over time and an unknown one must not kill a working answer.
			logging.Debug("claude CLI: unparsed line", "len", len(line))
			continue
		}

		switch env.Type {
		case "system":
			if env.Subtype == "init" {
				logging.Debug("claude CLI session",
					"model", env.Model, "apiKeySource", env.APIKeySource, "session", env.SessionID)
			}

		case "stream_event":
			if env.Event == nil {
				continue
			}
			ev := env.Event
			switch ev.Type {
			case "content_block_start":
				switch ev.ContentBlock.Type {
				case "text":
					if !started {
						started = true
						out <- ProviderEvent{Type: EventContentStart}
					}
				case "tool_use":
					// The child presents our tools as mcp__gorilla__<name>.
					// Showing that on screen would name an implementation
					// detail the user never chose; show the tool they know.
					shown := ev.ContentBlock.Name
					if local := LocalToolName(shown); local != "" {
						shown = local
					}
					toolNames[ev.Index] = shown
					out <- ProviderEvent{
						Type: EventToolUseStart,
						ToolCall: &message.ToolCall{
							ID: ev.ContentBlock.ID, Name: shown, Type: "function",
						},
					}
				}
			case "content_block_delta":
				switch ev.Delta.Type {
				case "text_delta":
					if ev.Delta.Text == "" {
						continue
					}
					if !started {
						started = true
						out <- ProviderEvent{Type: EventContentStart}
					}
					content.WriteString(ev.Delta.Text)
					out <- ProviderEvent{Type: EventContentDelta, Content: ev.Delta.Text}
				case "thinking_delta":
					if ev.Delta.Thinking != "" {
						out <- ProviderEvent{Type: EventThinkingDelta, Thinking: ev.Delta.Thinking}
					}
				}
			case "content_block_stop":
				if name, ok := toolNames[ev.Index]; ok {
					out <- ProviderEvent{
						Type:     EventToolUseStop,
						ToolCall: &message.ToolCall{Name: name, Finished: true},
					}
					delete(toolNames, ev.Index)
				}
			case "message_delta":
				if ev.Usage != nil {
					applyCLIUsage(&resp.Usage, ev.Usage)
				}
			}

		case "rate_limit_event":
			if len(env.RateLimitInfo) > 0 {
				logging.Debug("claude CLI rate limit", "info", string(env.RateLimitInfo))
			}

		case "result":
			if env.Usage != nil {
				applyCLIUsage(&resp.Usage, env.Usage)
			}
			if env.IsError {
				_ = cmd.Wait()
				out <- ProviderEvent{Type: EventError, Error: describeCLIError(env, stderr.String())}
				return
			}
			// The streamed deltas are the source of truth for content. The
			// result's own text is used only when nothing streamed, which
			// happens when --include-partial-messages produced no text block.
			if content.Len() == 0 && env.Result != "" {
				content.WriteString(env.Result)
			}
			resp.FinishReason = mapCLIStopReason(env.StopReason)
		}
	}

	scanErr := sc.Err()
	waitErr := cmd.Wait()

	if ctx.Err() != nil {
		resp.Content = content.String()
		resp.FinishReason = message.FinishReasonCanceled
		out <- ProviderEvent{Type: EventComplete, Response: resp}
		return
	}
	if scanErr != nil {
		out <- ProviderEvent{Type: EventError, Error: fmt.Errorf("claude CLI: reading output: %w", scanErr)}
		return
	}
	if waitErr != nil && resp.FinishReason == message.FinishReasonUnknown {
		out <- ProviderEvent{Type: EventError, Error: describeCLIExit(waitErr, stderr.String())}
		return
	}

	if started {
		out <- ProviderEvent{Type: EventContentStop}
	}
	resp.Content = content.String()
	if resp.FinishReason == message.FinishReasonUnknown {
		resp.FinishReason = message.FinishReasonEndTurn
	}

	// Everything the child actually ran, handed up so the session record and the
	// receipt show it. Not ToolCalls: those mean "please run these", and these
	// are already done.
	for _, c := range bridge.Calls() {
		resp.ExecutedToolCalls = append(resp.ExecutedToolCalls, message.ToolCall{
			ID: c.ID, Name: c.Name, Input: c.Input, Type: "function",
			Finished: true, Executed: true,
		})
		resp.ExecutedToolResults = append(resp.ExecutedToolResults, message.ToolResult{
			ToolCallID: c.ID, Name: c.Name, Content: c.Output, IsError: c.IsError,
		})
	}

	// Only now is the transcript recorded as handed over. Doing it earlier
	// would make a failed turn look delivered, and the next turn would resume a
	// session that never got the message.
	c.mu.Lock()
	sess.sent = len(messages)
	c.mu.Unlock()

	out <- ProviderEvent{Type: EventComplete, Response: resp}
}

// send drains stream so there is only one parser to keep correct. The same
// choice is made in chatgpt.go, for the same reason: a second parse of the same
// wire format is a second thing to rot.
func (c *claudeCLIClient) send(ctx context.Context, messages []message.Message, toolList []tools.BaseTool) (*ProviderResponse, error) {
	var last *ProviderResponse
	for ev := range c.stream(ctx, messages, toolList) {
		switch ev.Type {
		case EventError:
			return nil, ev.Error
		case EventComplete:
			last = ev.Response
		}
	}
	if last == nil {
		return nil, errors.New("claude CLI: the process ended without producing an answer")
	}
	return last, nil
}

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

func applyCLIUsage(dst *TokenUsage, src *cliUsage) {
	// Replace rather than add. The CLI reports cumulative totals for the turn
	// on both message_delta and result, so summing them double-counts. That
	// mistake is already recorded in this package's history
	// (usage_not_summed_test.go).
	if src.InputTokens > 0 {
		dst.InputTokens = src.InputTokens
	}
	if src.OutputTokens > 0 {
		dst.OutputTokens = src.OutputTokens
	}
	if src.CacheCreationInputTokens > 0 {
		dst.CacheCreationTokens = src.CacheCreationInputTokens
	}
	if src.CacheReadInputTokens > 0 {
		dst.CacheReadTokens = src.CacheReadInputTokens
	}
}

func mapCLIStopReason(s string) message.FinishReason {
	switch s {
	case "end_turn", "stop_sequence", "":
		return message.FinishReasonEndTurn
	case "max_tokens":
		return message.FinishReasonMaxTokens
	case "tool_use":
		// The child ran its own tools and then finished; from this program's
		// side the turn is over.
		return message.FinishReasonEndTurn
	default:
		return message.FinishReasonEndTurn
	}
}

func describeSpawnFailure(binary string, err error) error {
	if errors.Is(err, exec.ErrNotFound) {
		return fmt.Errorf("could not find the %q program on your PATH. "+
			"This provider runs Claude Code itself rather than calling an API, so that program has to be installed. "+
			"Install it, then run `claude login` once and pick your Claude subscription", binary)
	}
	return fmt.Errorf("could not start %q: %w", binary, err)
}

// describeCLIError turns a failed result line into something a user can act on.
// The CLI's own wording is kept, because it is usually the clearest thing
// available, and the actionable sentence is added in front of it.
func describeCLIError(env cliEnvelope, stderr string) error {
	msg := strings.TrimSpace(env.Result)
	if msg == "" {
		msg = strings.TrimSpace(stderr)
	}
	switch {
	case strings.Contains(msg, "Not logged in") || strings.Contains(msg, "/login"):
		return errors.New("the claude program is not signed in. " +
			"Run `claude login` in a terminal and choose your Claude subscription, then try again")
	case env.APIErrorStatus == 404 || strings.Contains(stderr, "unrecognized_model"):
		return fmt.Errorf("the claude program does not recognise this model. "+
			"Your installed Claude Code may be older than this model name: %s", msg)
	case env.APIErrorStatus == 429 || strings.Contains(strings.ToLower(msg), "rate limit"):
		return fmt.Errorf("your Claude plan's usage window is full. "+
			"This route is limited by your subscription rather than by money, so it will work again when the window resets: %s", msg)
	}
	if msg == "" {
		msg = "the claude program reported an error with no message"
	}
	return errors.New(msg)
}

func describeCLIExit(err error, stderr string) error {
	if s := strings.TrimSpace(stderr); s != "" {
		return fmt.Errorf("the claude program exited: %w (%s)", err, firstLine(s))
	}
	return fmt.Errorf("the claude program exited: %w", err)
}

func firstLine(s string) string {
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return s[:i]
	}
	return s
}

// ClaudeCLIAvailable reports whether the binary is on PATH. Used by the portal
// and by config detection so a row is only offered when it can work.
func ClaudeCLIAvailable() bool {
	_, err := exec.LookPath(ClaudeCLIBinary)
	return err == nil
}

// ClaudeCLIStatus is what `claude auth status` tells us, narrowed to the fields
// this program is allowed to look at.
//
// That narrowing is deliberate. The real output also carries the account's email
// address, organisation name and organisation id. None of it is needed to decide
// whether a provider works, so none of it is decoded, which means it cannot
// reach a log, a crash report or a screenshot by accident. Directive §7: the
// safest handling of a value is not to hold it.
type ClaudeCLIStatus struct {
	LoggedIn bool `json:"loggedIn"`
	// AuthMethod is "claude.ai" for a subscription sign-in, or names an API key
	// when one is configured instead.
	AuthMethod string `json:"authMethod"`
	// SubscriptionType is the plan name, e.g. "max". Empty when not reported.
	SubscriptionType string `json:"subscriptionType"`
}

// OnSubscription reports a sign-in backed by a Claude plan rather than a key.
func (s ClaudeCLIStatus) OnSubscription() bool {
	return s.LoggedIn && s.AuthMethod == "claude.ai"
}

// CheckClaudeCLI asks the binary whether it is signed in. Measured at 237ms on
// the development machine and it spends no tokens, so it is cheap enough to run
// while the provider portal draws.
//
// This is the "a listing is not proof" rule from CLAUDE.md applied in reverse:
// here there IS a cheap authoritative check, so the portal reports what the CLI
// actually says rather than inferring readiness from a file existing.
func CheckClaudeCLI(ctx context.Context) (ClaudeCLIStatus, error) {
	var st ClaudeCLIStatus
	bin, err := exec.LookPath(ClaudeCLIBinary)
	if err != nil {
		return st, describeSpawnFailure(ClaudeCLIBinary, exec.ErrNotFound)
	}
	cctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	out, err := exec.CommandContext(cctx, bin, "auth", "status").Output()
	if err != nil {
		return st, fmt.Errorf("could not ask the claude program whether it is signed in: %w", err)
	}
	if err := json.Unmarshal(out, &st); err != nil {
		return st, fmt.Errorf("could not read the claude program's sign-in status: %w", err)
	}
	return st, nil
}

var _ = models.ProviderClaudeCLI
