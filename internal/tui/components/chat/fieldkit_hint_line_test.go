package chat

import (
	"strings"
	"testing"

	"github.com/opencode-ai/opencode/internal/fieldkit"
	"github.com/opencode-ai/opencode/internal/message"
)

// 2026-10-19: the hint reached the model but was never drawn; the owner could
// not see what the model had been shown.
func TestTheFieldkitHintIsDrawnAsOneLine(t *testing.T) {
	c, err := fieldkit.Load()
	if err != nil {
		t.Fatal(err)
	}
	q := "Is my laptop's TV connected over HDMI right now?"
	msg := message.Message{Role: message.User, Parts: []message.ContentPart{
		message.TextContent{Text: q},
		message.TextContent{Text: fieldkit.Hint(c, q, 5)},
	}}
	got := fieldkitHintLine(msg)
	want := "Fieldkit tools offered to the AI: hdmi-status, "
	if len(got) < len(want) || got[:len(want)] != want {
		t.Fatalf("line %q, want it to start %q", got, want)
	}
	// 2026-10-20: a recipe offered beside the tools is marked as one
	r := "build a Go program for an old Mac on High Sierra 10.13"
	withRecipe := message.Message{Role: message.User, Parts: []message.ContentPart{
		message.TextContent{Text: r},
		message.TextContent{Text: fieldkit.Hint(c, r, 5)},
	}}
	if l := fieldkitHintLine(withRecipe); !strings.Contains(l, "go-binary-for-macos-10-13 (recipe)") {
		t.Errorf("the recipe is not marked in %q", l)
	}
	plain := message.Message{Role: message.User, Parts: []message.ContentPart{message.TextContent{Text: q}}}
	if l := fieldkitHintLine(plain); l != "" {
		t.Errorf("a message with no hint drew %q", l)
	}
	// the person's own words are never mistaken for a hint, whatever they say
	own := message.Message{Role: message.User, Parts: []message.ContentPart{
		message.TextContent{Text: "[Gorilla: Fieldkit tools that match\n- fake [x] y"}}}
	if l := fieldkitHintLine(own); l != "" {
		t.Errorf("the person's own text was drawn as a hint: %q", l)
	}
}
