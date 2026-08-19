// Package plain is an interactive mode with no TUI at all.
//
// GORILLA OVERRIDE: this exists for one reason — being able to select five hours
// of a session and copy it into a text editor.
//
// The TUI runs in the terminal's ALTERNATE screen buffer, which by design has no
// scrollback. Nothing the program has drawn is ever handed to the terminal, so
// there is nothing for Ctrl+A to select and no amount of feature work inside the
// TUI can change that. Measured with a minimal Bubble Tea program: with the
// alternate screen active, lines pushed via tea.Println reach the terminal 0 times
// out of 3; without it, 3 out of 3. Bubble Tea says so itself — "If the altscreen
// is active no output will be printed."
//
// So this mode writes ordinary lines to stdout and never takes the screen.
// Selection, copy, mouse, scroll and the terminal's own search all work because
// the terminal owns every byte. That is the whole design.
//
// It is deliberately NOT a second implementation of the TUI. There are no panels,
// no boxes and no redraws: one message per block, in order, with whatever detail
// the extras settings ask for. The per-turn cost on a slow machine is zero frames.
package plain

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/opencode-ai/opencode/internal/tui/styles"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/opencode-ai/opencode/internal/app"
	"github.com/opencode-ai/opencode/internal/config"
	"github.com/opencode-ai/opencode/internal/export"
	"github.com/opencode-ai/opencode/internal/llm/agent"
	"github.com/opencode-ai/opencode/internal/message"
	"github.com/opencode-ai/opencode/internal/permission"
	"github.com/opencode-ai/opencode/internal/pubsub"
	"github.com/opencode-ai/opencode/internal/session"
	"github.com/opencode-ai/opencode/internal/tui/components/dialog"
)

// Session runs the read-run-print loop until stdin closes or the user exits.
type Session struct {
	app *app.App
	in  *bufio.Scanner
	out io.Writer

	sess session.Session

	// printed tracks how much of each streaming message has already been written,
	// because every update carries the WHOLE message rather than a delta. Without
	// this the reply is re-printed from the start on every token.
	printed map[string]int
	// reasoningPrinted is tracked separately: reasoning and answer arrive
	// interleaved and are shown as distinct blocks.
	reasoningPrinted map[string]int
	// openBlock is the kind of block currently being streamed, so a heading is
	// printed once per block rather than once per token.
	openBlock string
	// toolNames maps a call id to its tool name. Stored tool RESULTS carry an
	// empty Name — only the call has it — so without this a result prints as
	// "<- tool (ERROR)" with no indication of WHICH tool failed. Same defect that
	// showed up in the export renderer when it was run against real data.
	toolNames map[string]string
}

func New(a *app.App, in io.Reader, out io.Writer) *Session {
	s := bufio.NewScanner(in)
	// A pasted prompt can be long; the default 64K token limit is not enough for
	// a pasted stack trace or file.
	s.Buffer(make([]byte, 0, 64*1024), 8*1024*1024)
	return &Session{
		app:              a,
		in:               s,
		out:              out,
		printed:          map[string]int{},
		reasoningPrinted: map[string]int{},
		toolNames:        map[string]string{},
	}
}

func (s *Session) Run(ctx context.Context) error {
	sess, err := s.app.Sessions.Create(ctx, "Plain session "+time.Now().Format("2006-01-02 15:04:05"))
	if err != nil {
		return fmt.Errorf("could not start a session: %w", err)
	}
	s.sess = sess

	s.banner()

	for {
		fmt.Fprint(s.out, "\n> ")
		if !s.in.Scan() {
			if err := s.in.Err(); err != nil {
				return err
			}
			fmt.Fprintln(s.out, "\nbye")
			return nil // stdin closed
		}
		line := strings.TrimSpace(s.in.Text())
		if line == "" {
			continue
		}

		if strings.HasPrefix(line, "/") {
			stop, err := s.command(ctx, line)
			if err != nil {
				fmt.Fprintf(s.out, "error: %v\n", err)
			}
			if stop {
				return nil
			}
			continue
		}

		if err := s.turn(ctx, line); err != nil {
			if errors.Is(err, context.Canceled) {
				fmt.Fprintln(s.out, "\n(interrupted)")
				continue
			}
			fmt.Fprintf(s.out, "\nerror: %v\n", err)
		}
	}
}

func (s *Session) banner() {
	fmt.Fprintf(s.out, "gorilla-opencode — plain mode\n")
	fmt.Fprintf(s.out, "Everything here is ordinary terminal output: select it, copy it, search it.\n")
	fmt.Fprintf(s.out, "folder:  %s\n", config.WorkingDirectory())
	if m := config.Get().Agents[config.AgentCoder].Model; m != "" {
		fmt.Fprintf(s.out, "model:   %s\n", m)
	}
	fmt.Fprintf(s.out, "showing: %s\n", config.ExtrasSummary())
	fmt.Fprintf(s.out, "/help for commands, /exit to leave, Ctrl-C to interrupt a reply.\n")
}

