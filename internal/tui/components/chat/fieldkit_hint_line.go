package chat

import (
	"strings"

	"github.com/opencode-ai/opencode/internal/message"
)

// fieldkitHintPrefix opens the hint block internal/fieldkit.Hint writes.
const fieldkitHintPrefix = "[Gorilla: Fieldkit tools"

// fieldkitHintLine is the one line drawn under a user message that carried a
// Fieldkit hint: the tools offered to the model, by exact name. "" when the
// message has no hint.
func fieldkitHintLine(msg message.Message) string {
	first := true
	for _, p := range msg.Parts {
		tc, ok := p.(message.TextContent)
		if !ok {
			continue
		}
		if first { // the person's own words
			first = false
			continue
		}
		if !strings.HasPrefix(tc.Text, fieldkitHintPrefix) {
			continue
		}
		var ids []string
		for _, l := range strings.Split(tc.Text, "\n") {
			if strings.HasPrefix(l, "- ") {
				if f := strings.Fields(l[2:]); len(f) > 0 {
					ids = append(ids, f[0])
				}
			}
		}
		if len(ids) == 0 {
			return ""
		}
		return "Fieldkit tools offered to the AI: " + strings.Join(ids, ", ")
	}
	return ""
}
