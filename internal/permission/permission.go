package permission

import (
	"errors"
	"path/filepath"
	"slices"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/opencode-ai/opencode/internal/config"
	"github.com/opencode-ai/opencode/internal/logging"
	"github.com/opencode-ai/opencode/internal/pubsub"
)

var ErrorPermissionDenied = errors.New("permission denied")

// PermissionWait is how long a tool waits for an answer before giving up.
//
// GORILLA FIX (2026-08-17): there was NO timeout. The line read
//
//	// Wait for the response with a timeout
//	resp := <-respCh
//
// — a comment describing a feature that did not exist. A tool call that asked
// for permission and never got an answer blocked on that channel forever.
//
// Observed on a real run: three research helpers stopped mid-flow at 19:43 and
// 19:53 and were still reported as "running" at 20:09. The process held no
// network connections and had written nothing for fifteen minutes. The user
// was watching a counter that said 3 helpers and a footer that said "waiting
// for tool response", both of which were true and neither of which was useful,
// because nothing was ever going to arrive.
//
// Generous on purpose: someone may reasonably walk away mid-prompt and come
// back. What must not happen is waiting forever. When it expires the request is
// DENIED, which the tool reports as a failure — a lane that failed loudly is
// recoverable; a lane that hangs silently poisons the whole run.
var permissionWait = 10 * time.Minute

// PermissionWait is the shipped value, for documentation and display.
const PermissionWait = 10 * time.Minute

// TimedOutError distinguishes "nobody answered" from "the user said no", so a
// report can tell the difference between a refusal and an absence.
var TimedOutError = errors.New("permission request timed out with no answer")

// GrantWholeTool is the GrantKey for a tool whose grants are deliberately
// tool-wide rather than per-item.
//
// GORILLA FIX (2026-08-23): a SENTINEL, not an empty string. An empty key is
// what a forgotten key looks like, and TestEveryPermissionRequestSetsAGrantKey
// exists precisely because four tools once forgot theirs. "Chose the whole
// tool, on purpose" and "nobody thought about it" must not be the same value.
//
// WHY ANY TOOL WOULD CHOOSE THIS. The grain of a grant should match what the
// tool can do harm with, and for some tools the item is not that thing. A
// search term leaves the machine, comes back as text, and touches nothing you
// own; pinning the grant to the exact string means "Allow for session" only
// ever covers a search you are unlikely to repeat, so the button does nothing
// and the user is asked again every single time. Maximum interruption, zero
// information transferred, which is how a consent dialog becomes a thing people
// click through without reading.
//
// Compare web_fetch, which keys on scheme://host: a fetch names an arbitrary
// server, and the server is exactly what a poisoned page would change, so the
// host is worth pinning. Same mechanism, different grain, chosen per tool.
//
// A tool using this MUST say so on its dialog. A grant wider than the sentence
// the user read is the failure this whole file is trying to avoid.
const GrantWholeTool = "*"

type CreatePermissionRequest struct {
	SessionID   string `json:"session_id"`
	ToolName    string `json:"tool_name"`
	Description string `json:"description"`
	Action      string `json:"action"`
	Params      any    `json:"params"`
	Path        string `json:"path"`
	// GrantKey identifies WHAT is being approved, so a remembered grant covers
	// that thing rather than the whole tool. A bash request sets the command; a
	// file tool sets the resolved path. Empty keeps the old tool-wide behaviour
	// for callers that have no meaningful identity to offer.
	//
	// GORILLA FIX (2026-08-18): without this, "Allow for session" on one
	// `cat README.md` silently authorised every later bash command in the
	// session tree — the dialog showed one command in a fenced block and the
	// grant matched only ToolName+Action+SessionID+Path, with Params stored and
	// never compared.
	GrantKey string `json:"grant_key"`
	// Egress marks a request that sends something OFF this machine — a fetch,
	// a search, an MCP call to a remote server. Auto-approve does not cover
	// egress unconditionally, because the sink is where a prompt injection
	// gets paid. See mustAskAnyway.
	Egress bool `json:"egress"`
}

