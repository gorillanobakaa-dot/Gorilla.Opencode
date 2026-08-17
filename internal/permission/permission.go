package permission

import (
	"errors"
	"slices"
	"sync"

	"github.com/google/uuid"
	"github.com/opencode-ai/opencode/internal/config"
	"github.com/opencode-ai/opencode/internal/pubsub"
)

var ErrorPermissionDenied = errors.New("permission denied")

type CreatePermissionRequest struct {
	SessionID   string `json:"session_id"`
	ToolName    string `json:"tool_name"`
	Description string `json:"description"`
	Action      string `json:"action"`
	Params      any    `json:"params"`
	Path        string `json:"path"`
}

type PermissionRequest struct {
	ID          string `json:"id"`
	SessionID   string `json:"session_id"`
	ToolName    string `json:"tool_name"`
	Description string `json:"description"`
	Action      string `json:"action"`
	Params      any    `json:"params"`
	Path        string `json:"path"`
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
	// IsAutoApproved reports whether a session (or its root) is running
	// unattended, so the UI can keep saying so.
	IsAutoApproved(sessionID string) bool
	// RegisterChildSession records that child belongs to parent, so a grant
	// made in the conversation the USER can see also covers the helper
	// sessions spawned underneath it. See rootSession.
	RegisterChildSession(child, parent string)
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

func (s *permissionService) GrantPersistant(permission PermissionRequest) {
	respCh, ok := s.pendingRequests.Load(permission.ID)
	if ok {
		respCh.(chan bool) <- true
	}
	s.mu.Lock()
	s.sessionPermissions = append(s.sessionPermissions, permission)
	s.mu.Unlock()
}

func (s *permissionService) Grant(permission PermissionRequest) {
	respCh, ok := s.pendingRequests.Load(permission.ID)
	if ok {
		respCh.(chan bool) <- true
	}
}

func (s *permissionService) Deny(permission PermissionRequest) {
	respCh, ok := s.pendingRequests.Load(permission.ID)
	if ok {
		respCh.(chan bool) <- false
	}
}

func (s *permissionService) Request(opts CreatePermissionRequest) bool {
	// Grants belong to the conversation, not to whichever helper happened to
	// ask, so both the auto-approve list and the remembered grants are checked
	// against the root of the session tree.
	s.mu.RLock()
	root := s.rootSession(opts.SessionID)
	grants := slices.Clone(s.sessionPermissions)
	autoApproved := slices.Contains(s.autoApproveSessions, root) || slices.Contains(s.autoApproveSessions, opts.SessionID)
	s.mu.RUnlock()

	if autoApproved {
		return true
	}
	dir := normalisePermissionPath(opts.Path)
	permission := PermissionRequest{
		ID:          uuid.New().String(),
		Path:        dir,
		SessionID:   root,
		ToolName:    opts.ToolName,
		Description: opts.Description,
		Action:      opts.Action,
		Params:      opts.Params,
	}

	for _, p := range grants {
		if p.ToolName == permission.ToolName && p.Action == permission.Action && p.SessionID == permission.SessionID && p.Path == permission.Path {
			return true
		}
	}

	respCh := make(chan bool, 1)

	s.pendingRequests.Store(permission.ID, respCh)
	defer s.pendingRequests.Delete(permission.ID)

	s.Publish(pubsub.CreatedEvent, permission)

	// Wait for the response with a timeout
	resp := <-respCh
	return resp
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
