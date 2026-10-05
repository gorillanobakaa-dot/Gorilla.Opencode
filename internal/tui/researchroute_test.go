package tui

// GORILLA (2026-10-05): tests for what /research and /osint do with the text
// typed after them. Audit findings S1 (neither command checked that the
// research tool was switched on) and S2 (`help` was investigated as a
// question), plus the help texts and prompts those commands show and send.

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/opencode-ai/opencode/internal/commands"
	"github.com/opencode-ai/opencode/internal/config"
	"github.com/opencode-ai/opencode/internal/llm/agent"
)

// Every spelling of "help" someone will type. Before the fix each of these
// opened the cost dialog with the word as the question; one press of enter
// later, four to ten paid model sessions were researching it.
var helpWords = []string{"help", "--help", "-h", "-help", "?", "/?", "  HELP  ", "Help"}

func TestResearchHelpIsNeverAQuestion(t *testing.T) {
	for _, w := range helpWords {
		for _, on := range []bool{true, false} {
			if got := routeResearch(w, on); got != routeHelp {
				t.Errorf("/research %q (tool on=%v) routed to %d, want the explanation", w, on, got)
			}
			for _, armed := range []bool{true, false} {
				if got := routeOsint(w, on, armed); got != routeHelp {
					t.Errorf("/osint %q (tool on=%v, armed=%v) routed to %d, want the explanation", w, on, armed, got)
				}
			}
		}
	}
	// A real question that merely contains the word still runs.
	for _, q := range []string{"help me understand why the build fails", "what does --help print for scoop?", "is -h a valid flag"} {
		if got := routeResearch(q, true); got != routeRun {
			t.Errorf("/research %q was not treated as a question (route %d)", q, got)
		}
		if got := routeOsint(q, true, true); got != routeRun {
			t.Errorf("/osint %q was not treated as a question (route %d)", q, got)
		}
	}
}

// S1. With tool.research off the model has no research tool, and both commands
// went on to tell it to use one. The low-bandwidth trim switches it off and
// never puts it back (toolgate.go).
func TestResearchAndOsintRefuseWhenTheResearchToolIsOff(t *testing.T) {
	if got := routeResearch("does X work?", false); got != routeToolOff {
		t.Errorf("/research with the tool off routed to %d, want the tool-off refusal", got)
	}
	if got := routeResearch("does X work?", true); got != routeRun {
		t.Errorf("/research with the tool on routed to %d, want the cost dialog", got)
	}

	// /osint needs BOTH rows. It used to check only its own.
	if got := routeOsint("who owns this company?", false, true); got != routeToolOff {
		t.Errorf("/osint armed, research tool off: routed to %d, want the tool-off refusal", got)
	}
	if got := routeOsint("who owns this company?", true, false); got != routeDossierOff {
		t.Errorf("/osint not armed: routed to %d, want the arm-it message", got)
	}
	if got := routeOsint("who owns this company?", false, false); got != routeDossierOff {
		t.Errorf("/osint with both off: routed to %d, want the dossier's own switch named first", got)
	}
	if got := routeOsint("who owns this company?", true, true); got != routeRun {
		t.Errorf("/osint armed with the tool on routed to %d, want the gate", got)
	}

	// Nothing typed is not a refusal: it is the hint, or the page.
	if got := routeResearch("   ", false); got != routeEmpty {
		t.Errorf("bare /research routed to %d", got)
	}
	if got := routeOsint("", false, false); got != routeEmpty {
		t.Errorf("bare /osint routed to %d, want the capability page", got)
	}
}

// Recovery reads findings already on disk and sends out no helpers. Refusing it
// because a tool is off would strand the user it exists for: the one whose
// connection dropped, on the low-bandwidth trim that switched the tools off.
func TestRecoveryNeedsNeitherSwitch(t *testing.T) {
	for _, flag := range []string{"--recover", "-recover", "recover", "--resume"} {
		for _, on := range []bool{true, false} {
			for _, armed := range []bool{true, false} {
				if got := routeOsint(flag, on, armed); got != routeRecover {
					t.Errorf("/osint %s (tool on=%v, armed=%v) routed to %d, want recovery", flag, on, armed, got)
				}
			}
		}
	}
}

