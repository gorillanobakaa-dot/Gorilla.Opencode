package tui

// GORILLA OVERRIDE (2026-08-23): nine questions you were never shown are not
// nine questions you refused.
//
// The permission dialog holds ONE request in a plain field and Update used to
// assign it unconditionally. A fan-out that raised ten at once painted each
// over the last; only the tenth was ever seen; the other nine stayed parked
// until PermissionWait elapsed and were then denied. It fails closed, so it was
// never a hole. It is dishonest, which in this project is the worse fault: the
// run looked like a network fault for ten minutes and then reported refusals
// nobody made.

import (
	"testing"

	"github.com/opencode-ai/opencode/internal/permission"
)

func req(id, tool, key string) permission.PermissionRequest {
	return permission.PermissionRequest{
		ID: id, SessionID: "root", ToolName: tool, Action: "run", GrantKey: key,
	}
}

func neverCovered(permission.PermissionRequest) bool { return false }
func mustNotGrant(t *testing.T) func(permission.PermissionRequest) {
	return func(p permission.PermissionRequest) {
		t.Errorf("granted %s without asking, and nothing covered it", p.ID)
	}
}

// THE BUG, counted. Every queued request must eventually be offered. Draining
// repeatedly is what the dialog does as each answer comes back.
func TestEveryQueuedRequestIsEventuallyShown(t *testing.T) {
	var queue []permission.PermissionRequest
	for i := 0; i < 10; i++ {
		queue = append(queue, req(string(rune('a'+i)), "web_search", "query"+string(rune('a'+i))))
	}

	var shown []string
	for {
		next, rest := drainPermissionQueue(queue, neverCovered, mustNotGrant(t))
		queue = rest
		if next == nil {
			break
		}
		shown = append(shown, next.ID)
	}

	if len(shown) != 10 {
		t.Fatalf("showed %d of 10 queued requests: %v\n\n"+
			"  The other %d are goroutines parked on a channel that nothing will\n"+
			"  ever republish. They wait out PermissionWait and are then DENIED,\n"+
			"  which the user never saw and never chose.", len(shown), shown, 10-len(shown))
	}
	// Order matters: the first question asked should be the first one raised,
	// or a burst answers itself backwards and the dialog explains nothing.
	if shown[0] != "a" || shown[9] != "j" {
		t.Errorf("shown in order %v, want a..j: the queue is not first-in-first-out", shown)
	}
}

// The point of IsCovered. Approving one thing can settle others already queued,
// and asking again for something just approved is the complaint that started
// all of this, only now with extra steps.
func TestQueuedRequestsSettledByTheLastAnswerAreNotAskedAgain(t *testing.T) {
	queue := []permission.PermissionRequest{
		req("a", "web_search", "one"),
		req("b", "web_search", "two"),
		req("c", "bash", "rm -rf /"),
	}

	// A fleet grant for web_search arrived with the answer just given.
	covered := func(p permission.PermissionRequest) bool { return p.ToolName == "web_search" }

	var granted []string
	next, rest := drainPermissionQueue(queue, covered, func(p permission.PermissionRequest) {
		granted = append(granted, p.ID)
	})

	if next == nil {
		t.Fatal("nothing offered; the bash request is not covered and must still be asked")
	}
	if next.ID != "c" {
		t.Errorf("offered %q, want the bash request %q: the two searches were already "+
			"approved for the run and asking again is the original complaint", next.ID, "c")
	}
	if len(granted) != 2 || granted[0] != "a" || granted[1] != "b" {
		t.Errorf("granted %v, want both searches granted silently", granted)
	}
	if len(rest) != 0 {
		t.Errorf("%d left in the queue, want none", len(rest))
	}
}

// A fleet grant must not swallow the queue whole. If everything is covered the
// dialog closes, and nothing may be left parked.
func TestAFullyCoveredQueueClosesTheDialogAndStrandsNobody(t *testing.T) {
	queue := []permission.PermissionRequest{
		req("a", "web_search", "one"), req("b", "web_fetch", "http://x"),
	}
	var granted int
	next, rest := drainPermissionQueue(queue,
		func(permission.PermissionRequest) bool { return true },
		func(permission.PermissionRequest) { granted++ })

	if next != nil {
		t.Errorf("offered %q although everything was covered", next.ID)
	}
	if granted != 2 {
		t.Errorf("granted %d of 2; a covered request that is neither shown nor granted "+
			"is a goroutine parked until the timeout denies it", granted)
	}
	if len(rest) != 0 {
		t.Errorf("%d left queued", len(rest))
	}
}

// An empty queue is the normal case and must not offer a zero-valued request:
// SetPermissions on an empty ID renders a dialog with no question in it.
func TestAnEmptyQueueOffersNothing(t *testing.T) {
	next, rest := drainPermissionQueue(nil, neverCovered, mustNotGrant(t))
	if next != nil {
		t.Errorf("offered %+v from an empty queue", *next)
	}
	if len(rest) != 0 {
		t.Errorf("rest is %d long", len(rest))
	}
}
