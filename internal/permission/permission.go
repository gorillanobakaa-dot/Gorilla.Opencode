package permission

import (
	"errors"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
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
var permissionWait = PermissionWait

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
	// Irreversible, when not empty, is the plain sentence saying why a wrong
	// yes here cannot be taken back ("it deletes the whole disk ..."). The
	// caller sets it; this package does not judge commands.
	//
	// GORILLA OVERRIDE (2026-10-04): auto-approve does not cover these. The
	// policy is OpenHands' ConfirmRisky (openhands-sdk security/
	// confirmation_policy.py, MIT) reduced to the one level that matters here:
	// everything else may be waved through, this may not. See mustAskAnyway.
	Irreversible string `json:"irreversible,omitempty"`
	// FamilySuggestion is the family the dialog proposes for "Allow family":
	// a prefix of GrantKey, chosen by the caller from what this session has
	// already asked (see AskedKeys). Empty means the dialog offers no family.
	//
	// GORILLA FIX (2026-10-11): measured on a 58-minute Nemotron session —
	// 31 bash prompts, 30 distinct commands, 16 of them the same program with
	// a different last argument. "Allow for session" keyed on the exact text
	// could not help, and the person was asked thirty times for one answer.
	// See GrantFamily for what a family is allowed to cover.
	FamilySuggestion string `json:"family_suggestion,omitempty"`
	// HardStop, when not empty, is the plain sentence saying why auto-approve
	// does not cover this request although it can be undone: it installs
	// software system-wide or runs as the administrator ("installs packages
	// with brew", "runs as the administrator (sudo)"). The caller sets it.
	//
	// GORILLA OVERRIDE (2026-10-23): one of the two things auto-approve still
	// stops for (see mustAskAnyway). Unlike Irreversible, an unattended run
	// logs it and goes ahead: a script that installs its own dependencies is
	// what -p is for.
	HardStop string `json:"hard_stop,omitempty"`
	// MayReachNetwork marks a shell command that can send data off the machine
	// or run code that can (ssh, git push, pip, an interpreter running a
	// script). It changes nothing in an ordinary turn. In a turn that has read
	// untrusted content it is the shell's exfiltration path, so auto-approve
	// asks. See mustAskAnyway.
	MayReachNetwork bool `json:"may_reach_network,omitempty"`
	// SessionClass is the class "Allow for session" remembers for this request,
	// as a family of GrantKey (bash: `pip3 install`), or "" for the exact key.
	// The place is Path. The dialog prints both before the person answers.
	SessionClass string `json:"session_class,omitempty"`
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
	// Irreversible — see CreatePermissionRequest.Irreversible. Carried on the
	// published request so a queued prompt can be judged by IsCovered with the
	// same rule Request applies: a family never covers one of these.
	Irreversible string `json:"irreversible,omitempty"`
	// FamilySuggestion — see CreatePermissionRequest.FamilySuggestion.
	FamilySuggestion string `json:"family_suggestion,omitempty"`
	// HardStop, MayReachNetwork, SessionClass — see CreatePermissionRequest.
	HardStop        string `json:"hard_stop,omitempty"`
	MayReachNetwork bool   `json:"may_reach_network,omitempty"`
	SessionClass    string `json:"session_class,omitempty"`
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
	// Deny is a PERSON saying no: the model is told and the turn carries on.
	Deny(permission PermissionRequest)
	// Refuse answers no when nobody answered (input closed, the editor went
	// away): the turn ends, as a timeout does. See TakePersonDenial.
	Refuse(permission PermissionRequest)
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
	// GrantFamily answers a pending request yes AND remembers, for the whole
	// session tree, every later request of the same tool, action and path
	// whose grant key the tool's FamilyMatcher places inside family.
	//
	// What a family never covers, however it was chosen: a request marked
	// Irreversible, a request marked Egress, and any request raised while the
	// turn is tainted. Those ask exactly as they did before families existed.
	// A tool with no registered matcher, or a family the matcher does not
	// accept for this request, degrades to a one-off allow and is logged.
	GrantFamily(permission PermissionRequest, family string)
	// AskedKeys returns the grant keys this tool has PROMPTED for in this
	// session tree, oldest first, so a caller can propose a family from what
	// the person has already been asked. Bounded; covered requests are not
	// recorded, only ones that reached a person.
	AskedKeys(sessionID, toolName string) []string
}