// The guard the route leads to must name the row and the way back, for both
// commands, from the live loadout rather than from a typed sentence.
func TestTheToolOffRefusalNamesTheRow(t *testing.T) {
	if _, err := config.Load(t.TempDir(), false); err != nil {
		t.Fatalf("config.Load: %v", err)
	}
	if !config.LoadoutEnabled(researchToolID) {
		t.Fatalf("test premise broken: %s ships switched off", researchToolID)
	}
	if msg := disabledToolWarning(researchToolID, "/research"); msg != "" {
		t.Errorf("the tool is on and /research was refused: %s", msg)
	}
	found := false
	for _, c := range config.LoadoutComponents {
		found = found || c.ID == researchToolID
	}
	if !found {
		t.Fatalf("%s is not a loadout row; the guard would check a switch that does not exist", researchToolID)
	}

	config.ToggleLoadout(researchToolID)
	defer config.ToggleLoadout(researchToolID)
	for _, cmd := range []string{"/research", "/osint"} {
		msg := disabledToolWarning(researchToolID, cmd)
		for _, want := range []string{cmd + " cannot run", toolRowName(researchToolID), "/context", "press space"} {
			if !strings.Contains(msg, want) {
				t.Errorf("%s with the research tool off: the refusal does not say %q: %q", cmd, want, msg)
			}
		}
	}
}

// The dispatch in tui.go must go through the route functions before it opens
// either dialog. Read from the source, as TestEveryDispatchedCommandIsDocumented
// does, because the switch cannot be driven without a terminal.
func TestTheDispatchAsksTheRouteBeforeOpeningADialog(t *testing.T) {
	src, err := os.ReadFile("tui.go")
	if err != nil {
		t.Fatal(err)
	}
	code := strings.ReplaceAll(string(src), "\r\n", "\n")
	for _, c := range []struct{ open, route, guard string }{
		{"dialog.NewResearchDialogCmp(", "routeResearch(msg.Args, config.LoadoutEnabled(researchToolID))", `a.requireTool(researchToolID, "/research")`},
		{"dialog.NewOsintDialogCmp(", "routeOsint(msg.Args, config.LoadoutEnabled(researchToolID), config.LoadoutEnabled(config.DossierComponentID))", `a.requireTool(researchToolID, "/osint")`},
	} {
		open := strings.Index(code, c.open)
		route := strings.Index(code, c.route)
		guard := strings.Index(code, c.guard)
		if open < 0 || route < 0 || guard < 0 {
			t.Fatalf("could not find %q, its route or its guard in tui.go (%d, %d, %d)", c.open, open, route, guard)
		}
		if !(route < guard && guard < open) {
			t.Errorf("%s is opened before its route is asked and its tool checked", c.open)
		}
		if strings.Count(code, c.open) != 1 {
			t.Errorf("%s is opened from %d places; each needs the route in front of it", c.open, strings.Count(code, c.open))
		}
	}
}

// ─── the prompts ───

func TestOsintPromptStatesTheFollowUpAsOptionalAndBounded(t *testing.T) {
	p := osintPrompt("supervised", 7, "who owns this company?")
	for _, want := range []string{
		`doctrine="dossier"`, `mode="supervised"`, "agents=7", "QUESTION: who owns this company?",
		"OPTIONAL", "at most once", "the tool refuses a second",
		"Do not pass `roles`",
		"Never write the dossier into the working folder",
	} {
		if !strings.Contains(p, want) {
			t.Errorf("the /osint prompt does not say %q:\n%s", want, p)
		}
	}
	// Before: "run the gap check the tool's report demands" — a duty nothing demanded.
	if strings.Contains(p, "demands") || strings.Contains(p, "run the gap") {
		t.Errorf("the /osint prompt still orders a gap round:\n%s", p)
	}
	if !strings.Contains(p, filepath.Base(config.DossierDir())) {
		t.Errorf("the /osint prompt does not name the dossier folder %q", filepath.Base(config.DossierDir()))
	}

	r := researchPrompt("parallel", 4, "does X work?")
	for _, want := range []string{`mode="parallel"`, "agents=4", "QUESTION: does X work?", "evidence tiers"} {
		if !strings.Contains(r, want) {
			t.Errorf("the /research prompt does not say %q", want)
		}
	}
	if strings.Contains(r, "doctrine") {
		t.Error("the /research prompt mentions a doctrine; the everyday run has none")
	}
}

