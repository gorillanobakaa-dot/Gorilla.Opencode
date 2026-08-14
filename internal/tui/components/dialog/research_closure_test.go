package dialog

import (
	"fmt"
	"math"
	"regexp"
	"strings"
	"testing"

	"github.com/opencode-ai/opencode/internal/config"
	"github.com/opencode-ai/opencode/internal/llm/agent"
	"github.com/opencode-ai/opencode/internal/llm/models"
)

// THE STANDING REQUIREMENT FOR THIS SCREEN, after four failed attempts to fix
// it by hand and one independent audit that found 20 confirmed defects:
//
//	EVERY NUMBER PRINTED HERE MUST BE CHECKABLE WITH A CALCULATOR BY SOMEONE
//	WHO CANNOT READ THE SOURCE.
//
// That is not a style preference. This dialog exists to tell a user on a metered
// connection and a free tier what a run will cost BEFORE they commit to it. A
// figure they cannot verify is worth nothing, and a figure that contradicts the
// one beside it is worse than nothing — it destroys trust in the whole screen.
//
// The faults this file locks out, all of them real and all of them shipped:
//   - "$0.01 PER MINUTE. PER HOUR: $0"        (%.0f on the hour)
//   - per-minute identical for parallel and supervised beside the word "DOUBLE"
//     with no duration anywhere to explain it
//   - "8 helpers" printed three lines under a selector reading "Helpers: 4"
//   - "20 helpers" on a screen stating the maximum is 10
//   - a session count of agents*2 for runs that create 9, 11, 13, 15, 17 or 18

var modes = []string{"sequential", "parallel", "supervised"}

// pricedConfig pins the research helper agent to a model with a REAL per-token
// rate, so these tests exercise the priced branch of costLines().
//
// Without this the package's isolated config has no usable priced model,
// ResearchCost returns priced=false, and the dialog renders "CANNOT PRICE" —
// at which point every assertion below finds no line and passes having checked
// nothing. The first version of this file did exactly that, and only the
// "no rate line at all" failure in a sibling test exposed it.
func pricedConfig(t *testing.T) {
	t.Helper()
	if _, err := config.Load(t.TempDir(), false); err != nil {
		t.Fatalf("config.Load: %v", err)
	}
	const id = models.ModelID("local.test/priced-helper")
	if _, ok := models.SupportedModels[id]; !ok {
		models.SupportedModels[id] = models.Model{
			ID: id, Name: "Priced Test Helper", Provider: models.ProviderLocal,
			CostPer1MIn: 0.10, CostPer1MOut: 0.40,
			ContextWindow: 128000, DefaultMaxTokens: 4096,
		}
		models.RegisterLocalRouteForTestNamed(id, "http://127.0.0.1:1/v1", "k", "test-endpoint")
		t.Cleanup(func() {
			delete(models.SupportedModels, id)
			models.ClearLocalRouteForTest(id)
		})
	}
	for _, a := range []config.AgentName{config.AgentTask, config.AgentResearch} {
		if err := config.UpdateAgentModel(a, id); err != nil {
			t.Fatalf("pin %s to a priced model: %v", a, err)
		}
	}
	if _, _, per1MIn, _, priced := config.ResearchCost(4); !priced || per1MIn <= 0 {
		t.Fatalf("setup did not produce a priced model (priced=%v per1MIn=%v); "+
			"every test in this file would pass vacuously", priced, per1MIn)
	}
}

func dialogAt(mode string, agents int) ResearchDialogCmp {
	m := NewResearchDialogCmp("does the arithmetic close?")
	for i, o := range researchOptions {
		if o.mode == mode {
			m.selected = i
		}
	}
	m.agents = agents
	m.width, m.height = 160, 60
	return m
}

var moneyRe = regexp.MustCompile(`\$[0-9]+\.[0-9]+`)

// findLine returns the first cost line containing all the given substrings.
func findLine(t *testing.T, m ResearchDialogCmp, want ...string) string {
	t.Helper()
	for _, l := range m.costLines() {
	next:
		for range []int{0} {
			for _, w := range want {
				if !strings.Contains(l.text, w) {
					break next
				}
			}
			return l.text
		}
	}
	return ""
}

// THE CORE INVARIANT: printed rate x printed duration = printed run total.
// This is what makes "DOUBLE" verifiable instead of a bare assertion.
func TestPrintedRateTimesPrintedDurationIsThePrintedTotal(t *testing.T) {
	pricedConfig(t)
	for _, mode := range modes {
		for agents := 4; agents <= 10; agents++ {
			m := dialogAt(mode, agents)
			line := findLine(t, m, "per minute = ")
			if line == "" {
				// Never "continue" here. An absent line used to make this test
				// pass having checked nothing at all.
				t.Fatalf("%s/%d: no arithmetic line on screen — the total cannot be checked", mode, agents)
			}
			// "   45s x $0.0243 per minute = $0.02"
			nums := moneyRe.FindAllString(line, -1)
			if len(nums) != 2 {
				t.Fatalf("%s/%d: cannot read the arithmetic line: %q", mode, agents, line)
			}
			perMin, total := parseBack(nums[0]), parseBack(nums[1])

			_, _, _, seconds := agent.RunShape(mode, agents)
			got := perMin * (seconds / 60)
			// Half a cent: the resolution the total is printed at.
			if math.Abs(got-total) > 0.005 {
				t.Errorf("%s/%d: screen says %s, but %.4f/min x %.0fs = $%.4f, not $%.4f",
					mode, agents, line, perMin, seconds, got, total)
			}
		}
	}
}

