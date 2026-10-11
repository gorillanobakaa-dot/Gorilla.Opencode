package agent

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/opencode-ai/opencode/internal/llm/tools"
	"github.com/opencode-ai/opencode/internal/message"
)

const goodLane = `## ANSWER
Yes.
BASIS: https://ex.org/a | TIER: primary_source

## FINDINGS
- CLAIM: the fox jumps | EVIDENCE: https://ex.org/a | QUOTE: "the quick brown fox jumps over the lazy dog" | TIER: primary_source
- **CLAIM:** it is on page b | **EVIDENCE:** https://Ex.org/b/?utm_source=x#top | **QUOTE:** "a sentence that is not on the page at all" | **TIER:** single_claim
- CLAIM: a search snippet | EVIDENCE: https://never.opened.org/x | QUOTE: none | TIER: rumour

## SOURCES TRIED
- web_fetch ex.org/a: found

## CONFIDENCE
strong

## NOT ESTABLISHED
nothing`

func TestParseLaneReadsTheContractAsLines(t *testing.T) {
	p := parseLane(goodLane)
	if len(p.Missing) != 0 || p.Basis == nil || len(p.Claims) != 3 || p.Confidence != "strong" {
		t.Fatalf("parse wrong: %+v", p)
	}
	c := p.Claims[1]
	if c.Claim != "it is on page b" || c.Tier != "single_claim" || c.URLs[0] != "https://ex.org/b" {
		t.Errorf("bold markers or URL spelling not handled: %+v", c)
	}
	if p.Claims[2].Quote != "" {
		t.Errorf("QUOTE: none is no quote, got %q", p.Claims[2].Quote)
	}
	// A heading named in a sentence is not a heading: the substring check passed this.
	mention := "## ANSWER\nx\nThe ## FINDINGS, ## SOURCES TRIED, ## CONFIDENCE and ## NOT ESTABLISHED follow."
	if got := checkContract(mention); len(got) != 4 {
		t.Errorf("headings in a sentence counted as headings: missing=%v", got)
	}
}

func TestOneSpellingPerPage(t *testing.T) {
	for in, want := range map[string]string{
		"HTTPS://Example.ORG:443/a/?utm_source=x&id=2#top": "https://example.org/a?id=2",
		"https://doi.org/10.1038/NATURE12373":              "doi:10.1038/nature12373",
		"http://dx.doi.org/10.1/x":                         "doi:10.1/x",
		"http://ex.org:8080/":                              "http://ex.org:8080/",
	} {
		if got := normalizeEvidenceURL(in); got != want {
			t.Errorf("normalize(%q) = %q, want %q", in, got, want)
		}
	}
	if got := extractEvidenceURLs("see https://a.org/x, and doi:10.5555/abc."); strings.Join(got, " ") != "https://a.org/x doi:10.5555/abc" {
		t.Errorf("extract: %v", got)
	}
}

func TestQuoteMatchingIgnoresWhatDoesNotMatter(t *testing.T) {
	page := "Șapte: the “quick”  brown\nfox — jumps over the evi-\ndence."
	for _, q := range []string{`the "QUICK" brown fox - jumps`, "jumps over the evidence"} {
		if !quoteIn(page, q) {
			t.Errorf("%q should be found", q)
		}
	}
	if quoteIn(page, "the quick brown cat") || quoteIn(page, "") {
		t.Error("a different or empty quote was found")
	}
}

// helperMessages fakes one helper session: web_fetch calls and their results.
type helperMessages struct {
	message.Service
	msgs []message.Message
}

func (h *helperMessages) List(context.Context, string) ([]message.Message, error) { return h.msgs, nil }

func fetchCall(id, u string) message.Message {
	in, _ := json.Marshal(tools.FetchParams{URL: u})
	return message.Message{Role: message.Assistant, Parts: []message.ContentPart{
		message.ToolCall{ID: id, Name: tools.FetchToolName, Input: string(in)}}}
}

func fetchResult(id, content string, isErr bool) message.Message {
	return message.Message{Role: message.Tool, Parts: []message.ContentPart{
		message.ToolResult{ToolCallID: id, Name: tools.FetchToolName, Content: content, IsError: isErr}}}
}