// ─── the help texts ───

func detailOf(t *testing.T, name string) string {
	t.Helper()
	for _, c := range commands.All {
		if c.Name == name {
			return c.Detail
		}
	}
	t.Fatalf("no /%s in the command registry", name)
	return ""
}

// S8 and S4: /research's help must say that findings are written and where,
// and must not promise a verifier at the default four helpers.
func TestResearchHelpStatesWhatTheCodeDoes(t *testing.T) {
	d := detailOf(t, "research")

	// The bounds are the engine's.
	if want := fmt.Sprintf("%d to %d helpers", agent.ResearchMinAgents, agent.ResearchMaxAgents); !strings.Contains(d, want) {
		t.Errorf("/research help does not state the engine's bounds (%q)", want)
	}
	// The findings write, and the folder it really goes to.
	folder := filepath.Base(config.DossierDir())
	for _, want := range []string{"saves each helper's report", folder, "readable by your account only", "/osint --recover"} {
		if !strings.Contains(d, want) {
			t.Errorf("/research help does not say %q — every run writes there and the help was silent about it", want)
		}
	}
	// The verifier only runs from the first helper count whose roles include it.
	first := 0
	for n := agent.ResearchMinAgents; n <= agent.ResearchMaxAgents && first == 0; n++ {
		if _, audited := agent.SupervisedSessions(n); audited < n {
			first = n // the first count at which a lane reads the others' work
		}
	}
	if want := fmt.Sprintf("from %d helpers up a verifier", first); !strings.Contains(d, want) {
		t.Errorf("/research help must say %q; it used to promise a verifier on every run", want)
	}
	for _, want := range []string{"/research help", "switched off"} {
		if !strings.Contains(d, want) {
			t.Errorf("/research help does not mention %q", want)
		}
	}
	if strings.Contains(d, "rounds") {
		t.Error("/research help still describes /osint as running rounds")
	}
}

// O1, O2, O4, O6: /osint's help described a planning stage, hundreds of
// sources and a gap hunt that do not exist, and never said the question leaves.
func TestOsintHelpStatesWhatTheCodeDoes(t *testing.T) {
	d := detailOf(t, "osint")
	for _, gone := range []string{"sub-questions", "plans your", "hundreds", "hunts its own gaps", "985"} {
		if strings.Contains(d, gone) {
			t.Errorf("/osint help still claims %q", gone)
		}
	}
	for _, want := range []string{
		"fixed lanes",
		"No lane searches your own machine",
		"ONE follow-up",
		"refuses a second",
		"are told to grade",
		"needs the research helpers switched on",
		"go to your model provider and to the search services",
		"/osint help",
		fmt.Sprintf("%d-%d helpers is %d-%d full model sessions", agent.ResearchMinAgents, agent.ResearchMaxAgents, agent.ResearchMinAgents, agent.ResearchMaxAgents),
	} {
		if !strings.Contains(d, want) {
			t.Errorf("/osint help does not say %q", want)
		}
	}
	// No number of sources is typed here at all: the page computes it.
	if m := regexp.MustCompile(`\b\d+ (free |primary |real )?sources\b`).FindString(d); m != "" {
		t.Errorf("/osint help types a source count (%q); it belongs on the page, computed", m)
	}
	// The four mandatory lanes the help names are the ones the engine runs first.
	for _, title := range agent.DossierLaneTitles(agent.ResearchMinAgents) {
		word := strings.ToLower(strings.Fields(title)[0]) // OFFICIAL, SCHOLARLY, REPORTING, COUNTER-EVIDENCE
		if word == "counter-evidence" {
			word = "case against"
		}
		if !strings.Contains(strings.ToLower(d), word) {
			t.Errorf("/osint help does not mention the mandatory lane %q", title)
		}
	}
}