// turn runs one prompt and streams everything it produces.
func (s *Session) turn(ctx context.Context, prompt string) error {
	turnCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	// Subscribe BEFORE starting the agent, or the first tokens are missed.
	msgs := s.app.Messages.Subscribe(turnCtx)
	perms := s.app.Permissions.Subscribe(turnCtx)

	done, err := s.app.CoderAgent.Run(turnCtx, s.sess.ID, prompt)
	if err != nil {
		return err
	}

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()

		case ev := <-msgs:
			if ev.Payload.SessionID == s.sess.ID {
				s.render(ev.Payload)
			}

		// A tool wanting permission is asked about on the same stdin. The loop is
		// not reading prompts during a turn, so there is no contention — and
		// auto-approving everything in an INTERACTIVE session would hand the agent
		// the shell without asking, which is not a decision to make on someone's
		// behalf.
		case req := <-perms:
			s.askPermission(req.Payload)

		case result := <-done:
			// Drain whatever is still queued so the tail of the reply is not lost
			// when the agent finishes before the last update is consumed.
			s.drain(msgs)
			s.closeBlock()
			if result.Error != nil {
				if errors.Is(result.Error, context.Canceled) || errors.Is(result.Error, agent.ErrRequestCancelled) {
					fmt.Fprintln(s.out, "\n(cancelled)")
					return nil
				}
				return result.Error
			}
			return nil
		}
	}
}

// drain consumes any updates already queued, without blocking.
func (s *Session) drain(msgs <-chan pubsub.Event[message.Message]) {
	for {
		select {
		case ev := <-msgs:
			if ev.Payload.SessionID == s.sess.ID {
				s.render(ev.Payload)
			}
		default:
			return
		}
	}
}

// describePermissionParams renders a permission request's parameters for a
// terminal that has no dialog. Params is an `any` carried from the tool, so it
// is inspected via JSON rather than by importing every tool's type — which would
// be an import cycle, and would need updating for each new tool.
//
// A "diff" or "command" field is printed verbatim, indented: those are the whole
// point of the prompt and must be readable, not JSON-escaped.
func describePermissionParams(params any) string {
	if params == nil {
		return ""
	}
	raw, err := json.Marshal(params)
	if err != nil {
		return ""
	}
	var fields map[string]any
	if err := json.Unmarshal(raw, &fields); err != nil {
		return ""
	}

	var b strings.Builder
	for _, key := range []string{"command", "diff", "content", "file_path", "filePath"} {
		v, ok := fields[key]
		if !ok {
			continue
		}
		text, ok := v.(string)
		if !ok || strings.TrimSpace(text) == "" {
			continue
		}
		for _, line := range strings.Split(strings.TrimRight(text, "\n"), "\n") {
			b.WriteString("    " + line + "\n")
		}
		// The diff or command IS the decision; once shown, stop.
		if key == "diff" || key == "command" {
			break
		}
	}
	return strings.TrimRight(b.String(), "\n")
}

func (s *Session) askPermission(req permission.PermissionRequest) {
	s.closeBlock()
	fmt.Fprintf(s.out, "\n--- permission needed ---\n")
	fmt.Fprintf(s.out, "  tool:   %s\n", req.ToolName)
	if req.Path != "" {
		fmt.Fprintf(s.out, "  path:   %s\n", req.Path)
	}
	if req.Description != "" {
		fmt.Fprintf(s.out, "  action: %s\n", req.Description)
	}
	// GORILLA FIX (2026-08-18): show WHAT is being approved, not just which tool.
	//
	// The TUI renders the diff and the command in a fenced block; this mode
	// showed neither, so a file rewrite was approved blind. Plain mode is
	// documented as equivalent and is the mode steered to on weak terminals —
	// PHILOSOPHY.md's whole claim is that the user can CHECK what was done, and
	// a consent screen that hides the change defeats it.
	if body := describePermissionParams(req.Params); body != "" {
		fmt.Fprintf(s.out, "  details:\n%s\n", body)
	}
	fmt.Fprint(s.out, "allow? [y]es / [n]o / [a]lways for this session: ")

	if !s.in.Scan() {
		// stdin closed mid-turn: refuse rather than assume consent.
		s.app.Permissions.Deny(req)
		fmt.Fprintln(s.out, "\n(input closed — denied)")
		return
	}
	switch strings.ToLower(strings.TrimSpace(s.in.Text())) {
	case "y", "yes":
		s.app.Permissions.Grant(req)
		fmt.Fprintln(s.out, "allowed once")
	case "a", "always":
		s.app.Permissions.GrantPersistant(req)
		fmt.Fprintln(s.out, "allowed for this session")
	default:
		s.app.Permissions.Deny(req)
		fmt.Fprintln(s.out, "denied")
	}
	fmt.Fprintln(s.out, "-------------------------")
}

