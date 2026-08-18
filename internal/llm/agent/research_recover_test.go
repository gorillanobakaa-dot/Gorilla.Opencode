package agent

// GORILLA OVERRIDE (2026-08-18): tests for the recovery path.
//
// The thing being protected is a run that already cost real money and already
// failed once. A recovery that quietly loses a lane, upgrades a grade, or hands
// the model an instruction it reads as "go and research this" would be worse
// than no recovery at all — it would look like a dossier.

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/opencode-ai/opencode/internal/message"
	"github.com/opencode-ai/opencode/internal/session"
)

// The two stores, faked by embedding the interface and overriding only what
// recovery reads. Embedding rather than implementing keeps the fake from
// growing every time an unrelated method is added to either service — a nil
// embedded interface panics loudly if anything else is called, which is the
// behaviour a test wants.
type fakeSessions struct {
	session.Service
	helpers []session.Session
}

func (f *fakeSessions) ListResearchHelpers(context.Context) ([]session.Session, error) {
	return f.helpers, nil
}

type fakeMessages struct {
	message.Service
	question string
}

func (f *fakeMessages) List(context.Context, string) ([]message.Message, error) {
	return []message.Message{{
		Role: message.User,
		Parts: []message.ContentPart{message.TextContent{
			Text: "You are helper 1 of 1.\n\nTHE QUESTION UNDER INVESTIGATION:\n" + f.question + "\n\nrest",
		}},
	}}, nil
}

func TestRecoverFromASavedFindingsFileReadsItBackUnchanged(t *testing.T) {
	home := tempHome(t)
	dir := filepath.Join(home, "Documents", "Gorilla-OSINT-Dossiers")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	want := "# Raw findings — who is Kelexine?\n\n## LOCAL\n\n- CLAIM: x | GRADE: C3\n"
	path := filepath.Join(dir, "findings-26-08-17-22-08-who-is-kelexine.md")
	if err := os.WriteFile(path, []byte(want), 0o644); err != nil {
		t.Fatal(err)
	}

	runs := ListRecoverableRuns(context.Background(), nil, nil)
	if len(runs) != 1 {
		t.Fatalf("expected the saved findings file to be listed, got %d runs", len(runs))
	}
	if runs[0].Question != "who is Kelexine?" {
		t.Errorf("question not recovered from the file: %q", runs[0].Question)
	}
	if !runs[0].FromDisk() {
		t.Errorf("a file-sourced run should report FromDisk")
	}

	gotPath, body, err := RecoverFindings(context.Background(), runs[0], nil, nil)
	if err != nil {
		t.Fatalf("recovery failed: %v", err)
	}
	if gotPath != path {
		t.Errorf("recovery moved the record: %q -> %q", path, gotPath)
	}
	// The saved copy is the record. Regenerating it would be how a record gets
	// quietly altered.
	if body != want {
		t.Errorf("the findings file was not returned byte-for-byte")
	}
}

// ListRecoverableRuns must survive a store it cannot read. Someone reaching for
// this has already lost two hours once; an error is not an acceptable answer
// when the files on disk are right there.
func TestListingSurvivesAnUnreadableSessionStore(t *testing.T) {
	tempHome(t)
	if runs := ListRecoverableRuns(context.Background(), nil, nil); runs != nil && len(runs) != 0 {
		t.Errorf("expected an empty list with no findings and no store, got %d", len(runs))
	}
}

func TestAssemblyPromptForbidsCollectingAnythingNew(t *testing.T) {
	tempHome(t)
	findings := "## LOCAL\n\n- CLAIM: the file exists | EVIDENCE: /tmp/x | GRADE: A2\n"
	p := AssemblyPrompt("who is Kelexine?", findings, "/tmp/findings.md")

	// The single most expensive mistake this prompt could make is being read as
	// "research this", which would spend the run all over again.
	for _, want := range []string{
		"Do NOT research anything",
		"Do not call the research tool",
		"who is Kelexine?",
		"GRADE: A2",
		"UNCHANGED",
		"NOT ESTABLISHED",
		"Gorilla-OSINT-Dossiers",
	} {
		if !strings.Contains(p, want) {
			t.Errorf("the assembly prompt is missing %q", want)
		}
	}
	// The findings must travel inline, not as a path the model has to go and
	// read — a tool result is exactly the bulk that killed the original run.
	if !strings.Contains(p, "--- FINDINGS BEGIN ---") || !strings.Contains(p, findings) {
		t.Errorf("the findings are not carried inline")
	}
}

func TestRunLabelsStayReadableOnANarrowScreen(t *testing.T) {
	long := RecoverableRun{Question: strings.Repeat("why does this happen ", 20)}
	if got := len([]rune(long.Label())); got > 80 {
		t.Errorf("a long question produced a %d-character label; it will wrap and strand a row", got)
	}
	blank := RecoverableRun{}
	if !strings.Contains(blank.Label(), "not recorded") {
		t.Errorf("a run with no recorded question should say so, not render an empty row: %q", blank.Label())
	}
}

