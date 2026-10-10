package agent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/opencode-ai/opencode/internal/config"
	"github.com/opencode-ai/opencode/internal/llm/models"
	"github.com/opencode-ai/opencode/internal/llm/prompt"
	"github.com/opencode-ai/opencode/internal/llm/provider"
	"github.com/opencode-ai/opencode/internal/llm/tools"
	"github.com/opencode-ai/opencode/internal/logging"
	"github.com/opencode-ai/opencode/internal/message"
	"github.com/opencode-ai/opencode/internal/peers"
	"github.com/opencode-ai/opencode/internal/permission"
	"github.com/opencode-ai/opencode/internal/pubsub"
	"github.com/opencode-ai/opencode/internal/session"
)

// Common errors
var (
	ErrRequestCancelled = errors.New("request cancelled by user")
	ErrSessionBusy      = errors.New("session is currently processing another request")
)

type AgentEventType string

const (
	AgentEventTypeError     AgentEventType = "error"
	AgentEventTypeResponse  AgentEventType = "response"
	AgentEventTypeSummarize AgentEventType = "summarize"
)

type AgentEvent struct {
	Type    AgentEventType
	Message message.Message
	Error   error

	// When summarizing
	SessionID string
	Progress  string
	Done      bool
}

type Service interface {
	pubsub.Suscriber[AgentEvent]
	Model() models.Model
	Run(ctx context.Context, sessionID string, content string, attachments ...message.Attachment) (<-chan AgentEvent, error)
	Cancel(sessionID string)
	IsSessionBusy(sessionID string) bool
	IsBusy() bool
	// LastUsage is the most recent turn's token usage, so /context can report
	// how much of the prompt was served from cache. Zero means either no turn
	// has completed or the provider reports no cache figures -- see
	// UsageReportsCache, which is not the same question.
	LastUsage() provider.TokenUsage
	Update(agentName config.AgentName, modelID models.ModelID) (models.Model, error)
	Summarize(ctx context.Context, sessionID string) error
	// GORILLA OVERRIDE: swap the active tool set at runtime so context
	// loadout changes take effect without a restart.
	ReloadTools(newTools []tools.BaseTool)
	// RebuildProvider recreates the provider so a fresh system prompt
	// (which now honours the loadout's env/LSP toggles) takes effect
	// without a restart.
	//
	// Returns true when the rebuild had to be DEFERRED because a request is in
	// flight. It is remembered and applied when the turn ends, so the caller
	// must tell the user "after this turn" rather than implying it already
	// happened. It previously returned nothing and silently dropped the
	// request, which made a setting look applied when it was not.
	RebuildProvider() (deferred bool)
}

type agent struct {
	*pubsub.Broker[AgentEvent]
	agentName config.AgentName
	sessions  session.Service
	messages  message.Service

	tools []tools.BaseTool

	// lastUsage is the most recent turn's token usage, kept so /context can
	// report how much of the prompt was served from cache.
	//
	// GORILLA OVERRIDE (2026-09-02): TrackUsage sums the three input figures
	// into sess.PromptTokens -- input + cache creation + cache read -- which is
	// correct for the context gauge and destroys the one number that says
	// whether prompt caching is working at all. Cache reuse was worth 8m14s to
	// 15s on this project's own hardware and nothing in the interface showed
	// it, so a regression would have been invisible exactly as the original
	// fault was.
	//
	// In memory rather than in the session row: /context reports the LIVE
	// session, sqlc is not installed here so a schema change would mean
	// hand-editing generated code, and a number that resets on restart is the
	// honest shape for something describing the turn that just happened.
	lastUsage   provider.TokenUsage
	lastUsageMu sync.RWMutex
	toolsMu     sync.RWMutex
	// Read with a.prov() and written with a.setProv(): /model swaps it from
	// the UI goroutine while a turn is reading it on another.
	providerMu sync.RWMutex
	provider   provider.Provider

	titleProvider     provider.Provider
	summarizeProvider provider.Provider

	activeRequests sync.Map

	// GORILLA OVERRIDE: a provider rebuild requested while a turn is in flight
	// is recorded here and applied when the turn ends, instead of being dropped.
	pendingRebuild atomic.Bool
}

func NewAgent(
	agentName config.AgentName,
	sessions session.Service,
	messages message.Service,
	agentTools []tools.BaseTool,
) (Service, error) {
	agentProvider, err := createAgentProvider(agentName)
	if err != nil {
		return nil, err
	}
	var titleProvider provider.Provider
	// Only generate titles for the coder agent
	if agentName == config.AgentCoder {
		titleProvider, err = createAgentProvider(config.AgentTitle)
		if err != nil {
			return nil, err
		}
	}
	var summarizeProvider provider.Provider
	if agentName == config.AgentCoder {
		summarizeProvider, err = createAgentProvider(config.AgentSummarizer)
		if err != nil {
			return nil, err
		}
	}

	agent := &agent{
		Broker:            pubsub.NewBroker[AgentEvent](),
		agentName:         agentName,
		provider:          agentProvider,
		messages:          messages,
		sessions:          sessions,
		tools:             agentTools,
		titleProvider:     titleProvider,
		summarizeProvider: summarizeProvider,
		activeRequests:    sync.Map{},
	}

	return agent, nil
}

func (a *agent) Model() models.Model {
	return a.prov().Model()
}

// GORILLA OVERRIDE: getTools/ReloadTools let the context loadout swap the
// active tool set at runtime under a lock (the stream loop reads it).
func (a *agent) getTools() []tools.BaseTool {
	a.toolsMu.RLock()
	defer a.toolsMu.RUnlock()
	return a.tools
}

// visibleTools is what actually goes on the wire.
//
// The filter sits here, above every provider, because deferral for a local
// endpoint means NOT SENDING the schema -- there is no server to do it for us.
// One implementation covers Anthropic, OpenAI, Gemini and llama.cpp alike.
//
// Off by default, in which case this returns the full set unchanged and costs
// one map lookup.
func (a *agent) visibleTools(sessionID string) []tools.BaseTool {
	all := a.getTools()
	if !config.LoadoutEnabled(config.ToolSearchComponentID) {
		return all
	}

	// STOP DEFERRING once it has stopped paying.
	//
	// Deferral buys back schema tokens and charges two permanent ones: the
	// tool_search schema and the catalogue index. Measured on this toolset that
	// is 486 tokens a turn. Early in a session the withheld schemas are worth
	// far more than that, so it is a clear saving. But every discovery hands
	// some of that saving back, and after the fourth tool the overhead is all
	// that is left -- from then on the mechanism costs MORE than never having
	// had it, for the rest of the conversation, and nothing says so.
	//
	// That is a silent leak, and this program is used by people for whom a
	// leak is not an annoyance. So the rule is absolute: deferral must never
	// cost more than not deferring. When what remains withheld is worth less
	// than the overhead, the mechanism switches itself off for this session --
	// every tool is sent, and tool_search is dropped because there is nothing
	// left worth finding.
	//
	// It costs one cache miss at the moment of the switch. That is a single
	// one-off against a permanent per-turn loss, and it only happens in
	// sessions that were about to start losing anyway.
	if !a.deferralStillPays(all, sessionID) {
		return withoutToolSearch(all)
	}
	return tools.VisibleTools(all, sessionID, true)
}

// deferralStillPays compares what is still being withheld against what the
// mechanism costs to run.
func (a *agent) deferralStillPays(all []tools.BaseTool, sessionID string) bool {
	withheld, overhead := 0, len(prompt.DeferredCatalogue())/4
	for _, t := range all {
		name := t.Info().Name
		if name == tools.ToolSearchToolName {
			overhead += toolTokens(t)
			continue
		}
		if tools.IsDeferrable(name) && !tools.IsDiscovered(sessionID, name) {
			withheld += toolTokens(t)
		}
	}
	return withheld > overhead
}

