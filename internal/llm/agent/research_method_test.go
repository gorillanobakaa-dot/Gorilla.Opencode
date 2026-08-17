package agent

import (
	"strings"
	"testing"

	"github.com/opencode-ai/opencode/internal/config"
)

// GORILLA: these tests pin the OSINT-cycle port of 2026-08-17. The measured
// failures of 2026-08-07 (one keyword tried, synonym never tried, invented
// DOIs padding a table) were method failures; the method now rides in every
// helper prompt and the contract makes fabrication auditable.

func TestHelperPromptCarriesTheCollectionMethod(t *testing.T) {
	p := buildPrompt(researchRoles[1], "why does X fail", "ctx", "", 1, 4)

	for _, must := range []string{
		"DIRECTION",      // requirements before collection
		"VETTING",        // source evaluation before recording
		"STOP CONDITION", // budget + diminishing returns
		"synonyms",       // the deception/lie lesson
		"find:",          // the local tool, named
		"web_search:",    // the collection sources, named
		"SOURCES TRIED",  // the collection log
	} {
		if !strings.Contains(p, must) {
			t.Errorf("helper prompt is missing %q — the collection method is incomplete", must)
		}
	}
	// The budget the user is billed against is the budget the helper is told.
	if !strings.Contains(p, "about 3 tool calls") {
		t.Errorf("helper prompt does not state the %d-step budget the cost forecast assumes", config.ResearchStepsPerHelper)
	}
}

func TestContractRequiresSourcesTried(t *testing.T) {
	// A reply that lists only its hits cannot be audited — the SOURCES TRIED
	// log is required, and its absence is reported as malformed.
	missingLog := "## ANSWER\nx\n## FINDINGS\n- CLAIM: a | EVIDENCE: b | TIER: config\n## CONFIDENCE\nstrong\n## NOT ESTABLISHED\nnothing"
	missing := checkContract(missingLog)
	found := false
	for _, m := range missing {
		if m == "## SOURCES TRIED" {
			found = true
		}
	}
	if !found {
		t.Fatalf("a reply without SOURCES TRIED passed the contract; got missing=%v", missing)
	}
}

func TestContractForbidsUnopenedCitations(t *testing.T) {
	// The invented-DOI incident: the contract must say, in plain words, that a
	// constructed citation is an invention.
	if !strings.Contains(researchOutputContract, "Never cite a source you did not open") {
		t.Error("the contract no longer forbids citing unopened sources — the invented-DOI lesson is unenforced")
	}
}

func TestResearchDescriptionScalesEffortToTheQuestion(t *testing.T) {
	d := NewResearchTool(nil, nil, nil, nil).Info().Description
	for _, must := range []string{"SCALE THE RUN", "4 (the mandatory lanes)", "Over-spawning"} {
		if !strings.Contains(d, must) {
			t.Errorf("description is missing the effort-scaling guidance: %q", must)
		}
	}
}
