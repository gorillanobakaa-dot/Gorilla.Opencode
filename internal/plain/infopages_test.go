package plain

// GORILLA (2026-10-10): /editor, /helpers and /hooks in plain mode print the
// same pages as the full interface, as ordinary selectable text. The command
// switch is driven directly; none of the three needs the app.

import (
	"bytes"
	"context"
	"runtime"
	"strings"
	"testing"

	"github.com/opencode-ai/opencode/internal/config"
	"github.com/opencode-ai/opencode/internal/helppages"
)

func TestPlainModeShowsTheThreePages(t *testing.T) {
	if _, err := config.Load(t.TempDir(), false); err != nil {
		t.Fatalf("config.Load: %v", err)
	}
	for typed, want := range map[string][]string{
		"/editor":  {strings.Join(helppages.ZedBlock, "\n"), "JetBrains"},
		"/acp":     {strings.Join(helppages.ZedBlock, "\n")},
		"/helpers": {"explore:", "plan:", "coder:", "use a plan helper to work out how to"},
		"/roles":   {"coder:"},
		"/hooks":   {"None. No hook is configured", config.GorillaConfigFile()},
	} {
		s, buf := newRenderer()
		stop, err := s.command(context.Background(), typed)
		if stop || err != nil {
			t.Fatalf("%s: stop=%v err=%v", typed, stop, err)
		}
		out := buf.String()
		if strings.Contains(out, "not available in plain mode") {
			t.Errorf("%s fell through to the not-available answer", typed)
		}
		for _, w := range want {
			if !strings.Contains(out, w) {
				t.Errorf("%s output does not contain %q:\n%s", typed, w, out)
			}
		}
		if bytes.IndexByte(buf.Bytes(), 0x1b) >= 0 {
			t.Errorf("%s printed a terminal escape; plain mode output must be copyable text", typed)
		}
	}
}

// The example /hooks prints is for the system it runs on.
func TestPlainHooksExampleIsForThisSystem(t *testing.T) {
	if _, err := config.Load(t.TempDir(), false); err != nil {
		t.Fatalf("config.Load: %v", err)
	}
	s, buf := newRenderer()
	if _, err := s.command(context.Background(), "/hooks"); err != nil {
		t.Fatal(err)
	}
	ps := strings.Contains(buf.String(), "$env:GORILLA_HOOK_TOOL_INPUT")
	if (runtime.GOOS == "windows") != ps {
		t.Errorf("running on %s, PowerShell example = %v", runtime.GOOS, ps)
	}
}

func TestPlainHelpListsTheThreePages(t *testing.T) {
	s, buf := newRenderer()
	s.help()
	for _, w := range []string{"/editor", "/helpers", "/hooks"} {
		if !strings.Contains(buf.String(), w) {
			t.Errorf("plain /help does not list %s", w)
		}
	}
}
