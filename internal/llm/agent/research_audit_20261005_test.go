package agent

// GORILLA (2026-10-05): tests for the /research and /osint audit fixes. Each
// test names the audit finding it holds shut (S = /research, O = /osint), and
// states the failure as it was observed, because a test that only says what it
// asserts gets deleted the first time the wording changes.

import (
	"context"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"sync/atomic"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/opencode-ai/opencode/internal/config"
	"github.com/opencode-ai/opencode/internal/llm/tools"
	"github.com/opencode-ai/opencode/internal/message"
	"github.com/opencode-ai/opencode/internal/session"
)

// tempCache points the cache directory (helper timings, remembered run sizes)
// at a throwaway folder, so a test neither reads nor writes the developer's
// real measurements.
func tempCache(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", dir)
	if !strings.HasPrefix(researchRunsPath(), dir) {
		t.Fatalf("cache redirection failed: %s", researchRunsPath())
	}
	return dir
}

// ─── O1: a dossier runs open-source lanes, not software-engineering ones ───

// Observed: a dossier on any question ran LOCAL ("search THIS machine and THIS
// project"), PRIOR ART (repositories, issue trackers) and REQUIREMENT ("its
// feature detection, its API contract"), because one role table served both
// doctrines.
func TestADossierNeverRunsTheEngineeringLanes(t *testing.T) {
	engineering := map[string]bool{}
	for _, r := range researchRoles {
		engineering[r.ID] = true
	}
	// completeness is a lane of both tables on purpose; it reads the others'
	// findings and touches no machine.
	delete(engineering, "completeness")

	localWords := regexp.MustCompile(`(?i)this machine|this project|installed (packages|tools)|on disk|repositor|issue tracker|feature detection`)
	for n := ResearchMinAgents; n <= ResearchMaxAgents; n++ {
		roles, _ := selectRolesFor(DoctrineDossier, "", n)
		if len(roles) != n {
			t.Fatalf("a dossier of %d helpers selected %d lanes — the dossier table is too short", n, len(roles))
		}
		for _, r := range roles {
			if engineering[r.ID] {
				t.Errorf("dossier of %d runs the engineering lane %q", n, r.ID)
			}
			if m := localWords.FindString(r.Lane + " " + r.Title); m != "" {
				t.Errorf("dossier lane %q speaks of %q — no dossier lane may work on the user's own machine or code", r.ID, m)
			}
			if r.Title == "" || r.Lane == "" || r.Prevents == "" {
				t.Errorf("dossier lane %q is missing its title, lane or reason", r.ID)
			}
		}
	}

	// The everyday run is unchanged: it still opens with LOCAL.
	if std, _ := selectRoles("", ResearchMinAgents); std[0].ID != "local" {
		t.Errorf("the standard run no longer opens with the local lane: %q", std[0].ID)
	}
}

// The mandatory lanes must be able to work blind, or a four-helper dossier
// would have a lane waiting for findings nobody produces first.
func TestTheMandatoryDossierLanesWorkBlind(t *testing.T) {
	roles, _ := selectRolesFor(DoctrineDossier, "", ResearchMinAgents)
	for _, r := range roles {
		if rolePeeksAtOthers(r.ID) {
			t.Errorf("mandatory dossier lane %q needs the other lanes' findings", r.ID)
		}
	}
	// And the provenance lane, whose instructions open "You are given the other
	// helpers' findings", must actually be given them.
	if !rolePeeksAtOthers("provenance") {
		t.Error("the provenance lane is told it has the others' findings and is scheduled before they exist")
	}
}

// Asking for an engineering lane by name on a dossier run must not bring it
// back through the `roles` parameter.
func TestADossierRefusesAnEngineeringLaneByName(t *testing.T) {
	roles, note := selectRolesFor(DoctrineDossier, "local, dissent", ResearchMinAgents)
	for _, r := range roles {
		if r.ID == "local" {
			t.Fatal("roles=\"local\" put the search-this-machine lane into a dossier")
		}
	}
	if roles[0].ID != "dissent" {
		t.Errorf("the valid explicit role should lead, got %q", roles[0].ID)
	}
	if !strings.Contains(note, `"local"`) || !strings.Contains(note, "official") {
		t.Errorf("the refusal must name the role and list the ones that exist; note=%q", note)
	}
}

// Helper session ids are "<call id>-<role id>" and recovery splits on the role.
// A hyphen inside a role id would make that split ambiguous.
func TestNoRoleIDContainsAHyphen(t *testing.T) {
	for _, id := range knownRoleIDs() {
		if strings.ContainsAny(id, "-: ") {
			t.Errorf("role id %q contains a character the helper session id is split on", id)
		}
	}
}