// withoutToolSearch returns the full toolset with the search tool removed.
// Offering a way to find tools the model already has is pure cost.
func withoutToolSearch(all []tools.BaseTool) []tools.BaseTool {
	out := make([]tools.BaseTool, 0, len(all))
	for _, t := range all {
		if t.Info().Name == tools.ToolSearchToolName {
			continue
		}
		out = append(out, t)
	}
	return out
}

func (a *agent) ReloadTools(newTools []tools.BaseTool) {
	a.toolsMu.Lock()
	a.tools = newTools
	a.toolsMu.Unlock()
}

// RebuildProvider recreates the provider so the system prompt is re-rendered
// against the current loadout (env/LSP blocks). The provider must not be swapped
// mid-request, so while busy the rebuild is REMEMBERED and applied when the turn
// ends (see the drain in Run).
//
// GORILLA OVERRIDE: this used to `return` while busy and drop the request on the
// floor, with no return value and no feedback. A /context toggle made during a
// turn therefore reported a new token count and changed nothing — the exact
// class of quiet dishonesty this fork exists to avoid. It now reports that it
// deferred so the UI can say so.
func (a *agent) RebuildProvider() (deferred bool) {
	if a.IsBusy() {
		a.pendingRebuild.Store(true)
		return true
	}
	a.rebuildProviderNow()
	return false
}

func (a *agent) rebuildProviderNow() {
	if p, err := createAgentProvider(a.agentName); err == nil {
		a.setProv(p)
	} else {
		logging.Error("failed to rebuild agent provider", "agent", a.agentName, "error", err)
	}
}

// drainPendingRebuild applies a rebuild that was requested while a turn was in
// flight. Called once a request finishes. It re-checks IsBusy because a
// concurrent session may still be running, in which case the flag stays set and
// the next completion picks it up.
func (a *agent) drainPendingRebuild() {
	if !a.pendingRebuild.Load() || a.IsBusy() {
		return
	}
	// CompareAndSwap so two sessions finishing together rebuild once, not twice.
	if a.pendingRebuild.CompareAndSwap(true, false) {
		logging.Info("applying deferred provider rebuild", "agent", a.agentName)
		a.rebuildProviderNow()
	}
}

// activeRun is one Run's entry in activeRequests. A pointer, so the run that
// stored it can remove exactly its own entry and nobody else's.
type activeRun struct {
	cancel context.CancelFunc
}

func (a *agent) Cancel(sessionID string) {
	// GORILLA FIX (2026-10-05): cancelling ASKS the run to stop; it does not
	// declare the session free.
	//
	// This used LoadAndDelete, so the session read as idle the instant Esc was
	// pressed, while the old goroutine was still inside a tool. A new message
	// started a second run beside it; when the old one finally ended it deleted
	// the NEW run's entry, so the new run could no longer be cancelled and a
	// third could start. The entry is now removed by the run that owns it, when
	// it has really finished (see Run).
	if v, exists := a.activeRequests.Load(sessionID); exists {
		if run, ok := v.(*activeRun); ok && run.cancel != nil {
			logging.InfoPersist(fmt.Sprintf("Request cancellation initiated for session: %s", sessionID))
			run.cancel()
		}
	}
	// A tool parked on a permission prompt does not watch the context; without
	// this it stayed parked for up to ten minutes after the cancel.
	permission.CancelForSession(sessionID)

	// Also check for summarize requests
	if cancelFunc, exists := a.activeRequests.LoadAndDelete(sessionID + "-summarize"); exists {
		if cancel, ok := cancelFunc.(context.CancelFunc); ok {
			logging.InfoPersist(fmt.Sprintf("Summarize cancellation initiated for session: %s", sessionID))
			cancel()
		}
	}
}

func (a *agent) IsBusy() bool {
	busy := false
	a.activeRequests.Range(func(key, value interface{}) bool {
		// Any entry is a run in progress, whichever kind stored it.
		busy = true
		return false
	})
	return busy
}

func (a *agent) IsSessionBusy(sessionID string) bool {
	_, busy := a.activeRequests.Load(sessionID)
	return busy
}

func (a *agent) generateTitle(ctx context.Context, sessionID string, content string) error {
	if content == "" {
		return nil
	}
	if a.titleProvider == nil {
		return nil
	}
	session, err := a.sessions.Get(ctx, sessionID)
	if err != nil {
		return err
	}
	ctx = context.WithValue(ctx, tools.SessionIDContextKey, sessionID)
	parts := []message.ContentPart{message.TextContent{Text: content}}
	response, err := a.titleProvider.SendMessages(
		ctx,
		[]message.Message{
			{
				Role:  message.User,
				Parts: parts,
			},
		},
		make([]tools.BaseTool, 0),
	)
	if err != nil {
		return err
	}

	// GORILLA OVERRIDE: the reply is untrusted text, not a title. See title.go —
	// small models narrate, and the old trim-and-store put the narration in the
	// sidebar. The user's own first message is the fallback.
	title := sanitiseTitle(response.Content, content)
	if title == "" {
		return nil
	}

	session.Title = title
	_, err = a.sessions.Save(ctx, session)
	return err
}

func (a *agent) err(err error) AgentEvent {
	return AgentEvent{
		Type:  AgentEventTypeError,
		Error: err,
	}
}

func (a *agent) Run(ctx context.Context, sessionID string, content string, attachments ...message.Attachment) (<-chan AgentEvent, error) {
	if !a.prov().Model().SupportsAttachments && attachments != nil {
		attachments = nil
	}
	events := make(chan AgentEvent)
	// Claim the session in ONE step. "Is it busy? no; then store" let two
	// messages sent together both pass the check.
	run := &activeRun{}
	if _, taken := a.activeRequests.LoadOrStore(sessionID, run); taken {
		return nil, ErrSessionBusy
	}

	// GORILLA OVERRIDE: reset the per-turn helper-leash tally for a new
	// top-level (coder) request. Sub-agent (task) runs must NOT reset it.
	if a.agentName == config.AgentCoder {
		resetSubAgentSpawns(sessionID)
		// GORILLA (2026-10-10): other sessions see this one as busy while the
		// main conversation is in a turn. See peers.go.
		peerSetBusy(true)
	}

	genCtx, cancel := context.WithCancel(ctx)
	run.cancel = cancel
	go func() {
		logging.Debug("Request started", "sessionID", sessionID)
		defer logging.RecoverPanic("agent.Run", func() {
			events <- a.err(fmt.Errorf("panic while running the agent"))
		})
		var attachmentParts []message.ContentPart
		for _, attachment := range attachments {
			attachmentParts = append(attachmentParts, message.BinaryContent{Path: attachment.FilePath, MIMEType: attachment.MimeType, Data: attachment.Content})
		}
		result := a.processGeneration(genCtx, sessionID, content, attachmentParts)
		if result.Error != nil && !errors.Is(result.Error, ErrRequestCancelled) && !errors.Is(result.Error, context.Canceled) {
			logging.ErrorPersist(result.Error.Error())
		}
		logging.Debug("Request completed", "sessionID", sessionID)
		// Only this run's own entry. See Cancel.
		a.activeRequests.CompareAndDelete(sessionID, run)
		cancel()
		// A helper's loop history is finished with its run; the coder's is
		// cleared at the next turn. Without this every helper session ever
		// started stayed in the detector's map for the life of the program.
		if a.agentName != config.AgentCoder {
			stuck.Reset(sessionID)
		}
		// GORILLA OVERRIDE (2026-10-09): the user's turn_end hooks, once per
		// turn, on success and failure alike. Run before the result is handed
		// on, so a hook has finished (or been stopped at its timeout) by the
		// time anyone is told the turn is over. See hooks.go.
		a.runTurnEndHooks(sessionID, turnFinishReason(result))
		// Delete first, so IsBusy inside the drain sees this session as finished.
		// This is the single point a request completes on both the success and
		// the error path, which is why the drain belongs here.
		a.drainPendingRebuild()
		// GORILLA (2026-10-10): the turn is over; if nothing else is running,
		// peers now see this session as idle and any waiting idle notice is
		// sent, once. IsBusy, not false: another session may still be in a turn.
		if a.agentName == config.AgentCoder {
			peerSetBusy(a.IsBusy())
		}
		a.Publish(pubsub.CreatedEvent, result)
		events <- result
		close(events)
	}()
	return events, nil
}