type PermissionRequest struct {
	ID          string `json:"id"`
	SessionID   string `json:"session_id"`
	ToolName    string `json:"tool_name"`
	Description string `json:"description"`
	Action      string `json:"action"`
	Params      any    `json:"params"`
	Path        string `json:"path"`
	// GrantKey — see CreatePermissionRequest.GrantKey.
	GrantKey string `json:"grant_key"`
	// Egress — see CreatePermissionRequest.Egress.
	Egress bool `json:"egress"`
	// AutoApproveOverridden records that this prompt is being shown DESPITE
	// auto-approve being on, and why. The dialog says so: a prompt appearing
	// in a mode the user believes is unattended is confusing unless it
	// explains itself.
	AutoApproveOverridden string `json:"auto_approve_overridden,omitempty"`
}

type Service interface {
	pubsub.Suscriber[PermissionRequest]
	GrantPersistant(permission PermissionRequest)
	Grant(permission PermissionRequest)
	Deny(permission PermissionRequest)
	Request(opts CreatePermissionRequest) bool
	AutoApproveSession(sessionID string)
	// RevokeAutoApprove turns YOLO mode back off for a session.
	RevokeAutoApprove(sessionID string)
	// SetUnattended declares that NOBODY IS WATCHING — a -p run, a cron job,
	// a script. See the comment on mustAskAnyway.
	SetUnattended(bool)
	// IsAutoApproved reports whether a session (or its root) is running
	// unattended, so the UI can keep saying so.
	IsAutoApproved(sessionID string) bool
	// RegisterChildSession records that child belongs to parent, so a grant
	// made in the conversation the USER can see also covers the helper
	// sessions spawned underneath it. See rootSession.
	RegisterChildSession(child, parent string)
	// GrantFleet approves a set of TOOLS for everything under root until
	// RevokeFleet is called. See the fleetGrants field for why this exists and
	// what it deliberately gives up.
	GrantFleet(root string, tools []string)
	// RevokeFleet ends a fleet grant. The caller that opened one MUST close it,
	// normally with a defer, or the widening outlives the run.
	RevokeFleet(root string)
	// HasFleetGrant reports whether a fleet grant covers this tool, so a caller
	// can tell "approved for the run" apart from "not asked yet".
	HasFleetGrant(sessionID, toolName string) bool
	// IsCovered reports whether an ALREADY PUBLISHED request would now be
	// approved without asking, because a grant arrived after it was raised.
	//
	// GORILLA FIX (2026-08-23): a burst of requests is queued rather than
	// overwritten, so a queue can hold questions that answering an earlier one
	// has already settled. Asking them anyway is how "allow for session" came
	// to mean "and now answer the same thing four more times".
	IsCovered(p PermissionRequest) bool
}