// Observed: the dossier helper was told "find: THIS machine ... Check here
// before the web" and held the find and view tools.
func TestADossierHelperIsNeitherToldNorAbleToSearchThisMachine(t *testing.T) {
	dos := buildPrompt(dossierRoles[0], "q", "", "", 0, 4, DoctrineDossier)
	if strings.Contains(dos, "find: THIS machine") {
		t.Error("a dossier helper is still told to search this machine first")
	}
	if !strings.Contains(dos, "OUT OF BOUNDS") {
		t.Error("a dossier helper is not told the machine is out of bounds")
	}
	std := buildPrompt(researchRoles[0], "q", "", "", 0, 4, "")
	if !strings.Contains(std, "find: THIS machine") {
		t.Error("the standard helper lost its local-search instruction")
	}

	kept := dossierHelperTools(ResearchAgentTools(nil, nil))
	if len(kept) != len(researchEgressTools()) {
		t.Fatalf("a dossier helper holds %d tools, want the %d web tools", len(kept), len(researchEgressTools()))
	}
	for _, tool := range kept {
		name := tool.Info().Name
		if name != tools.WebSearchToolName && name != tools.FetchToolName {
			t.Errorf("a dossier helper holds %q, which is not a web tool", name)
		}
	}
}

// ─── O2: the follow-up ceiling is counted, not asked for ───

// Observed: "ONE follow-up call is the ceiling" was a sentence in the report.
// A second, third and tenth dossier call passed the same checks as the first.
func TestTheDossierCeilingIsEnforced(t *testing.T) {
	sid := "ceiling-" + t.Name()

	n, first, ok := reserveDossierCall(sid, "turn-1", ModeParallel)
	if !ok || n != 1 || first != ModeParallel {
		t.Fatalf("first call: got (%d, %q, %v)", n, first, ok)
	}
	// The follow-up is told the FIRST call's mode, whatever it asked for.
	n, first, ok = reserveDossierCall(sid, "turn-1", ModeSupervised)
	if !ok || n != 2 || first != ModeParallel {
		t.Fatalf("follow-up: got (%d, %q, %v), want (2, parallel, true)", n, first, ok)
	}
	for i := 0; i < 3; i++ {
		if _, _, ok := reserveDossierCall(sid, "turn-1", ModeParallel); ok {
			t.Fatalf("dossier call %d for one question was allowed; the ceiling is %d", 3+i, DossierCallsPerQuestion)
		}
	}
	// The user speaking again is the user deciding to spend more.
	if n, _, ok := reserveDossierCall(sid, "turn-2", ModeSequential); !ok || n != 1 {
		t.Errorf("a new question did not get a fresh allowance: (%d, %v)", n, ok)
	}
	// And one conversation's allowance is not another's.
	if _, _, ok := reserveDossierCall(sid+"-other", "turn-1", ModeParallel); !ok {
		t.Error("a different conversation was refused on someone else's count")
	}
}

// The refusal has to come out of Run itself, as a tool response the model can
// read, before any helper session is created.
func TestRunRefusesAThirdDossierCall(t *testing.T) {
	if _, err := config.Load(t.TempDir(), false); err != nil {
		t.Fatalf("config.Load: %v", err)
	}
	if !config.LoadoutEnabled(config.DossierComponentID) {
		config.ToggleLoadout(config.DossierComponentID)
		defer config.ToggleLoadout(config.DossierComponentID)
	}
	if config.MaxSubAgents() != config.SubAgentsUnlimited {
		t.Skip("the helper leash is set in this environment; the refusal under test sits behind it")
	}

	sid := "run-ceiling-" + t.Name()
	// messages is nil, so the question key is "": spend the allowance on it.
	for i := 0; i < DossierCallsPerQuestion; i++ {
		if _, _, ok := reserveDossierCall(sid, "", ModeParallel); !ok {
			t.Fatal("could not set the scene")
		}
	}

	ctx := context.WithValue(context.Background(), tools.SessionIDContextKey, sid)
	ctx = context.WithValue(ctx, tools.MessageIDContextKey, "m1")
	// No session service: reaching for one would panic, which is the proof that
	// the refusal came before any helper was created.
	resp, err := (&researchTool{}).Run(ctx, tools.ToolCall{
		ID:    "call_x",
		Input: `{"question":"q","doctrine":"dossier","agents":10,"mode":"supervised"}`,
	})
	if err != nil {
		t.Fatalf("the ceiling must refuse through the response: %v", err)
	}
	if !resp.IsError || !strings.Contains(resp.Content, "ceiling") || !strings.Contains(resp.Content, "Nothing was started") {
		t.Errorf("third dossier call was not refused plainly: %q", resp.Content)
	}
}