func (a *agent) processGeneration(ctx context.Context, sessionID, content string, attachmentParts []message.ContentPart) AgentEvent {
	cfg := config.Get()

	// GORILLA OVERRIDE (2026-08-18): give this turn an upload budget.
	//
	// Measured on a link that dropped every 8 seconds: 14 attempts, 1.01 MB
	// uploaded, no answer, no error — because the application loop and Go's
	// http.Transport each retried without knowing the other existed, so the
	// real ceiling was their product rather than the maxRetries written down.
	//
	// The budget is attached HERE, once per turn, and every request the turn
	// makes carries it — including retries the transport starts on its own.
	// One counter, at the one place they all pass through. See
	// internal/llm/provider/uploadbudget.go for why it counts bytes and not
	// attempts.
	ctx = provider.WithUploadBudget(ctx, provider.NewUploadBudget(config.TurnUploadBudgetBytes()))
	// List existing messages; if none, start title generation asynchronously.
	msgs, err := a.messages.List(ctx, sessionID)
	msgs = answerOrphanedToolCalls(msgs)
	if err != nil {
		return a.err(fmt.Errorf("failed to list messages: %w", err))
	}
	if len(msgs) == 0 {
		go func() {
			defer logging.RecoverPanic("agent.Run", func() {
				logging.ErrorPersist("panic while generating title")
			})
			// GORILLA OVERRIDE: the title request used to fire at the same
			// instant as the user's message — two CONCURRENT requests. On
			// providers that cap concurrency (NVIDIA NIM's free tier does),
			// the second is 429'd, which triggered a retry storm on a
			// plain "yo". Wait for the main request to finish first so we
			// only ever have one request in flight.
			deadline := time.Now().Add(titleWaitBudget())
			for a.IsSessionBusy(sessionID) && time.Now().Before(deadline) {
				time.Sleep(150 * time.Millisecond)
			}
			// GORILLA OVERRIDE (2026-09-01): if the wait ran out, ABANDON the
			// title. Do not fire it anyway.
			//
			// The loop above exists so the title request never runs beside the
			// user's own — that concurrency caused a 429 retry storm on capped
			// providers. But when the deadline expired the code proceeded
			// regardless, which produces exactly the thing the wait was written
			// to prevent, just 120 seconds later.
			//
			// Nobody noticed because on a cloud model a turn finishes well
			// inside the budget. On a local model it does not: measured on this
			// machine, one turn spends 6-8 minutes in prompt processing alone,
			// so the deadline ALWAYS expired and the title always fired into a
			// busy single-slot server — visible in the LM Studio log as a second
			// /v1/chat/completions arriving mid-turn and queueing behind the
			// real work. The user waits twice for a cosmetic string.
			//
			// A session without a generated title is not broken; it keeps the
			// default one. Losing that is much cheaper than doubling every turn.
			if a.IsSessionBusy(sessionID) {
				logging.Info("skipping title generation: the session is still busy",
					"session", sessionID, "waited", titleWaitBudget())
				return
			}
			titleErr := a.generateTitle(context.Background(), sessionID, content)
			if titleErr != nil {
				logging.ErrorPersist(fmt.Sprintf("failed to generate title: %v", titleErr))
			}
		}()
	}
	session, err := a.sessions.Get(ctx, sessionID)
	if err != nil {
		return a.err(fmt.Errorf("failed to get session: %w", err))
	}
	if session.SummaryMessageID != "" {
		summaryMsgInex := -1
		for i, msg := range msgs {
			if msg.ID == session.SummaryMessageID {
				summaryMsgInex = i
				break
			}
		}
		if summaryMsgInex != -1 {
			msgs = msgs[summaryMsgInex:]
			msgs[0].Role = message.User
		}
	}

	// GORILLA (2026-10-10): messages from other sessions open the user's turn,
	// fenced as text from another program that grants nothing. A new variable,
	// not a reassignment of content: the title goroutine above reads content.
	var peerBlock, peerTaint string
	var peerMsgs []peers.Message
	if a.agentName == config.AgentCoder {
		peerBlock, peerTaint, peerMsgs = takePeerMessages()
	}
	userMsg, err := a.createUserMessage(ctx, sessionID, withPeerBlock(peerBlock, content), attachmentParts)
	if err != nil {
		requeuePeerMessages(peerMsgs)
		return a.err(fmt.Errorf("failed to create user message: %w", err))
	}
	// GORILLA FIX (2026-08-19): a new user turn clears the taint bit.
	//
	// The user typing IS the trust boundary — they have seen what the last
	// turn did and are asking for the next thing. Taint that never cleared
	// would be permanently set after the first web search, every egress would
	// prompt forever, and a prompt that always fires is a prompt nobody reads.
	// That is how a control gets switched off in practice while still looking
	// present in the source. See internal/permission/taint.go.
	//
	// GORILLA FIX (2026-10-05): ... and ONLY the user typing. This line ran for
	// every agent, and ClearTaint resolves a helper's session to its root. So a
	// page that had tainted the turn only had to get the model to start a helper
	// (the agent tool, or research) and the root's taint was wiped: the next
	// action was auto-approved without the "this turn has read untrusted
	// content" prompt. A helper starting is the model acting, not the person.
	if a.agentName == config.AgentCoder {
		permission.ClearTaint(sessionID)
		// GORILLA (2026-10-10): ... unless this turn opens with a message from
		// another session. The person typed, but the fenced text above their
		// words was written by another program, so the turn starts tainted and
		// auto-approve still asks before anything leaves the machine.
		markPeerTaint(sessionID, peerTaint)
	}
	// Same boundary, same reason: the person has seen the last turn. Whatever
	// the model repeated then is not held against what they ask for now.
	stuck.Reset(sessionID)
	// Append the new user message to the conversation history.
	msgHistory := append(msgs, userMsg)

	for {
		// Check for cancellation before each iteration
		select {
		case <-ctx.Done():
			return a.err(ctx.Err())
		default:
			// Continue processing
		}
		agentMessage, toolResults, err := a.streamAndHandleEvents(ctx, sessionID, msgHistory)
		if err != nil {
			if errors.Is(err, context.Canceled) {
				agentMessage.AddFinish(message.FinishReasonCanceled)
				a.messages.Update(context.Background(), agentMessage)
				return a.err(ErrRequestCancelled)
			}
			// GORILLA FIX: no "failed to process events:" prefix. It is internal
			// jargon that told the user nothing and buried the one sentence that
			// did — the provider errors below are already written as plain
			// sentences ("... is not available to your account", "still failing
			// after N retries — the provider is busy"), so a prefix about event
			// processing only made a clear message look like a crash.
			return a.err(err)
		}
		if cfg.Debug {
			seqId := (len(msgHistory) + 1) / 2
			toolResultFilepath := logging.WriteToolResultsJson(sessionID, seqId, toolResults)
			logging.Info("Result", "message", agentMessage.FinishReason(), "toolResults", "{}", "filepath", toolResultFilepath)
		} else {
			logging.Info("Result", "message", agentMessage.FinishReason(), "toolResults", toolResults)
		}
		if (agentMessage.FinishReason() == message.FinishReasonToolUse) && toolResults != nil {
			// We are not done, we need to respond with the tool response
			msgHistory = append(msgHistory, agentMessage, *toolResults)

			// GORILLA OVERRIDE (2026-09-01): check the request FITS before
			// spending a round trip discovering that it does not.
			//
			// This is the exact point where a conversation overflows: the tool
			// results just appended can be thousands of tokens (a `view` of a
			// large file), and nothing else measures them. The footer is derived
			// from the last completed response's usage, so it is still showing a
			// number from before these results existed — 9.7K, in the session
			// where the request that followed was 18.5K against a 15.1K window.
			//
			// Catching it here rather than at the provider means the user gets
			// one clear sentence naming the fix, instead of two levels of nested
			// vendor JSON after a wasted upload. The turn's work is not lost:
			// everything up to this point is already recorded, so /compact
			// carries it forward.
			if over := ContextOverflowMessage(
				EstimateRequestTokens(a.prov().SystemPrompt(), a.wireView(msgHistory), a.visibleTools(sessionID)),
				a.prov().Model(),
			); over != "" {
				agentMessage.AddFinish(message.FinishReasonError)
				a.messages.Update(context.Background(), agentMessage)
				return a.err(errors.New(over))
			}
			continue
		}
		return AgentEvent{
			Type:    AgentEventTypeResponse,
			Message: agentMessage,
			Done:    true,
		}
	}
}

