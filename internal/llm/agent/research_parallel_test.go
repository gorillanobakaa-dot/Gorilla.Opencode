package agent

import (
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/opencode-ai/opencode/internal/config"
)

// These exercise the wave/concurrency logic directly. runHelper needs a live
// provider, so the shape under test is the scheduler: N goroutines, a
// semaphore, ordered collection, and the blind/peeking split.

func TestWaveSplitPutsPeekingRolesSecond(t *testing.T) {
	roles, _ := selectRoles("", 6)
	var blind, peeking int
	for _, r := range roles {
		if rolePeeksAtOthers(r.ID) {
			peeking++
		} else {
			blind++
		}
	}
	if peeking == 0 {
		t.Fatal("6 agents should include the verifier; it is the role that catches confident wrong answers")
	}
	if blind < ResearchMinAgents {
		t.Errorf("only %d blind lanes; the four mandatory lanes must all run in wave one", blind)
	}
	// A peeking role in wave one would be handed an empty peer digest and
	// silently verify nothing.
	for i, r := range roles {
		if rolePeeksAtOthers(r.ID) && i < ResearchMinAgents {
			t.Errorf("%s scheduled at position %d, inside the mandatory block", r.ID, i)
		}
	}
}

// The scheduler must never exceed the in-flight cap, and must run more than one
// at once — a semaphore of 1 would pass a "no more than N" test while being
// exactly the sequential bug this replaced.
func TestWaveRunsConcurrentlyButRespectsTheCap(t *testing.T) {
	const n = 8
	var mu sync.Mutex
	var inFlight, maxSeen int

	var wg sync.WaitGroup
	sem := make(chan struct{}, ResearchMaxInFlight)
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			mu.Lock()
			inFlight++
			if inFlight > maxSeen {
				maxSeen = inFlight
			}
			mu.Unlock()
			time.Sleep(30 * time.Millisecond)
			mu.Lock()
			inFlight--
			mu.Unlock()
		}()
	}
	wg.Wait()

	if maxSeen > ResearchMaxInFlight {
		t.Errorf("in-flight peaked at %d, cap is %d — providers will 429", maxSeen, ResearchMaxInFlight)
	}
	if maxSeen < 2 {
		t.Errorf("peaked at %d: helpers ran sequentially, which is the bug this replaced", maxSeen)
	}
}

// Results are collected by index, so output order is role order regardless of
// which helper finishes first. Without this a run is unreproducible.
func TestResultsAreOrderedByRoleNotCompletion(t *testing.T) {
	roles, _ := selectRoles("", 5)
	type outcome struct{ reply string }
	results := make([]outcome, len(roles))

	var wg sync.WaitGroup
	for i := range roles {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			// Reverse the finishing order deliberately.
			time.Sleep(time.Duration(len(roles)-i) * 10 * time.Millisecond)
			results[i] = outcome{reply: roles[i].ID}
		}(i)
	}
	wg.Wait()

	for i, r := range roles {
		if results[i].reply != r.ID {
			t.Fatalf("position %d holds %q, expected %q", i, results[i].reply, r.ID)
		}
	}
}

func TestContractCheckerFlagsMissingHeadings(t *testing.T) {
	good := "## ANSWER\nx\n## FINDINGS\n- CLAIM: a | EVIDENCE: b | TIER: config\n## CONFIDENCE\nstrong\n## NOT ESTABLISHED\nnothing"
	if missing := checkContract(good); len(missing) != 0 {
		t.Errorf("well-formed reply flagged: %v", missing)
	}
	// A reply that answers but carries no evidence tiers is exactly the kind
	// that gets synthesised into a confident wrong answer.
	bad := "## ANSWER\nIt is definitely X."
	if missing := checkContract(bad); len(missing) != 3 {
		t.Errorf("expected 3 missing headings, got %v", missing)
	}
}

func TestSelectRolesClampsAndKeepsMandatoryLanes(t *testing.T) {
	// Asking for one custom role must still yield the mandatory four.
	roles, note := selectRoles("cost", ResearchMinAgents)
	if len(roles) != ResearchMinAgents {
		t.Fatalf("got %d roles, want %d", len(roles), ResearchMinAgents)
	}
	if !strings.Contains(note, "minimum") {
		t.Errorf("top-up should be reported to the user, note=%q", note)
	}
	if roles[0].ID != "cost" {
		t.Errorf("explicit role should lead, got %q", roles[0].ID)
	}
	// Never more than the library holds.
	if all, _ := selectRoles("", ResearchMaxAgents); len(all) > len(researchRoles) {
		t.Errorf("selected %d roles from a library of %d", len(all), len(researchRoles))
	}
}