// FamilyMatcher decides whether grantKey falls inside a family a person
// approved for this tool. It is the tool's own syntax that decides — for bash,
// what counts as "the same command with a different tail" — so this package
// never guesses: a tool that registers no matcher gets no family grants.
type FamilyMatcher func(grantKey, family string) bool

var (
	familyMatchersMu sync.RWMutex
	familyMatchers   = map[string]FamilyMatcher{}
)

// RegisterFamilyMatcher opts a tool into family grants. Called from the tool's
// package init; the matcher must be a pure function of its two arguments.
func RegisterFamilyMatcher(toolName string, m FamilyMatcher) {
	if toolName == "" || m == nil {
		return
	}
	familyMatchersMu.Lock()
	familyMatchers[toolName] = m
	familyMatchersMu.Unlock()
}

func familyMatcherFor(toolName string) (FamilyMatcher, bool) {
	familyMatchersMu.RLock()
	defer familyMatchersMu.RUnlock()
	m, ok := familyMatchers[toolName]
	return m, ok
}

// familyGrant is one "Allow family" answer: the tool, action and path of the
// request it answered, and the family the person read on the dialog.
type familyGrant struct {
	SessionID string
	ToolName  string
	Action    string
	Path      string
	Family    string
}

// maxAskedKeys bounds the per-tool history AskedKeys returns.
const maxAskedKeys = 64

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
	// familyGrants are the "Allow family" answers, keyed by nothing: a scan
	// over a slice that is a few entries long for the life of a session.
	familyGrants []familyGrant
	// askedKeys records, per root session and tool, the grant keys that were
	// actually published as prompts, so the next prompt can propose a family.
	askedKeys map[string][]string
	// unattended is set by the non-interactive entry point.
	unattended bool
	// knownEgress records, per root session, the egress grant keys (a fetch's
	// scheme://host, a search tool's "*") that have already been let through
	// in this conversation. Under auto-approve a tainted turn asks only for a
	// key NOT in here: reading a second page on the site the person named is
	// not an exfiltration path; a site the page just suggested is.
	knownEgress map[string]map[string]bool
	// personDenied records, per root session, that the last refusal came from a
	// person pressing Deny (not a timeout, not an unattended refusal). The
	// agent takes it to decide whether the turn carries on. See TakePersonDenial.
	personDenied map[string]bool
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
	root := s.rootSession(p.SessionID)
	if s.hasFleetGrantLocked(root, p.ToolName) {
		s.mu.RUnlock()
		return true
	}
	for _, g := range s.sessionPermissions {
		if g.SessionID == root && sessionGrantCovers(g, p.ToolName, p.Action, p.Path, p.GrantKey) {
			s.mu.RUnlock()
			return true
		}
	}
	s.mu.RUnlock()
	// Outside the lock: the taint lookup takes this service's read lock itself.
	return s.familyCovers(root, p.ToolName, p.Action, p.Path, p.GrantKey, p.Irreversible, p.Egress)
}

// familyCovers reports whether an "Allow family" answer covers this request.
//
// The three refusals come first and are unconditional. They are the same
// three things a person is asked about even under auto-approve (mustAskAnyway),
// because they are the cases where "the same command with a different tail"
// is exactly the attack: an injected tail that deletes, that sends, or that
// was written after the turn read something a stranger wrote.
func (s *permissionService) familyCovers(root, toolName, action, path, grantKey, irreversible string, egress bool) bool {
	if irreversible != "" || egress || grantKey == "" {
		return false
	}
	if IsTainted(root) {
		return false
	}
	m, ok := familyMatcherFor(toolName)
	if !ok {
		return false
	}
	s.mu.RLock()
	grants := slices.Clone(s.familyGrants)
	s.mu.RUnlock()
	for _, g := range grants {
		if g.ToolName != toolName || g.Action != action || g.SessionID != root {
			continue
		}
		// The path is the scope the person saw: a family approved on a command
		// naming C:\work\Fieldkit covers commands naming that folder or files
		// inside it, never a different place outside the roots.
		if !pathWithin(path, g.Path) {
			continue
		}
		if m(grantKey, g.Family) {
			return true
		}
	}
	return false
}

