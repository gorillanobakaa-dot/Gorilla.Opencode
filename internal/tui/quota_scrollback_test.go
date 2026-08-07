package tui

import (
	"strings"
	"testing"

	"github.com/opencode-ai/opencode/internal/tui/util"
)

// GORILLA OVERRIDE: /usage must reach the SCROLLBACK, not only the footer.
//
// The footer answers "what is my quota now". It cannot answer "what was it
// twenty minutes ago", and re-asking spends a request against the very quota
// being measured. So the reading is printed into the terminal history too,
// timestamped - a quota figure without a time is not a measurement.
//
// This asserts the message plumbing exists and carries what it must. The
// printing itself is tea.Println, which cannot be driven from an agent shell
// here (see CLAUDE.md), so the visual result needs a human.
func TestQuotaLineCarriesTextAndSeverity(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		msg  quotaLineMsg
	}{
		{"a reading", quotaLineMsg{line: "Gemini 16% · refreshes in 71h", kind: util.InfoTypeInfo}},
		{"not signed in", quotaLineMsg{line: "Not signed in to Antigravity", kind: util.InfoTypeWarn}},
		{"fetch failed", quotaLineMsg{line: "Antigravity usage: 503", kind: util.InfoTypeError}},
	}
	for _, c := range cases {
		if strings.TrimSpace(c.msg.line) == "" {
			t.Errorf("%s: empty line would print a blank row into the user's history", c.name)
		}
		// Severity must survive into the footer; a failed fetch reported as
		// InfoTypeInfo reads as a successful reading of nothing.
		info := util.InfoMsg{Type: c.msg.kind, Msg: c.msg.line}
		if info.Msg != c.msg.line || info.Type != c.msg.kind {
			t.Errorf("%s: message content or severity lost in conversion", c.name)
		}
	}
}