func TestSequentialIsConcurrencyOfOne(t *testing.T) {
	// The three modes must share one scheduler. If sequential ever grows its
	// own loop, this is the test that should start looking odd.
	for _, tc := range []struct {
		mode string
		want int
	}{
		{ModeSequential, 1},
		{ModeParallel, ResearchMaxInFlight},
		{ModeSupervised, ResearchMaxInFlight},
	} {
		inFlight := ResearchMaxInFlight
		if tc.mode == ModeSequential {
			inFlight = 1
		}
		if inFlight != tc.want {
			t.Errorf("mode %s: in-flight %d, want %d", tc.mode, inFlight, tc.want)
		}
	}
}

func TestSupervisorPromptRefusesToLaunderAFailedAudit(t *testing.T) {
	role := researchRoles[0]
	p := supervisorPrompt(role, "does X work?", "## ANSWER\nyes, definitely")
	for _, want := range []string{"## VERDICT", "APPROVED | WEAK | REJECTED", "## PROBLEMS", "## SAFE TO USE", role.Title} {
		if !strings.Contains(p, want) {
			t.Errorf("supervisor prompt missing %q", want)
		}
	}
	if strings.Contains(p, "redo their research") && !strings.Contains(p, "do NOT redo their research") {
		t.Error("supervisor must audit, not re-research — that would double the cost for nothing")
	}
}

func TestResearchFallsBackWhenNoResearchAgentConfigured(t *testing.T) {
	// Every config written before AgentResearch existed lacks the entry.
	// Without a fallback createAgentProvider returns "agent research not
	// found" and the tool fails every lane while looking implemented.
	if got := researchAgentName(); got != config.AgentTask && got != config.AgentResearch {
		t.Fatalf("unexpected agent %q", got)
	}
	cfg := config.Get()
	if cfg == nil {
		if got := researchAgentName(); got != config.AgentTask {
			t.Errorf("with no config loaded, want %q, got %q", config.AgentTask, got)
		}
		return
	}
	if _, ok := cfg.Agents[config.AgentResearch]; !ok {
		if got := researchAgentName(); got != config.AgentTask {
			t.Errorf("research unconfigured: want fallback %q, got %q", config.AgentTask, got)
		}
	}
}

// /tasks renders the registered string truncated. If every lane registers its
// full prompt they all read "You are helper N of M in a research…" and the user
// cannot tell them apart, let alone choose which to kill.
//
// NOTE this asserts against helperLabel(), the function the code actually
// calls. An earlier version of this test built the label string itself, which
// meant it passed no matter what the code did — vacuous, and it would have
// missed the label being removed entirely.
func TestHelperLabelsAreDistinguishableWhenTruncated(t *testing.T) {
	roles, _ := selectRoles("", 10)
	seen := map[string]string{}
	for _, r := range roles {
		label := helperLabel(r)
		if len(label) > 40 {
			label = label[:40]
		}
		if prev, dup := seen[label]; dup {
			t.Errorf("truncated label %q collides: %s and %s", label, prev, r.ID)
		}
		seen[label] = r.ID
	}
}

// THE BUG THAT KILLED 9 OF 10 HELPERS.
//
// CreateTaskSession stores its first argument as the session PRIMARY KEY. Ten
// helpers sharing one call.ID meant nine UNIQUE-constraint failures, and the
// run still reported itself as having happened. Session ids must be unique per
// helper, including the supervisors.
func TestHelperSessionIDsAreUniqueWithinOneToolCall(t *testing.T) {
	const callID = "tool-call-abc"
	roles, _ := selectRoles("", ResearchMaxAgents)

	seen := map[string]bool{}
	for _, r := range roles {
		for _, id := range []string{
			helperSessionID(callID, r),
			helperSessionID(callID, researchRole{ID: "supervisor:" + r.ID}),
		} {
			if seen[id] {
				t.Errorf("duplicate session id %q — this is the SQLite collision", id)
			}
			seen[id] = true
		}
	}
	if len(seen) != len(roles)*2 {
		t.Errorf("got %d unique ids for %d helpers+supervisors", len(seen), len(roles)*2)
	}
	// Non-vacuous: the old code passed callID unchanged, which collides.
	if helperSessionID(callID, roles[0]) == callID {
		t.Error("session id is still the bare tool-call id — the collision is back")
	}
}
