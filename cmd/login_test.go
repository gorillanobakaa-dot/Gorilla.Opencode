package cmd

import (
	"bufio"
	"strings"
	"testing"

	"github.com/opencode-ai/opencode/internal/llm/models"
)

// These tests deliberately exercise ONLY pure selection logic. They never call
// persistProviderKey, so they cannot write the developer's real
// ~/.config/gorilla-opencode/config.json — which is exactly what happened while
// probing this flow with a built binary and piped stdin.

// The whole point of the rewrite: every provider must be reachable, not just
// Google. A regression that dropped the API-key rows would leave the menu
// looking fine while silently offering only Google again.
func TestLoginOptionsCoverEveryProvider(t *testing.T) {
	want := []models.ModelProvider{
		models.ProviderAnthropic,
		models.ProviderOpenAI,
		models.ProviderGemini,
		models.ProviderGROQ,
		models.ProviderCerebras,
		models.ProviderXAI,
		models.ProviderOpenRouter,
	}
	got := map[models.ModelProvider]bool{}
	for _, o := range loginOptions {
		if o.provider != "" {
			got[o.provider] = true
		}
	}
	for _, p := range want {
		if !got[p] {
			t.Errorf("provider %q has no login option — users cannot configure it from the CLI", p)
		}
	}

	// Exactly one auto-free-tier Google row and exactly one GCP-project row.
	var oauth, gcp int
	for _, o := range loginOptions {
		switch {
		case o.gcpPath:
			gcp++
		case o.provider == "":
			oauth++
		}
	}
	if oauth != 1 {
		t.Errorf("found %d plain Google OAuth rows, want exactly 1", oauth)
	}
	if gcp != 1 {
		t.Errorf("found %d GCP-project rows, want exactly 1", gcp)
	}
}

// Every API-key row must carry an envHint. Without it the user is never told
// they could have exported a variable instead, and runAPIKeyLogin loses its
// "already in your environment" shortcut.
func TestEveryAPIKeyOptionHasAnEnvHint(t *testing.T) {
	for _, o := range loginOptions {
		if o.provider == "" || o.gcpPath {
			continue
		}
		if o.envHint == "" {
			t.Errorf("option %q (provider %q) has no envHint", o.label, o.provider)
		}
		if !strings.HasSuffix(o.envHint, "_API_KEY") {
			t.Errorf("option %q envHint %q does not look like an API-key variable", o.label, o.envHint)
		}
	}
}

// Menu input handling. Out-of-range, empty and garbage must all fall back to
// option 1 (Google OAuth) rather than panicking on a bad index.
func TestSelectedIndexBoundaries(t *testing.T) {
	n := len(loginOptions)
	for _, tc := range []struct {
		raw  string
		want int
	}{
		{"", 0},       // bare enter -> default
		{"1", 0},      // first
		{"5", 4},      // middle
		{"9", 8},      // last, given 9 options
		{"0", 0},      // below range -> default
		{"999", 0},    // above range -> default
		{"-3", 0},     // negative -> default
		{"banana", 0}, // unparseable -> default
		{"  7  ", 6},  // already trimmed by caller, but tolerate
	} {
		t.Run(tc.raw, func(t *testing.T) {
			if got := selectedIndex(strings.TrimSpace(tc.raw), n); got != tc.want {
				t.Errorf("selectedIndex(%q, %d) = %d, want %d", tc.raw, n, got, tc.want)
			}
		})
	}

	// A zero-length menu must not index out of bounds.
	if got := selectedIndex("3", 0); got != 0 {
		t.Errorf("selectedIndex on an empty menu = %d, want 0", got)
	}
}

// pickLoginOption must read the choice AND, for the GCP row, the follow-up
// project id from the SAME reader. Two readers would lose the second line to
// the first one's buffer — the bug that made the piped probe report EOF.
func TestPickLoginOptionReadsBothLinesFromOneReader(t *testing.T) {
	opts := []loginOption{
		{label: "google-oauth"},
		{label: "groq", provider: models.ProviderGROQ, envHint: "GROQ_API_KEY"},
		{label: "gcp", gcpPath: true},
	}

	t.Run("api-key row consumes only the choice", func(t *testing.T) {
		r := bufio.NewReader(strings.NewReader("2\nsk-the-key\n"))
		chosen, gcp := pickLoginOption(r, opts)
		if chosen.provider != models.ProviderGROQ {
			t.Fatalf("chose %q, want groq", chosen.provider)
		}
		if gcp != "" {
			t.Errorf("gcp project = %q, want empty for a non-GCP row", gcp)
		}
		// The key line must still be available to runAPIKeyLogin.
		rest, err := r.ReadString('\n')
		if err != nil {
			t.Fatalf("key line was swallowed: %v", err)
		}
		if strings.TrimSpace(rest) != "sk-the-key" {
			t.Errorf("next line = %q, want %q", strings.TrimSpace(rest), "sk-the-key")
		}
	})

	t.Run("gcp row consumes the project id too", func(t *testing.T) {
		r := bufio.NewReader(strings.NewReader("3\nmy-project-123\n"))
		chosen, gcp := pickLoginOption(r, opts)
		if !chosen.gcpPath {
			t.Fatalf("chose %q, want the gcp row", chosen.label)
		}
		if gcp != "my-project-123" {
			t.Errorf("gcp project = %q, want %q", gcp, "my-project-123")
		}
	})

	t.Run("bare enter selects google oauth", func(t *testing.T) {
		r := bufio.NewReader(strings.NewReader("\n"))
		chosen, _ := pickLoginOption(r, opts)
		if chosen.provider != "" || chosen.gcpPath {
			t.Errorf("bare enter chose %q, want the plain Google OAuth row", chosen.label)
		}
	})
}