func (a *agent) createUserMessage(ctx context.Context, sessionID, content string, attachmentParts []message.ContentPart) (message.Message, error) {
	parts := []message.ContentPart{message.TextContent{Text: content}}
	parts = append(parts, attachmentParts...)
	return a.messages.Create(ctx, sessionID, message.CreateMessageParams{
		Role:  message.User,
		Parts: parts,
	})
}

func (a *agent) streamAndHandleEvents(ctx context.Context, sessionID string, msgHistory []message.Message) (message.Message, *message.Message, error) {
	ctx = context.WithValue(ctx, tools.SessionIDContextKey, sessionID)
	eventChan := a.prov().StreamResponse(ctx, msgHistory, a.visibleTools(sessionID))

	assistantMsg, err := a.messages.Create(ctx, sessionID, message.CreateMessageParams{
		Role:  message.Assistant,
		Parts: []message.ContentPart{},
		Model: a.prov().Model().ID,
	})
	if err != nil {
		return assistantMsg, nil, fmt.Errorf("failed to create assistant message: %w", err)
	}

	// Add the session and message ID into the context if needed by tools.
	ctx = context.WithValue(ctx, tools.MessageIDContextKey, assistantMsg.ID)

	// Process each event in the stream.
	for event := range eventChan {
		if processErr := a.processEvent(ctx, sessionID, &assistantMsg, event); processErr != nil {
			// GORILLA FIX: do NOT relabel a provider failure as "canceled".
			//
			// processEvent's EventError branch has just recorded
			// FinishReasonError together with the provider's own words. AddFinish
			// REMOVES any existing finish part before appending, so calling it
			// again here overwrote both the reason and the details — every
			// provider error was reported as "Canceled — no answer was produced"
			// with the explanation deleted one line after being stored.
			//
			// Observed 2026-08-05 on a live 404: the status bar showed the real
			// message while the transcript said "canceled", and every recorded
			// finish in the session database read reason=canceled details=<empty>.
			// This defeated the whole point of storing the detail in v0.1.68.
			if errors.Is(processErr, context.Canceled) {
				a.finishMessage(ctx, &assistantMsg, message.FinishReasonCanceled)
			}
			return assistantMsg, a.cancelPendingToolCalls(&assistantMsg), processErr
		}
		if ctx.Err() != nil {
			a.finishMessage(context.Background(), &assistantMsg, message.FinishReasonCanceled)
			return assistantMsg, a.cancelPendingToolCalls(&assistantMsg), ctx.Err()
		}
	}
	// GORILLA FIX (2026-10-05): a stream that closed WITHOUT saying it was
	// finished is a failure, not an answer. A provider goroutine that returned
	// early (a dropped connection, a cancelled retry wait) closed the channel
	// with neither a completion nor an error; this loop simply ended, and half
	// a reply, or an empty one, was stored and shown as the model's answer.
	if assistantMsg.FinishPart() == nil {
		if ctx.Err() != nil {
			a.finishMessage(context.Background(), &assistantMsg, message.FinishReasonCanceled)
			return assistantMsg, a.cancelPendingToolCalls(&assistantMsg), ctx.Err()
		}
		incomplete := errors.New("the connection to the model ended before the reply was complete; nothing after this point was received. Send the message again")
		assistantMsg.AddFinish(message.FinishReasonError, incomplete.Error())
		_ = a.messages.Update(ctx, assistantMsg)
		return assistantMsg, a.cancelPendingToolCalls(&assistantMsg), incomplete
	}

	toolResults := make([]message.ToolResult, len(assistantMsg.ToolCalls()))
	toolCalls := assistantMsg.ToolCalls()
	// ran[i] is true once call i has been attempted, whatever came of it. A
	// call that was cancelled or refused stays false. See aftertool.go.
	ran := make([]bool, len(toolCalls))
	stuckStop := ""
	for i, toolCall := range toolCalls {
		select {
		case <-ctx.Done():
			a.finishMessage(context.Background(), &assistantMsg, message.FinishReasonCanceled)
			// Make all future tool calls cancelled
			for j := i; j < len(toolCalls); j++ {
				toolResults[j] = message.ToolResult{
					ToolCallID: toolCalls[j].ID,
					Content:    "Tool execution canceled by user",
					IsError:    true,
				}
			}
			goto out
		default:
			// Continue processing
			ran[i] = true
			// GORILLA OVERRIDE: exact match, then ONE control-token cleaning
			// pass, then exact match again. Nothing else. The rules and the
			// reasons live in toolname.go — read them before changing this.
			//
			// The commented-out strings.HasPrefix "monkey patch" that used to
			// sit here is deleted rather than left as a temptation: prefix
			// matching lets `bash_readonly` resolve to `bash`, which turns a
			// typo-fix into a privilege-escalation primitive.
			available := a.getTools()
			namers := make([]toolNamer, len(available))
			for j, t := range available {
				namers[j] = toolInfoNamer{t}
			}
			idx, resolved, cleaned := findTool(namers, toolCall.Name)

			// Tool not found
			if idx < 0 {
				// Report BOTH names. The model that emitted "ls<|message|>" and
				// the human reading the transcript both need to see what was
				// actually sent — diagnosing this the first time took a dig
				// through the session database.
				toolResults[i] = message.ToolResult{
					ToolCallID: toolCall.ID,
					// retiredToolHint teaches models that ask for grep/glob/ls
					// by name — the call still fails; nothing is rerouted.
					Content: fmt.Sprintf("Tool not found: %q. Call the tool by its exact name, with no extra characters.%s",
						toolCall.Name, retiredToolHint(toolCall.Name)),
					IsError: true,
				}
				continue
			}
			// GORILLA FIX (2026-08-19): a tool call whose ARGUMENTS arrived
			// corrupted must say so, and must not be blamed on the model.
			//
			// Recovered from the session database after a real run: a bash
			// call arrived as
			//
			//     {"\ufffd\ufffd\ufffd\ufffd\ufffd\ufffdcommand":""}
			//
			// Six U+FFFD REPLACEMENT CHARACTERs prepended to the parameter
			// name. U+FFFD is what a decoder emits for bytes that were not
			// valid UTF-8, so this is a TRANSPORT fault — a stream chunk split
			// mid-character, or a byte sequence decoded before it was
			// complete. The model did not send that.
			//
			// What happened without this check: the key was no longer
			// "command", the bash tool saw no command, and the model — given
			// an empty result and no explanation — apologised for a mistake it
			// had not made ("That command was malformed") and retried. A model
			// misled into blaming itself will keep doing the same thing,
			// because it is fixing the wrong thing.
			if reason := corruptedToolInput(toolCall.Input); reason != "" {
				logging.Warn("tool call arguments arrived corrupted",
					"tool", toolCall.Name, "id", toolCall.ID, "reason", reason,
					"raw", toolCall.Input)
				toolResults[i] = message.ToolResult{
					ToolCallID: toolCall.ID,
					Content: fmt.Sprintf(
						"The arguments for %s could not be used: %s. %s",
						toolCall.Name, reason, corruptedToolAdvice(reason)),
					IsError: true,
				}
				continue
			}

			tool := available[idx]
			if cleaned {
				// Never silently. A name that had to be repaired is recorded,
				// so a strange name can never quietly become a powerful tool
				// with nobody able to see that it happened.
				logging.Warn("repaired a malformed tool name from the model",
					"sent", toolCall.Name, "resolved", resolved, "agent", string(a.agentName))
			}
			// GORILLA FIX (2026-09-02): tell the model when it sent a parameter
			// this tool does not have.
			//
			// json.Unmarshal drops unknown fields without a word, so a malformed
			// call and a correct one look identical from the model's side. Gemma 4
			// called view with view="tree", which does not exist, and a turn later
			// was still guessing about it -- there was no way for it to find out.
			//
			// Computed BEFORE the call so the note survives whatever the tool
			// returns, and appended rather than substituted: the work still
			// happened and its result is still what the model asked for.
			unknown := tools.UnknownParams(tool.Info(), toolCall.Input)
			if len(unknown) > 0 {
				logging.Warn("tool call carried parameters the tool does not declare",
					"tool", toolCall.Name, "unknown", unknown, "agent", string(a.agentName))
			}
			// GORILLA OVERRIDE (2026-10-09): the user's before_tool hooks may
			// refuse this call; a refused call is not run. Keyed on the
			// RESOLVED name, so a repaired name cannot slip past a filter. A
			// refusal does not end the turn: the model is told and may adapt,
			// and each later call is gated on its own. See hooks.go.
			if refusal := a.runBeforeToolHooks(ctx, sessionID, tool.Info().Name, toolCall.Input); refusal != "" {
				ran[i] = false // the user's gate said no; the model did not loop
				toolResults[i] = message.ToolResult{
					ToolCallID: toolCall.ID,
					Content:    refusal,
					IsError:    true,
				}
				continue
			}
			toolResult, toolErr := tool.Run(ctx, tools.ToolCall{
				ID:    toolCall.ID,
				Name:  toolCall.Name,
				Input: toolCall.Input,
			})
			if toolErr != nil {
				if errors.Is(toolErr, permission.ErrorPermissionDenied) {
					ran[i] = false // the person said no; the model did not loop
					// The tool may have wrapped the sentinel with its reason
					// ("permission denied: this command was NOT run, because
					// ..."). Keep it: the bare two words told nobody anything.
					denied := "Permission denied"
					if msg := toolErr.Error(); len(msg) > len("permission denied") {
						denied = strings.ToUpper(msg[:1]) + msg[1:]
					}
					toolResults[i] = message.ToolResult{
						ToolCallID: toolCall.ID,
						Content:    denied,
						IsError:    true,
					}
					for j := i + 1; j < len(toolCalls); j++ {
						toolResults[j] = message.ToolResult{
							ToolCallID: toolCalls[j].ID,
							Content:    "Tool execution canceled by user",
							IsError:    true,
						}
					}
					a.finishMessage(ctx, &assistantMsg, message.FinishReasonPermissionDenied)
					// GORILLA FIX (2026-10-05): this was a bare `break`. It sits
					// inside `select { default: }`, so it left the SELECT and the
					// loop went on: the calls just marked "canceled by user" were
					// then RUN, and their real results replaced the cancellation.
					// The person said no to the first of three commands and the
					// other two executed. Found by audit; no test drove a denial
					// with a call after it.
					goto out
				}

				// GORILLA FIX (2026-08-18): report the failure instead of
				// swallowing it.
				//
				// Only permission denial was handled above; every OTHER error
				// fell through to the assignment below, which reads toolResult —
				// the ZERO VALUE when the tool returned an error. So a failed
				// tool handed the model Content:"" with IsError:false: an empty
				// result flagged as SUCCESS. The model then reasons as though
				// the work was done, which is the same defect as a silent
				// truncation, one layer up. A tool that failed must say so.
				logging.Error("tool returned an error",
					"tool", toolCall.Name, "id", toolCall.ID, "err", toolErr)
				toolResults[i] = message.ToolResult{
					ToolCallID: toolCall.ID,
					Content:    fmt.Sprintf("The %s tool failed and produced no result: %v", toolCall.Name, toolErr),
					IsError:    true,
				}
				a.runAfterToolHooks(ctx, sessionID, tool.Info().Name, toolCall.Input, toolResults[i].Content, true)
				continue
			}
			toolResults[i] = message.ToolResult{
				ToolCallID: toolCall.ID,
				Content:    toolResult.Content,
				Metadata:   toolResult.Metadata,
				IsError:    toolResult.IsError,
			}
			// The note travels with the RESULT, not to the log: a log line is not
			// something the model can read, and the model is who needs to know.
			if len(unknown) > 0 {
				toolResults[i].Content += fmt.Sprintf(
					"\n\nnote: %s ignored the unknown parameter(s) %v. It accepts "+
						"only %v. The result above came from the parameters it did "+
						"understand.",
					toolCall.Name, unknown, tools.DeclaredParams(tool.Info()))
			}
			a.runAfterToolHooks(ctx, sessionID, tool.Info().Name, toolCall.Input, toolResults[i].Content, toolResults[i].IsError)
		}
	}
out:
	if len(toolResults) == 0 {
		return assistantMsg, nil, nil
	}
	// GORILLA OVERRIDE (2026-10-04): mask credentials and watch for loops, for
	// every tool, before anything is stored or sent. See aftertool.go.
	stuckStop = finishToolResults(sessionID, toolCalls, toolResults, ran)
	parts := make([]message.ContentPart, 0)
	for _, tr := range toolResults {
		parts = append(parts, tr)
	}
	msg, err := a.messages.Create(context.Background(), assistantMsg.SessionID, message.CreateMessageParams{
		Role:  message.Tool,
		Parts: parts,
	})
	if err != nil {
		return assistantMsg, nil, fmt.Errorf("failed to create cancelled tool message: %w", err)
	}
	if stuckStop != "" {
		// The results are already stored, so the history is whole and the
		// person can pick up from here. Only the next round trip is not made.
		logging.Warn("turn stopped: the model was repeating itself",
			"session", sessionID, "agent", string(a.agentName), "reason", stuckStop)
		return assistantMsg, &msg, &StuckError{Reason: stuckStop}
	}

	return assistantMsg, &msg, err
}

