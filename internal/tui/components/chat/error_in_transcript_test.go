package chat

import (
	"strings"
	"testing"

	"github.com/opencode-ai/opencode/internal/message"
)

// The raw error as the provider gives it: a plain sentence, then the machine's
// own words. Both have to reach the transcript.
const providerFailure = `Llama 3.3 70B isn't enabled for your account (HTTP 404 — your key is fine). ` +
	`Pick another with /models.  ⟨POST "https://integrate.api.nvidia.com/v1/chat/completions": 404 Not Found ⟩`

// THE BUG: a failed turn left NOTHING in the transcript. The error went only to
// the status bar, which truncates at roughly 100 columns and is overwritten by
// the next message — so the evidence was gone before it could be read, and there
// was no way to scroll back to it or paste it into a bug report.
//
// Observed 2026-08-04: repeated 404s from NVIDIA showed as a red one-liner and
// nothing else; the transcript recorded the turns as having simply produced no
// answer.
func TestAFailedTurnPutsTheProvidersOwnWordsInTheTranscript(t *testing.T) {
	const at int64 = 1785228225

	failed := message.Message{
		ID: "a1", Role: message.Assistant, CreatedAt: at,
		Parts: []message.ContentPart{
			message.Finish{Reason: message.FinishReasonError, Details: providerFailure},
		},
	}

	m := printerFor(t, 120, failed)
	out := strings.Join(plainLines(m.printPending()), "\n")

	// The human translation.
	if !strings.Contains(out, "returned an error") {
		t.Errorf("the transcript does not explain that the turn failed:\n%s", out)
	}
	// AND the geeky detail behind it — the whole point of storing it.
	for _, want := range []string{"404", "integrate.api.nvidia.com"} {
		if !strings.Contains(out, want) {
			t.Errorf("the provider's own words are missing %q, so the transcript "+
				"explains the failure without evidencing it:\n%s", want, out)
		}
	}
}

// A failure with no detail must not render an empty code fence, which reads as
// though something was lost.
func TestAFailureWithoutDetailRendersNoEmptyBlock(t *testing.T) {
	const at int64 = 1785228225

	failed := message.Message{
		ID: "a1", Role: message.Assistant, CreatedAt: at,
		Parts: []message.ContentPart{
			message.Finish{Reason: message.FinishReasonError},
		},
	}

	m := printerFor(t, 120, failed)
	out := strings.Join(plainLines(m.printPending()), "\n")

	if !strings.Contains(out, "returned an error") {
		t.Fatalf("the failure is not reported at all:\n%s", out)
	}
	if strings.Contains(out, "```") {
		t.Errorf("an empty detail block was rendered, which looks like missing output:\n%s", out)
	}
}

// Details is variadic on AddFinish so every existing caller keeps working; a
// finish recorded without details must stay empty rather than pick up a stray.
func TestAddFinishKeepsWorkingWithoutDetails(t *testing.T) {
	var m message.Message
	m.AddFinish(message.FinishReasonEndTurn)

	for _, p := range m.Parts {
		f, ok := p.(message.Finish)
		if !ok {
			continue
		}
		if f.Reason != message.FinishReasonEndTurn {
			t.Errorf("reason came back as %q", f.Reason)
		}
		if f.Details != "" {
			t.Errorf("a finish recorded without details acquired some: %q", f.Details)
		}
		return
	}
	t.Fatal("AddFinish recorded no finish part at all")
}

// And with details, it round-trips them.
func TestAddFinishCarriesDetailsWhenGiven(t *testing.T) {
	var m message.Message
	m.AddFinish(message.FinishReasonError, providerFailure)

	for _, p := range m.Parts {
		if f, ok := p.(message.Finish); ok {
			if f.Details != providerFailure {
				t.Errorf("details did not round-trip:\ngot:  %q\nwant: %q", f.Details, providerFailure)
			}
			return
		}
	}
	t.Fatal("AddFinish recorded no finish part at all")
}
