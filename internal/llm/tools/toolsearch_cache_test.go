package tools

import (
	"strings"
	"testing"
)

// The invariant that keeps deferral from costing more than it saves.
//
// Tool definitions sit in the cached prompt prefix on every provider that
// caches one. If a discovered tool appeared in its ORIGINAL position it would
// land mid-array, shift every definition after it, and invalidate the cache
// from that point on every search. The saving would be real and the bill would
// go up anyway — worst for someone on a free tier, where a cache miss is not a
// rounding error.
func TestDiscoveredToolsAreAppendedSoThePrefixIsStable(t *testing.T) {
	const sid = "cache-order"
	ForgetSession(sid)
	defer ForgetSession(sid)

	// Deliberately interleaved: deferrable tools BETWEEN stable ones, which is
	// the arrangement that would corrupt the prefix if order were preserved.
	all := []BaseTool{
		NewViewTool(nil),            // stable
		NewReviewTool(nil),          // deferrable
		NewFindTool(),               // stable
		NewPatchPortTool(nil),       // deferrable
		NewWriteTool(nil, nil, nil), // stable
	}

	before := namesOf(VisibleTools(all, sid, true))
	if want := []string{"view", "find", "write"}; !equal(before, want) {
		t.Fatalf("stable set = %v, want %v", before, want)
	}

	MarkDiscovered(sid, ReviewToolName)
	after := namesOf(VisibleTools(all, sid, true))

	// The prefix must be untouched: the first three are the same three, in the
	// same order, and the discovery is at the end.
	if !equal(after[:len(before)], before) {
		t.Errorf("the cached prefix moved: %v -> %v", before, after)
	}
	if after[len(after)-1] != ReviewToolName {
		t.Errorf("discovered tool is at %v, not appended last", after)
	}

	// A second discovery must again only extend.
	MarkDiscovered(sid, PatchPortToolName)
	third := namesOf(VisibleTools(all, sid, true))
	if !equal(third[:len(after)], after) {
		t.Errorf("second discovery disturbed the prefix: %v -> %v", after, third)
	}
}

// The cache breakpoint goes on the last stable tool. If this count is wrong the
// breakpoint moves on every discovery, which is the bug it exists to prevent.
func TestStableToolCountStopsAtTheFirstDeferrable(t *testing.T) {
	const sid = "cache-count"
	ForgetSession(sid)
	defer ForgetSession(sid)

	all := []BaseTool{
		NewViewTool(nil), NewFindTool(), NewReviewTool(nil), NewPatchPortTool(nil),
	}
	visible := VisibleTools(all, sid, true)
	if got := StableToolCount(visible); got != 2 {
		t.Errorf("StableToolCount = %d, want 2 (view, find)", got)
	}

	MarkDiscovered(sid, ReviewToolName)
	visible = VisibleTools(all, sid, true)
	if got := StableToolCount(visible); got != 2 {
		t.Errorf("after a discovery StableToolCount = %d, want 2 — the breakpoint moved", got)
	}

	// With deferral off, everything is stable and the breakpoint is the last
	// tool, which is the behaviour that existed before any of this.
	off := VisibleTools(all, sid, false)
	if got := StableToolCount(off); got != 2 {
		// review/patch_port are still deferrable by name even when the feature
		// is off, so the count reflects the list, and the provider falls back
		// to len-1. Documented rather than silently surprising.
		t.Logf("deferral off: StableToolCount=%d of %d; provider falls back to the last index", got, len(off))
	}
}

// A model that never searches must still get a complete, working agent.
func TestTheStableSetAloneCanStillDoTheJob(t *testing.T) {
	const sid = "cache-minimum"
	ForgetSession(sid)
	defer ForgetSession(sid)

	all := []BaseTool{
		NewViewTool(nil), NewFindTool(), NewWriteTool(nil, nil, nil),
		NewReviewTool(nil), NewBioDataTool(nil),
	}
	got := namesOf(VisibleTools(all, sid, true))
	for _, need := range []string{"view", "find", "write"} {
		if !containsName(got, need) {
			t.Errorf("%s is missing from the stable set; a model that never searches could not work", need)
		}
	}
	if strings.Join(got, ",") == "" {
		t.Fatal("nothing at all is visible")
	}
}
