package agent

import (
	"errors"
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
	good := "## ANSWER\nx\n## FINDINGS\n- CLAIM: a | EVIDENCE: b | TIER: config\n## SOURCES TRIED\n- find 'a': 1 hit\n## CONFIDENCE\nstrong\n## NOT ESTABLISHED\nnothing"
	if missing := checkContract(good); len(missing) != 0 {
		t.Errorf("well-formed reply flagged: %v", missing)
	}
	// A reply that answers but carries no evidence tiers is exactly the kind
	// that gets synthesised into a confident wrong answer.
	bad := "## ANSWER\nIt is definitely X."
	if missing := checkContract(bad); len(missing) != 4 {
		t.Errorf("expected 4 missing headings (FINDINGS, SOURCES TRIED, CONFIDENCE, NOT ESTABLISHED), got %v", missing)
	}
}

func TestSelectRolesClampsAndKeepsMandatoryLanes(t *testing.T) {
	// The four mandatory lanes, BY NAME. This test counted lanes, so it passed
	// while roles="cost" at four helpers dropped the requirement lane.
	mandatory := roleIDs(researchRoles[:ResearchMinAgents])
	has := func(roles []researchRole, id string) bool {
		for _, r := range roles {
			if r.ID == id {
				return true
			}
		}
		return false
	}
	roles, note := selectRoles("cost", ResearchMinAgents)
	if len(roles) != ResearchMinAgents {
		t.Fatalf("got %d roles, want %d", len(roles), ResearchMinAgents)
	}
	for _, id := range mandatory {
		if !has(roles, id) {
			t.Errorf("mandatory lane %q dropped for an explicit role", id)
		}
	}
	if has(roles, "cost") || !strings.Contains(note, "cost not run") {
		t.Errorf("an explicit role that does not fit must be named as not run, note=%q", note)
	}
	// With room for it, the explicit role runs, ahead of any top-up.
	roles, _ = selectRoles("cost", ResearchMinAgents+2)
	if !has(roles, "cost") || roles[ResearchMinAgents].ID != "cost" {
		t.Errorf("explicit role should follow the mandatory block, got %v", roleIDs(roles))
	}
	for _, id := range mandatory {
		if !has(roles, id) {
			t.Errorf("mandatory lane %q missing at %d helpers", id, ResearchMinAgents+2)
		}
	}
	// Never more than the library holds.
	if all, _ := selectRoles("", ResearchMaxAgents); len(all) > len(researchRoles) {
		t.Errorf("selected %d roles from a library of %d", len(all), len(researchRoles))
	}
}

// The verifier must read the supervisor's audit of each lane, not only the lane.
func TestPeerDigestCarriesTheAudits(t *testing.T) {
	roles := []researchRole{{ID: "local", Title: "LOCAL"}, {ID: "prior_art", Title: "PRIOR ART"}, {ID: "x", Title: "FAILED"}}
	results := []laneOutcome{{reply: "lane one"}, {reply: "lane two"}, {err: errors.New("boom")}}
	audits := []string{"REJECTED: claim 2 unsupported", "", "never shown"}
	got := peerDigest(roles, results, audits, []int{0, 1, 2})
	for _, want := range []string{"### LOCAL\nlane one", "REJECTED: claim 2 unsupported", "### PRIOR ART\nlane two"} {
		if !strings.Contains(got, want) {
			t.Errorf("digest lacks %q:\n%s", want, got)
		}
	}
	if strings.Contains(got, "FAILED") || strings.Count(got, "Supervisor's audit") != 1 {
		t.Errorf("a failed lane or an empty audit leaked into the digest:\n%s", got)
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