// StuckError ends a turn in which the model was going round in circles. Its
// text is written for the person, not for a log.
type StuckError struct{ Reason string }

func (e *StuckError) Error() string { return e.Reason }

// cancelPendingToolCalls writes a "canceled" result for every tool call the
// assistant announced but never got to run. Returns nil when there are none.
//
// GORILLA FIX: pressing Esc while the model was still STREAMING left the UI
// stuck on "Waiting for response..." forever, with no way out short of killing
// the program. The status bar said "request cancelled by user" and the context
// really was cancelled — but the two early returns in the streaming loop above
// returned before ever reaching the tool loop, which is the only place that
// wrote "Tool execution canceled by user" results. So the assistant message kept
// tool calls with no matching result, and message.go renders exactly that as
// "Waiting for response...".
//
// The tool loop's own cancellation branch only covers a cancel that arrives
// AFTER streaming finished, which is the rarer case: by then there is usually
// nothing left to wait for. Cancelling mid-think is the normal case and it was
// the one not handled.
//
// context.Background() throughout, deliberately: the request context is already
// cancelled, and writing these results is exactly what must still happen. Using
// the dead context would make the write fail and leave the UI stuck again.
func (a *agent) cancelPendingToolCalls(assistantMsg *message.Message) *message.Message {
	toolCalls := assistantMsg.ToolCalls()
	if len(toolCalls) == 0 {
		return nil
	}
	parts := make([]message.ContentPart, 0, len(toolCalls))
	for _, tc := range toolCalls {
		parts = append(parts, message.ToolResult{
			ToolCallID: tc.ID,
			Content:    "Tool execution canceled by user",
			IsError:    true,
		})
	}
	msg, err := a.messages.Create(context.Background(), assistantMsg.SessionID,
		message.CreateMessageParams{Role: message.Tool, Parts: parts})
	if err != nil {
		// Log and carry on. Failing to record the cancellation must not also
		// take down the cancellation itself — the user asked for this to stop.
		logging.ErrorPersist(fmt.Sprintf(
			"failed to record cancelled tool calls for session %s: %v",
			assistantMsg.SessionID, err))
		return nil
	}
	return &msg
}