func TestRoleTitleFallsBackForARoleThatNoLongerExists(t *testing.T) {
	if got := roleTitle("local"); !strings.HasPrefix(got, "LOCAL") {
		t.Errorf("a known role lost its heading: %q", got)
	}
	// A recovered run predates whatever the roles are now. Dropping the lane
	// would be losing findings that were paid for.
	if got := roleTitle("some_retired_lane"); got != "SOME RETIRED LANE" {
		t.Errorf("an unknown role should still get a usable heading, got %q", got)
	}
}

func TestHelperIDPatternSeparatesSupervisorsFromTheirLanes(t *testing.T) {
	for _, c := range []struct{ id, call, role string }{
		{"call_2d3bca9114a643ff9b2edd4d-local", "call_2d3bca9114a643ff9b2edd4d", "local"},
		{"call_2d3bca9114a643ff9b2edd4d-supervisor:prior_art", "call_2d3bca9114a643ff9b2edd4d", "supervisor:prior_art"},
		{"call_462c68845e6a49eebb7722f8-primary_source", "call_462c68845e6a49eebb7722f8", "primary_source"},
	} {
		m := helperIDPattern.FindStringSubmatch(c.id)
		if m == nil {
			t.Fatalf("%s did not parse as a helper session id", c.id)
		}
		if m[1] != c.call || m[2] != c.role {
			t.Errorf("%s parsed as (%q, %q), want (%q, %q)", c.id, m[1], m[2], c.call, c.role)
		}
	}
	// An ordinary conversation must never be mistaken for a research lane.
	if helperIDPattern.MatchString("56d29c8d-9109-4284-a21b-68fb008f36a1") {
		t.Errorf("a normal session id parsed as a research helper")
	}
}

// Counting "## " headings read a six-lane run as thirty, because each lane's
// own report carries "## ANSWER" and "## FINDINGS". Measured on a real
// recovered file before this was fixed.
func TestCoverageComesFromTheSummaryLineNotFromCountingHeadings(t *testing.T) {
	body := "# Raw findings — q\n\n## LOCAL\n\n## ANSWER\nx\n\n## FINDINGS\n- y\n\n" +
		"## PRIOR ART\n\n## ANSWER\nz\n\n2 of 3 lanes produced findings.\n"
	covered, lanes := parseCoverage(body)
	if covered != 2 || lanes != 3 {
		t.Errorf("parsed %d of %d, want 2 of 3 — heading counting would have said 5", covered, lanes)
	}
	if c, l := parseCoverage("no summary line here"); c != 0 || l != 0 {
		t.Errorf("a file without the summary line should report nothing, got %d of %d", c, l)
	}
}

// The counts must reach the picker. A run listed as "30 of 30 lanes" on a
// six-lane run is a lie about how much of the ground was covered.
func TestFileSourcedRunReportsItsRealCoverage(t *testing.T) {
	home := tempHome(t)
	dir := filepath.Join(home, "Documents", "Gorilla-OSINT-Dossiers")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	body := "# Raw findings — q\n\n## LOCAL\n\n## ANSWER\nx\n\n## FINDINGS\n- y\n\n" +
		"## PRIOR ART\n\n## ANSWER\nz\n\n2 of 3 lanes produced findings.\n"
	if err := os.WriteFile(filepath.Join(dir, "findings-26-08-18-00-00-q.md"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	runs := ListRecoverableRuns(context.Background(), nil, nil)
	if len(runs) != 1 {
		t.Fatalf("expected one run, got %d", len(runs))
	}
	if runs[0].Lanes != 3 || runs[0].Covered != 2 {
		t.Errorf("listed %d of %d lanes, want 2 of 3", runs[0].Covered, runs[0].Lanes)
	}
	if !strings.Contains(runs[0].Detail(), "2 of 3 lanes reported") {
		t.Errorf("the picker line does not show the real coverage: %q", runs[0].Detail())
	}
}

// Driving the picker live for the first time showed eleven entries for six
// runs: every run that had already been recovered was listed twice, once as its
// findings file and once as the helper sessions it was built from. That reads
// as "you ran this twice", on a screen whose whole job is to be clear about
// what survived.
func TestARunAlreadySavedIsNotListedTwice(t *testing.T) {
	home := tempHome(t)
	dir := filepath.Join(home, "Documents", "Gorilla-OSINT-Dossiers")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	q := "Who is Kelexine?"
	body := "# Raw findings — " + q + "\n\n## LOCAL\n\nx\n\n1 of 1 lanes produced findings.\n"
	if err := os.WriteFile(filepath.Join(dir, "findings-26-08-18-00-00-who.md"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}

	sessions := &fakeSessions{helpers: []session.Session{
		{ID: "call_abc123-local", Title: "Research: local", CompletionTokens: 100},
	}}
	messages := &fakeMessages{question: "  who is   KELEXINE?  "}

	runs := ListRecoverableRuns(context.Background(), sessions, messages)
	if len(runs) != 1 {
		for _, r := range runs {
			t.Logf("listed: %q (from disk: %v)", r.Question, r.FromDisk())
		}
		t.Fatalf("the same run was listed %d times", len(runs))
	}
	if !runs[0].FromDisk() {
		t.Errorf("the saved file should win: it survives a database reset, the sessions do not")
	}
}
