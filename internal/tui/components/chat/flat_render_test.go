package chat

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/opencode-ai/opencode/internal/config"
	"github.com/opencode-ai/opencode/internal/message"
)

// bgSGR is the escape that sets a truecolor background. Its ABSENCE is the property
// under test, and it is checked directly rather than via a uniformity comparison:
// a row painted entirely in one colour is uniform but still painted.
const bgSGR = "48;2;"

// Outside the alternate screen no message may paint a background. A coloured slab
// sits on the terminal's own colour and reads as a floating widget rather than
// text — reported as "the ridiculous greys".
func TestMessagesPaintNoBackgroundOutsideTheAlternateScreen(t *testing.T) {
	if _, err := config.Load(t.TempDir(), false); err != nil {
		t.Fatalf("Load: %v", err)
	}
	if config.AlternateScreenEnabled() {
		t.Fatal("this test assumes the default, which is the alternate screen OFF")
	}

	const w = 76
	u := userMsgID("u1", "a question", 1785228225)
	a := finishedAssistant("a1", "An answer with `code` and a list:\n\n- one\n- two", 1785228229)
	msgs := []message.Message{u, a}

	for name, out := range map[string]string{
		"user":      RenderForScrollback(u, 0, msgs, nil, w),
		"assistant": RenderForScrollback(a, 1, msgs, nil, w),
	} {
		if strings.TrimSpace(out) == "" {
			t.Fatalf("%s rendered nothing, so the check below is vacuous", name)
		}
		if strings.Contains(out, bgSGR) {
			t.Errorf("%s message sets a background colour; outside the alternate screen "+
				"that paints a slab on top of the terminal's own colour", name)
		}
	}
}

// The two speakers must be distinguishable without any fill, because the fill is
// gone. Distinguished by TYPE rather than decoration, so it survives being copied
// into a text file — which a background colour does not.
func TestUserAndAssistantReadDifferently(t *testing.T) {
	if _, err := config.Load(t.TempDir(), false); err != nil {
		t.Fatalf("Load: %v", err)
	}

	const w = 76
	u := userMsgID("u1", "my question", 1785228225)
	a := finishedAssistant("a1", "my answer", 1785228229)
	msgs := []message.Message{u, a}

	user := RenderForScrollback(u, 0, msgs, nil, w)
	asst := RenderForScrollback(a, 1, msgs, nil, w)

	// The prefix is the part that survives copy-paste into a plain text file.
	if !strings.Contains(user, "> my question") {
		t.Errorf("the user's own words are not marked with a prefix, so a copied "+
			"transcript cannot show who said what:\n%q", user)
	}
	if strings.Contains(asst, "> my answer") {
		t.Errorf("the assistant's reply carries the user's prefix:\n%q", asst)
	}
	// And they must differ in styling, not only in prefix.
	if styleOf(user) == styleOf(asst) {
		t.Error("user and assistant text carry identical styling; on a copied-out " +
			"transcript the prefix is all that separates them, and on screen nothing does")
	}
}

// styleOf extracts the leading SGR sequence of a rendered block.
func styleOf(s string) string {
	i := strings.Index(s, "\x1b[")
	if i < 0 {
		return ""
	}
	j := strings.Index(s[i:], "m")
	if j < 0 {
		return ""
	}
	return s[i : i+j+1]
}

// With the alternate screen ON the program owns every cell, so the painted, bordered
// form must be preserved exactly as before.
func TestTheAlternateScreenKeepsItsPaintedPanels(t *testing.T) {
	if _, err := config.Load(t.TempDir(), false); err != nil {
		t.Fatalf("Load: %v", err)
	}
	if err := config.SetAlternateScreen(true); err != nil {
		t.Fatalf("SetAlternateScreen: %v", err)
	}
	t.Cleanup(func() { _ = config.SetAlternateScreen(false) })

	const w = 76
	u := userMsgID("u1", "a question", 1785228225)
	out := RenderForScrollback(u, 0, []message.Message{u}, nil, w)

	// The thick left rule, NOT merely "some background escape somewhere".
	//
	// An earlier version of this test checked for a background escape and passed
	// against a flat body, because the timestamp line underneath supplies one of its
	// own through BaseStyle. The border glyph belongs to the panel form alone, so it
	// distinguishes the two renderers rather than the two styles of one line.
	const leftRule = "┃"
	if !strings.Contains(out, leftRule) {
		t.Errorf("on the alternate screen the message has no left rule, so the flat "+
			"renderer is being used where the program owns the whole screen:\n%q", out)
	}
	if lipgloss.Height(out) == 0 {
		t.Error("rendered nothing")
	}
}

// And the mirror of it: outside the alternate screen there must be no rule at all.
// A border is a panel edge, and there is no panel — just text in a terminal.
func TestNoBorderRuleOutsideTheAlternateScreen(t *testing.T) {
	if _, err := config.Load(t.TempDir(), false); err != nil {
		t.Fatalf("Load: %v", err)
	}

	const w = 76
	u := userMsgID("u1", "a question", 1785228225)
	out := RenderForScrollback(u, 0, []message.Message{u}, nil, w)

	for _, rule := range []string{"┃", "│", "▌"} {
		if strings.Contains(out, rule) {
			t.Errorf("message draws a %q rule outside the alternate screen; that is a "+
				"panel edge around something that is not a panel:\n%q", rule, out)
		}
	}
}