func (a *agent) finishMessage(ctx context.Context, msg *message.Message, finishReson message.FinishReason) {
	msg.AddFinish(finishReson)
	_ = a.messages.Update(ctx, *msg)
}

func (a *agent) processEvent(ctx context.Context, sessionID string, assistantMsg *message.Message, event provider.ProviderEvent) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
		// Continue processing.
	}

	switch event.Type {
	case provider.EventThinkingDelta:
		// GORILLA FIX (2026-10-05): Anthropic and the ChatGPT sign-in put the
		// reasoning text in Thinking, not Content. Reading only Content stored
		// an empty string per delta: the reasoning was thrown away and each
		// nothing still cost a database write.
		text := event.Content
		if text == "" {
			text = event.Thinking
		}
		if text == "" {
			return nil
		}
		assistantMsg.AppendReasoningContent(text)
		return a.messages.Update(ctx, *assistantMsg)
	case provider.EventContentDelta:
		assistantMsg.AppendContent(event.Content)
		return a.messages.Update(ctx, *assistantMsg)
	case provider.EventToolUseStart:
		assistantMsg.AddToolCall(*event.ToolCall)
		return a.messages.Update(ctx, *assistantMsg)
	// TODO: see how to handle this
	// case provider.EventToolUseDelta:
	// 	tm := time.Unix(assistantMsg.UpdatedAt, 0)
	// 	assistantMsg.AppendToolCallInput(event.ToolCall.ID, event.ToolCall.Input)
	// 	if time.Since(tm) > 1000*time.Millisecond {
	// 		err := a.messages.Update(ctx, *assistantMsg)
	// 		assistantMsg.UpdatedAt = time.Now().Unix()
	// 		return err
	// 	}
	case provider.EventToolUseStop:
		assistantMsg.FinishToolCall(event.ToolCall.ID)
		return a.messages.Update(ctx, *assistantMsg)
	case provider.EventError:
		if errors.Is(event.Error, context.Canceled) {
			logging.InfoPersist(fmt.Sprintf("Event processing canceled for session: %s", sessionID))
			return context.Canceled
		}
		// GORILLA FIX: put the failure in the TRANSCRIPT, not only in the status
		// bar. The status line truncates at roughly 100 columns and the next
		// message overwrites it, so the one thing needed to diagnose a failed
		// turn was the one thing that could not be read or copied. Recording it
		// on the assistant message keeps the provider's exact words in the
		// conversation — scrollable, selectable, and still there tomorrow.
		//
		// The status bar keeps its short flash; this is the durable copy.
		logging.ErrorPersist(event.Error.Error())
		assistantMsg.AddFinish(message.FinishReasonError, event.Error.Error())
		if err := a.messages.Update(ctx, *assistantMsg); err != nil {
			logging.Error("could not record the failure on the message", "err", err)
		}
		return event.Error
	case provider.EventComplete:
		assistantMsg.SetToolCalls(event.Response.ToolCalls)
		// GORILLA OVERRIDE (2026-08-18): if the model wrote a tool call as TEXT
		// instead of making one, say so rather than printing the raw JSON as the
		// answer. Labelled only, never dispatched — see leakedtoolcall.go.
		if len(assistantMsg.ToolCalls()) == 0 {
			if name := LeakedToolCallName(assistantMsg.Content().String(), a.toolNames()); name != "" {
				logging.Warn("model emitted a tool call as text", "tool", name)
				assistantMsg.AddFinish(event.Response.FinishReason, LeakedToolCallNotice(name))
				if err := a.messages.Update(ctx, *assistantMsg); err != nil {
					return fmt.Errorf("failed to update message: %w", err)
				}
				return a.TrackUsage(ctx, sessionID, a.prov().Model(), event.Response.Usage)
			}
		}
		if len(assistantMsg.ToolCalls()) > 0 {
			// What a model does in use outranks what one test question found.
			models.NoteAnsweredInUse(a.prov().Model().ID, config.CacheBase())
		}
		assistantMsg.AddFinish(event.Response.FinishReason)
		if err := a.messages.Update(ctx, *assistantMsg); err != nil {
			return fmt.Errorf("failed to update message: %w", err)
		}
		return a.TrackUsage(ctx, sessionID, a.prov().Model(), event.Response.Usage)
	}

	return nil
}

// toolNames lists the tools registered for this turn, so a leaked tool call can
// be required to name a REAL one before it is labelled as such.
func (a *agent) toolNames() []string {
	tools := a.getTools()
	names := make([]string, 0, len(tools))
	for _, t := range tools {
		names = append(names, t.Info().Name)
	}
	return names
}

// LastUsage reports the most recent turn's token usage, so the interface can
// show how much of the prompt was served from cache.
//
// The zero value means no turn has completed yet. A provider that does not
// report cache figures leaves CacheReadTokens and CacheCreationTokens at zero,
// which is NOT the same as "nothing was cached" -- LM Studio, measured on
// 2026-09-02, sends no prompt_tokens_details at all while demonstrably reusing
// the prefix. Callers must distinguish the two; see UsageReportsCache.
func (a *agent) LastUsage() provider.TokenUsage {
	a.lastUsageMu.RLock()
	defer a.lastUsageMu.RUnlock()
	return a.lastUsage
}

func (a *agent) TrackUsage(ctx context.Context, sessionID string, model models.Model, usage provider.TokenUsage) error {
	a.lastUsageMu.Lock()
	a.lastUsage = usage
	a.lastUsageMu.Unlock()

	sess, err := a.sessions.Get(ctx, sessionID)
	if err != nil {
		return fmt.Errorf("failed to get session: %w", err)
	}

	cost := model.CostPer1MInCached/1e6*float64(usage.CacheCreationTokens) +
		model.CostPer1MOutCached/1e6*float64(usage.CacheReadTokens) +
		model.CostPer1MIn/1e6*float64(usage.InputTokens) +
		model.CostPer1MOut/1e6*float64(usage.OutputTokens)

	sess.Cost += cost
	// GORILLA FIX (2026-08-19): a cache READ is INPUT, not output.
	//
	// Upstream did `CompletionTokens = OutputTokens + CacheReadTokens` and left
	// cache reads out of PromptTokens entirely. Cache reads are prompt tokens
	// served from cache — they are input by definition. So the ledger inflated
	// output and understated input by the same amount.
	//
	// This matters more than a cosmetic mislabel: tool schemas live INSIDE the
	// cached prefix, so the very number a user would consult to answer "what are
	// my tool schemas costing me" was the one attributing them to the wrong
	// column. The MONEY figure survived by luck — CostPer1MOutCached happens to
	// hold the cache-read rate despite its name — but the token counts did not.
	sess.CompletionTokens = usage.OutputTokens
	sess.PromptTokens = usage.InputTokens + usage.CacheCreationTokens + usage.CacheReadTokens

	// GORILLA FIX (2026-08-19): the two fields above are the CURRENT context
	// occupancy — assigned, not accumulated, because the status bar and
	// sidebar compare their sum against the model's context window and a
	// running total would climb past 100% and sit there showing a false
	// warning. Compaction depends on the same reading: it zeroes PromptTokens
	// because the context really has been emptied.
	//
	// The problem was that three OTHER readers treated the same two fields as
	// lifetime totals. The session export writes "Tokens: N in / M out" into a
	// document meant to be kept, and the sidebar labels them "Input" and
	// "Output" — all of them reporting the last turn only, with nothing on
	// screen to say so. Meanwhile Cost, three lines up, has always used `+=`,
	// so one panel could show a cost accumulated over an hour next to a token
	// count from thirty seconds ago.
	//
	// Both readings are legitimate and they are different numbers, so both are
	// now stored. Accumulating the existing fields — which is what the
	// proposal asked for — would have fixed the export and broken the gauge.
	sess.CumulativeCompletionTokens += usage.OutputTokens
	sess.CumulativePromptTokens += usage.InputTokens + usage.CacheCreationTokens + usage.CacheReadTokens

	_, err = a.sessions.Save(ctx, sess)
	if err != nil {
		return fmt.Errorf("failed to save session: %w", err)
	}
	return nil
}

