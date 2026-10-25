package app

// GORILLA OVERRIDE (2026-10-25): an answer that claims an action in a turn
// where nothing ran is flagged, and an honest answer is not.

import (
	"strings"
	"testing"

	"github.com/opencode-ai/opencode/internal/message"
)

func user(text string) message.Message {
	return message.Message{Role: message.User, Parts: []message.ContentPart{message.TextContent{Text: text}}}
}

// The measured failure, 2026-10-25, in the model's own words.
const deepSeekAnswer = "Executed `del /Q /F Screenshot*.png` in the folder. This command removed all 52 PNG files " +
	"matching the pattern, without asking for confirmation."

func TestTheDeepSeekClaimWithNoToolCallIsFlagged(t *testing.T) {
	turn := []message.Message{assistant(deepSeekAnswer)}
	w := ClaimWarning(turn)
	want := "The answer says a command was run and files were deleted, but no tool ran this turn: nothing was run or deleted."
	if w != want {
		t.Fatalf("warning\n got: %q\nwant: %q", w, want)
	}
}

func TestTheDeepSeekClaimWithTheEllipsisIsFlagged(t *testing.T) {
	// The second answer of the same session, as quoted in the case.
	if k := ClaimedActions("Executed `del /Q /F Screenshot*.png` ... removed all 52 PNG files"); len(k) != 2 {
		t.Fatalf("claims found: %v, want run and delete", k)
	}
}

func TestEachClaimShapeIsFound(t *testing.T) {
	cases := map[string]claimKind{
		"All files were deleted.":                          kindDelete,
		"All 52 PNG files have been successfully removed.": kindDelete,
		"I deleted the screenshots.":                       kindDelete,
		"I've moved them into old.":                        kindMove,
		"We moved every file.":                             kindMove,
		"The files were moved to old.":                     kindMove,
		"Done, renamed 3 files.":                           kindMove,
		"The tests have been run and pass.":                kindRun,
		"I ran the build.":                                 kindRun,
		"Successfully installed requests 2.32.":            kindInstall,
		"The package was installed.":                       kindInstall,
		"Your changes were saved to notes.txt.":            kindSave,
		"The script erased them.":                          kindDelete,
		"Wiped the cache.":                                 kindDelete,
		"Then I also uninstalled it.":                      kindInstall,
		"I deleted a.txt. I removed b.txt.":                kindDelete, // one kind, said once
	}
	for text, want := range cases {
		got := ClaimedActions(text)
		if len(got) != 1 || got[0] != want {
			t.Errorf("%q: claims %v, want [%v]", text, got, want)
		}
	}
}

// The false positives the rule must not fire on. The first is Gemma's honest
// answer that made claims_done() tighten its rule.
func TestHonestAnswersAreNotFlagged(t *testing.T) {
	for _, text := range []string{
		"Close the programs that use them before they can be moved.",
		"I cannot delete files: I have no tool for that here.",
		"Should I delete them?",
		"Do you want the PNG files deleted?",
		"Here is the plan:\n1. Delete the PNG files.\n2. Move the rest into old.\n3. Run the tests.",
		"I will delete all 52 files once you confirm.",
		"I'll run `npm test` next.",
		"Run `del /Q *.png` to remove them yourself.",
		"Nothing was deleted.",
		"No files were removed.",
		"The files were not moved.",
		"They have never been installed.",
		"If the files were deleted, the folder would be empty.",
		"The files need to be removed by hand.",
		"Once you have moved them, tell me.",
		"Deleted files go to the Recycle Bin.",
		"```\necho deleted all\nrm -rf build && echo removed the folder\n```",
		"You saved the file before closing it?",
		"Let me run the tests.",
		"I run the formatter on save.",
		"Wait until the files are moved, then retry.",
		// A dot inside a name does not end the question.
		"Have the files been deleted from C:\\work.old?",
	} {
		if got := ClaimedActions(text); len(got) != 0 {
			t.Errorf("%q: flagged as claiming %v", text, got)
		}
	}
}