// Every text about the follow-up must say what the code does: optional, once,
// and how much it can cost.
func TestTheGapRoundIsDescribedAsWhatItIs(t *testing.T) {
	for _, mode := range []string{ModeSequential, ModeParallel, ModeSupervised} {
		want, _, _, _ := RunShapeFor(DoctrineDossier, mode, ResearchMinAgents)
		if got := DossierFollowUpSessions(mode); got != want {
			t.Errorf("%s: follow-up ceiling %d, scheduler would create %d", mode, got, want)
		}
		first := gapRoundDuty(1, mode)
		for _, must := range []string{"OPTIONAL", "ONCE", mode, "refused"} {
			if !strings.Contains(first, must) {
				t.Errorf("%s: the first call's duty does not say %q:\n%s", mode, must, first)
			}
		}
		if !regexp.MustCompile(`at most \d+ more`).MatchString(strings.Join(strings.Fields(first), " ")) {
			t.Errorf("%s: the duty does not state what the follow-up can cost:\n%s", mode, first)
		}
	}
	if last := gapRoundDuty(DossierCallsPerQuestion, ModeParallel); !strings.Contains(last, "refuse") || strings.Contains(last, "you may call") {
		t.Errorf("the follow-up's own report must close the door, not reopen it:\n%s", last)
	}

	blurb := dossierSchemaBlurb()
	if strings.Contains(blurb, "adds a bounded gap round") {
		t.Error("the schema still says the doctrine ADDS a gap round; nothing adds one")
	}
	for _, id := range roleIDs(dossierRoles) {
		if !strings.Contains(blurb, id) {
			t.Errorf("the dossier schema does not name the role id %q the model must choose from", id)
		}
	}
}

// ─── S3: the forecast uses the measurement the screen prints ───

// Observed: "THIS RUN ... 45s of running" from an assumed 15 seconds a step,
// printed a few lines above "MEASURED: Ns per helper" from this machine.
func TestRunShapeUsesTheMeasuredHelperDuration(t *testing.T) {
	tempCache(t)
	config.ResetHelperTimingForTest()
	defer config.ResetHelperTimingForTest()

	_, _, batches, assumed := RunShape(ModeParallel, ResearchMinAgents)
	wantAssumed := float64(batches) * float64(config.ResearchStepsPerHelper) * config.ResearchSecondsPerStep
	if assumed != wantAssumed {
		t.Fatalf("unmeasured forecast %.1fs, want the labelled assumption %.1fs", assumed, wantAssumed)
	}

	for i := 0; i < 5; i++ {
		config.RecordHelperDuration(100 * time.Second)
	}
	secs, _, ok := config.MeasuredSecondsPerHelper()
	if !ok || secs != 100 {
		t.Fatalf("could not seed a measurement: (%v, %v)", secs, ok)
	}
	if _, _, b, got := RunShape(ModeParallel, ResearchMinAgents); got != float64(b)*100 {
		t.Errorf("with helpers measured at 100s, a %d-batch run is forecast at %.0fs — the forecast ignores the measurement", b, got)
	}
	// Supervised is a second pass over the blind lanes: more batches, same
	// measured batch length.
	_, _, pb, ps := RunShape(ModeParallel, ResearchMaxAgents)
	_, _, sb, ss := RunShape(ModeSupervised, ResearchMaxAgents)
	if sb <= pb || ss/float64(sb) != ps/float64(pb) {
		t.Errorf("supervised must take more batches of the same length: parallel %d/%.0fs, supervised %d/%.0fs", pb, ps, sb, ss)
	}
}

// ─── S6/O7: one answer to "how many at once" ───

func TestPeakInFlightIsTheSchedulersWidestWave(t *testing.T) {
	for _, doctrine := range []string{"", DoctrineDossier} {
		for n := ResearchMinAgents; n <= ResearchMaxAgents; n++ {
			if got := PeakInFlight(doctrine, ModeSequential, n); got != 1 {
				t.Errorf("sequential runs one at a time, got %d", got)
			}
			roles, _ := selectRolesFor(doctrine, "", n)
			blind := auditableLanes(roles)
			for _, mode := range []string{ModeParallel, ModeSupervised} {
				got := PeakInFlight(doctrine, mode, n)
				if got != blind || got > n || got > ResearchMaxInFlight {
					t.Errorf("%q %s %d helpers: peak %d, the widest wave is the %d blind lanes", doctrine, mode, n, got, blind)
				}
			}
		}
	}
}