func (a *agent) Update(agentName config.AgentName, modelID models.ModelID) (models.Model, error) {
	if a.IsBusy() {
		return models.Model{}, fmt.Errorf("cannot change model while processing requests")
	}

	// GORILLA OVERRIDE (2026-08-20): BUILD FIRST, COMMIT SECOND.
	//
	// This used to write the config and then build the provider. When the build
	// failed - a disabled provider, an unknown model, a missing key - the error
	// was returned and looked like a refusal, but the config had ALREADY moved.
	// The footer and status bar read the config, so they showed the model the
	// user picked, while the agent still held its previous provider and answered
	// every message with the old model. Nothing in the interface disagreed with
	// itself, so there was nothing to notice.
	//
	// Building first makes the failure honest: if the provider cannot be made,
	// the config is untouched and the interface keeps telling the truth.
	provider, err := createAgentProviderFor(agentName, modelID)
	if err != nil {
		return models.Model{}, fmt.Errorf("failed to create provider for model %s: %w", modelID, err)
	}

	if err := config.UpdateAgentModel(agentName, modelID); err != nil {
		return models.Model{}, fmt.Errorf("failed to update config: %w", err)
	}

	a.setProv(provider)

	return a.prov().Model(), nil
}

func (a *agent) Summarize(ctx context.Context, sessionID string) error {
	if a.summarizeProvider == nil {
		return fmt.Errorf("summarize provider not available")
	}

	// Check if session is busy
	if a.IsSessionBusy(sessionID) {
		return ErrSessionBusy
	}

	// Create a new context with cancellation
	summarizeCtx, cancel := context.WithCancel(ctx)

	// Store the cancel function in activeRequests to allow cancellation
	a.activeRequests.Store(sessionID+"-summarize", cancel)

	go func() {
		defer a.activeRequests.Delete(sessionID + "-summarize")
		defer cancel()
		event := AgentEvent{
			Type:     AgentEventTypeSummarize,
			Progress: "Starting summarization...",
		}

		a.Publish(pubsub.CreatedEvent, event)
		// Get all messages from the session
		msgs, err := a.messages.List(summarizeCtx, sessionID)
		msgs = answerOrphanedToolCalls(msgs)
		if err != nil {
			event = AgentEvent{
				Type:  AgentEventTypeError,
				Error: fmt.Errorf("failed to list messages: %w", err),
				Done:  true,
			}
			a.Publish(pubsub.CreatedEvent, event)
			return
		}
		summarizeCtx = context.WithValue(summarizeCtx, tools.SessionIDContextKey, sessionID)

		if len(msgs) == 0 {
			event = AgentEvent{
				Type:  AgentEventTypeError,
				Error: fmt.Errorf("no messages to summarize"),
				Done:  true,
			}
			a.Publish(pubsub.CreatedEvent, event)
			return
		}

		event = AgentEvent{
			Type:     AgentEventTypeSummarize,
			Progress: "Analyzing conversation...",
		}
		a.Publish(pubsub.CreatedEvent, event)

		// Add a system message to guide the summarization
		summarizePrompt := "Provide a detailed but concise summary of our conversation above. Focus on information that would be helpful for continuing the conversation, including what we did, what we're doing, which files we're working on, and what we're going to do next."

		// Create a new message with the summarize prompt
		promptMsg := message.Message{
			Role:  message.User,
			Parts: []message.ContentPart{message.TextContent{Text: summarizePrompt}},
		}

		// Append the prompt to the messages
		msgsWithPrompt := append(msgs, promptMsg)

		event = AgentEvent{
			Type:     AgentEventTypeSummarize,
			Progress: "Generating summary...",
		}

		a.Publish(pubsub.CreatedEvent, event)

		// Send the messages to the summarize provider
		response, err := a.summarizeProvider.SendMessages(
			summarizeCtx,
			msgsWithPrompt,
			make([]tools.BaseTool, 0),
		)
		if err != nil {
			event = AgentEvent{
				Type:  AgentEventTypeError,
				Error: fmt.Errorf("failed to summarize: %w", err),
				Done:  true,
			}
			a.Publish(pubsub.CreatedEvent, event)
			return
		}

		summary := strings.TrimSpace(response.Content)
		if summary == "" {
			event = AgentEvent{
				Type:  AgentEventTypeError,
				Error: fmt.Errorf("empty summary returned"),
				Done:  true,
			}
			a.Publish(pubsub.CreatedEvent, event)
			return
		}
		event = AgentEvent{
			Type:     AgentEventTypeSummarize,
			Progress: "Creating new session...",
		}

		a.Publish(pubsub.CreatedEvent, event)
		oldSession, err := a.sessions.Get(summarizeCtx, sessionID)
		if err != nil {
			event = AgentEvent{
				Type:  AgentEventTypeError,
				Error: fmt.Errorf("failed to get session: %w", err),
				Done:  true,
			}

			a.Publish(pubsub.CreatedEvent, event)
			return
		}
		// Create a message in the new session with the summary
		msg, err := a.messages.Create(summarizeCtx, oldSession.ID, message.CreateMessageParams{
			Role: message.Assistant,
			Parts: []message.ContentPart{
				message.TextContent{Text: summary},
				message.Finish{
					Reason: message.FinishReasonEndTurn,
					Time:   time.Now().Unix(),
				},
			},
			Model: a.summarizeProvider.Model().ID,
		})
		if err != nil {
			event = AgentEvent{
				Type:  AgentEventTypeError,
				Error: fmt.Errorf("failed to create summary message: %w", err),
				Done:  true,
			}

			a.Publish(pubsub.CreatedEvent, event)
			return
		}
		oldSession.SummaryMessageID = msg.ID
		// The context has genuinely been emptied and refilled with a summary,
		// so the gauge resets. The cumulative counters do not — summarising
		// costs real tokens, and a ledger that forgets them at every
		// compaction would understate a long session by exactly the amount
		// the user is most likely to be asking about.
		oldSession.CompletionTokens = response.Usage.OutputTokens
		oldSession.PromptTokens = 0
		oldSession.CumulativeCompletionTokens += response.Usage.OutputTokens
		oldSession.CumulativePromptTokens += response.Usage.InputTokens +
			response.Usage.CacheCreationTokens + response.Usage.CacheReadTokens
		model := a.summarizeProvider.Model()
		usage := response.Usage
		cost := model.CostPer1MInCached/1e6*float64(usage.CacheCreationTokens) +
			model.CostPer1MOutCached/1e6*float64(usage.CacheReadTokens) +
			model.CostPer1MIn/1e6*float64(usage.InputTokens) +
			model.CostPer1MOut/1e6*float64(usage.OutputTokens)
		oldSession.Cost += cost
		_, err = a.sessions.Save(summarizeCtx, oldSession)
		if err != nil {
			event = AgentEvent{
				Type:  AgentEventTypeError,
				Error: fmt.Errorf("failed to save session: %w", err),
				Done:  true,
			}
			a.Publish(pubsub.CreatedEvent, event)
		}

		event = AgentEvent{
			Type:      AgentEventTypeSummarize,
			SessionID: oldSession.ID,
			Progress:  "Summary complete",
			Done:      true,
		}
		a.Publish(pubsub.CreatedEvent, event)
		// Send final success event with the new session ID
	}()

	return nil
}

// createAgentProvider builds a provider for whatever model the agent is
// CONFIGURED to use.
func createAgentProvider(agentName config.AgentName) (provider.Provider, error) {
	cfg := config.Get()
	if cfg == nil {
		return nil, fmt.Errorf("config not loaded")
	}
	agentConfig, ok := cfg.Agents[agentName]
	if !ok {
		return nil, fmt.Errorf("agent %s not found", agentName)
	}
	return createAgentProviderFor(agentName, agentConfig.Model)
}