func TestTheEvidenceCheckReadsWhatTheHelperFetched(t *testing.T) {
	msgs := &helperMessages{msgs: []message.Message{
		fetchCall("1", "https://ex.org/a"),
		fetchResult("1", "Fetched: https://ex.org/a\n\nThe quick brown fox jumps over the lazy dog.", false),
		fetchCall("2", "http://ex.org/b"),
		fetchResult("2", "Fetched: https://ex.org/b/\n\nSomething else entirely.", false),
		fetchCall("3", "https://never.opened.org/x"),
		fetchResult("3", "connection refused", true), // a failed fetch is not an opened page
	}}
	fetched := laneFetches(context.Background(), msgs, "s")
	if _, ok := fetched["https://ex.org/b"]; !ok {
		t.Fatalf("the page reached after a redirect is not recorded: %v", fetched)
	}
	got := map[string]int{}
	for _, f := range auditLane(parseLane(goodLane), fetched, "") {
		got[f.Rule] = f.Claim
	}
	want := map[string]int{"quote-not-found": 2, "cited-not-opened": 3, "claim-tier": 3}
	if len(got) != len(want) {
		t.Fatalf("findings %v, want %v", got, want)
	}
	for r, n := range want {
		if got[r] != n {
			t.Errorf("rule %s on claim %d, want claim %d", r, got[r], n)
		}
	}
	// Without a session store nothing is known about fetches: no claim is
	// accused of anything it cannot be checked for.
	for _, f := range auditLane(parseLane(goodLane), nil, "") {
		if f.Rule == "cited-not-opened" || f.Rule == "quote-not-found" {
			t.Errorf("an unknown fetch log produced %s", f.Rule)
		}
	}
}

func TestDossierLanesAreHeldToGradesAndTheYardstick(t *testing.T) {
	reply := "## ANSWER\nx\nBASIS: none | GRADE: F6\n## FINDINGS\n" +
		"- CLAIM: a | EVIDENCE: e | QUOTE: none | GRADE: B2\n- CLAIM: b | EVIDENCE: e | GRADE: G7\n- CLAIM: c | EVIDENCE: e | TIER: config\n" +
		"## SOURCES TRIED\n- a\n## CONFIDENCE\nLIKELIHOOD: fairly probable\nCONFIDENCE: MODERATE, information base\n## NOT ESTABLISHED\n-"
	var rules []string
	for _, f := range auditLane(parseLane(reply), map[string]string{}, DoctrineDossier) {
		rules = append(rules, f.Rule)
	}
	if strings.Join(rules, ",") != "contract-likelihood,claim-grade,claim-scale" {
		t.Errorf("got %v", rules)
	}
}

func TestTheReportFlagsUnsupportedClaims(t *testing.T) {
	var b strings.Builder
	p := parseLane(goodLane)
	writeEvidenceSection(&b, p, auditLane(p, map[string]string{}, ""), true)
	if !strings.Contains(b.String(), "UNSUPPORTED") || !strings.Contains(b.String(), "cited-not-opened") {
		t.Errorf("section:\n%s", b.String())
	}
	b.Reset()
	clean := strings.Replace(goodLane, "- **CLAIM:** it is on page b", "- not a claim", 1)
	clean = strings.Replace(clean, "- CLAIM: a search snippet", "- not a claim either", 1)
	p = parseLane(clean)
	writeEvidenceSection(&b, p, auditLane(p, map[string]string{"https://ex.org/a": "the quick brown fox jumps over the lazy dog"}, ""), true)
	if !strings.Contains(b.String(), "all 1 claim(s) hold") {
		t.Errorf("a clean lane is not said to hold:\n%s", b.String())
	}
}

