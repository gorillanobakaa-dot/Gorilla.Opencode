package chat

import (
	"strings"
	"sync"
	"testing"

	"github.com/charmbracelet/x/ansi"
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

	lines := plainLines(reasoningSwitchedOffNotice(0))
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

	if n := len(reasoningSwitchedOffNotice(0)); n == 0 {
		t.Fatal("first call printed nothing")
	}
	if n := len(reasoningSwitchedOffNotice(0)); n != 0 {
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
		if n := len(reasoningSwitchedOffNotice(0)); n != 0 {
			t.Errorf("show=%v generate=%v produced a notice it should not", c.show, c.generate)
		}
	}
}

// MacBook Air, 2026-10-23 (air-2026-10-23-024915-main-be058ea.png, 169
// columns): the notice was one 230-column line and ran past the right edge,
// cut at "Som". It is wrapped to the width it is given, and nothing is lost.
func TestNoticeFitsTheWindow(t *testing.T) {
	resetNotice()
	config.SetExtra("extras-reasoning-show", true)
	config.SetExtra("extras-reasoning-generate", false)

	const width = 168 // 169 columns less the last one
	lines := plainLines(reasoningSwitchedOffNotice(width))
	if len(lines) < 2 {
		t.Fatalf("notice is %d line(s) at width %d; it is longer than that, so it was not wrapped: %q", len(lines), width, lines)
	}
	for _, l := range lines {
		l = strings.TrimPrefix(l, "\r")
		if w := ansi.StringWidth(l); w > width {
			t.Errorf("line is %d columns, wider than %d: %q", w, width, l)
		}
	}
	if joined := strings.Join(lines, " "); !strings.Contains(joined, "most do not.") {
		t.Errorf("the end of the notice is missing:\n%s", joined)
	}
}
