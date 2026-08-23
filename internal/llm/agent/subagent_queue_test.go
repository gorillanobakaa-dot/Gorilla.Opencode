package agent

import (
	"context"
	"sync"
	"testing"
	"time"
)

// THE BUG, reported 2026-08-14 with screenshots: 10 helpers requested, 4 shown.
//
// RegisterSubAgent was called INSIDE runHelper, which a goroutine only reaches
// after winning a slot on the concurrency semaphore. So a helper waiting its
// turn was alive and completely invisible: absent from /tasks, absent from the
// status count, and — the part that actually mattered — absent from the kill
// switch. KillAllSubAgents walks the registry, so the Nuclear Option cancelled
// the four holding slots, which RELEASED those slots, which let the next four
// start. Pressing it did not stop a research run.
//
// Registration now happens before the helper queues.

func clearRegistry(t *testing.T) {
	t.Helper()
	for _, info := range ListSubAgents() {
		UnregisterSubAgent(info.ID)
	}
	t.Cleanup(func() {
		for _, info := range ListSubAgents() {
			UnregisterSubAgent(info.ID)
		}
	})
}

// Every helper in a run must be visible from the moment it is scheduled, even
// when the concurrency cap means most of them are waiting.
func TestEveryScheduledHelperIsVisibleWhileStillQueued(t *testing.T) {
	clearRegistry(t)

	const helpers, slots = 10, 4
	sem := make(chan struct{}, slots)
	release := make(chan struct{})
	var wg sync.WaitGroup

	for i := 0; i < helpers; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			// This is the ordering under test: REGISTER, then queue.
			entry := RegisterSubAgentState("s", "parent", "call", "lane", SubAgentQueued, cancel)
			defer UnregisterSubAgent(entry.ID)

			select {
			case sem <- struct{}{}:
			case <-ctx.Done():
				return
			}
			defer func() { <-sem }()
			SetSubAgentState(entry.ID, SubAgentRunning)
			<-release
		}(i)
	}

	// Give the goroutines a moment to register and contend.
	deadline := time.Now().Add(2 * time.Second)
	for len(ListSubAgents()) < helpers && time.Now().Before(deadline) {
		time.Sleep(5 * time.Millisecond)
	}

	got := ListSubAgents()
	if len(got) != helpers {
		t.Errorf("registry holds %d helpers, want %d — the queued ones are invisible again, "+
			"which is what made a 10-helper run look like a 4-helper one", len(got), helpers)
	}
	counts := SubAgentStateCounts()
	if counts[SubAgentQueued] == 0 {
		t.Error("no helper is reported as QUEUED; the state is not being used and the cap is invisible")
	}
	if counts[SubAgentRunning] > slots {
		t.Errorf("%d helpers reported RUNNING with only %d slots", counts[SubAgentRunning], slots)
	}

	close(release)
	wg.Wait()
}

// THE LOAD-BEARING FIX: a QUEUED helper must be killable. If it is not, the
// Nuclear Option just makes room for the next batch.
func TestKillingAQueuedHelperStopsItBeforeItSpendsAnything(t *testing.T) {
	clearRegistry(t)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	entry := RegisterSubAgentState("s", "parent", "call", "queued lane", SubAgentQueued, cancel)

	started := make(chan struct{})
	finished := make(chan struct{})
	go func() {
		defer close(finished)
		close(started)
		// Stand in for waiting on a full semaphore.
		select {
		case <-ctx.Done():
		case <-time.After(5 * time.Second):
			t.Error("queued helper was never cancelled; killing it did nothing")
		}
	}()
	<-started

	if _, ok := KillSubAgent(entry.ID); !ok {
		t.Fatal("a queued helper could not be killed — it is not in the registry")
	}
	select {
	case <-finished:
	case <-time.After(2 * time.Second):
		t.Fatal("kill did not cancel the queued helper's context")
	}
}

// The Nuclear Option must reach queued helpers too.
func TestNuclearOptionReachesQueuedHelpers(t *testing.T) {
	clearRegistry(t)

	var cancels int
	var mu sync.Mutex
	for i := 0; i < 6; i++ {
		state := SubAgentQueued
		if i < 2 {
			state = SubAgentRunning
		}
		RegisterSubAgentState("s", "parent", "call", "lane", state, func() {
			mu.Lock()
			cancels++
			mu.Unlock()
		})
	}

	if n := KillAllSubAgents(); n != 6 {
		t.Errorf("nuclear option killed %d of 6 — queued helpers survived, so releasing "+
			"the slots would simply start the next batch", n)
	}
	mu.Lock()
	defer mu.Unlock()
	if cancels != 6 {
		t.Errorf("%d cancel funcs called, want 6", cancels)
	}
}

// A finished helper must stop counting as active without vanishing, so the user
// can tell "it answered" from "it was never there".
// SCOPE WARNING, added 2026-08-23. This drives the registry API directly and
// never a goroutine, so it says nothing about whether a row survives the helper
// that made it. It passed for the entire life of ROADMAP item 2, while /tasks
// deleted every finished row microseconds after showing it.
//
// It is still worth having: it pins the count-vs-visibility split. But the
// lifecycle is covered by TestAFinishedHelperRowSurvivesItsOwnGoroutine below,
// and that is the one that fails if the per-helper unregister comes back.
func TestFinishedHelpersStopCountingButStayVisible(t *testing.T) {
	clearRegistry(t)

	e1 := RegisterSubAgentState("s1", "p", "c", "one", SubAgentRunning, func() {})
	e2 := RegisterSubAgentState("s2", "p", "c", "two", SubAgentQueued, func() {})
	if got := ActiveSubAgentCount(); got != 2 {
		t.Fatalf("active count %d, want 2", got)
	}

	SetSubAgentState(e1.ID, SubAgentDone)
	if got := ActiveSubAgentCount(); got != 1 {
		t.Errorf("after one finished, active count is %d, want 1 — a finished helper is "+
			"still being counted as if it were spending", got)
	}
	if len(ListSubAgents()) != 2 {
		t.Error("the finished helper vanished from the list; the user cannot tell it landed")
	}
	SetSubAgentState(e2.ID, SubAgentFailed)
	if got := ActiveSubAgentCount(); got != 0 {
		t.Errorf("active count %d with nothing live, want 0", got)
	}
}

// NON-VACUOUS GUARD: the OLD ordering — register only after winning a slot —
// must fail the visibility test above.
func TestTheOldRegisterAfterSlotOrderingHidesTheQueue(t *testing.T) {
	clearRegistry(t)

	const helpers, slots = 10, 4
	sem := make(chan struct{}, slots)
	release := make(chan struct{})
	var wg sync.WaitGroup

	for i := 0; i < helpers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			sem <- struct{}{} // OLD ORDER: queue first...
			defer func() { <-sem }()
			entry := RegisterSubAgent("s", "parent", "call", "lane", func() {}) // ...register second
			defer UnregisterSubAgent(entry.ID)
			<-release
		}()
	}

	deadline := time.Now().Add(500 * time.Millisecond)
	for len(ListSubAgents()) < slots && time.Now().Before(deadline) {
		time.Sleep(5 * time.Millisecond)
	}
	if n := len(ListSubAgents()); n > slots {
		t.Fatalf("the old ordering exposed %d helpers; it cannot exceed the %d slots, "+
			"so this guard is not reproducing the bug", n, slots)
	}
	close(release)
	wg.Wait()
}