// The hour must remain sixty times the minute, as printed.
func TestPrintedHourIsSixtyTimesPrintedMinute(t *testing.T) {
	pricedConfig(t)
	for _, mode := range modes {
		for agents := 4; agents <= 10; agents++ {
			line := findLine(t, dialogAt(mode, agents), "PER MINUTE.", "PER HOUR:")
			if line == "" {
				t.Fatalf("%s/%d: no rate line at all", mode, agents)
			}
			nums := moneyRe.FindAllString(line, -1)
			if len(nums) != 2 {
				t.Fatalf("%s/%d: cannot read %q", mode, agents, line)
			}
			perMin, perHour := parseBack(nums[0]), parseBack(nums[1])
			if perHour == 0 && perMin > 0 {
				t.Errorf("%s/%d: %q — an hour of a real rate printed as zero", mode, agents, line)
			}
			if math.Abs(perMin*60-perHour) > 0.005 {
				t.Errorf("%s/%d: %q — %.4f x 60 = %.4f, not %.4f", mode, agents, line, perMin, perMin*60, perHour)
			}
		}
	}
}

// THE "8 HELPERS" BUG: no line may claim more helpers than the selector allows.
func TestNoLineClaimsMoreHelpersThanTheUserCanSelect(t *testing.T) {
	pricedConfig(t)
	helperRe := regexp.MustCompile(`(\d+) helpers`)
	for _, mode := range modes {
		for agents := 4; agents <= 10; agents++ {
			for _, l := range dialogAt(mode, agents).costLines() {
				for _, mt := range helperRe.FindAllStringSubmatch(l.text, -1) {
					var v int
					fmt.Sscanf(mt[1], "%d", &v)
					if v > 10 {
						t.Errorf("%s/%d: %q claims %d helpers; the stated maximum is 10",
							mode, agents, l.text, v)
					}
					if v != agents {
						t.Errorf("%s/%d: %q says %d helpers while the selector says %d",
							mode, agents, l.text, v, agents)
					}
				}
			}
		}
	}
}

// The session count on screen must be the one the scheduler will really create.
func TestSupervisedSessionCountMatchesTheScheduler(t *testing.T) {
	pricedConfig(t)
	// Ground truth, measured from the real selectRoles: agents*2 is right ONLY
	// at 4. Everywhere else supervision skips the peeking lanes.
	want := map[int]int{4: 8, 5: 9, 6: 11, 7: 13, 8: 15, 9: 17, 10: 18}
	for agents, wantSessions := range want {
		got, _, _, _ := agent.RunShape("supervised", agents)
		if got != wantSessions {
			t.Errorf("supervised/%d: RunShape says %d sessions, scheduler creates %d", agents, got, wantSessions)
		}
		if agents != 4 && got == agents*2 {
			t.Errorf("supervised/%d: back to agents*2 (%d) — the peeking lanes are being billed for audits that never run",
				agents, agents*2)
		}
		if m := dialogAt("supervised", agents); m.sessionCount() != wantSessions {
			t.Errorf("supervised/%d: dialog sessionCount %d, want %d", agents, m.sessionCount(), wantSessions)
		}
	}
}

// Supervised must cost more than parallel in TOTAL while running at the same
// RATE — that is what "double" actually means here, and the screen has to be
// able to show both facts without contradicting itself.
func TestSupervisedCostsMoreInTotalAtTheSameRate(t *testing.T) {
	const agents = 6
	pSessions, _, _, pSecs := agent.RunShape("parallel", agents)
	sSessions, _, _, sSecs := agent.RunShape("supervised", agents)

	if sSessions <= pSessions {
		t.Errorf("supervised creates %d sessions, parallel %d — supervision is not happening", sSessions, pSessions)
	}
	if sSecs <= pSecs {
		t.Errorf("supervised runs %.0fs, parallel %.0fs — the audit pass costs no time, which cannot be right", sSecs, pSecs)
	}
	// Same per-session cost, more sessions => the total must rise.
	if float64(sSessions)/float64(pSessions) < 1.5 {
		t.Errorf("supervised is only %.2fx parallel in sessions; the screen calls it nearly double",
			float64(sSessions)/float64(pSessions))
	}
}

// NON-VACUOUS GUARD: the old agents*2 rule must fail the scheduler check above.
func TestTheOldDoublingRuleDisagreesWithTheScheduler(t *testing.T) {
	disagreements := 0
	for agents := 4; agents <= 10; agents++ {
		real, _, _, _ := agent.RunShape("supervised", agents)
		if agents*2 != real {
			disagreements++
		}
	}
	if disagreements == 0 {
		t.Fatal("agents*2 now agrees with the scheduler everywhere — TestSupervisedSessionCountMatchesTheScheduler is vacuous")
	}
	t.Logf("old agents*2 rule was wrong at %d of 7 helper counts", disagreements)
}

// A run must always report a duration; without it "DOUBLE" is unverifiable.
func TestEveryModeShowsHowLongTheRunTakes(t *testing.T) {
	pricedConfig(t)
	for _, mode := range modes {
		for agents := 4; agents <= 10; agents++ {
			if findLine(t, dialogAt(mode, agents), "of running.") == "" {
				t.Errorf("%s/%d: no run duration on screen — the doubling is invisible and uncheckable", mode, agents)
			}
		}
	}
}
