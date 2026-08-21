// Version: 1.0.0 · updated 26-08-21-15-30
package tui

import (
	"strings"
	"testing"

	"github.com/opencode-ai/opencode/internal/tui/util"
)

// GORILLA FIX (2026-08-21): the provider portal must move the RUNNING agent, not
// only the config file.
//
// It used to write config.UpdateAgentModel and stop. The footer reads config, so
// it said "Claude Sonnet 4.6 (Antigravity free)" immediately, while the live
// agent kept the provider it was constructed with and went on answering from
// NVIDIA NIM's Llama 3.3 70B. The user was spending the wrong quota under a
// label that said otherwise, and the only honest thing on screen was the
// per-message model name in the transcript.
//
// With no app wired up — the state this test can construct — the function must
// still degrade to a plain, TRUE statement rather than claiming a switch it did
// not perform.
func TestPortalAdoptionNeverClaimsASwitchItDidNotMake(t *testing.T) {
	a := appModel{} // no app, no agent: nothing can have been switched
	msg, ok := a.adoptPortalModel().(util.InfoMsg)
	if !ok {
		t.Fatalf("expected an InfoMsg, got %T", a.adoptPortalModel())
	}
	if strings.Contains(strings.ToLower(msg.Msg), "now answering as") {
		t.Errorf("claims a model switch with no agent to switch: %q", msg.Msg)
	}
	if msg.Type == util.InfoTypeError {
		t.Errorf("a portal run with nothing to adopt is not an error: %q", msg.Msg)
	}
	if strings.TrimSpace(msg.Msg) == "" {
		t.Error("said nothing at all; the user has no way to know the portal did anything")
	}
}

// The old wording told the user to go and pick a model, which is what made the
// mismatch survivable-looking: it implied the switch was still pending when the
// footer already claimed it had happened.
func TestPortalMessageDoesNotDeferTheSwitchToTheUser(t *testing.T) {
	a := appModel{}
	msg := a.adoptPortalModel().(util.InfoMsg)
	if strings.Contains(msg.Msg, "use /models if you want") {
		t.Error("still the old wording, which implied the switch had not happened yet")
	}
}
