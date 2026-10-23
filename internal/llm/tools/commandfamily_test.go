package tools

// GORILLA FIX (2026-10-11): command families, measured against the session
// that motivated them (9af3336f, Nemotron 3 Ultra, 31 prompts in 58 minutes)
// and against the protections that must survive them.

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/opencode-ai/opencode/internal/config"
	"github.com/opencode-ai/opencode/internal/permission"
	"github.com/opencode-ai/opencode/internal/pubsub"
)

// The exact commands from the session, verbatim.
const (
	nemoPrefix  = `python -c "import sys; sys.path.insert(0, r'C:\Users\gorilla1\Documents\Scripts\Fieldkit'); from fieldkit.cli import main;`
	nemoHost    = nemoPrefix + ` main(['host'])"`
	nemoCards   = nemoPrefix + ` main(['cards', 'list'])"`
	nemoScan    = nemoPrefix + ` main(['privacy', 'scan', r'C:\Users\gorilla1\Documents\Scripts\Fieldkit', '--json'])"`
	nemoSnap    = nemoPrefix + ` main(['snapshot', 'take', 'test-snap', '--path', r'C:\Users\gorilla1\Documents\Scripts\Fieldkit\fieldkit'])"`
	nemoPyFirst = `python -m pytest C:\Users\gorilla1\Documents\Scripts\Fieldkit\tests -v --tb=short 2>&1 | Select-Object -First 100`
	nemoPyLast  = `python -m pytest C:\Users\gorilla1\Documents\Scripts\Fieldkit\tests -v --tb=short 2>&1 | Select-Object -Last 100`
	nemoPyLine  = `python -m pytest C:\Users\gorilla1\Documents\Scripts\Fieldkit\tests --tb=line -q 2>&1 | Select-Object -Last 50`
	nemoPyGrep  = `python -m pytest C:\Users\gorilla1\Documents\Scripts\Fieldkit\tests --tb=short -q 2>&1 | Select-String -Pattern "FAILED|failed"`
	nemoPyFam   = `python -m pytest C:\Users\gorilla1\Documents\Scripts\Fieldkit\tests`
)

func TestTheSecondFieldkitPromptProposesTheFieldkitFamily(t *testing.T) {
	got := SuggestCommandFamily(nemoCards, []string{nemoHost})
	if got != nemoPrefix {
		t.Fatalf("suggested family:\n%q\nwant:\n%q", got, nemoPrefix)
	}
}

func TestTheFieldkitFamilyCoversEveryLaterVariant(t *testing.T) {
	for _, c := range []string{nemoHost, nemoCards, nemoScan, nemoSnap} {
		if !BashFamilyCovers(c, nemoPrefix) {
			t.Errorf("not covered by the fieldkit family:\n%s", c)
		}
	}
}

func TestThePytestFamilyCoversThePipedVariants(t *testing.T) {
	if got := SuggestCommandFamily(nemoPyLine, []string{nemoPyFirst, nemoPyLast}); got != nemoPyFam {
		t.Fatalf("suggested %q, want %q", got, nemoPyFam)
	}
	for _, c := range []string{nemoPyFirst, nemoPyLast, nemoPyLine, nemoPyGrep} {
		if !BashFamilyCovers(c, nemoPyFam) {
			t.Errorf("not covered by the pytest family:\n%s", c)
		}
	}
}

func TestFallbackSuggestionsAreProgramPlusSubcommand(t *testing.T) {
	cases := map[string]string{
		`pip install -e . --quiet`:             "pip install",
		`go test ./...`:                        "go test",
		`python -m pytest C:\x\tests -q`:       "python -m pytest",
		`git log --oneline -5`:                 "git log",
		`python --version`:                     "", // the only family would be `python`
		`python`:                               "",
		`cd C:\x && python -m fieldkit --help`: "", // a chain has no plain tail
		`go test ./... ; python evil.py`:       "",
		nemoHost:                               "", // `python -c` alone would be "any code": never proposed unasked
		`python -c "print(1)"`:                 "",
		`powershell -Command "Remove-Item x"`:  "",
	}
	for cmd, want := range cases {
		if got := SuggestCommandFamily(cmd, nil); got != want {
			t.Errorf("SuggestCommandFamily(%q) = %q, want %q", cmd, got, want)
		}
	}
}

