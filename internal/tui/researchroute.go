// GORILLA (2026-10-05): what /research and /osint do with what was typed after
// them, decided in one place and before anything can cost money.
//
// Audit findings S1 and S2. Both commands went straight from the typed text to
// the cost dialog, and from the cost dialog to a prompt telling the model to
// call the research tool. Two things were never asked on the way:
//
//   - Is the research tool switched on? toolgate.go records this exact failure
//     for /review: the low-bandwidth trim in /context switches tool.research
//     off and nothing puts it back. With it off, the user read the cost screen,
//     pressed enter, and the model was ordered to use a tool it had not been
//     given. /osint checked its own row (tool.dossier) and not the engine's.
//
//   - Is this a question at all? `/research help`, `/research --help`,
//     `/osint -h` and `/osint ?` were taken as the subject to investigate. One
//     press of enter later, four to ten paid model sessions were researching
//     the word "help". The same fault as `/osint --recover` being read as a
//     question on 2026-08-18, left open one word further along.
//
// The decision is a pure function of the text and two switches so that it can
// be tested without a terminal, and it lives outside the dispatch switch in
// tui.go for the reason reviewargs.go does: a nested switch on strings there
// reads to TestEveryDispatchedCommandIsDocumented as new slash commands.
package tui

import (
	"fmt"
	"strings"

	"github.com/opencode-ai/opencode/internal/config"
	"github.com/opencode-ai/opencode/internal/llm/agent"
)

// researchToolID is the loadout row both commands run on.
const researchToolID = "tool.research"

// researchRoute is what a /research or /osint invocation should do.
type researchRoute int

const (
	// routeRun opens the cost dialog: the only route that can lead to spending.
	routeRun researchRoute = iota
	// routeHelp explains the command instead of running it.
	routeHelp
	// routeEmpty is the command with nothing after it.
	routeEmpty
	// routeRecover is /osint --recover: lists saved runs, spends nothing.
	routeRecover
	// routeToolOff refuses because the research tool is switched off.
	routeToolOff
	// routeDossierOff refuses because the dossier row is not armed.
	routeDossierOff
)

// routeResearch decides /research. Help is recognised before the tool check on
// purpose: someone whose tool is off still deserves to be told what the command
// is, and the explanation costs nothing.
func routeResearch(args string, researchOn bool) researchRoute {
	switch {
	case wantsCommandHelp(args):
		return routeHelp
	case isBlank(args):
		return routeEmpty
	case !researchOn:
		return routeToolOff
	}
	return routeRun
}

// routeOsint decides /osint. Recovery comes first and needs neither switch: it
// reads findings already on disk and sends out no helpers, so refusing it
// because the research tool is off would strand exactly the user it exists
// for — the one whose connection dropped and whose tools the trim turned off.
//
// The dossier row is checked before the research row so that someone who has
// armed neither is told about the dossier's own switch first; with the
// dossier armed and the engine off, the engine's row is named.
func routeOsint(args string, researchOn, dossierOn bool) researchRoute {
	switch {
	case isRecoverFlag(args):
		return routeRecover
	case wantsCommandHelp(args):
		return routeHelp
	case isBlank(args):
		return routeEmpty
	case !dossierOn:
		return routeDossierOff
	case !researchOn:
		return routeToolOff
	}
	return routeRun
}

func isBlank(s string) bool { return strings.TrimSpace(s) == "" }

// researchPrompt is the instruction a confirmed /research sends to the model.
func researchPrompt(mode string, agents int, question string) string {
	return fmt.Sprintf(
		"Use the research tool to investigate the following. "+
			"Set mode=%q and agents=%d exactly as given — the user chose these and they decide what this costs. "+
			"Pass everything already established in this conversation as `context` so no helper pays to re-derive it. "+
			"When the helpers report, check at least one load-bearing claim yourself, carry the evidence tiers through, "+
			"and say plainly what nobody established.\n\nQUESTION: %s",
		mode, agents, question)
}

// osintPrompt is the instruction a confirmed /osint sends to the model.
//
// GORILLA OVERRIDE: the dossier product is WRITTEN OUTSIDE the working folder,
// always. A working folder is often a git repo; a personal question swept into
// a commit and pushed is the worst failure this feature could have. The dossier
// folder (config.DossierDir) is nobody's repo.
//
// GORILLA FIX (2026-10-05): audit finding O2. This said "run the gap check the
// tool's report demands". The report demands nothing of the kind: the follow-up
// is optional, and the tool now holds it to one. The prompt says so, so the
// model is not told by one text that a second call is a duty and by another
// that it is a privilege.
func osintPrompt(mode string, agents int, question string) string {
	return fmt.Sprintf(
		"Use the research tool with doctrine=%q, mode=%q and agents=%d exactly as given — "+
			"the user chose these on the warning screen and they decide what this costs. "+
			"Do not pass `roles`: a dossier runs its own open-source lanes and none of them searches this machine. "+
			"Pass everything already established in this conversation as `context` so no helper pays to re-derive it. "+
			"When the helpers report: a follow-up call is OPTIONAL and allowed at most once, only for a gap the answer "+
			"depends on (the tool refuses a second); verify at least one load-bearing claim yourself, carry every "+
			"two-axis grade through unchanged, and assemble the dossier product "+
			"(BLUF first, graded claims, SOURCES TRIED, NOT ESTABLISHED, recommended action). "+
			"Then write the complete dossier as markdown to a NEW timestamped file under %q using the write tool "+
			"(create the folder if it is missing), tell the user the exact path, and give them the BLUF and key "+
			"findings in the conversation. Never write the dossier into the working folder: it may be a git "+
			"repository, and a private question must never end up in a commit.\n\nQUESTION: %s",
		agent.DoctrineDossier, mode, agents, config.DossierDir(), question)
}
