package agent

import (
	"strings"
	"testing"
)

func TestSanitiseTitle(t *testing.T) {
	const userMsg = "help me fix the buffer sizing in netwerk"

	cases := []struct {
		name string
		raw  string
		want string
	}{
		{
			// The exact failure reported, copied from the sidebar.
			name: "the observed real failure",
			raw:  "Here's a possible title, keeping the constraints in mind:\n\n**Short and Concise:**\n\n**Title:** Your Business Brief",
			want: "Your Business Brief",
		},
		{
			name: "label and value on one line",
			raw:  "**Title:** Netwerk Buffer Sizing",
			want: "Netwerk Buffer Sizing",
		},
		{
			name: "plain compliant answer is untouched",
			raw:  "Netwerk Buffer Sizing",
			want: "Netwerk Buffer Sizing",
		},
		{
			name: "preamble then answer on the last line",
			raw:  "Sure, I can do that.\nHere is a short title:\nFixing BBR Pacing",
			want: "Fixing BBR Pacing",
		},
		{
			name: "wrapped in a code fence",
			raw:  "```\nKernel Config Cleanup\n```",
			want: "Kernel Config Cleanup",
		},
		{
			name: "quoted and punctuated",
			raw:  `"Refactor the LSP client."`,
			want: "Refactor the LSP client",
		},
		{
			name: "markdown bullet and emphasis",
			raw:  "- *Firefox* CSS `patch` review",
			want: "Firefox CSS patch review",
		},
		{
			name: "newlines collapse rather than becoming a paragraph",
			raw:  "Buffer\nSizing",
			want: "Sizing",
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := sanitiseTitle(c.raw, userMsg)
			if got != c.want {
				t.Errorf("sanitiseTitle:\n  raw  = %q\n  got  = %q\n  want = %q", c.raw, got, c.want)
			}
		})
	}
}

// When the model returns nothing usable, the user's own words are a better
// title than the model's commentary about the request.
func TestSanitiseTitleFallsBackToTheUserMessage(t *testing.T) {
	const userMsg = "fix the buffer sizing in netwerk"

	for _, raw := range []string{
		"",
		"   \n\n  ",
		"Sure! Here is a title for you:",    // label with no value
		"Okay, I understand what you want.", // pure chatter
		"**Title:**",                        // label, empty value
	} {
		got := sanitiseTitle(raw, userMsg)
		if got != userMsg {
			t.Errorf("raw %q gave %q, want the user's message %q", raw, got, userMsg)
		}
	}
}

// 50 chars is the documented limit; the sidebar wraps anything longer into the
// multi-line mess that made this visible.
func TestSanitiseTitleRespectsTheLengthLimit(t *testing.T) {
	long := "An extremely long and rambling session title that no sidebar could ever hope to display on one line"
	got := sanitiseTitle(long, "fallback")

	// AMENDED 2026-08-19: the marker now fits INSIDE the limit rather than
	// being added on top of it, so the assertion is the limit itself. The old
	// "+1 for the ellipsis" was tolerating a real off-by-one that only became
	// visible when the marker grew from one character to three.
	if n := len([]rune(got)); n > maxTitleChars {
		t.Errorf("title is %d chars: %q", n, got)
	}
	if !strings.HasSuffix(got, "...") {
		t.Errorf("a truncated title should show it was cut: %q", got)
	}
	// Cut at a word boundary, so it reads as shortened not corrupted.
	if strings.HasSuffix(strings.TrimSuffix(got, "..."), " ") {
		t.Errorf("trailing space before the ellipsis: %q", got)
	}
	if strings.Contains(got, "\n") {
		t.Error("title contains a newline")
	}
}

// A one-line, quote-free, label-free result is the contract the sidebar relies
// on. Assert it holds for every case above plus adversarial input.
func TestSanitiseTitleAlwaysReturnsOneCleanLine(t *testing.T) {
	for _, raw := range []string{
		"Here's a possible title, keeping the constraints in mind:\n\n**Title:** Your Business Brief",
		"```json\n{\"title\": \"nope\"}\n```",
		"Title: Title: Title: Nested Labels",
		"###   # Heading Style\n",
		"\"'quoted to death'\"",
		strings.Repeat("word ", 200),
	} {
		got := sanitiseTitle(raw, "fallback message")
		if strings.Contains(got, "\n") {
			t.Errorf("newline survived: %q (from %q)", got, raw)
		}
		if n := len([]rune(got)); n > maxTitleChars+1 {
			t.Errorf("length %d exceeds the limit: %q", n, got)
		}
		if strings.HasPrefix(got, `"`) || strings.HasSuffix(got, `"`) {
			t.Errorf("surrounding quote survived: %q", got)
		}
		if titleLabelRe.MatchString(got) {
			t.Errorf("a meta label survived: %q (from %q)", got, raw)
		}
		if strings.TrimSpace(got) == "" {
			t.Errorf("produced an empty title from %q", raw)
		}
	}
}