// ─── S4, S13: the tool description states computed facts ───

func TestTheToolDescriptionDoesNotOverclaim(t *testing.T) {
	info := (&researchTool{}).infoBase()
	modeDesc, _ := info.Parameters["mode"].(map[string]any)["description"].(string)
	all := info.Description + "\n" + modeDesc

	for _, gone := range []string{
		"roughly doubles", "roughly DOUBLE", // false above four helpers
		"audits each lane", "auditing each lane's", // the peeking lanes are never audited
		"seven keyless", // the method text listed six; web_search serves neither number
		"hundreds",
	} {
		if strings.Contains(all, gone) {
			t.Errorf("the tool description still says %q", gone)
		}
	}
	minSup, _ := SupervisedSessions(ResearchMinAgents)
	maxSup, _ := SupervisedSessions(ResearchMaxAgents)
	for _, n := range []int{minSup, maxSup} {
		if !strings.Contains(all, " "+itoa(n)+" sessions") && !strings.Contains(all, "become "+itoa(n)) {
			t.Errorf("the description does not carry the scheduler's supervised session count %d", n)
		}
	}

	keyless := tools.KeylessSearchSources()
	if want := itoa(len(keyless)) + " sources that need no key"; !strings.Contains(info.Description, want) {
		t.Errorf("the description does not state the real keyless source count (%q)", want)
	}
	method := researchMethod(config.ResearchStepsPerHelper, "")
	for _, s := range keyless {
		if !strings.Contains(method, s) {
			t.Errorf("the helper method does not name the keyless source %q", s)
		}
	}
	// The roles parameter lists the standard table as it is, not as it was typed.
	rolesDesc, _ := info.Parameters["roles"].(map[string]any)["description"].(string)
	for _, id := range roleIDs(researchRoles) {
		if !strings.Contains(rolesDesc, id) {
			t.Errorf("the roles parameter does not offer %q", id)
		}
	}
}

// ─── S11, O4: the atlas is what helpers get, and it says so ───

func TestTheAtlasDoesNotPointAtFilesThatAreNotShipped(t *testing.T) {
	for _, gone := range []string{"985", "ships in docs", "docs/"} {
		if strings.Contains(sourceAtlas, gone) {
			t.Errorf("the atlas still tells helpers about %q, which is not in the binary", gone)
		}
	}
	total, needKey := SourceAtlasCount()
	// Counted a second way, so the regular expression cannot quietly match nothing.
	byHand := 0
	for _, line := range strings.Split(strings.ReplaceAll(sourceAtlas, "\r\n", "\n"), "\n") {
		if strings.HasPrefix(line, "  ") && !strings.HasPrefix(line, "   ") && strings.Contains(line, " [") {
			byHand++
		}
	}
	if total == 0 || total != byHand {
		t.Fatalf("SourceAtlasCount says %d sources, the file has %d entry lines", total, byHand)
	}
	if needKey < 0 || needKey >= total {
		t.Errorf("%d of %d atlas sources need a key — implausible", needKey, total)
	}
	if got := strings.Count(sourceAtlas, "API-key]"); got != needKey {
		t.Errorf("counted %d key-gated sources, the file marks %d", needKey, got)
	}
}

// ─── S12: the permission sentence is well-formed markdown ───

func TestTheFleetPromptQuotesEachToolName(t *testing.T) {
	p := researchFleetPrompt(4, researchEgressTools())
	want := "`" + tools.WebSearchToolName + "` and `" + tools.FetchToolName + "`"
	if !strings.Contains(p, want) {
		t.Errorf("tool names are not each in their own backticks; want %s in:\n%s", want, p)
	}
	if strings.Count(p, "`")%2 != 0 {
		t.Errorf("unbalanced backticks in the permission text:\n%s", p)
	}
	if strings.Contains(p, "**`") || strings.Contains(p, "`**") || strings.Contains(p, "**"+tools.WebSearchToolName) {
		t.Errorf("the old half-quoted form is back:\n%s", p)
	}
}

// ─── S8: findings are private, and one run cannot overwrite another ───

