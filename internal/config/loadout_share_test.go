package config

import (
	"runtime"
	"strings"
	"testing"
)

// GORILLA OVERRIDE (2026-09-01): the /context screen priced everything in
// dollars, so on a model running on your own machine it said
// "~ $0.00/turn (free / flat-rate tier)" — technically true and completely
// useless. The loadout is not free there, it is billed in context and in time:
// measured on a laptop, 10K of loadout against a 20K window is half the
// conversation gone before the first word, and minutes of prompt processing on
// every turn.
func TestContextShareIsReportedForAModelWithAWindow(t *testing.T) {
	if _, err := Load(t.TempDir(), false); err != nil {
		t.Fatalf("Load: %v", err)
	}
	tokens, window, _, share := LoadoutContextShare()
	if tokens <= 0 {
		t.Fatal("the loadout reports zero tokens; it is never actually zero")
	}
	if window > 0 {
		want := float64(tokens) / float64(window)
		if share < want-0.001 || share > want+0.001 {
			t.Errorf("share = %.4f, want %.4f", share, want)
		}
	}
}

// A model that declares no window must produce share 0 so the caller can omit
// the line rather than print a meaningless percentage.
func TestNoWindowMeansNoShare(t *testing.T) {
	if _, err := Load(t.TempDir(), false); err != nil {
		t.Fatalf("Load: %v", err)
	}
	a := cfg.Agents[AgentCoder]
	a.Model = "definitely-not-a-real-model"
	cfg.Agents[AgentCoder] = a

	_, window, _, share := LoadoutContextShare()
	if window != 0 || share != 0 {
		t.Errorf("window=%d share=%.2f, want 0/0 for an unknown model", window, share)
	}
	if LoadoutCrowdsTheContext() {
		t.Error("an unknown model must not be reported as crowding the context")
	}
}

// GORILLA OVERRIDE (2026-09-01): the screen listed eighteen rows and their costs
// and left the arithmetic to the reader. Someone who thinks in token budgets can
// do it; the person this program is written for closes the screen having changed
// nothing. These tests cover the advice that now does the arithmetic for them.

func TestBiggestCutsNeverRecommendsBreakingTheAgent(t *testing.T) {
	if _, err := Load(t.TempDir(), false); err != nil {
		t.Fatalf("Load: %v", err)
	}
	cuts, _ := LoadoutBiggestCuts(5)
	if len(cuts) == 0 {
		t.Fatal("no cuts suggested at all; the default loadout is not empty")
	}
	critical := map[string]bool{}
	for _, c := range LoadoutComponents {
		if c.Critical {
			critical[c.Name] = true
		}
	}
	for _, c := range cuts {
		if critical[c.Name] {
			t.Errorf("suggested turning off %q, which is marked Critical — bash and edit are the "+
				"two most expensive rows on the screen, and an agent that cannot run a command or "+
				"change a file is not a cheaper agent, it is a broken one", c.Name)
		}
	}
}

// Largest first, and the total must be the sum of what is listed — a number the
// reader is asked to act on has to be the number they would actually get back.
func TestBiggestCutsAreOrderedAndTheTotalIsHonest(t *testing.T) {
	if _, err := Load(t.TempDir(), false); err != nil {
		t.Fatalf("Load: %v", err)
	}
	cuts, total := LoadoutBiggestCuts(3)
	sum := 0
	for i, c := range cuts {
		sum += c.Tokens
		if i > 0 && cuts[i-1].Tokens < c.Tokens {
			t.Errorf("cuts are not largest-first: %q (%d) came before %q (%d)",
				cuts[i-1].Name, cuts[i-1].Tokens, c.Name, c.Tokens)
		}
	}
	if sum != total {
		t.Errorf("total = %d but the listed cuts sum to %d", total, sum)
	}
	if len(cuts) > 3 {
		t.Errorf("asked for 3 cuts, got %d", len(cuts))
	}
}

// A component already switched off must not be offered again — suggesting a
// saving the reader has already taken makes the advice look broken.
func TestBiggestCutsSkipsWhatIsAlreadyOff(t *testing.T) {
	if _, err := Load(t.TempDir(), false); err != nil {
		t.Fatalf("Load: %v", err)
	}
	cuts, _ := LoadoutBiggestCuts(3)
	if len(cuts) == 0 {
		t.Skip("nothing switchable is on")
	}
	target := cuts[0].Name
	var id string
	for _, c := range LoadoutComponents {
		if c.Name == target {
			id = c.ID
			break
		}
	}
	if id == "" {
		t.Fatalf("could not find the component id for %q", target)
	}
	ToggleLoadout(id) // it was on (LoadoutBiggestCuts only lists enabled rows)
	t.Cleanup(func() { ToggleLoadout(id) })
	if LoadoutEnabled(id) {
		t.Fatalf("%q did not switch off", target)
	}

	after, _ := LoadoutBiggestCuts(3)
	for _, c := range after {
		if c.Name == target {
			t.Errorf("%q is switched off but is still being suggested as a saving", target)
		}
	}
}

// GORILLA OVERRIDE (2026-09-01): the shell row must not invite a Windows user to
// break their own install.
//
// Asked verbatim by a user reading this screen: "do I need the bash tool on in a
// Windows environment?" The honest answer to what the row SAID was no — nobody
// on Windows uses bash. The honest answer to what it DOES is that it is the most
// important tool in the program: switching it off leaves an agent that cannot
// build, test, run git, or execute anything at all.
func TestTheShellRowIsNamedForTheShellThatRunsIt(t *testing.T) {
	var row *LoadoutComponent
	for i := range LoadoutComponents {
		if LoadoutComponents[i].ID == "tool.bash" {
			row = &LoadoutComponents[i]
		}
	}
	if row == nil {
		t.Fatal("the shell row is gone")
	}
	if !row.Critical {
		t.Error("the shell row is not marked Critical — it must never be suggested as a saving")
	}
	if runtime.GOOS != "windows" {
		if !strings.Contains(strings.ToLower(row.Name), "bash") {
			t.Errorf("row name %q should say bash on this platform", row.Name)
		}
		return
	}
	if strings.EqualFold(row.Name, "Bash tool") {
		t.Errorf("row is still called %q on Windows, where it drives PowerShell — "+
			"the label tells the reader they do not need the one tool they cannot work without", row.Name)
	}
	if !strings.Contains(row.Name, "PowerShell") {
		t.Errorf("row name %q does not name PowerShell", row.Name)
	}
}

// The ID is a persisted config key and the gate every caller checks. Renaming it
// would silently reset the setting for everyone who has already chosen.
func TestTheShellRowKeepsItsStableID(t *testing.T) {
	found := false
	for _, c := range LoadoutComponents {
		if c.ID == "tool.bash" {
			found = true
		}
	}
	if !found {
		t.Fatal(`the shell component id changed from "tool.bash" — every saved loadout that ` +
			`switched it off silently reverts to the default`)
	}
}