// render writes whatever is new in this message. Every update carries the whole
// message, so only the unprinted suffix is emitted.
func (s *Session) render(m message.Message) {
	switch m.Role {
	case message.Assistant:
		if config.ExtraEnabled("extras-reasoning-show") {
			s.stream("thinking", m.ID+":r", m.ReasoningContent().Thinking, s.reasoningPrinted)
		}
		s.stream(s.assistantHeading(m), m.ID, m.Content().String(), s.printed)

		if config.ExtraEnabled("extras-toolcalls-show") {
			s.toolCalls(m)
		}

	case message.Tool:
		if config.ExtraEnabled("extras-toolcalls-show") {
			s.toolResults(m)
		}
	}
}

func (s *Session) assistantHeading(m message.Message) string {
	h := "assistant"
	if m.Model != "" {
		h += " (" + string(m.Model) + ")"
	}
	return h
}

// stream emits the unprinted tail of text under a heading written once.
func (s *Session) stream(heading, key, text string, seen map[string]int) {
	if text == "" {
		return
	}
	already := seen[key]
	if len(text) <= already {
		return
	}
	if s.openBlock != key {
		s.closeBlock()
		fmt.Fprintf(s.out, "\n%s%s:\n", s.stamp(), heading)
		s.openBlock = key
	}
	fmt.Fprint(s.out, text[already:])
	seen[key] = len(text)
}

func (s *Session) closeBlock() {
	if s.openBlock != "" {
		fmt.Fprintln(s.out)
		s.openBlock = ""
	}
}

func (s *Session) toolCalls(m message.Message) {
	for _, tc := range m.ToolCalls() {
		key := "call:" + tc.ID
		if s.printed[key] != 0 {
			continue
		}
		s.printed[key] = 1
		if tc.ID != "" && tc.Name != "" {
			s.toolNames[tc.ID] = tc.Name
		}
		s.closeBlock()
		fmt.Fprintf(s.out, "\n%s-> %s %s\n", s.stamp(), tc.Name, oneLine(tc.Input))
	}
}

func (s *Session) toolResults(m message.Message) {
	for _, tr := range m.ToolResults() {
		key := "result:" + tr.ToolCallID
		if s.printed[key] != 0 {
			continue
		}
		s.printed[key] = 1
		s.closeBlock()
		status := "ok"
		if tr.IsError {
			status = "ERROR"
		}
		name := tr.Name
		if name == "" {
			name = s.toolNames[tr.ToolCallID]
		}
		if name == "" {
			name = "unknown tool"
		}
		fmt.Fprintf(s.out, "\n%s<- %s (%s)\n", s.stamp(), name, status)
		if c := strings.TrimSpace(tr.Content); c != "" {
			for _, l := range strings.Split(c, "\n") {
				fmt.Fprintf(s.out, "   %s\n", l)
			}
		}
	}
}

// stamp is the per-line time, when the timestamps extra is on.
func (s *Session) stamp() string {
	if !config.ExtraEnabled("extras-timestamps-show") {
		return ""
	}
	return time.Now().Format("15:04:05") + " "
}

// oneLine collapses tool input to a single readable line. The full input is in
// the export; here it is a label, and a pasted file would bury the conversation.
func oneLine(s string) string {
	s = strings.Join(strings.Fields(s), " ")
	const max = 120
	if len([]rune(s)) > max {
		return string([]rune(s)[:max-3]) + styles.Ellipsis
	}
	return s
}