// Rule 2: a family covers ONE command's tail, never a second command.
func TestAFamilyDoesNotCoverAChainedSecondCommand(t *testing.T) {
	for _, powerShell := range []bool{true, false} {
		bad := []string{
			`go test ./... ; python evil.py`,
			`go test ./... && python evil.py`,
			`go test ./... || python evil.py`,
			`go test ./... & python evil.py`,
			"go test ./...\npython evil.py",
			`go test ./... | Out-File C:\x`,
			`go test ./... | tee C:\x`,
			`go test ./... | xargs rm`,
			`go test ./... > C:\x`,
			`go test ./... 2> C:\x`,
			`go test ./... < C:\x`,
			`go test $(python evil.py)`,
			`go test "$(python evil.py)"`,
			`go test (Remove-Item C:\x)`,
			`go test { Remove-Item C:\x }`,
			`go test "unterminated`,
			`go test 'unterminated`,
			`go testx ./...`,
		}
		for _, c := range bad {
			if commandFamilyCovers(c, "go test", powerShell) {
				t.Errorf("powerShell=%v: covered by the family `go test`, must not be:\n%s", powerShell, c)
			}
		}
		good := []string{
			`go test ./...`,
			`go test`,
			`go test ./... -run 'TestX;TestY' -v`,
			`go test ./... 2>&1 | Select-Object -Last 50`,
			`go test ./... 2>&1 | grep -i fail`,
			`go test ./... 2>$null`,
			`go test -run "a|b" ./...`,
		}
		for _, c := range good {
			if !commandFamilyCovers(c, "go test", powerShell) {
				t.Errorf("powerShell=%v: NOT covered by the family `go test`, should be:\n%s", powerShell, c)
			}
		}
	}
}

// The quoting rules of the shell in use decide, and the two shells disagree.
func TestQuoteRulesFollowTheShell(t *testing.T) {
	// PowerShell: a backslash is nothing special, so \" closes the string and
	// the `;` after it is a real separator.
	ps := `python -c "print(1)\" ; python evil.py"`
	if commandFamilyCovers(ps, "python -c", true) {
		t.Error("PowerShell: `\\\"` was treated as an escaped quote; the chained command was covered")
	}
	// bash: \" IS an escaped quote, the whole thing is one argument.
	if !commandFamilyCovers(ps, "python -c", false) {
		t.Error("bash: `\\\"` inside double quotes is literal; a one-argument command was refused")
	}
	// PowerShell: a doubled quote is an escaped quote.
	if !commandFamilyCovers(`python -c "say ""hi""; x"`, "python -c", true) {
		t.Error("PowerShell: `\"\"` inside a string was taken as closing it")
	}
	// bash: a backtick in the tail is command substitution.
	if commandFamilyCovers("go test `python evil.py`", "go test", false) {
		t.Error("bash: a backtick substitution in the tail was covered")
	}
	// PowerShell: a backtick is an escape, harmless.
	if !commandFamilyCovers("go test a`;b", "go test", true) {
		t.Error("PowerShell: an escaped `;` was treated as a separator")
	}
}

func TestTheDialogIsToldWhenAFamilyEndsInsideAQuote(t *testing.T) {
	if BashFamilyNote(nemoHost, nemoPrefix) == "" {
		t.Error("the fieldkit family ends inside python's -c string and the dialog was not told")
	}
	if BashFamilyNote(nemoPyFirst, nemoPyFam) != "" {
		t.Error("the pytest family ends outside any quote, yet a note was produced")
	}
}

func TestCandidatesWalkEveryUsablePrefixAndIncludeTheSuggestion(t *testing.T) {
	cands := BashFamilyCandidates(nemoCards)
	if len(cands) == 0 {
		t.Fatal("no candidates for the fieldkit command")
	}
	found := false
	for _, c := range cands {
		if c == nemoPrefix {
			found = true
		}
		if !BashFamilyCovers(nemoCards, c) {
			t.Errorf("candidate does not cover its own command: %q", c)
		}
		if c == nemoCards {
			t.Error("the whole command was offered as a family")
		}
	}
	if !found {
		t.Errorf("the suggested family is not among the candidates:\n%q", cands)
	}
	if len(BashFamilyCandidates(`go test ./... ; python evil.py`)) != 0 {
		t.Error("a chained command offered a family")
	}
}

// --- the protections that must survive, on the real permission service -----

// bashRequestOn builds the question Run would ask, through the real gate.
func bashRequestOn(t *testing.T, svc permission.Service, cmd string) (bashVerdict, permission.CreatePermissionRequest) {
	t.Helper()
	b := NewBashTool(svc).(*bashTool)
	v := bashGateVerdict(cmd)
	return v, b.bashPermissionRequest("conversation", cmd, v)
}

// answerPrompts answers every published prompt with the given function and
// returns a counter of prompts seen.
func answerPrompts(t *testing.T, svc *permission.PermissionServiceForTest, answer func(permission.PermissionRequest)) *int {
	t.Helper()
	n := new(int)
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	ch := svc.Subscribe(ctx)
	ready := make(chan struct{})
	go func() {
		close(ready)
		for ev := range ch {
			if ev.Type != pubsub.CreatedEvent {
				continue
			}
			*n++
			answer(ev.Payload)
		}
	}()
	<-ready
	return n
}