func TestFindingsFilesArePrivateAndNeverOverwritten(t *testing.T) {
	home := tempHome(t)
	// An install from before the fix: the folder exists and is world-readable.
	dir := filepath.Join(home, "Documents", "Gorilla-OSINT-Dossiers")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	roles := []researchRole{{ID: "local", Title: "LOCAL"}}

	// The same question, back to back: well inside one second, let alone the one
	// minute the old stamp resolved to.
	first := writeRawFindings("is my landlord allowed to do this?", roles, []string{"first run"}, nil, "")
	second := writeRawFindings("is my landlord allowed to do this?", roles, []string{"second run"}, nil, "")
	if first == "" || second == "" {
		t.Fatal("a findings file was not written")
	}
	if first == second {
		t.Fatalf("two runs of one question were written to the same file: %s", first)
	}
	for path, want := range map[string]string{first: "first run", second: "second run"} {
		body, err := os.ReadFile(path)
		if err != nil || !strings.Contains(string(body), want) {
			t.Errorf("%s does not hold its own run (%q): %v", filepath.Base(path), want, err)
		}
	}
	if !regexp.MustCompile(`^findings-\d\d(-\d\d){5}-`).MatchString(filepath.Base(first)) {
		t.Errorf("the stamp does not carry seconds: %s", filepath.Base(first))
	}

	if runtime.GOOS == "windows" {
		return // Windows does not carry Unix permission bits; nothing to assert.
	}
	if info, err := os.Stat(first); err != nil || info.Mode().Perm() != 0o600 {
		t.Errorf("findings file mode is %v, want 0600: a private question readable by every account", info.Mode().Perm())
	}
	if info, err := os.Stat(dir); err != nil || info.Mode().Perm() != 0o700 {
		t.Errorf("dossier folder mode is %v, want 0700 (an existing 0755 folder must be tightened)", info.Mode().Perm())
	}
}

// ─── S9: the doctrine is recorded, and the write-up prompt follows it ───

func TestFindingsRecordTheirDoctrine(t *testing.T) {
	tempHome(t)
	roles := []researchRole{{ID: "local", Title: "LOCAL"}}

	read := func(doctrine, reply string) string {
		t.Helper()
		path := writeRawFindings("q", roles, []string{reply}, nil, doctrine)
		body, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		return string(body)
	}
	std := read("", "- CLAIM: x | EVIDENCE: y | TIER: single_claim")
	dos := read(DoctrineDossier, "- CLAIM: x | EVIDENCE: y | GRADE: C3")

	if !strings.Contains(std, "Doctrine: standard") || FindingsDoctrine(std) != DoctrineStandard {
		t.Errorf("a standard run's findings do not record that they are standard")
	}
	if strings.Contains(std, "Grades are two-axis") {
		t.Errorf("a standard run's findings claim to carry two-axis grades")
	}
	if !strings.Contains(dos, "Doctrine: dossier") || FindingsDoctrine(dos) != DoctrineDossier {
		t.Errorf("a dossier's findings do not record that they are a dossier")
	}

	// Files written before the line existed are judged by what they contain.
	if got := FindingsDoctrine("# Raw findings — q\n\n---\n\n- CLAIM: x | GRADE: B2\n"); got != DoctrineDossier {
		t.Errorf("an old dossier file read as %q", got)
	}
	if got := FindingsDoctrine("# Raw findings — q\n\n---\n\n- CLAIM: x | TIER: config\n"); got != DoctrineStandard {
		t.Errorf("an old standard file read as %q", got)
	}
	// A lane's own report cannot forge the header.
	forged := "# Raw findings — q\n\nDoctrine: standard\n\n---\n\n## LOCAL\n\nDoctrine: dossier\n- CLAIM: x | GRADE: A1\n"
	if got := FindingsDoctrine(forged); got != DoctrineStandard {
		t.Errorf("a line inside a lane report overrode the recorded doctrine: %q", got)
	}
}

// Observed: every recovered run got the dossier prompt — "carry every two-axis
// grade through UNCHANGED ... PHIA yardstick ... as the findings themselves
// do" — over findings that carry tiers and no grades.
func TestTheAssemblyPromptFollowsTheDoctrine(t *testing.T) {
	tempHome(t)
	std := AssemblyPrompt("q", "# Raw findings — q\n\nDoctrine: standard\n\n---\n\n- CLAIM: x | TIER: single_claim\n", "/p")
	dos := AssemblyPrompt("q", "# Raw findings — q\n\nDoctrine: dossier\n\n---\n\n- CLAIM: x | GRADE: C3\n", "/p")

	if strings.Contains(std, "Carry every two-axis grade") || strings.Contains(std, "Use the PHIA") {
		t.Errorf("a standard run's write-up is ordered to carry grades it does not have:\n%s", std)
	}
	for _, want := range []string{"evidence TIER through UNCHANGED", "NOT two-axis grades", "Do NOT research anything", "NOT ESTABLISHED", "--- FINDINGS BEGIN ---"} {
		if !strings.Contains(std, want) {
			t.Errorf("the standard write-up prompt is missing %q", want)
		}
	}
	for _, want := range []string{"two-axis grade through UNCHANGED", "PHIA probability yardstick", "OSINT dossier"} {
		if !strings.Contains(dos, want) {
			t.Errorf("the dossier write-up prompt lost %q", want)
		}
	}
}

