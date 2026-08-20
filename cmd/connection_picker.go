// GORILLA OVERRIDE: this file did not exist upstream. It builds the connection
// profile rows and applies the answer, keeping the same layering as the other
// pickers (startup renders, cmd knows about config).
package cmd

import (
	"fmt"
	"os"

	"github.com/opencode-ai/opencode/internal/config"
	"github.com/opencode-ai/opencode/internal/tui/startup"
)

// adviceFor is the honest, specific suggestion for a given profile.
//
// The numbers are measured, not guessed: a full loadout is ~13,900 tokens of
// system prompt and tool schemas, re-uploaded on EVERY message, because the
// model has no memory between turns. Tool schemas are 200-850 tokens each and
// are the bulk of it; the eight prompt sections together are ~460 tokens, so
// switching those off saves tens of tokens, not thousands. Anyone told to "trim
// the prompt" would be aiming at the wrong thing.
//
// The reassurance at the end is load-bearing and it is TRUE: bash, view, edit,
// patch, find and write are a complete coding loop. Nothing about reading,
// changing or running code is lost by dropping the research and delegation
// tools.
// adviceFor is the honest, specific suggestion for a given profile.
//
// THE ORDER MATTERS AND IT WAS WRONG AT FIRST. The initial version told people
// to switch tools off first. Measured 2026-08-20, that is the LAST thing worth
// doing and the only one that costs capability:
//
//  1. non-streaming        27x less downlink   costs: the typing effect
//  2. keep "# output" on   prevents a 30x loss costs: nothing
//  3. cap reply length     proportional        costs: truncated long answers
//  4. drop tool schemas    200-850 tok each    costs: THOSE TOOLS
//
// Items 1-3 remove no capability at all. Recommending 4 first told people to
// give up abilities to save the smallest amount, while the free wins sat
// unmentioned. The numbers behind this: a full loadout is ~13,900 tokens of
// prompt and tool schemas re-uploaded every message, but a streamed reply costs
// 377 bytes PER OUTPUT TOKEN, so the answer is usually the bigger half.
func adviceFor(id config.ConnProfileID) string {
	switch id {
	case config.ProfileAustere, config.ProfileConstrained:
		return "WHAT ELSE HELPS, in the order that actually pays. " +
			"First: this profile already receives answers in one piece rather " +
			"than word by word, which is the single biggest saving and costs you " +
			"nothing but the typing effect. " +
			"Second: leave the reply-style instructions switched on in /context - " +
			"they are what keep answers short, and short answers are the largest " +
			"cost on a slow link. Switching them off saves a few hundred bytes " +
			"and can cost you tens of thousands in a longer reply. " +
			"Third: if answers are still too long, lower the reply length limit " +
			"in /settings. " +
			"Only last, if you still need to save more, switch off tools you are " +
			"not using in /context - agent, research, dossier, websearch, " +
			"sourcegraph and sparse are the expensive ones. Keep bash, view, " +
			"edit, patch, find and write and you can still read, change, patch " +
			"and run code perfectly well. This is last because it is the only one " +
			"that takes an ability away from you."
	case config.ProfileModest:
		return "If turns start failing, /context lets you switch off tools you are " +
			"not using; the research and delegation ones cost the most per message. " +
			"Try shortening replies before removing abilities."
	default:
		return ""
	}
}

func effectFor(p config.ConnProfile) string {
	mode := "The answer appears a word at a time, as it is written."
	if !p.Stream {
		mode = "The answer appears all at once when it is finished, not word by word."
	}
	return fmt.Sprintf(
		"Waits up to %s for an answer to start, gives up after %s of silence "+
			"mid-answer, allows %.1f MB per message, and tries %d times before "+
			"reporting a failure. %s",
		p.FirstByte, p.StreamStall, p.UploadMB, p.MaxRetries, mode)
}