// The record is the format fieldkit evidence check reads. FIELDKIT_ROOT, when
// set, runs the real thing against it.
func TestTheEvidenceRecordIsWhatFieldkitReads(t *testing.T) {
	dir := t.TempDir()
	md := filepath.Join(dir, "findings-26-10-11-00-00-00-q.md")
	if err := os.WriteFile(md, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	roles := []researchRole{{ID: "local", Title: "LOCAL"}, {ID: "prior_art", Title: "PRIOR ART"}}
	parses := []laneParse{parseLane(goodLane), parseLane("")}
	fetched := []map[string]string{{"https://ex.org/a": "x"}, nil}
	path := writeEvidenceRecord(md, "q?", "", "call_1", roles, parses, fetched)
	if path != strings.TrimSuffix(md, ".md")+".jsonl" {
		t.Fatalf("record path %q", path)
	}
	raw, _ := os.ReadFile(path)
	var types []string
	for _, line := range strings.Split(strings.TrimSpace(string(raw)), "\n") {
		var rec map[string]any
		if err := json.Unmarshal([]byte(line), &rec); err != nil {
			t.Fatalf("not JSON: %s", line)
		}
		types = append(types, rec["type"].(string))
	}
	if strings.Join(types, ",") != "run,lane,claim,claim,claim,fetched,lane" {
		t.Errorf("records %v", types)
	}
	if info, _ := os.Stat(path); info.Mode().Perm() != findingsFileMode {
		t.Errorf("record mode %v, want %v", info.Mode().Perm(), findingsFileMode)
	}
	if writeEvidenceRecord(md, "q?", "", "call_1", roles, parses, fetched) != "" {
		t.Error("a second write replaced the record")
	}

	// The real reader, when Fieldkit is here: FIELDKIT_ROOT=/path/to/Gorilla.Fieldkit.
	root := os.Getenv("FIELDKIT_ROOT")
	if root == "" {
		t.Skip("FIELDKIT_ROOT not set: fieldkit evidence check not run against the record")
	}
	cmd := exec.Command("python3", "-m", "fieldkit", "evidence", "check", path, "--json")
	cmd.Dir = root
	out, _ := cmd.Output()
	var res struct {
		Findings []struct{ Rule, Where string } `json:"findings"`
	}
	if err := json.Unmarshal(out, &res); err != nil {
		t.Fatalf("fieldkit answered %q: %v", out, err)
	}
	var got []string
	for _, f := range res.Findings {
		got = append(got, f.Where+" "+f.Rule)
	}
	// Fieldkit reads the same claims and the same fetches, and agrees with auditLane.
	want := "LOCAL claim 2 cited-not-opened,LOCAL claim 3 claim-tier,LOCAL claim 3 cited-not-opened"
	if strings.Join(got, ",") != want {
		t.Errorf("fieldkit found %v, want %s", got, want)
	}
}

// A sub-heading a helper writes inside FINDINGS does not end the section: the
// claims under it are still claims, and still checked.
func TestASubHeadingInsideFindingsDoesNotEndIt(t *testing.T) {
	reply := "## ANSWER\nx\nBASIS: e | TIER: config\n## FINDINGS\n### On the build\n" +
		"- CLAIM: a | EVIDENCE: e | QUOTE: none | TIER: config\n## SOURCES TRIED\n- a\n## CONFIDENCE\nstrong\n## NOT ESTABLISHED\n-"
	if p := parseLane(reply); len(p.Claims) != 1 {
		t.Errorf("claims under a sub-heading were lost: %+v", p.Claims)
	}
}

// The plan reaches every helper, numbered, and a plan longer than a plan is
// refused rather than cut: which seven to keep is the orchestrator's call.
func TestThePlanReachesEveryHelperAndIsBounded(t *testing.T) {
	plan, msg := cleanPlan([]string{"  who decided it? ", "", "when\n did it start?"})
	if msg != "" || len(plan) != 2 || plan[1] != "when did it start?" {
		t.Fatalf("plan %q, msg %q", plan, msg)
	}
	p := buildPrompt(researchRoles[0], "q", "", "", 0, 4, "", plan...)
	if !strings.Contains(p, "THE PLAN") || !strings.Contains(p, "  1. who decided it?\n  2. when did it start?") {
		t.Errorf("the plan is not in the prompt:\n%s", p)
	}
	if strings.Contains(buildPrompt(researchRoles[0], "q", "", "", 0, 4, ""), "THE PLAN") {
		t.Error("a run without a plan was given an empty one")
	}
	long := make([]string, ResearchMaxPlan+1)
	for i := range long {
		long[i] = "q"
	}
	if _, msg := cleanPlan(long); !strings.Contains(msg, "at most 7") {
		t.Errorf("an eight-part plan was not refused: %q", msg)
	}
}