// command handles the small set of slash commands that make sense without a TUI.
//
// Deliberately small. A plain-text reimplementation of fifteen dialogs would be a
// second UI to keep in step with the first; anything not here is honestly
// reported as TUI-only rather than silently ignored.
func (s *Session) command(ctx context.Context, line string) (stop bool, err error) {
	fields := strings.Fields(line)
	name := strings.TrimPrefix(fields[0], "/")

	switch name {
	case "exit", "quit", "q":
		fmt.Fprintln(s.out, "bye")
		return true, nil

	case "help", "?", "commands":
		s.help()
		return false, nil

	case "export":
		return false, s.export(ctx, fields[1:])

	case "clear", "new":
		sess, err := s.app.Sessions.Create(ctx, "Plain session "+time.Now().Format("2006-01-02 15:04:05"))
		if err != nil {
			return false, err
		}
		s.sess = sess
		s.printed = map[string]int{}
		s.reasoningPrinted = map[string]int{}
		s.toolNames = map[string]string{}
		fmt.Fprintln(s.out, "started a new session")
		return false, nil

	case "extras", "context":
		s.extras()
		return false, nil

	case "show", "hide":
		return false, s.setExtra(name == "show", fields[1:])

	case "model":
		if m := config.Get().Agents[config.AgentCoder].Model; m != "" {
			fmt.Fprintf(s.out, "model: %s\n", m)
		}
		fmt.Fprintln(s.out, "Changing model needs the full interface — run without --plain and use /model.")
		return false, nil

	default:
		fmt.Fprintf(s.out, "/%s is not available in plain mode. /help lists what is.\n", name)
		fmt.Fprintln(s.out, "Everything else lives in the full interface — run without --plain.")
		return false, nil
	}
}

func (s *Session) help() {
	fmt.Fprintln(s.out, `
Plain mode keeps everything as ordinary terminal output, so you can select and
copy the whole session. It carries fewer commands than the full interface.

  /help                 this list
  /exit                 leave
  /clear                start a fresh session (drops the context)
  /export [folder]      write the full transcript: times, reasoning, tool calls
                        and results. Defaults to the working folder.
  /extras               what is being shown, and what it costs
  /show <name>          turn one on   (thinking | reasoning | tools | times)
  /hide <name>          turn one off
  /model                which model is in use

Type anything else to send it to the model. Ctrl-C interrupts a reply.
Anything not listed here needs the full interface — run without --plain.`)
}

func (s *Session) extras() {
	fmt.Fprintln(s.out)
	for _, e := range config.Extras {
		box := "[ ]"
		if config.ExtraEnabled(e.ID) {
			box = "[x]"
		}
		cost := "free"
		if e.Cost == config.CostGeneration {
			cost = "COSTS EXTRA"
		}
		fmt.Fprintf(s.out, "  %s %-32s %-12s %s\n", box, e.Name, cost, e.What)
	}
	fmt.Fprintln(s.out, "\n"+config.ExtraCostExplanation(config.CostGeneration))
}

// extraAliases map short words to registry IDs, so /show thinking works without
// anyone having to type extras-reasoning-generate.
var extraAliases = map[string]string{
	"thinking":  "extras-reasoning-generate",
	"reasoning": "extras-reasoning-show",
	"tools":     "extras-toolcalls-show",
	"times":     "extras-timestamps-show",
}

func (s *Session) setExtra(on bool, args []string) error {
	if len(args) == 0 {
		fmt.Fprintln(s.out, "name one of: thinking, reasoning, tools, times")
		return nil
	}
	id, ok := extraAliases[strings.ToLower(args[0])]
	if !ok {
		return fmt.Errorf("unknown name %q — try thinking, reasoning, tools or times", args[0])
	}
	if err := config.SetExtra(id, on); err != nil {
		return err
	}
	e, _ := config.ExtraByID(id)
	state := "off"
	if on {
		state = "on"
	}
	fmt.Fprintf(s.out, "%s is now %s\n", e.Name, state)

	// Say what it costs at the moment of the decision, not only in /extras.
	if on && e.Cost == config.CostGeneration {
		fmt.Fprintln(s.out, config.ExtraCostExplanation(e.Cost))
	}
	return nil
}

// export writes the full transcript. The renderer is shared with the TUI's
// /export, so a plain-mode session produces exactly the same record — timestamps,
// reasoning, tool calls and results — rather than a second, lesser format.
func (s *Session) export(ctx context.Context, args []string) error {
	dir := config.WorkingDirectory()
	if len(args) > 0 {
		resolved, err := dialog.ResolveExportDir(strings.Join(args, " "))
		if err != nil {
			return err
		}
		dir = resolved
	}

	msgs, err := s.app.Messages.List(ctx, s.sess.ID)
	if err != nil {
		return err
	}
	name := fmt.Sprintf("gorilla-opencode-plain-%s.md", time.Now().Format("20060102-150405"))
	dst := filepath.Join(dir, name)

	if _, err := os.Stat(dst); err == nil {
		return fmt.Errorf("%s already exists", dst)
	}
	body := export.Render(s.sess, msgs, time.Now())
	// 0o600: a transcript holds whatever was discussed, including file contents
	// and command output.
	if err := os.WriteFile(dst, []byte(body), 0o600); err != nil {
		return err
	}
	fmt.Fprintf(s.out, "wrote %d messages to %s\n", len(msgs), dst)
	return nil
}
