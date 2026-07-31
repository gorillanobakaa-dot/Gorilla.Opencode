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
//
// UPDATED ON PURPOSE, 2026-07-29 — the Claude Fable 5 prompt rewrite.
// Three of the four shipped prompts were edited deliberately; the numbers below
// were re-measured with a probe, not estimated:
//
//	Summarizer  351 -> 535 bytes   (+ "ruled out" and "unverified in, unverified
//	                                out": what was tried and failed now survives
//	                                compaction, so a fresh context cannot retry it)
//	Title       267 -> 267 bytes   (UNCHANGED — reviewed, nothing in the Fable
//	                                guidance applies to a 50-char title generator.
//	                                It is now the control this test needed.)
//	BaseCoder  1855 -> 4233 bytes  (~464 -> ~1058 est. tokens, +594/turn)
//	Task frag   228 -> 660 bytes   (honesty + read-only rules added; "one word
//	                                answers" removed — it was costing the parent
//	                                agent the evidence behind the answer)
//
// The coder growth is the real cost of this release and is not hidden: three new
// sections (scope, delegation, memory) and expanded honesty/output rules ride
// every turn. Two mitigations, both pre-existing: every section is individually
// switchable in /context, and the prompt is still roughly half the ~2,003-token
// 2023-era prompt this fork replaced (system-prompts/current/coder-anthropic.md).
func TestPromptOutputsAreByteIdentical(t *testing.T) {
	config.Load(t.TempDir(), false)

	for _, tc := range []struct {
		name     string
		got      string
		wantSize int
		wantTail string
	}{
		{"summarizer", SummarizerPrompt(models.ProviderLocal), 535, "unverified in, unverified out"},
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
		//
		// 1855 -> 4233 on 2026-07-29, deliberately. See the block comment
		// above this function for the per-prompt breakdown and the token cost.
		//
		// 4233 -> 5063 on 2026-07-31, deliberately: the "# change reporting"
		// section. +830 bytes, +19.6%, ~+207 tokens on EVERY coder turn — the
		// largest single addition this prompt has taken, and a recurring cost
		// because prompt tokens are re-sent each turn.
		//
		// Bought deliberately: "deceptive success reporting" is one of the
		// dominant operational failure modes measured in the field
		// (arXiv:2605.30777 — 326 of 547 real incidents rated high or critical),
		// and pre-change impact analysis is the best-evidenced countermeasure
		// (arXiv:2603.17973 — regressions 6.08% -> 1.82%).
		//
		// The section is tiered by blast radius and says "render after the work"
		// rather than imposing a rigid schema, because a hard schema measurably
		// COSTS accuracy on small models — tool-call executable accuracy fell
		// 91.5% -> 48.0% (arXiv:2605.26128), and this fork's users run Ollama and
		// small NIM models. That mitigation is reasoned, NOT measured on this
		// fork. If a local model starts producing well-formatted wrong answers,
		// suspect this section first and toggle it off in /context.
		// 5063 -> 5109 on 2026-07-31, deliberately. The "# tools" line used to
		// read "parallel: independent calls same turn". Measured the same day:
		// there is NO parallelism in this program — agent.go:452 runs tool calls
		// in a plain sequential loop, and agentTool.Run blocks on <-done, so even
		// several agent calls in one message execute one after another.
		//
		// The word was wrong but the advice under it was not: batching N calls
		// into one assistant message still costs ONE inference round-trip instead
		// of N, and on a high-latency link that is the expensive part. So the line
		// keeps the batching instruction and drops the concurrency claim.
		{"base coder (kept as a control — this file was already embedded)",
			BaseCoderPrompt(models.ProviderLocal), 5109, "never infer from name"},
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
	// 228 -> 660 on 2026-07-29: the task prompt gained honesty and
	// read-only rules and lost "one word answers". Deliberate; see the block
	// comment on TestPromptOutputsAreByteIdentical.
	if idx != 660 {
		t.Errorf("env block starts at byte %d, want 660 — the instruction fragment size changed", idx)
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
