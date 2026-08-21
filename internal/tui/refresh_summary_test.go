// Version: 1.0.0 · updated 26-08-21-10-55
package tui

import (
	"strings"
	"testing"
)

// GORILLA OVERRIDE (2026-08-21): the /update summary must reach the transcript.
//
// It is one joined line of six or more notes — always wider than a terminal, so
// the footer shows the head and drops the tail. The tail is where "built-in
// providers unchanged" and the hidden-model count live, i.e. the part that
// stops silence being read as success. Echo prints the whole of it above the
// frame; without it the report exists only as a truncation.
func TestRefreshSummaryEchoesToTranscript(t *testing.T) {
	t.Parallel()
	notes := []string{
		"OpenRouter 289 usable (+0, -0)",
		"Antigravity 20 usable",
		"2 configured endpoint(s) re-asked",
		"built-in providers unchanged (they update with the app)",
		"3 hidden stayed hidden (H to review)",
	}
	msg := refreshSummaryMsg(notes)
	if !msg.Echo {
		t.Error("refresh summary does not echo; the footer will cut it and the tail is lost")
	}
	for _, n := range notes {
		if !strings.Contains(msg.Msg, n) {
			t.Errorf("note dropped from the summary: %q", n)
		}
	}
	if len(msg.Msg) < 100 {
		t.Fatalf("summary unexpectedly short (%d cols) — check the join: %q", len(msg.Msg), msg.Msg)
	}
}