// createAgentProviderFor builds a provider for an EXPLICIT model, without
// consulting the agent's configured model.
//
// GORILLA OVERRIDE (2026-08-20): this split exists so a model switch can be
// made atomic. Update() used to write the config first and build the provider
// second; when the build failed, the config had already moved and the live
// agent kept its old provider. The result was a program whose config, footer
// and status bar all said one model while a completely different one answered
// every message - observed on 2026-08-20, where the picker was set to
// antigravity.claude-sonnet-4-6 sixteen seconds before the session started and
// every reply in the database came from local.meta/llama-3.3-70b-instruct.
//
// That is this project's oldest failure class wearing new clothes: the NAME of
// a thing is not its CONTENTS, and silence and success must not look alike.
func createAgentProviderFor(agentName config.AgentName, modelID models.ModelID) (provider.Provider, error) {
	cfg := config.Get()
	// GORILLA OVERRIDE: config.Get() returns nil until Load() has run, and
	// cfg.Agents on a nil *Config panics. Production always loads first, so this
	// was never hit in anger — but RebuildProvider now calls this from a deferred
	// drain at the end of a turn, and a panic there would take down the request
	// goroutine. An error is the right answer for "not configured yet".
	if cfg == nil {
		return nil, fmt.Errorf("config not loaded")
	}
	agentConfig, ok := cfg.Agents[agentName]
	if !ok {
		return nil, fmt.Errorf("agent %s not found", agentName)
	}
	model, ok := models.SupportedModels[modelID]
	if !ok {
		return nil, fmt.Errorf("model %s not supported", modelID)
	}

	// GORILLA OVERRIDE: local models don't use a single providers.local key —
	// each routes to its own endpoint with that endpoint's key (multi-endpoint).
	var apiKey string
	if model.Provider == models.ProviderLocal {
		_, apiKey, _ = models.LocalRouteFor(model.ID)
	} else {
		providerCfg, ok := cfg.Providers[model.Provider]
		if !ok {
			return nil, fmt.Errorf("provider %s not supported", model.Provider)
		}
		if providerCfg.Disabled {
			return nil, fmt.Errorf("provider %s is not enabled", model.Provider)
		}
		apiKey = providerCfg.APIKey
	}
	maxTokens := model.DefaultMaxTokens
	if agentConfig.MaxTokens > 0 {
		maxTokens = agentConfig.MaxTokens
	}
	opts := []provider.ProviderClientOption{
		provider.WithAPIKey(apiKey),
		provider.WithModel(model),
		provider.WithSystemMessage(prompt.GetAgentPrompt(agentName, model.Provider)),
		provider.WithMaxTokens(maxTokens),
	}
	if model.Provider == models.ProviderOpenAI || model.Provider == models.ProviderLocal && model.CanReason {
		opts = append(
			opts,
			provider.WithOpenAIOptions(
				provider.WithReasoningEffort(agentConfig.ReasoningEffort),
			),
		)
	} else if model.Provider == models.ProviderAnthropic && model.CanReason && agentName == config.AgentCoder {
		opts = append(
			opts,
			provider.WithAnthropicOptions(
				provider.WithAnthropicShouldThinkFn(provider.DefaultShouldThinkFn),
			),
		)
	}
	agentProvider, err := provider.NewProvider(
		model.Provider,
		opts...,
	)
	if err != nil {
		return nil, fmt.Errorf("could not create provider: %v", err)
	}

	return agentProvider, nil
}

// corruptedToolInput reports arguments that were damaged on the wire, or ""
// if they look intact.
//
// It deliberately checks only for evidence a DECODER produced — replacement
// characters, and JSON that will not parse at all — rather than trying to
// judge whether the arguments are sensible. Sensible is the tool's job;
// intact is this layer's.
func corruptedToolInput(input string) string {
	if strings.ContainsRune(input, '\uFFFD') {
		return "it contains Unicode replacement characters, which means bytes arrived that were not valid UTF-8"
	}
	if strings.TrimSpace(input) == "" {
		return ""
	}
	var probe any
	if err := json.Unmarshal([]byte(input), &probe); err != nil {
		// GORILLA FIX (2026-10-05): try the repair BEFORE refusing.
		//
		// tools.UnmarshalToolInput mends the things a model really sends: an
		// unescaped Windows path, a real newline inside a string, a list that
		// arrived as text. But this check ran first and refused anything a
		// strict decode rejected, so the call never reached the tool and the
		// repair never ran in the loop at all. Its tests called the function
		// directly and passed. Found by audit.
		var mended map[string]any
		if tools.UnmarshalToolInput(input, &mended) == nil {
			return ""
		}
		return notValidJSON + " (" + err.Error() + ")"
	}
	return ""
}

// notValidJSON marks the one reason that is NOT a transport fault. Arguments
// that stop being JSON part-way through were almost always cut off by the
// model's output limit, and telling the model "send the same call again
// unchanged" sends it round the same loop until the stuck detector ends the
// turn. See corruptedToolAdvice.
const notValidJSON = "the arguments are not complete, valid JSON"

// corruptedToolAdvice is what the model is told to do about a refused call.
func corruptedToolAdvice(reason string) string {
	if strings.HasPrefix(reason, notValidJSON) {
		return "The call was NOT run. The usual cause is that the call was cut off by the " +
			"output limit before it was finished, so sending it again unchanged will fail " +
			"the same way. Send a SMALLER call: write a long file in several parts (create " +
			"it with the first part, then add the rest with edit), or shorten the text."
	}
	return "This is NOT a mistake in what you sent — the text was corrupted between " +
		"the model and this program. Send the same call again unchanged."
}

// prov returns the provider this agent is using right now.
func (a *agent) prov() provider.Provider {
	a.providerMu.RLock()
	defer a.providerMu.RUnlock()
	return a.provider
}

func (a *agent) setProv(p provider.Provider) {
	a.providerMu.Lock()
	a.provider = p
	a.providerMu.Unlock()
}

// answerOrphanedToolCalls gives every tool call in a loaded history a result.
//
// GORILLA FIX (2026-10-05): if the program was killed, or crashed, while a tool
// was running, the session is left holding a tool call that was never answered.
// OpenAI and Anthropic refuse any history shaped like that, on EVERY later
// turn, and /compact sends the same history, so the conversation could never be
// used again. Nothing repaired it.
//
// The repair is made on the copy being sent, not in the database: what is
// stored stays the record of what happened. The model is told the truth, that
// the call never came back.
func answerOrphanedToolCalls(msgs []message.Message) []message.Message {
	out := make([]message.Message, 0, len(msgs))
	for i := 0; i < len(msgs); i++ {
		m := msgs[i]
		out = append(out, m)
		calls := m.ToolCalls()
		if m.Role != message.Assistant || len(calls) == 0 {
			continue
		}
		answered := map[string]bool{}
		j := i + 1
		for ; j < len(msgs) && msgs[j].Role == message.Tool; j++ {
			for _, r := range msgs[j].ToolResults() {
				answered[r.ToolCallID] = true
			}
			out = append(out, msgs[j])
		}
		i = j - 1
		var missing []message.ContentPart
		for _, c := range calls {
			if !answered[c.ID] {
				missing = append(missing, message.ToolResult{
					ToolCallID: c.ID,
					Content: "This call never returned a result: the program stopped while it was running. " +
						"It may or may not have taken effect. Check the current state before relying on it.",
					IsError: true,
				})
			}
		}
		if len(missing) > 0 {
			out = append(out, message.Message{Role: message.Tool, SessionID: m.SessionID, Parts: missing})
		}
	}
	return out
}

// wireView returns the history as the provider will actually SEND it.
//
// GORILLA FIX (2026-10-05): the "will this fit?" check measured the raw history
// and every tool. The request that goes out has had stale and aged file reads
// replaced by one-line stubs and carries only the tools in view, so it is much
// smaller. A long session was therefore refused with "the request was not
// sent" although it fitted, and each "continue" did one round and was refused
// again.
func (a *agent) wireView(msgs []message.Message) []message.Message {
	if w, ok := a.prov().(interface {
		WireMessages([]message.Message) []message.Message
	}); ok {
		return w.WireMessages(msgs)
	}
	return msgs
}
