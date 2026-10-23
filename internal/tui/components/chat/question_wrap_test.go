// Version: 1.0.0 · updated 26-10-23-02-46
package chat

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
	"github.com/opencode-ai/opencode/internal/message"
)

// MacBook Air, 2026-10-23 (air-2026-10-23-015446.png, 169 columns): a pasted
// question was printed cut at whatever character met the edge of the window,
// "su" / "do" and "questio" / "n". Every ordinary word must stay on one line,
// and no line may be wider than the width it was given.
func TestALongQuestionWrapsAtWordsNotMidWord(t *testing.T) {
	q := "I'm a new user on this Mac and I'm not a programmer. I want to install Fieldkit and start " +
		"using it. Its page is https://github.com/gorillanobakaa-dot/Gorilla.Fieldkit/tree/feat/stranger-install " +
		"(use that branch, it is the newest). Rules: Work ONLY inside the folder ~/Stranger-Test. Do not " +
		"touch anything outside it. Do NOT use sudo and do NOT install anything system-wide. Whenever " +
		"something needs my permission, an admin password, or a system-wide install, do not do it: write " +
		"the exact question you would ask me (and why) into ~/Stranger-Test/QUESTIONS.txt, then carry on."
	msg := message.Message{Role: message.User, Parts: []message.ContentPart{message.TextContent{Text: q}}}
	for _, width := range []int{166, 168, 80} {
		out := RenderForScrollback(msg, 0, []message.Message{msg}, nil, width)
		lines := strings.Split(ansi.Strip(out), "\n")
		if len(lines) < 2 {
			t.Fatalf("width %d: the question fitted on one line, so nothing here tests wrapping", width)
		}
		words := map[string]bool{}
		for _, l := range lines {
			if w := ansi.StringWidth(l); w > width {
				t.Errorf("width %d: line is %d columns: %q", width, w, l)
			}
			for _, f := range strings.Fields(l) {
				words[f] = true
			}
		}
		for _, f := range strings.Fields(q) {
			if strings.ContainsAny(f, "-/") {
				continue // a URL or path may break at its own punctuation
			}
			if !words[f] {
				t.Errorf("width %d: %q was split across two lines:\n%s", width, f, strings.Join(lines, "\n"))
			}
		}
	}
}
