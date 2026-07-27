package agent

import (
	"context"
	"sync"
	"testing"
)

// newBusyAgent returns an agent with a fake in-flight request, so IsBusy() is
// true without needing a provider, a session store, or a network call.
func newBusyAgent(t *testing.T, sessionID string) *agent {
	t.Helper()
	a := &agent{activeRequests: sync.Map{}}
	_, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	a.activeRequests.Store(sessionID, cancel)
	if !a.IsBusy() {
		t.Fatal("setup failed: agent should report busy with a stored cancel func")
	}
	return a
}

// A rebuild requested mid-turn must be REMEMBERED and reported as deferred.
// It used to be dropped silently with no return value, so a /context toggle
// during a turn reported a new token count and changed nothing.
func TestRebuildProviderDefersWhileBusy(t *testing.T) {
	a := newBusyAgent(t, "session-1")

	if deferred := a.RebuildProvider(); !deferred {
		t.Error("RebuildProvider() = false while busy, want true so the UI can say the change is queued")
	}
	if !a.pendingRebuild.Load() {
		t.Error("the deferred rebuild was not recorded — it would be lost entirely")
	}
}

// Once the turn ends the pending rebuild must be consumed. The provider itself
// cannot be constructed in a unit test (it needs a configured model), so the
// assertion is on the flag being drained — that is the part that decides whether
// the change is ever applied.
func TestPendingRebuildIsDrainedWhenTheTurnEnds(t *testing.T) {
	a := newBusyAgent(t, "session-1")
	a.RebuildProvider()

	// Nothing may happen while still busy.
	a.drainPendingRebuild()
	if !a.pendingRebuild.Load() {
		t.Fatal("pending rebuild was consumed while a turn was still in flight — the provider must not be swapped mid-request")
	}

	// Turn finishes: Run deletes the session before draining.
	a.activeRequests.Delete("session-1")
	a.drainPendingRebuild()

	if a.pendingRebuild.Load() {
		t.Error("pending rebuild still set after the turn ended — the queued change would never be applied")
	}
}

// With two sessions running, the first to finish must NOT apply the rebuild —
// the second is still mid-request. The flag has to survive until the last one
// completes.
func TestPendingRebuildSurvivesUntilTheLastSessionFinishes(t *testing.T) {
	a := newBusyAgent(t, "session-1")
	_, cancel2 := context.WithCancel(context.Background())
	t.Cleanup(cancel2)
	a.activeRequests.Store("session-2", cancel2)

	a.RebuildProvider()

	a.activeRequests.Delete("session-1")
	a.drainPendingRebuild()
	if !a.pendingRebuild.Load() {
		t.Error("rebuild applied while session-2 was still running")
	}

	a.activeRequests.Delete("session-2")
	a.drainPendingRebuild()
	if a.pendingRebuild.Load() {
		t.Error("rebuild not applied after the last session finished")
	}
}

// Draining with nothing pending must be a no-op, so an ordinary turn does not
// pay for a provider rebuild it never asked for.
func TestDrainWithNothingPendingIsANoOp(t *testing.T) {
	a := &agent{activeRequests: sync.Map{}}
	a.drainPendingRebuild()
	if a.pendingRebuild.Load() {
		t.Error("drain set the pending flag")
	}
}

// Concurrent completions must not race on the flag. Run with -race.
func TestDrainIsSafeUnderConcurrentCompletions(t *testing.T) {
	a := &agent{activeRequests: sync.Map{}}
	a.pendingRebuild.Store(true)

	var wg sync.WaitGroup
	for i := 0; i < 32; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			a.drainPendingRebuild()
		}()
	}
	wg.Wait()

	if a.pendingRebuild.Load() {
		t.Error("flag still set after concurrent drains")
	}
}
