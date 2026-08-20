package chat

import (
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/opencode-ai/opencode/internal/config"
)

// The regression this guards: with streaming off, NOTHING arrives until the whole
// answer is done, so the pre-token phase is quiet for the entire generation. The
// original message calls that a probably-warming-up endpoint — which on a healthy
// fast link is simply wrong, fires every turn, and sends the user hunting a
// connection fault that does not exist.
func TestColdStartMessageDoesNotBlameTheLinkWhenNotStreaming(t *testing.T) {
	config.UseConnProfileForTest(t, config.ProfileModest) // streams
	streaming := coldStartMessage()
	if !strings.Contains(streaming, "warming up") {
		t.Errorf("streaming profile should keep the cold-endpoint wording: %q", streaming)
	}

	config.UseConnProfileForTest(t, config.ProfileAustere) // does not stream
	whole := coldStartMessage()
	if strings.Contains(whole, "warming up") {
		t.Error("non-streaming must not describe expected silence as a warming-up endpoint")
	}
	for _, want := range []string{"expected", "one piece", "/connection"} {
		if !strings.Contains(whole, want) {
			t.Errorf("non-streaming message should mention %q: %q", want, whole)
		}
	}
	if streaming == whole {
		t.Error("the two situations must not share one message")
	}
}

// The footer is ONE row and lipgloss wraps rather than truncates, so the hint has
// to stay short or it silently becomes a second row and breaks the frame.
func TestFooterHintStaysShortEnoughForOneRow(t *testing.T) {
	config.UseConnProfileForTest(t, config.ProfileAustere)
	m := &messagesCmp{}
	label := m.workingLabel("Generating...", 37*time.Second)
	if !strings.Contains(label, "one piece") {
		t.Errorf("footer should say how the answer arrives: %q", label)
	}
	if w := lipgloss.Width(label); w > 60 {
		t.Errorf("footer label is %d columns (%q); it must stay well under one row", w, label)
	}

	config.UseConnProfileForTest(t, config.ProfileModest)
	if got := m.workingLabel("Generating...", 37*time.Second); strings.Contains(got, "one piece") {
		t.Errorf("a streaming profile must not claim the answer arrives whole: %q", got)
	}
}

// The hint belongs only to the phases where nothing has arrived yet.
func TestHintOnlyOnPreTokenPhases(t *testing.T) {
	config.UseConnProfileForTest(t, config.ProfileAustere)
	m := &messagesCmp{}
	if got := m.workingLabel("Waiting for tool response...", 20*time.Second); strings.Contains(got, "one piece") {
		t.Errorf("tool phase should not carry the delivery hint: %q", got)
	}
}
