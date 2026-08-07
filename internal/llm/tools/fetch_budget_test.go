package tools

import "testing"

// GORILLA OVERRIDE: the budget must be a signal before it is a limit. These
// pin the three regimes against real measured sizes, so a future "let's just
// lower the cap" cannot silently start truncating research.
func TestTokenBudgetRegimes(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name   string
		tokens int
		want   string // "quiet" | "warn" | "cut"
	}{
		{"arXiv API record", 734, "quiet"},
		{"a README", 363, "quiet"},
		{"converted arXiv abs page", 10744, "quiet"}, // real research must pass clean
		{"a long documentation page", 20000, "warn"},
		{"Romeo and Juliet, whole book", 42376, "cut"}, // legitimate, but must be flagged
		{"researchsquare article page", 84772, "cut"},  // the observed failure
	}
	for _, c := range cases {
		var got string
		switch {
		case c.tokens > maxTokens:
			got = "cut"
		case c.tokens > warnTokens:
			got = "warn"
		default:
			got = "quiet"
		}
		if got != c.want {
			t.Errorf("%s (~%d tok): got %q, want %q", c.name, c.tokens, got, c.want)
		}
	}
	// The guarantee that matters: a full abstract page is never truncated.
	if 10744 > maxTokens {
		t.Fatal("budget would truncate a converted arXiv abstract page — too low to do research")
	}
	// And the observed failure is caught.
	if 84772 <= maxTokens {
		t.Fatal("budget would pass the 85k-token fetch that took a session to 88% context")
	}
}