// pathWithin reports whether p is dir itself or lies underneath it. Case is
// ignored on Windows, where the filesystem ignores it too.
func pathWithin(p, dir string) bool {
	if p == "" || dir == "" {
		return p == dir
	}
	p, dir = filepath.Clean(p), filepath.Clean(dir)
	if runtime.GOOS == "windows" {
		p, dir = strings.ToLower(p), strings.ToLower(dir)
	}
	if p == dir {
		return true
	}
	return strings.HasPrefix(p, dir+string(filepath.Separator))
}

func (s *permissionService) GrantFamily(p PermissionRequest, family string) {
	family = strings.TrimSpace(family)
	m, ok := familyMatcherFor(p.ToolName)
	switch {
	case family == "":
		logging.Warn("family grant without a family; allowing this one request only",
			"tool", p.ToolName, "grant_key", p.GrantKey)
		s.Grant(p)
		return
	case !ok:
		logging.Warn("family grant for a tool that registered no family matcher; allowing this one request only",
			"tool", p.ToolName, "family", family)
		s.Grant(p)
		return
	case !m(p.GrantKey, family):
		// The family must cover the very request it was chosen on, or the
		// person approved a sentence that does not describe what just ran.
		logging.Warn("family does not cover the request it was chosen on; allowing this one request only",
			"tool", p.ToolName, "grant_key", p.GrantKey, "family", family)
		s.Grant(p)
		return
	}
	s.Grant(p)
	s.mu.Lock()
	s.familyGrants = append(s.familyGrants, familyGrant{
		SessionID: p.SessionID, // already the tree root: Request stores it so
		ToolName:  p.ToolName,
		Action:    p.Action,
		Path:      p.Path,
		Family:    family,
	})
	s.mu.Unlock()
	logging.Info("family grant recorded", "tool", p.ToolName, "action", p.Action,
		"path", p.Path, "family", family, "session", p.SessionID)
}

func (s *permissionService) AskedKeys(sessionID, toolName string) []string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	root := s.rootSession(sessionID)
	return slices.Clone(s.askedKeys[root+"\x00"+toolName])
}

// recordAsked remembers a key that is about to be published as a prompt.
func (s *permissionService) recordAsked(root, toolName, key string) {
	if key == "" || key == GrantWholeTool {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.askedKeys == nil {
		s.askedKeys = make(map[string][]string)
	}
	k := root + "\x00" + toolName
	keys := append(s.askedKeys[k], key)
	if len(keys) > maxAskedKeys {
		keys = keys[len(keys)-maxAskedKeys:]
	}
	s.askedKeys[k] = keys
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

// GrantPersistant is "Allow for session": a class plus a place.
//
// GORILLA OVERRIDE (2026-10-23): it remembered one exact string, so a person
// who allowed `pip3 install --user rich` was asked again for `pip3 install
// --user typer`. It now remembers the CLASS the dialog printed (the request's
// SessionClass, recorded as a family, so a bash class is bounded by the same
// rules as "Allow family": plain tail, same place, never irreversible, never
// egress, never in a tainted turn) and, for file-changing tools, every change
// inside the same workspace root (see sessionGrantCovers). The exact request
// is still stored too, so a request no class covers is answered as before.
func (s *permissionService) GrantPersistant(permission PermissionRequest) {
	if v, ok := s.pendingRequests.Load(permission.ID); ok {
		if p, ok := v.(pendingRequest); ok {
			p.ch <- true
		}
	}
	s.mu.Lock()
	s.sessionPermissions = append(s.sessionPermissions, permission)
	if c := strings.TrimSpace(permission.SessionClass); c != "" && permission.Irreversible == "" {
		if m, ok := familyMatcherFor(permission.ToolName); ok && m(permission.GrantKey, c) {
			s.familyGrants = append(s.familyGrants, familyGrant{
				SessionID: permission.SessionID, ToolName: permission.ToolName,
				Action: permission.Action, Path: permission.Path, Family: c,
			})
		}
	}
	s.mu.Unlock()
}

// sessionGrantCovers reports whether a remembered "Allow for session" answer
// covers a request. The exact match is the rule for every tool. File-changing
// tools also match by place: a change anywhere inside the same root, by any of
// the edit tools, because the dialog said "every file change inside <root>"
// and a diff in git is where such a change is reviewed anyway.
func sessionGrantCovers(g PermissionRequest, toolName, action, path, grantKey string) bool {
	if g.ToolName == toolName && g.Action == action && g.Path == path && g.GrantKey == grantKey {
		return true
	}
	return editTools[g.ToolName] && editTools[toolName] && g.Path != "" && g.Path == path &&
		g.Irreversible == ""
}

// SessionScope is the sentence the dialog prints next to "Allow for session":
// exactly what the answer will cover, in the words a person reads.
func SessionScope(p PermissionRequest) string {
	place := p.Path
	switch {
	case p.Irreversible != "":
		return "this exact command, once more in this session"
	case editTools[p.ToolName] && place != "":
		return "every file change inside " + place
	case strings.TrimSpace(p.SessionClass) != "":
		return "every `" + strings.TrimSpace(p.SessionClass) + " ...` command run in " + place +
			" (one command each, no chaining; not after untrusted content was read)"
	case p.GrantKey == GrantWholeTool:
		return "every call of " + p.ToolName + " in this session"
	case p.Egress && strings.Contains(p.GrantKey, "://"):
		return "every page on " + p.GrantKey
	}
	return "this exact request"
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
			// Recorded BEFORE the waiter wakes, so the tool's error and this
			// mark reach the agent in that order.
			s.mu.Lock()
			if s.personDenied == nil {
				s.personDenied = make(map[string]bool)
			}
			s.personDenied[p.sessionID] = true
			s.mu.Unlock()
			p.ch <- false
		}
	}
}