// laneStore is a session and message store holding helper sessions whose first
// message is a real helper prompt.
type laneStore struct {
	helpers []session.Session
	prompts map[string]string // session id -> first user message
	reports map[string]string // session id -> final assistant message
}

// Two views of it, one per service. Each embeds its interface so that anything
// recovery is not supposed to call panics loudly (see fakeSessions).
type laneSessions struct {
	session.Service
	*laneStore
}

type laneMessages struct {
	message.Service
	*laneStore
}

func (s laneSessions) ListResearchHelpers(context.Context) ([]session.Session, error) {
	return s.helpers, nil
}

func (s laneMessages) List(_ context.Context, id string) ([]message.Message, error) {
	return []message.Message{
		{Role: message.User, Parts: []message.ContentPart{message.TextContent{Text: s.prompts[id]}}},
		{Role: message.Assistant, Parts: []message.ContentPart{message.TextContent{Text: s.reports[id]}}},
	}, nil
}

// buildLaneStore makes the sessions one research call would leave behind, with
// the ids and titles runHelper really gives them.
func buildLaneStore(callID, doctrine, question string, roles []researchRole, supervised ...string) (laneSessions, laneMessages) {
	s := &laneStore{prompts: map[string]string{}, reports: map[string]string{}}
	mark := "TIER"
	if doctrine == DoctrineDossier {
		mark = "GRADE"
	}
	for i, r := range roles {
		id := helperSessionID(callID, r)
		s.helpers = append(s.helpers, session.Session{ID: id, Title: "Research: " + r.ID, CreatedAt: 1})
		s.prompts[id] = buildPrompt(r, question, "", "", i, len(roles), doctrine)
		s.reports[id] = "## ANSWER\nfrom " + r.ID + "\n\n## FINDINGS\n- CLAIM: x | EVIDENCE: y | " + mark + ": z"
	}
	for _, laneID := range supervised {
		for _, r := range roles {
			if r.ID != laneID {
				continue
			}
			sup := researchRole{ID: "supervisor:" + r.ID, Title: "SUPERVISOR"}
			id := helperSessionID(callID, sup)
			s.helpers = append(s.helpers, session.Session{ID: id, Title: "Research: " + sup.ID, CreatedAt: 1})
			s.prompts[id] = supervisorPrompt(r, question, "report")
			s.reports[id] = "## VERDICT\nWEAK — audit of " + r.ID
		}
	}
	return laneSessions{laneStore: s}, laneMessages{laneStore: s}
}

// Observed: store-sourced recovery wrote every run back as a dossier.
func TestAStandardRunIsRecoveredAsAStandardRun(t *testing.T) {
	for _, c := range []struct {
		doctrine, want string
		roles          []researchRole
	}{
		{"", DoctrineStandard, researchRoles[:4]},
		{DoctrineDossier, DoctrineDossier, dossierRoles[:4]},
	} {
		t.Run(c.want, func(t *testing.T) {
			tempHome(t)
			ss, ms := buildLaneStore("call_abc123", c.doctrine, "what does the rule say?", c.roles, c.roles[0].ID)
			runs := ListRecoverableRuns(context.Background(), ss, ms)
			if len(runs) != 1 {
				t.Fatalf("expected one run, got %d", len(runs))
			}
			if runs[0].Doctrine != c.want {
				t.Errorf("run listed with doctrine %q", runs[0].Doctrine)
			}
			_, body, err := RecoverFindings(context.Background(), runs[0], ss, ms)
			if err != nil {
				t.Fatal(err)
			}
			if FindingsDoctrine(body) != c.want {
				t.Errorf("run was recovered as %q", FindingsDoctrine(body))
			}
			// The dossier lanes keep their own headings rather than a made-up one.
			if !strings.Contains(body, "## "+c.roles[1].Title) {
				t.Errorf("recovered findings lost the lane heading %q", c.roles[1].Title)
			}
			// The audit sits under the lane it judged, and is not a lane itself.
			if !strings.Contains(body, "audit of "+c.roles[0].ID) || strings.Contains(body, "## SUPERVISOR") {
				t.Errorf("the supervisor audit was lost or listed as a lane")
			}
			if !strings.Contains(body, "4 of 4 lanes produced findings") {
				t.Errorf("wrong lane count in the recovered file")
			}
		})
	}
}

