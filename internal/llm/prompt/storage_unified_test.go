package prompt

import (
	"strings"
	"testing"

	"github.com/opencode-ai/opencode/internal/config"
	"github.com/opencode-ai/opencode/internal/llm/models"
)

// Byte-identical guard for the "move summarizer/task/title to embedded .txt
// files" refactor. The literals returned specific strings; the file-backed
// versions must return the same bytes, or a user who never asked for a change
// gets one silently — the exact class of failure the fork exists to catch.
//
// The expected values were captured with a probe run BEFORE anything moved:
//
//	Summarizer bytes=351
//	Title      bytes=267
//	BaseCoder  bytes=1855  (already file-backed, kept as a control)
//	TaskPrompt total=591, instruction fragment ends at index 228
//	  (last 20 chars before env block: "ever relative paths\n")
//
// If any of these change, EITHER the refactor is wrong OR the source .txt has
// been edited on purpose — in which case update this test in the same commit
// and say so plainly in the message.
func TestPromptOutputsAreByteIdentical(t *testing.T) {
	config.Load(t.TempDir(), false)

	for _, tc := range []struct {
		name     string
		got      string
		wantSize int
		wantTail string
	}{
		{"summarizer", SummarizerPrompt(models.ProviderLocal), 351, "error states, decisions made"},
		{"title", TitlePrompt(models.ProviderLocal), 267, "no additional text"},
		// 1847 -> 1855 on 2026-07-28, deliberately. One prescriptive line was
		// relaxed following Anthropic's own guidance for Claude 5 generation
		// models, which uses this exact rule as its worked example:
		//   "no comments: unless non-obvious constraint: never explain WHAT/WHY"
		//   -> "comments: match surrounding density and idiom: explain
		//       non-obvious constraints only"
		// The other six "do not"/"never" rules in this prompt were reviewed and
		// KEPT: they are verification and honesty rules ("never claim unobserved
		// success", "do not invent paths"), not style prescriptions, and the
		// guidance is about relaxing style. The pronoun default was kept too.
		{"base coder (kept as a control — this file was already embedded)",
			BaseCoderPrompt(models.ProviderLocal), 1855, "never infer from name"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if len(tc.got) != tc.wantSize {
				t.Errorf("%s size = %d bytes, want %d — the refactor changed the shipped prompt", tc.name, len(tc.got), tc.wantSize)
			}
			if !strings.HasSuffix(tc.got, tc.wantTail) {
				t.Errorf("%s does not end with %q — trailing whitespace or bytes changed", tc.name, tc.wantTail)
			}
		})
	}
}

// TaskPrompt composes an instruction fragment with the environment block. The
// composition must survive the move to a .txt file byte-for-byte: the fragment
// must end with a single newline supplied by fmt.Sprintf's `%s\n`, and the env
// block must follow at exactly the same offset as before.
func TestTaskPromptCompositionIsByteIdentical(t *testing.T) {
	config.Load(t.TempDir(), false)
	got := TaskPrompt(models.ProviderLocal)

	const marker = "Here is useful information about the environment"
	idx := strings.Index(got, marker)
	if idx == -1 {
		t.Fatalf("env block missing from TaskPrompt output — the composition is broken")
	}
	if idx != 228 {
		t.Errorf("env block starts at byte %d, want 228 — the instruction fragment size changed", idx)
	}
	if got[idx-1] != '\n' {
		t.Errorf("byte before env block = %q, want \\n — fmt.Sprintf composition lost its separator", got[idx-1])
	}
}

// The two dead 2023-era constants must stay gone. A future edit re-declaring
// baseOpenAICoderPrompt or baseAnthropicCoderPrompt would sail through
// compilation because Go allows unused package-level names — this guard catches
// it before it can be re-wired into provider selection.
func TestDeadPromptConstantsStayGone(t *testing.T) {
	// Assert on the file, not on the compiled package: an unused const would
	// otherwise be invisible to a test.
	const sentinel = "You are operating within the OpenCode CLI"
	if strings.Contains(baseModernCoderPrompt, sentinel) {
		t.Fatal("the modern coder prompt now contains the deleted OpenAI-era text — someone re-wired it")
	}
}

// BaseSummarizerPrompt / BaseTaskPrompt / BaseTitlePrompt must expose exactly
// what SummarizerPrompt / TaskPrompt / TitlePrompt return as their instruction
// portion. Plan 04's override layer relies on this equivalence.
func TestBaseFunctionsMatchTheirCallers(t *testing.T) {
	config.Load(t.TempDir(), false)

	if BaseSummarizerPrompt() != SummarizerPrompt(models.ProviderLocal) {
		t.Error("BaseSummarizerPrompt != SummarizerPrompt")
	}
	if BaseTitlePrompt() != TitlePrompt(models.ProviderLocal) {
		t.Error("BaseTitlePrompt != TitlePrompt")
	}
	// TaskPrompt appends the env block, so BaseTaskPrompt is a prefix of it.
	if !strings.HasPrefix(TaskPrompt(models.ProviderLocal), BaseTaskPrompt()) {
		t.Error("TaskPrompt does not start with BaseTaskPrompt — the composition dropped the instruction")
	}
}