func (s *permissionService) Refuse(permission PermissionRequest) {
	if v, ok := s.pendingRequests.Load(permission.ID); ok {
		if p, ok := v.(pendingRequest); ok {
			p.ch <- false
		}
	}
}

// TakePersonDenial reports, once, whether the last refusal in this session
// tree was a person pressing Deny, and clears the mark.
//
// GORILLA FIX (2026-10-23): a Deny ended the whole turn (the agent finished the
// message as "permission denied"), so the model never learned what the person
// wanted instead and the screen sat on the last tool until the person typed
// again. Observed on the MacBook Air: one Deny on a probe and the stranger
// install stopped dead. A person's Deny is now an answer the model reads; a
// timeout or an unattended refusal still ends the turn, because nobody is
// there to read what the model does next.
func TakePersonDenial(sessionID string) bool {
	svc, ok := active.(*permissionService)
	if !ok || sessionID == "" {
		return false
	}
	svc.mu.Lock()
	defer svc.mu.Unlock()
	root := svc.rootSession(sessionID)
	was := svc.personDenied[root]
	delete(svc.personDenied, root)
	return was
}

// DeniedByPersonResult is what the model reads after a person pressed Deny.
const DeniedByPersonResult = "Denied by the person: they saw this request and said no, so it was NOT run. " +
	"Do not retry it or a variant of it. Carry on with the rest of the task another way, " +
	"or ask the person about it in plain text in your reply."

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

	// GORILLA OVERRIDE (2026-10-11): the standing mode (config/permissionmode.go).
	// See modeDecision in mode.go for plan and accept-edits; "auto" takes the
	// same path /yolo always took, carve-outs included.
	if config.PermissionMode() == config.PermissionAuto {
		autoApproved = true
	}
	if decided, answer := s.modeDecision(opts, root); decided {
		if !answer {
			logging.Info("plan mode: refused without asking", "tool", opts.ToolName,
				"action", opts.Action, "grant_key", opts.GrantKey, "session", root)
		}
		return answer
	}

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
			s.noteEgress(root, opts)
			return true
		}
		s.mu.RLock()
		unattended := s.unattended
		s.mu.RUnlock()
		if unattended && opts.Irreversible != "" {
			// The other carve-outs log and proceed when nobody is watching,
			// because refusing them would make unattended runs useless. This
			// one refuses: a leak can be rotated, a deleted disk cannot.
			logging.Warn("refusing an irreversible action because nobody is watching",
				"tool", opts.ToolName, "action", opts.Action, "reason", override,
				"grant_key", opts.GrantKey, "session", root)
			return false
		}
		if unattended {
			logging.Warn("auto-approving under a carve-out because nobody is watching",
				"tool", opts.ToolName, "action", opts.Action, "reason", override,
				"grant_key", opts.GrantKey, "session", root)
			s.noteEgress(root, opts)
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
		Irreversible:          opts.Irreversible,
		FamilySuggestion:      opts.FamilySuggestion,
		HardStop:              opts.HardStop,
		MayReachNetwork:       opts.MayReachNetwork,
		SessionClass:          opts.SessionClass,
		AutoApproveOverridden: override,
	}

	for _, p := range grants {
		if p.SessionID == permission.SessionID &&
			sessionGrantCovers(p, permission.ToolName, permission.Action, permission.Path, permission.GrantKey) {
			s.noteEgress(root, opts)
			return true
		}
	}

	// A family grant is checked AFTER the exact grants and, like them, AFTER
	// the auto-approve carve-outs. It refuses on its own account the three
	// things a carve-out would have asked about (see familyCovers), so turning
	// auto-approve off never makes a family cover more.
	if s.familyCovers(root, permission.ToolName, permission.Action, permission.Path,
		permission.GrantKey, opts.Irreversible, opts.Egress) {
		logging.Info("covered by a family grant", "tool", permission.ToolName,
			"action", permission.Action, "grant_key", permission.GrantKey, "session", root)
		s.noteEgress(root, opts)
		return true
	}

	// A fleet grant is checked AFTER the exact-match grants and AFTER the
	// auto-approve carve-outs, so it changes nothing for a session that never
	// opened one. It is checked BEFORE publishing, which is the whole point:
	// the prompt this would have raised is the prompt the user already answered.
	if fleet {
		logging.Info("covered by a fleet grant approved for this run",
			"tool", permission.ToolName, "action", permission.Action,
			"grant_key", permission.GrantKey, "session", root)
		s.noteEgress(root, opts)
		return true
	}

	respCh := make(chan bool, 1)

	// The session travels with the waiter: a sweep that cannot tell whose
	// request it is would cancel other conversations' prompts too.
	s.pendingRequests.Store(permission.ID, pendingRequest{ch: respCh, sessionID: root})
	defer s.pendingRequests.Delete(permission.ID)

	// Recorded only for prompts that reach a person: a family is proposed
	// from what they have already been asked, not from what was waved through.
	s.recordAsked(root, permission.ToolName, permission.GrantKey)
	s.Publish(pubsub.CreatedEvent, permission)

	// Wait for an answer, but never forever. See PermissionWait.
	timer := time.NewTimer(permissionWait)
	defer timer.Stop()
	select {
	case resp := <-respCh:
		if resp {
			s.noteEgress(root, opts)
		}
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
	if config.PermissionMode() == config.PermissionAuto {
		return true
	}
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

// noteEgress remembers an egress key that was let through, so a later request
// to the same place is not "a new destination" (see mustAskAnyway).
func (s *permissionService) noteEgress(root string, opts CreatePermissionRequest) {
	if !opts.Egress || opts.GrantKey == "" {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.knownEgress == nil {
		s.knownEgress = make(map[string]map[string]bool)
	}
	if s.knownEgress[root] == nil {
		s.knownEgress[root] = make(map[string]bool)
	}
	s.knownEgress[root][opts.ToolName+"\x00"+opts.GrantKey] = true
}

func (s *permissionService) egressKnown(root string, opts CreatePermissionRequest) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.knownEgress[root][opts.ToolName+"\x00"+opts.GrantKey]
}

// mustAskAnyway returns the reason auto-approve does NOT cover this request,
// or "" if it does.
//
// GORILLA OVERRIDE (2026-10-23): auto-approve now stops only for what is
// destructive or can carry data away. Measured on the MacBook Air the same
// day (v0.1.149-dev, Gemini 3.8 Flash High, auto-approve on): one stranger
// install raised four dialogs, for a probe of /Library, a README fetch, a
// second page on the same site and `python3 --version && git --version`. None
// of the four could do harm. The owner: "asa nu se poate lucra cu un coding
// agent". Each earlier fear, and what still answers it, is in
// docs/PERMISSIONS.md; in short:
//
//  1. Irreversible (a wiped disk, a forced push): asks. Unchanged.
//  2. HardStop (a system-wide install, sudo): asks. New: these ran unasked
//     before whenever the turn was not tainted.
//  3. Egress: was "every request off the machine asks". Now asks only for a
//     destination this conversation has not used, and only after untrusted
//     content was read: that is the exfiltration path the 2026-08-19 fear
//     named (a poisoned page steering the model to its own server). The site
//     the person named, and further pages on it, do not ask.
//  4. Taint: was "every non-edit tool asks after untrusted content". For the
//     shell it now asks only for a command that can reach the network or run
//     a script (MayReachNetwork). `python3 --version` and other read-only
//     commands never reach this function at all (bash's read-only gate).
//     Other non-edit tools keep the old rule.
//  5. A path outside every root (a write, a delete, a non-read-only command
//     naming that place): asks. Unchanged. Reads outside the roots are not
//     requests: view never asks, and read-only shell commands skip the gate.
//
// The rule as it stood until 2026-10-23, kept so the change is readable:
//
//	Three carve-outs, and each one is a sink rather than a source:
//	 1. Egress. Whatever a hostile page talked the model into, it has to leave
//	    the machine to matter. This is the single control that turns "the model
//	    was tricked" into "the model was tricked and could not act on it".
//	 2. A path outside every configured root. Auto-approve is a statement about
//	    the work in front of you; it is not consent to touch ~/.ssh.
//	 3. A tainted turn. The conversation has read something a stranger wrote,
//	    so the next action is not necessarily the user's idea.
//
// It does not deny anything. It only declines to skip the question.
func (s *permissionService) mustAskAnyway(opts CreatePermissionRequest, root string) string {
	// First, because it is the one carve-out whose cost is not a leak or a
	// stray file but a loss that nothing restores.
	if opts.Irreversible != "" {
		return "this cannot be undone: " + opts.Irreversible
	}
	if opts.HardStop != "" {
		return opts.HardStop
	}
	reason, tainted := TaintOf(root)
	if opts.Egress {
		if tainted && !s.egressKnown(root, opts) {
			return "this sends a request to a place this conversation has not used, after reading untrusted content (" + reason.Reason + ")"
		}
		return ""
	}
	// GORILLA OVERRIDE (2026-10-11, owner decision "B"): after untrusted
	// content, an EDIT INSIDE THE ROOTS no longer asks. It used to: one web
	// page read made every later tool in the turn prompt, which turned auto
	// mode back into ask mode for the whole of any research turn. What still
	// asks after untrusted content: every non-edit tool (a shell command can
	// do anything), egress (above), a path outside the roots (below) and an
	// irreversible action (above). An edit inside the project is visible in
	// the diff and in git, which is where the person reviews it anyway.
	//
	// GORILLA OVERRIDE (2026-10-23): "every non-edit tool" is narrowed for the
	// shell to commands that can reach the network or run a script; see the
	// function comment, point 4.
	if tainted && !editTools[opts.ToolName] {
		if opts.ToolName != shellToolName || opts.MayReachNetwork {
			return "this turn has already read untrusted content (" + reason.Reason + "), and this can send data off the machine"
		}
	}
	// GORILLA OVERRIDE (2026-09-01): the filepath.IsAbs precondition is gone.
	//
	// It made this whole check unreachable on Windows for exactly the paths it
	// exists to catch. filepath.IsAbs("/etc/hosts") is FALSE there — Windows
	// calls that rooted, not absolute — so the carve-out skipped the root test
	// and auto-approved the write. The repo's own carve-out test says it plainly:
	// "YOLO authorised a write outside every workspace root without asking".
	//
	// config.RootFor now resolves rooted paths against the volume rather than
	// against the project, so it can answer for any spelling. A genuinely
	// relative path still resolves inside the primary root and is still
	// auto-approved, which is the behaviour ordinary work depends on.
	if p := opts.Path; p != "" && p != "." {
		if _, inRoot := config.RootFor(p); !inRoot {
			return "this targets " + p + ", which is outside every workspace root"
		}
	}
	return ""
}

// shellToolName is the bash tool's name. Spelled here rather than imported:
// the tools package imports this one.
const shellToolName = "bash"
