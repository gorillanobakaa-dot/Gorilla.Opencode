package dialog

// GORILLA OVERRIDE (2026-08-23): ROADMAP item 3. The parallel mode's wording is
// DERIVED from the concurrency constants, and this test proves no literal has
// crept back in.
//
// It said "up to 4 at a time" and "in batches", typed out. Both were already
// false by the time anyone looked: the cap had moved to ResearchMaxAgents + 1 =
// 11, so the real figure was nearly three times the one on screen, and because
// the selector stops at 10 while 11 may fly, nothing queues and there are no
// batches at all.
//
// It is the SECOND wrong number in those same two lines. The comment sitting
// above them recorded the first correction ("the selector goes to 10, ten lanes
// are three batches"), which was true when in-flight was 4 and stopped being
// true when the cap moved. Correcting prose by hand is what produced both.

import (
	"fmt"
	"strconv"
	"strings"
	"testing"

	"github.com/opencode-ai/opencode/internal/llm/agent"
)

func parallelOption(t *testing.T) researchOption {
	t.Helper()
	for _, o := range researchOptions {
		if o.mode == "parallel" {
			return o
		}
	}
	t.Fatal("no parallel mode in researchOptions")
	return researchOption{}
}

// The number on screen must be one the user can actually reach.
func TestParallelWordingQuotesTheReachableConcurrency(t *testing.T) {
	o := parallelOption(t)
	want := strconv.Itoa(effectiveParallel())

	if !strings.Contains(o.short, want) {
		t.Errorf("short line %q does not mention %s, the number of helpers that can "+
			"actually run together", o.short, want)
	}
	if !strings.Contains(o.what, want) {
		t.Errorf("what line %q does not mention %s", o.what, want)
	}
}

// Neither line may promise the headroom slot. ResearchMaxInFlight is
// ResearchMaxAgents + 1 so a full run never queues; that spare slot is not an
// extra helper anybody can select, and quoting it is the same class of error as
// the old hardcoded 4, only in the other direction.
func TestParallelWordingDoesNotPromiseTheHeadroomSlot(t *testing.T) {
	if agent.ResearchMaxInFlight <= agent.ResearchMaxAgents {
		t.Skip("no headroom slot in the current constants")
	}
	o := parallelOption(t)
	headroom := strconv.Itoa(agent.ResearchMaxInFlight)
	for name, line := range map[string]string{"short": o.short, "what": o.what} {
		if strings.Contains(line, headroom) {
			t.Errorf("%s line quotes %s, the raw in-flight cap: %q\n"+
				"  That +1 is headroom so a full run never queues, not a helper the "+
				"selector offers. Use effectiveParallel().", name, headroom, line)
		}
	}
}

// "in batches" is a claim about waiting, and it is only true when the cap is
// smaller than the number of lanes. At the current constants nothing queues, so
// saying it would invent a wait the user will not experience.
func TestBatchingIsOnlyClaimedWhenBatchingHappens(t *testing.T) {
	o := parallelOption(t)
	claimsBatches := strings.Contains(strings.ToLower(o.what), "batch")
	willBatch := agent.ResearchMaxInFlight < agent.ResearchMaxAgents

	if claimsBatches != willBatch {
		t.Errorf("what line says batching=%v but the constants give batching=%v "+
			"(cap %d, max helpers %d): %q",
			claimsBatches, willBatch, agent.ResearchMaxInFlight, agent.ResearchMaxAgents, o.what)
	}
}

// The wording must MOVE when the constants move. A line that happens to contain
// the right digits today but is still a literal would pass the tests above.
func TestTheWordingIsDerivedNotTyped(t *testing.T) {
	// Both builders must react to their inputs. Compare against a hand-rolled
	// expectation built from the same constants: if either were a literal, the
	// strings would not match once the constants differ from the old 4.
	wantShort := fmt.Sprintf("up to %d at a time, same price, much faster", effectiveParallel())
	if got := parallelShort(); got != wantShort {
		t.Errorf("parallelShort() = %q, want %q", got, wantShort)
	}
	if strings.Contains(parallelWhat(), " 4 ") {
		t.Errorf("the old hardcoded 4 is back in the what line: %q", parallelWhat())
	}
	if effectiveParallel() != min(agent.ResearchMaxAgents, agent.ResearchMaxInFlight) {
		t.Error("effectiveParallel() no longer tracks the constants")
	}
}
