package prompt

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/opencode-ai/opencode/internal/config"
)

// isolatePrompts points the override directory at a temp dir so these tests
// cannot touch the developer's real ~/.config/gorilla-opencode/prompts.
func isolatePrompts(t *testing.T) {
	t.Helper()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	if _, err := config.Load(t.TempDir(), false); err != nil {
		t.Fatalf("config.Load: %v", err)
	}
	// Drop any cached override state from an earlier test in this binary.
	for _, id := range AllPromptIDs {
		invalidateOverride(id)
	}
	t.Cleanup(func() {
		for _, id := range AllPromptIDs {
			invalidateOverride(id)
		}
	})
}

func TestTextFallsBackToFactoryWithNoOverride(t *testing.T) {
	isolatePrompts(t)
	for _, id := range AllPromptIDs {
		if got, want := Text(id), Factory(id); got != want {
			t.Errorf("%s: Text != Factory with no override (%d vs %d bytes)", id, len(got), len(want))
		}
		if IsOverridden(id) {
			t.Errorf("%s: reported as overridden with no file on disk", id)
		}
	}
}

func TestSaveAndResetOverrideRoundTrip(t *testing.T) {
	isolatePrompts(t)
	const body = "you are a deliberately different agent\n\n# rule\n- be terse\n"

	if err := SaveOverride(PromptCoder, body); err != nil {
		t.Fatalf("SaveOverride: %v", err)
	}
	if got := Text(PromptCoder); got != strings.TrimSpace(body) {
		t.Errorf("Text after save = %q, want the override", got)
	}
	if !IsOverridden(PromptCoder) {
		t.Error("IsOverridden false after saving — the /prompts dialog would not show EDITED and a tampered prompt would be invisible")
	}

	// The override file must not be world-readable; it lives beside API keys.
	info, err := os.Stat(OverridePath(PromptCoder))
	if err != nil {
		t.Fatalf("stat override: %v", err)
	}
	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Errorf("override mode = %04o, want 0600", perm)
	}

	if err := ResetPrompt(PromptCoder); err != nil {
		t.Fatalf("ResetPrompt: %v", err)
	}
	if got, want := Text(PromptCoder), Factory(PromptCoder); got != want {
		t.Errorf("reset did not restore the factory prompt (%d vs %d bytes)", len(got), len(want))
	}
	if IsOverridden(PromptCoder) {
		t.Error("still reported as overridden after reset")
	}

	// Resetting again must be a no-op, not an error: the caller asked for "no
	// override" and that is the state either way.
	if err := ResetPrompt(PromptCoder); err != nil {
		t.Errorf("second ResetPrompt returned %v, want nil", err)
	}
}

// A blank override is the nastiest failure mode: an empty system prompt does not
// error, it silently produces a much worse agent. Refuse to save one, and ignore
// one that somehow exists on disk.
func TestBlankOverrideIsRefusedAndIgnored(t *testing.T) {
	isolatePrompts(t)

	for _, blank := range []string{"", "   ", "\n\n\t\n"} {
		if err := SaveOverride(PromptCoder, blank); err == nil {
			t.Errorf("SaveOverride(%q) succeeded; a blank prompt must be refused", blank)
		}
	}

	// Simulate a blank file arriving by other means (hand-edited, truncated).
	dir := filepath.Dir(OverridePath(PromptCoder))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(OverridePath(PromptCoder), []byte("\n  \n"), 0o600); err != nil {
		t.Fatal(err)
	}
	invalidateOverride(PromptCoder)

	if got, want := Text(PromptCoder), Factory(PromptCoder); got != want {
		t.Errorf("a blank override on disk was used instead of the factory default")
	}
}

// Editing the coder prompt must change the section set the /context menu offers,
// or a user who adds a section gets no toggle for it.
func TestOverrideChangesTheSectionSet(t *testing.T) {
	isolatePrompts(t)

	before := len(CoderSections())

	if err := SaveOverride(PromptCoder, "identity line\n\n# alpha\n- one\n\n# beta\n- two\n"); err != nil {
		t.Fatalf("SaveOverride: %v", err)
	}
	secs := CoderSections()
	if len(secs) != 3 { // preamble + alpha + beta
		t.Fatalf("got %d sections after override, want 3: %+v", len(secs), secs)
	}
	var headers []string
	for _, s := range secs[1:] {
		headers = append(headers, s.Header)
	}
	if strings.Join(headers, ",") != "alpha,beta" {
		t.Errorf("headers = %v, want [alpha beta]", headers)
	}

	if err := ResetPrompt(PromptCoder); err != nil {
		t.Fatal(err)
	}
	if got := len(CoderSections()); got != before {
		t.Errorf("section count after reset = %d, want the original %d", got, before)
	}
}

func TestEveryPromptIDHasADisplayName(t *testing.T) {
	for _, id := range AllPromptIDs {
		name, ok := PromptDisplayName[id]
		if !ok || strings.TrimSpace(name) == "" {
			t.Errorf("prompt %q has no display name", id)
		}
		// The name should say WHEN it runs, or the user edits blind.
		if !strings.Contains(name, "—") {
			t.Errorf("display name for %q (%q) does not explain when it runs", id, name)
		}
	}
}

func TestFactoryIsNeverEmpty(t *testing.T) {
	for _, id := range AllPromptIDs {
		if strings.TrimSpace(Factory(id)) == "" {
			t.Errorf("factory prompt for %q is empty — reset would produce a blank prompt", id)
		}
	}
}
