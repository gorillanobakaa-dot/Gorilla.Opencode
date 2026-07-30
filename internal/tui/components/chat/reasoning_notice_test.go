package chat

import (
	"strings"
	"sync"
	"testing"

	"github.com/opencode-ai/opencode/internal/config"
)

// resetNotice lets each case start fresh; the notice is once-per-run by design.
func resetNotice() { reasoningOffNoticed = sync.Once{} }

// show ON + generate OFF produces nothing, forever, and used to say nothing
// about why. That combination cost a real diagnosis: reasoning appeared for
// Nemotron (which reasons by default) and vanished for DeepSeek and GLM (which
// do not), which looked exactly like a display bug and was not.
func TestNoticeAppearsWhenShowIsOnButGenerateIsOff(t *testing.T) {
	resetNotice()
	config.SetExtra("extras-reasoning-show", true)
	config.SetExtra("extras-reasoning-generate", false)

	lines := plainLines(reasoningSwitchedOffNotice())
	if len(lines) == 0 {
		t.Fatal("no notice: the user sees no thinking and is told nothing")
	}
	joined := strings.Join(lines, " ")
	for _, want := range []string{"think out loud", "/context"} {
		if !strings.Contains(joined, want) {
			t.Errorf("notice does not mention %q, so it does not say how to fix it:\n%s",
				want, joined)
		}
	}
}

// Once per run, not once per message. A line repeated on every turn is noise,
// and noise is ignored.
func TestNoticeIsPrintedOnlyOnce(t *testing.T) {
	resetNotice()
	config.SetExtra("extras-reasoning-show", true)
	config.SetExtra("extras-reasoning-generate", false)

	if n := len(reasoningSwitchedOffNotice()); n == 0 {
		t.Fatal("first call printed nothing")
	}
	if n := len(reasoningSwitchedOffNotice()); n != 0 {
		t.Errorf("printed again on the second call (%d cmds); it would repeat on "+
			"every single turn", n)
	}
}

// No notice when the combination is coherent: either the user is not asking to
// see thinking, or thinking is being generated and the absence is the model's
// doing rather than a setting.
func TestNoNoticeWhenTheSettingsAreCoherent(t *testing.T) {
	cases := []struct{ show, generate bool }{
		{show: false, generate: false},
		{show: false, generate: true},
		{show: true, generate: true},
	}
	for _, c := range cases {
		resetNotice()
		config.SetExtra("extras-reasoning-show", c.show)
		config.SetExtra("extras-reasoning-generate", c.generate)
		if n := len(reasoningSwitchedOffNotice()); n != 0 {
			t.Errorf("show=%v generate=%v produced a notice it should not", c.show, c.generate)
		}
	}
}