// The marker recovery looks for must be the heading the addendum opens with.
func TestTheDossierMarkerIsInTheAddendum(t *testing.T) {
	if !strings.Contains(dossierMethodAddendum, dossierDisciplineMarker) {
		t.Fatal("recovery detects a dossier by a heading the dossier prompt no longer carries")
	}
	if strings.Contains(researchMethod(3, "")+researchOutputContract, dossierDisciplineMarker) {
		t.Fatal("the standard prompt carries the dossier marker; every run would be recovered as a dossier")
	}
}

// ─── S10: helper ids are split on the role, whatever the call id looks like ───

// Observed, by running the old pattern ^(call_[A-Za-z0-9]+)-(.+)$ on real ids:
// Gemini's "call_"+UUID was cut at its first hyphen; Anthropic's "toolu_..."
// and anything not starting "call_" did not match at all.
func TestHelperIDsAreSplitForEveryProvidersCallIDShape(t *testing.T) {
	callIDs := []string{
		"call_2d3bca9114a643ff9b2edd4d",             // OpenAI-compatible
		"call_9a4d1c2e-7b3f-4e21-a1c5-0d9e8f7a6b5c", // Gemini direct: "call_" + UUID
		"toolu_01A9bCdEfGhIjKlMnOpQrStU",            // Anthropic, Antigravity-Claude
		"toolu_vrtx_01XyZ-abc_DEF",                  // Vertex-routed Anthropic
		"chatcmpl-tool-5f6e7d8c9b0a",                // local OpenAI-compatible servers
		"0",                                         // a server that numbers its calls
	}
	for _, call := range callIDs {
		for _, roleID := range knownRoleIDs() {
			role := researchRole{ID: roleID}
			gotCall, gotRole, ok := splitHelperID(helperSessionID(call, role), "Research: "+roleID)
			if !ok || gotCall != call || gotRole != roleID {
				t.Errorf("%s + %s split as (%q, %q, %v)", call, roleID, gotCall, gotRole, ok)
			}
			sup := researchRole{ID: "supervisor:" + roleID}
			gotCall, gotRole, ok = splitHelperID(helperSessionID(call, sup), "Research: "+sup.ID)
			if !ok || gotCall != call || gotRole != sup.ID {
				t.Errorf("%s + %s split as (%q, %q, %v)", call, sup.ID, gotCall, gotRole, ok)
			}
			// Without the title too: the role list alone must be enough.
			if c, r, ok := splitHelperID(helperSessionID(call, role), ""); !ok || c != call || r != roleID {
				t.Errorf("%s + %s without a title split as (%q, %q, %v)", call, roleID, c, r, ok)
			}
		}
		// A role this program no longer defines is still recoverable, from the
		// title its session was given.
		if c, r, ok := splitHelperID(call+"-retired_lane", "Research: retired_lane"); !ok || c != call || r != "retired_lane" {
			t.Errorf("%s + a retired role split as (%q, %q, %v)", call, c, r, ok)
		}
	}
}

// And the consequence that mattered: a Gemini-shaped or Anthropic-shaped run is
// listed as ONE run with its lanes together.
func TestARunIsGroupedWhateverItsCallIDLooksLike(t *testing.T) {
	tempHome(t)
	for _, call := range []string{"call_9a4d1c2e-7b3f-4e21-a1c5-0d9e8f7a6b5c", "toolu_01A9bCdEfGhIjKlMnOpQrStU"} {
		ss, ms := buildLaneStore(call, "", "q for "+call, researchRoles[:5], "local", "prior_art")
		runs := ListRecoverableRuns(context.Background(), ss, ms)
		if len(runs) != 1 {
			t.Fatalf("%s: listed as %d runs, want 1", call, len(runs))
		}
		if runs[0].CallID != call {
			t.Errorf("call id recovered as %q, want %q", runs[0].CallID, call)
		}
		if runs[0].Lanes != 7 { // five lanes and two supervisors, as before
			t.Errorf("%s: %d helper sessions grouped, want 7", call, runs[0].Lanes)
		}
	}
}

// ─── S14: a label is cut between characters, not through one ───