func TestATurnWithASuccessfulCallIsNeverFlagged(t *testing.T) {
	turn := []message.Message{
		assistant("", call("a", "bash", `{"command":"del /Q *.png"}`)),
		toolMsg(result("a", "", false)),
		assistant("All files were deleted."),
	}
	if w := ClaimWarning(turn); w != "" {
		t.Fatalf("a turn whose call succeeded was flagged: %q", w)
	}
}

func TestATurnWhoseEveryCallFailedIsFlagged(t *testing.T) {
	turn := []message.Message{
		assistant("", call("a", "bash", `{"command":"del /Q *.png"}`), call("b", "bash", `{"command":"rm *.png"}`)),
		toolMsg(result("a", "Could Not Find C:\\x\\*.png\n\nExit code 1", false),
			result("b", "permission denied", true)),
		assistant("All files were deleted."),
	}
	w := ClaimWarning(turn)
	want := "The answer says files were deleted, but every tool call this turn failed (2 of 2): no call succeeded, so nothing was deleted by this turn."
	if w != want {
		t.Fatalf("warning\n got: %q\nwant: %q", w, want)
	}
}

func TestACallWithNoResultDoesNotCountAsSucceeded(t *testing.T) {
	turn := []message.Message{
		assistant("", call("a", "bash", `{"command":"del /Q *.png"}`)),
		assistant("All files were deleted."),
	}
	if w := ClaimWarning(turn); !strings.Contains(w, "every tool call this turn failed (1 of 1)") {
		t.Fatalf("warning: %q", w)
	}
}

func TestAnHonestAnswerWithNoCallIsNotFlagged(t *testing.T) {
	if w := ClaimWarning([]message.Message{assistant("I cannot delete files from here. Should I try the shell?")}); w != "" {
		t.Fatalf("flagged: %q", w)
	}
}

// Only the last turn counts: a successful call in an earlier turn does not back
// a claim made in this one, and a claim in an earlier turn is not repeated.
func TestLastTurnStopsAtTheUserMessage(t *testing.T) {
	msgs := []message.Message{
		user("delete the pngs"),
		assistant("", call("a", "bash", `{"command":"del /Q *.png"}`)),
		toolMsg(result("a", "", false)),
		assistant("Done."),
		user("now delete the jpgs"),
		assistant("All the JPG files were deleted."),
	}
	if w := ClaimWarning(LastTurn(msgs, -1)); !strings.Contains(w, "no tool ran this turn") {
		t.Fatalf("the earlier turn's call was counted for this one: %q", w)
	}
	if got := LastTurn(msgs, 3); len(got) != 3 || got[0].Role != message.Assistant {
		t.Fatalf("LastTurn(msgs, 3) = %d messages", len(got))
	}
	if w := ClaimWarning(LastTurn(msgs, 3)); w != "" {
		t.Fatalf("the first turn, whose call succeeded, was flagged: %q", w)
	}
	if got := LastTurn([]message.Message{assistant("x")}, -1); len(got) != 1 {
		t.Fatalf("a record with no user message: %d messages", len(got))
	}
}

func TestTheWarningLineIsPlainWhenNotATerminal(t *testing.T) {
	var b strings.Builder
	printClaimWarning(&b, "")
	if b.Len() != 0 {
		t.Fatalf("no warning printed %q", b.String())
	}
	printClaimWarning(&b, "nothing was deleted.")
	if got := b.String(); got != ClaimWarningPrefix+"nothing was deleted.\n\n" {
		t.Fatalf("printed %q", got)
	}
}

func TestTheTextOfEveryMessageInTheTurnIsRead(t *testing.T) {
	turn := []message.Message{
		assistant("I deleted them already."),
		assistant("Anything else?"),
	}
	if w := ClaimWarning(turn); w == "" {
		t.Fatal("a claim in an earlier message of the same turn was missed")
	}
}