type permissionService struct {
	*pubsub.Broker[PermissionRequest]

	mu                  sync.RWMutex
	sessionPermissions  []PermissionRequest
	pendingRequests     sync.Map
	autoApproveSessions []string
	// childToParent maps a helper session to the session that spawned it.
	//
	// GORILLA FIX (2026-08-17): "Allow for session" was stored against the
	// session that happened to ask, and every research helper runs in its OWN
	// session (CreateTaskSession, one per lane). So approving a web search in
	// helper a3 did nothing for a1, a2 or a4 — a 10-helper run asked the same
	// question up to ten times, and again on the next run because new helpers
	// mean new session ids. Reported from a live run: the same `web_search` for
	// the same term prompted at 19:43, 19:45 and again at 19:49.
	//
	// A grant now applies to the whole tree under the session the user is
	// actually looking at. This does NOT widen what was approved — tool, action
	// and path still have to match exactly — it only stops the same approval
	// being re-asked by each sibling helper.
	childToParent map[string]string
	// fleetGrants records "asked once for the whole run" approvals: a root
	// session id to the set of tool names approved for it.
	//
	// GORILLA FIX (2026-08-23): ROADMAP item 4, and the owner's explicit call.
	//
	// The 2026-08-17 fix above closed the SESSION axis: a grant made in one
	// helper now covers its siblings. It did not close the GRANT KEY axis, and
	// for web_search the grant key is the QUERY. Ten helpers run ten different
	// queries, so they are ten different questions and the user was asked ten
	// times. No amount of answering made it stop, because being asked once per
	// query is what b333c23 deliberately built.
	//
	// The owner's ruling, 2026-08-23: "in order to get a web search either i
	// have to click ten or twenty times if the search has to restart... most of
	// the models will just allow you to accept once and that accept will work
	// for all the batched models."
	//
	// So a fleet grant IS a widening, and it is meant to be one. It is bounded
	// three ways so that it stays a considered trade rather than a hole:
	//   - by TOOL: only the tools named in the prompt, never the whole session.
	//   - by RUN: the research tool revokes it when the run returns, so it
	//     cannot outlive the thing it was granted for.
	//   - by DISCLOSURE: the prompt says the run will search for terms the
	//     model writes later from pages it reads. Consent to a category you
	//     cannot read in advance is still consent, but only if it says so.
	fleetGrants map[string]map[string]bool
	// unattended is set by the non-interactive entry point.
	unattended bool
}

// rootSession walks a helper session up to the conversation it belongs to.
// Depth-limited so a malformed cycle degrades to "treat it as its own root"
// instead of hanging the tool call that is waiting on this answer.
func (s *permissionService) rootSession(id string) string {
	for i := 0; i < 32; i++ {
		parent, ok := s.childToParent[id]
		if !ok || parent == "" || parent == id {
			return id
		}
		id = parent
	}
	return id
}

func (s *permissionService) RegisterChildSession(child, parent string) {
	if child == "" || parent == "" || child == parent {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.childToParent == nil {
		s.childToParent = make(map[string]string)
	}
	s.childToParent[child] = parent
}

func (s *permissionService) IsCovered(p PermissionRequest) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	root := s.rootSession(p.SessionID)
	if s.hasFleetGrantLocked(root, p.ToolName) {
		return true
	}
	for _, g := range s.sessionPermissions {
		if g.ToolName == p.ToolName && g.Action == p.Action &&
			g.SessionID == root && g.Path == p.Path && g.GrantKey == p.GrantKey {
			return true
		}
	}
	return false
}