func TestALongQuestionInAnyScriptMakesAValidLabel(t *testing.T) {
	for _, q := range []string{
		strings.Repeat("Pourquoi l'électricité coûte-t-elle si cher à Kinshasa ? ", 4),
		strings.Repeat("چرا قیمت نان در کابل بالا رفته است؟ ", 6),
		strings.Repeat("Kwa nini bei ya mafuta imepanda — na nani anafaidika? ", 4),
		strings.Repeat("é", 200),
	} {
		label := RecoverableRun{Question: q}.Label()
		if !utf8.ValidString(label) {
			t.Errorf("label is not valid UTF-8 (a character was cut in half): %q", label)
		}
		if n := utf8.RuneCountInString(label); n > 80 {
			t.Errorf("label is %d characters; it will wrap", n)
		}
		if !strings.HasSuffix(label, "...") {
			t.Errorf("a cut question does not say it was cut: %q", label)
		}
	}
}

// ─── S16: a cancelled wave still waits for what it started ───

// Observed in the code: on a cancelled context the launch loop returned without
// wg.Wait(), so helpers already running were still writing their results while
// Run read them for the receipt.
func TestACancelledWaveWaitsForTheHelpersItStarted(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	started := make(chan struct{})
	var finished, launched atomic.Int32

	go func() {
		<-started
		cancel() // the user pressed X while the first helper was mid-flight
	}()

	launchWave(ctx, []int{0, 1, 2, 3, 4, 5, 6, 7}, func(i int) {
		launched.Add(1)
		if i == 0 {
			close(started)
		}
		<-ctx.Done()                      // every helper notices the kill...
		time.Sleep(50 * time.Millisecond) // ...and takes a moment to unwind
		finished.Add(1)
	})

	if finished.Load() != launched.Load() {
		t.Errorf("the wave returned with %d of %d started helpers still running; their results and spend are read too early",
			launched.Load()-finished.Load(), launched.Load())
	}

	// A wave that begins already cancelled starts nothing.
	launched.Store(0)
	launchWave(ctx, []int{0, 1, 2}, func(int) { launched.Add(1) })
	if launched.Load() != 0 {
		t.Errorf("%d helpers were started on a cancelled run", launched.Load())
	}
}

// ─── O3, S15: the size of a run is remembered, not typed ───

func TestRunSizesAreMeasuredFromFinishedRuns(t *testing.T) {
	tempCache(t)

	if _, _, _, ok := MeasuredRunSize(); ok {
		t.Fatal("a machine that has never finished a run reports a measurement")
	}

	// A killed run, or one that lost lanes, says nothing about what a run costs.
	recordResearchRun(8, 5, 8, helperSpend{inTokens: 10, outTokens: 10}, time.Minute)
	recordResearchRun(8, 8, 8, helperSpend{}, time.Minute)
	if _, _, _, ok := MeasuredRunSize(); ok {
		t.Fatal("an incomplete or empty run was recorded as a measurement")
	}

	// The run this project quotes: eight helpers, 248,122 in and 32,622 out.
	recordResearchRun(8, 8, 8, helperSpend{inTokens: 248122, outTokens: 32622, toolCalls: 96}, 13*time.Minute)
	tokens, calls, runs, ok := MeasuredRunSize()
	if !ok || runs != 1 {
		t.Fatalf("a finished run was not remembered: runs=%d ok=%v", runs, ok)
	}
	if tokens != 35093 { // 280,744 / 8
		t.Errorf("tokens per session = %d, want 35093", tokens)
	}
	if calls != 12 {
		t.Errorf("tool calls per session = %v, want 12", calls)
	}

	// The median, so one retry storm does not set the figure everyone is shown.
	recordResearchRun(4, 4, 4, helperSpend{inTokens: 40000, outTokens: 0, toolCalls: 4}, time.Minute)
	recordResearchRun(4, 4, 4, helperSpend{inTokens: 4000000, outTokens: 0, toolCalls: 4}, time.Minute)
	if tokens, _, runs, _ := MeasuredRunSize(); runs != 3 || tokens != 35093 {
		t.Errorf("median of three runs = %d over %d runs, want 35093 over 3", tokens, runs)
	}

	// Bounded, newest kept.
	for i := 0; i < researchRunsKept+5; i++ {
		recordResearchRun(4, 4, 4, helperSpend{inTokens: 4000, toolCalls: 4}, time.Minute)
	}
	if tokens, _, runs, _ := MeasuredRunSize(); runs != researchRunsKept || tokens != 1000 {
		t.Errorf("history holds %d runs at %d tokens/session, want %d at 1000", runs, tokens, researchRunsKept)
	}

	// A corrupt file is "no measurement", never a crash and never a number.
	if err := os.WriteFile(researchRunsPath(), []byte("{not json"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, _, _, ok := MeasuredRunSize(); ok {
		t.Error("a corrupt history file produced a measurement")
	}
}