// streamingNote is the plain-language explanation of the one setting in these
// profiles that changes something a user can SEE. Everything else here is
// invisible until it fails; this one is noticeable immediately, so it gets
// explained properly rather than buried.
//
// The intent (owner, 2026-08-20) is that the user makes their own decision. So
// this states the cost, the benefit and what is NOT lost, and points at the way
// to overrule it. Data is money in the places this program is built for -
// South America, Chad, Afghanistan, remote Africa - where an allowance is paid
// for up front and often cannot be topped up with a card.
func streamingNote(p config.ConnProfile) string {
	if p.Stream {
		return "Answers arrive a word at a time here. That is the normal, " +
			"comfortable behaviour, and it costs about 27 times more data than " +
			"receiving the answer in one piece - which is why the two slowest " +
			"profiles switch it off."
	}
	return "WHY THE TYPING EFFECT IS OFF HERE. When the AI writes a word at a " +
		"time, each single word is sent in its own package with a full label " +
		"wrapped around it. The labels are far bigger than the words. Measured " +
		"on the same answer: 22,256 bytes word-by-word, against 834 bytes sent " +
		"in one piece - about 27 times more data to watch it type. " +
		"You are charged by the AI company for the same number of words either " +
		"way; this changes only the data your connection carries, and where " +
		"data is bought by the megabyte that is real money. " +
		"What you lose: the answer no longer appears gradually - the screen " +
		"stays quiet, then the whole reply arrives. It also becomes harder to " +
		"tell a slow answer from a stuck one. " +
		"What you do NOT lose: anything else. Same answer, same quality, same " +
		"abilities, same cost in words. " +
		"If you would rather watch it type, choose a faster profile from this " +
		"list, or set GORILLA_OPENCODE_STREAM=1 to overrule it."
}

// connectionRows builds the ladder, marking the active profile and the one the
// measurement suggests.
func connectionRows() ([]startup.ConnRow, string) {
	cur := config.CurrentConnProfile()
	rec, kbps, haveRec := config.RecommendProfile()

	measured := ""
	if haveRec {
		switch {
		case kbps < 1:
			measured = "under 1 KB/s"
		case kbps < 1000:
			measured = fmt.Sprintf("about %.0f KB/s", kbps)
		default:
			measured = fmt.Sprintf("about %.1f MB/s", kbps/1024)
		}
	}

	rows := make([]startup.ConnRow, 0, len(config.ConnProfiles))
	for _, p := range config.ConnProfiles {
		rows = append(rows, startup.ConnRow{
			ID:          string(p.ID),
			Name:        p.Name,
			Rate:        p.Rate,
			Links:       "Typical of: " + p.Links,
			What:        p.Layman,
			Effect:      effectFor(p),
			Advice:      streamingNote(p) + "\n\n" + adviceFor(p.ID),
			Recommended: haveRec && p.ID == rec,
			Active:      p.ID == cur.ID,
		})
	}
	return rows, measured
}

// runConnectionPicker shows the picker and applies the answer. It never blocks
// the launch: a picker that cannot run is a nuisance, not a reason to refuse to
// start.
func runConnectionPicker() {
	rows, measured := connectionRows()
	choice, err := startup.AskConnection(rows, measured)
	if err != nil {
		fmt.Fprintf(os.Stderr, "could not show the connection picker (%v); continuing with the current profile\n", err)
		return
	}
	if choice.Quit || choice.Keep || choice.ID == "" {
		// Esc means "leave it alone", but the user HAS now seen the screen, so
		// it must stop volunteering itself. Otherwise dismissing it once would
		// re-offer it on every launch, which is the nag the trigger policy
		// exists to avoid.
		_ = config.MarkProfileChosen()
		return
	}
	if err := config.SetConnProfile(config.ConnProfileID(choice.ID)); err != nil {
		fmt.Fprintf(os.Stderr, "could not set the connection profile: %v\n", err)
		return
	}
	_ = config.MarkProfileChosen()
}

// maybeOfferConnectionPicker applies the trigger policy: first run, or the
// measurement disagreeing with the active profile by two rungs or more.
func maybeOfferConnectionPicker() {
	if !config.ShouldOfferProfilePicker() {
		return
	}
	runConnectionPicker()
}