func (s *permissionService) GrantFleet(root string, tools []string) {
	if root == "" || len(tools) == 0 {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	// Resolve to the TREE ROOT, because that is what the lookup in Request
	// resolves to. Storing under whatever id the caller happened to hold would
	// file the grant somewhere nothing ever reads, and the symptom would be the
	// prompt storm the grant exists to stop, with no error anywhere.
	root = s.rootSession(root)
	if s.fleetGrants == nil {
		s.fleetGrants = make(map[string]map[string]bool)
	}
	set, ok := s.fleetGrants[root]
	if !ok {
		set = make(map[string]bool)
		s.fleetGrants[root] = set
	}
	for _, t := range tools {
		if t != "" {
			set[t] = true
		}
	}
}

func (s *permissionService) RevokeFleet(root string) {
	if root == "" {
		return
	}
	s.mu.Lock()
	delete(s.fleetGrants, s.rootSession(root))
	s.mu.Unlock()
}

// hasFleetGrant assumes the caller holds at least a read lock and that root is
// already resolved, so Request can do its whole lookup under one acquisition.
func (s *permissionService) hasFleetGrantLocked(root, toolName string) bool {
	return s.fleetGrants[root][toolName]
}

func (s *permissionService) HasFleetGrant(sessionID, toolName string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.hasFleetGrantLocked(s.rootSession(sessionID), toolName)
}

func (s *permissionService) GrantPersistant(permission PermissionRequest) {
	if v, ok := s.pendingRequests.Load(permission.ID); ok {
		if p, ok := v.(pendingRequest); ok {
			p.ch <- true
		}
	}
	s.mu.Lock()
	s.sessionPermissions = append(s.sessionPermissions, permission)
	s.mu.Unlock()
}

func (s *permissionService) Grant(permission PermissionRequest) {
	if v, ok := s.pendingRequests.Load(permission.ID); ok {
		if p, ok := v.(pendingRequest); ok {
			p.ch <- true
		}
	}
}

func (s *permissionService) Deny(permission PermissionRequest) {
	if v, ok := s.pendingRequests.Load(permission.ID); ok {
		if p, ok := v.(pendingRequest); ok {
			p.ch <- false
		}
	}
}

// hasGrant reports whether a remembered session grant covers this exact
// request. Exposed for tests so the matching rule can be asserted directly
// rather than inferred from a full Request round-trip.
func (s *permissionService) hasGrant(sessionID, toolName, action, path, grantKey string) bool {
	s.mu.RLock()
	grants := slices.Clone(s.sessionPermissions)
	s.mu.RUnlock()
	for _, p := range grants {
		if p.ToolName == toolName && p.Action == action &&
			p.SessionID == sessionID && p.Path == path && p.GrantKey == grantKey {
			return true
		}
	}
	return false
}

func (s *permissionService) Request(opts CreatePermissionRequest) bool {
	// Grants belong to the conversation, not to whichever helper happened to
	// ask, so both the auto-approve list and the remembered grants are checked
	// against the root of the session tree.
	s.mu.RLock()
	root := s.rootSession(opts.SessionID)
	grants := slices.Clone(s.sessionPermissions)
	autoApproved := slices.Contains(s.autoApproveSessions, root) || slices.Contains(s.autoApproveSessions, opts.SessionID)
	fleet := s.hasFleetGrantLocked(root, opts.ToolName)
	s.mu.RUnlock()

	// GORILLA FIX (2026-08-19): this used to be an unconditional `return true`
	// and it was the FIRST thing in the function — before grant matching,
	// before anything. Auto-approve meant total, with zero carve-outs, so
	// every "it's fine, the user gets asked" claim elsewhere in the codebase
	// was false the moment YOLO was on.
	//
	// It now has carve-outs. Note the ordering: a carve-out does not deny, it
	// falls THROUGH to the normal path, so a remembered grant still covers it
	// and the user is asked once per thing rather than once per call.
	override := ""
	if autoApproved {
		override = s.mustAskAnyway(opts, root)
		if override == "" {
			return true
		}
		s.mu.RLock()
		unattended := s.unattended
		s.mu.RUnlock()
		if unattended {
			logging.Warn("auto-approving under a carve-out because nobody is watching",
				"tool", opts.ToolName, "action", opts.Action, "reason", override,
				"grant_key", opts.GrantKey, "session", root)
			return true
		}
	}
	dir := normalisePermissionPath(opts.Path)
	permission := PermissionRequest{
		ID:                    uuid.New().String(),
		Path:                  dir,
		SessionID:             root,
		ToolName:              opts.ToolName,
		Description:           opts.Description,
		Action:                opts.Action,
		Params:                opts.Params,
		GrantKey:              opts.GrantKey,
		Egress:                opts.Egress,
		AutoApproveOverridden: override,
	}

	for _, p := range grants {
		if p.ToolName == permission.ToolName && p.Action == permission.Action &&
			p.SessionID == permission.SessionID && p.Path == permission.Path &&
			p.GrantKey == permission.GrantKey {
			return true
		}
	}

	// A fleet grant is checked AFTER the exact-match grants and AFTER the
	// auto-approve carve-outs, so it changes nothing for a session that never
	// opened one. It is checked BEFORE publishing, which is the whole point:
	// the prompt this would have raised is the prompt the user already answered.
	if fleet {
		logging.Info("covered by a fleet grant approved for this run",
			"tool", permission.ToolName, "action", permission.Action,
			"grant_key", permission.GrantKey, "session", root)
		return true
	}

	respCh := make(chan bool, 1)

	// The session travels with the waiter: a sweep that cannot tell whose
	// request it is would cancel other conversations' prompts too.
	s.pendingRequests.Store(permission.ID, pendingRequest{ch: respCh, sessionID: root})
	defer s.pendingRequests.Delete(permission.ID)

	s.Publish(pubsub.CreatedEvent, permission)

	// Wait for an answer, but never forever. See PermissionWait.
	timer := time.NewTimer(permissionWait)
	defer timer.Stop()
	select {
	case resp := <-respCh:
		return resp
	case <-timer.C:
		logging.Warn("permission request timed out with no answer — denying so the tool fails instead of hanging",
			"tool", permission.ToolName, "action", permission.Action, "session", permission.SessionID,
			"waited", permissionWait.String())
		return false
	}
}

// CancelSession denies every request still waiting for this session (or any
// helper beneath it), so killing a run actually releases the tools blocked
// inside it instead of leaving goroutines parked on a channel.
func (s *permissionService) CancelSession(sessionID string) int {
	s.mu.RLock()
	root := s.rootSession(sessionID)
	s.mu.RUnlock()

	released := 0
	s.pendingRequests.Range(func(key, value any) bool {
		p, ok := value.(pendingRequest)
		if !ok || p.sessionID != root {
			return true
		}
		// Non-blocking: the waiter's channel is buffered, and a request that
		// has already been answered must not deadlock the sweep.
		select {
		case p.ch <- false:
			released++
		default:
		}
		return true
	})
	return released
}

// pendingRequest is a waiter and the conversation it belongs to.
type pendingRequest struct {
	ch        chan bool
	sessionID string
}

// SetUnattended records that there is no human at the keyboard.
//
// GORILLA FIX (2026-08-19): the auto-approve carve-outs added the same day
// assume a prompt can be ANSWERED. In a -p run or a cron job there is no TUI
// subscribed to the permission broker, so a carve-out would publish a question
// into an empty room, wait the full ten-minute PermissionWait, and then deny —
// turning a working headless script into one that hangs for ten minutes and
// then fails. That is a worse outcome than the risk the carve-out addresses,
// and it would have shipped invisibly, because every test in the suite answers
// its own prompts.
//
// So in unattended mode the carve-outs LOG rather than prompt. The security
// argument is unchanged and honest: `-p` is the user explicitly saying "run
// without me". The value of the carve-out was always that a human sees the
// question; with no human, the only thing left worth doing is leaving a record
// of what was waved through.
func (s *permissionService) SetUnattended(v bool) {
	s.mu.Lock()
	s.unattended = v
	s.mu.Unlock()
}

func (s *permissionService) AutoApproveSession(sessionID string) {
	s.mu.Lock()
	s.autoApproveSessions = append(s.autoApproveSessions, sessionID)
	s.mu.Unlock()
}

// RevokeAutoApprove ends YOLO mode. Kept separate from Grant/Deny because it
// is a standing stance, not an answer to one request.
func (s *permissionService) RevokeAutoApprove(sessionID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	root := s.rootSession(sessionID)
	s.autoApproveSessions = slices.DeleteFunc(s.autoApproveSessions, func(id string) bool {
		return id == sessionID || id == root
	})
}

// IsAutoApproved answers for the whole session tree, so a helper cannot report
// a different stance from the conversation it belongs to.
func (s *permissionService) IsAutoApproved(sessionID string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return slices.Contains(s.autoApproveSessions, s.rootSession(sessionID)) ||
		slices.Contains(s.autoApproveSessions, sessionID)
}

// active is the process's permission service, so display code can ask about
// YOLO state without threading the service through every component. Mirrors
// agent.ActiveSubAgentCount(), which the status bar already uses for helpers.
// One service exists per process; NewPermissionService sets it.
var active Service

// SessionAutoApproved reports whether a session is running unattended. Safe
// before the service exists (returns false), because the status bar renders
// during startup.
// CancelForSession releases every tool blocked on a permission prompt for this
// session. Called when a helper is killed, so "kill" means released NOW rather
// than "released in up to PermissionWait".
func CancelForSession(sessionID string) int {
	if active == nil || sessionID == "" {
		return 0
	}
	if svc, ok := active.(*permissionService); ok {
		return svc.CancelSession(sessionID)
	}
	return 0
}

func SessionAutoApproved(sessionID string) bool {
	if active == nil || sessionID == "" {
		return false
	}
	return active.IsAutoApproved(sessionID)
}

func NewPermissionService() Service {
	svc := &permissionService{
		Broker:             pubsub.NewBroker[PermissionRequest](),
		sessionPermissions: make([]PermissionRequest, 0),
		childToParent:      make(map[string]string),
	}
	active = svc
	return svc
}

// normalisePermissionPath returns the path a permission request is recorded
// against. It is the scope of a "allow for this session" grant, so it must be
// exactly what the caller asked for and never wider.
//
// GORILLA OVERRIDE: this was filepath.Dir(opts.Path), taking the PARENT of a path
// the caller had already resolved to a directory. Every caller passes a
// directory: edit/write pass the workspace root chosen by tools.permissionScope,
// patch passes filepath.Dir of the file, and bash/fetch/MCP pass
// config.WorkingDirectory(). Taking Dir of those widened every grant by one
// level — a request scoped to /home/user/project was stored as /home/user — and
// because the session-permission check compares Path exactly, a single grant in
// one project then also matched edits in every SIBLING directory, which
// collapsed to the same stored parent. That silently undid the per-root scoping
// the tool layer computes.
//
// A path with no directory component still falls back to the working directory,
// which is what the old `dir == "."` branch existed for.
func normalisePermissionPath(p string) string {
	if p == "" || p == "." {
		return config.WorkingDirectory()
	}
	return p
}

// PermissionWaitForTest shortens the wait so the timeout path can be asserted
// in under a second, and returns a function restoring it. Test-only seam: the
// alternative is a test that takes ten minutes, which is a test nobody runs.
func PermissionWaitForTest(d time.Duration) func() {
	prev := permissionWait
	permissionWait = d
	return func() { permissionWait = prev }
}

// mustAskAnyway returns the reason auto-approve does NOT cover this request,
// or "" if it does.
//
// Three carve-outs, and each one is a sink rather than a source:
//
//  1. Egress. Whatever a hostile page talked the model into, it has to leave
//     the machine to matter. This is the single control that turns "the model
//     was tricked" into "the model was tricked and could not act on it".
//  2. A path outside every configured root. Auto-approve is a statement about
//     the work in front of you; it is not consent to touch ~/.ssh.
//  3. A tainted turn. The conversation has read something a stranger wrote,
//     so the next action is not necessarily the user's idea.
//
// It does not deny anything. It only declines to skip the question.
func (s *permissionService) mustAskAnyway(opts CreatePermissionRequest, root string) string {
	if opts.Egress {
		if reason, ok := TaintOf(root); ok {
			return "this leaves the machine, and this turn has already read untrusted content (" + reason.Reason + ")"
		}
		return "this sends a request off the machine"
	}
	if IsTainted(root) {
		reason, _ := TaintOf(root)
		return "this turn has already read untrusted content (" + reason.Reason + ")"
	}
	if p := opts.Path; p != "" && p != "." && filepath.IsAbs(p) {
		if _, inRoot := config.RootFor(p); !inRoot {
			return "this targets " + p + ", which is outside every workspace root"
		}
	}
	return ""
}