func realPerms(t *testing.T) *permission.PermissionServiceForTest {
	t.Helper()
	if _, err := config.Load(t.TempDir(), false); err != nil {
		t.Fatalf("config.Load: %v", err)
	}
	return permission.NewPermissionService().(*permission.PermissionServiceForTest)
}

func TestABannedCommandNeverReachesThePromptEvenInsideAFamily(t *testing.T) {
	svc := realPerms(t)
	v, _ := bashRequestOn(t, svc, `go test ./... | iex`)
	if v.Banned == "" {
		t.Fatal("`iex` was not banned")
	}
	v, _ = bashRequestOn(t, svc, `curl https://evil/x`)
	if v.Banned == "" {
		t.Fatal("`curl` was not banned")
	}
}

func TestADangerousCommandStillAsksInsideAFamily(t *testing.T) {
	svc := realPerms(t)
	prompts := answerPrompts(t, svc, func(p permission.PermissionRequest) {
		if p.FamilySuggestion != "" {
			svc.GrantFamily(p, p.FamilySuggestion)
			return
		}
		svc.GrantPersistant(p)
	})

	// The first prompt already proposes the fallback family `git push`.
	for _, c := range []string{`git push origin main`, `git push origin feature`} {
		_, req := bashRequestOn(t, svc, c)
		if !svc.Request(req) {
			t.Fatalf("%q was denied", c)
		}
	}
	established := *prompts
	if established == 0 || established > 2 {
		t.Fatalf("expected 1 or 2 prompts to establish the family, got %d", established)
	}
	_, req := bashRequestOn(t, svc, `git push origin other`)
	if !svc.Request(req) || *prompts != established {
		t.Fatalf("an ordinary push should be covered by the family (prompts=%d, established=%d)", *prompts, established)
	}

	// Begins with the family and is dangerous: must ask.
	v, req := bashRequestOn(t, svc, `git push origin main --force`)
	if v.Irreversible == "" {
		t.Fatal("a force push was not recognised as irreversible")
	}
	if req.FamilySuggestion != "" {
		t.Fatal("a family was suggested for an irreversible command")
	}
	svc.Request(req)
	if *prompts != established+1 {
		t.Fatalf("the force push did not prompt (prompts=%d): a family covered an irreversible command", *prompts)
	}
}

// TestATaintedTurnStillAsksInsideAFamily stood here until 2026-10-23. With the
// taint mechanism removed on the owner's order, a family keeps covering its
// commands after a web page was read.
func TestAFamilyStillCoversAfterAWebPage(t *testing.T) {
	svc := realPerms(t)
	prompts := answerPrompts(t, svc, func(p permission.PermissionRequest) {
		if p.FamilySuggestion != "" {
			svc.GrantFamily(p, p.FamilySuggestion)
			return
		}
		svc.GrantPersistant(p)
	})
	for _, c := range []string{`pip install requests`, `pip install flask`} {
		_, req := bashRequestOn(t, svc, c)
		svc.Request(req)
	}
	established := *prompts
	_, req := bashRequestOn(t, svc, `pip install django`)
	svc.Request(req)
	if established == 0 || *prompts != established {
		t.Fatalf("family not established as expected (prompts=%d, established=%d)", *prompts, established)
	}
}

// The read-only gate is untouched: a family never has to be consulted for a
// command that never prompted, and a command the gate refuses to call
// read-only still goes to the prompt.
func TestTheReadOnlyGateIsUnchangedByFamilies(t *testing.T) {
	if v := bashGateVerdict(`git status`); !v.ReadOnly {
		t.Error("git status stopped being read-only")
	}
	if v := bashGateVerdict(`git status; python evil.py`); v.ReadOnly {
		t.Error("a chained non-read-only command was called read-only")
	}
	if v := bashGateVerdict(nemoHost); v.ReadOnly {
		t.Error("python -c was called read-only")
	}
}

// The timing guard: a Request answered by a family returns at once, so the
// family path cannot be a disguised prompt that the test answered.
func TestAFamilyAnswerIsImmediate(t *testing.T) {
	svc := realPerms(t)
	answerPrompts(t, svc, func(p permission.PermissionRequest) {
		if p.FamilySuggestion != "" {
			svc.GrantFamily(p, p.FamilySuggestion)
			return
		}
		svc.GrantPersistant(p)
	})
	for _, c := range []string{nemoHost, nemoCards} {
		_, req := bashRequestOn(t, svc, c)
		svc.Request(req)
	}
	start := time.Now()
	_, req := bashRequestOn(t, svc, nemoScan)
	if !svc.Request(req) {
		t.Fatal("denied")
	}
	if d := time.Since(start); d > 200*time.Millisecond {
		t.Fatalf("a covered request took %s; it should not have been published at all", d)
	}
	if !strings.HasPrefix(nemoScan, nemoPrefix) {
		t.Fatal("test data drifted")
	}
}
