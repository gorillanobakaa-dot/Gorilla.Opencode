package cmd

import (
	"os"
	"strings"
	"testing"

	"github.com/opencode-ai/opencode/internal/auth"
)

// 2026-10-19: Google refuses the data-collection opt-out for this sign-in
// (403), so every user, many of them children, is told at sign-in what happens
// to what they type. The warning must say the four things that matter and must
// be printed BEFORE setup, so a failed setup does not skip it.
func TestAntigravitySignInWarnsWhatGoogleDoesWithTheData(t *testing.T) {
	w := strings.ToLower(auth.AntigravityDataWarning)
	for _, must := range []string{"you are the product", "train", "people at google may read",
		"18 months", "cannot switch that off", "never type passwords"} {
		if !strings.Contains(w, must) {
			t.Errorf("the sign-in warning no longer says %q", must)
		}
	}
	src, err := os.ReadFile("provider_portal.go")
	if err != nil {
		t.Fatal(err)
	}
	s := string(src)
	start := strings.Index(s, "func runAntigravityLogin(")
	if start < 0 {
		t.Fatal("runAntigravityLogin not found")
	}
	body := s[start:]
	body = body[:strings.Index(body, "\n}\n")]
	warn, setup := strings.Index(body, "auth.AntigravityDataWarning"), strings.Index(body, "creds.SetupProject(ctx)")
	if warn < 0 || setup < 0 || warn > setup {
		t.Fatalf("the warning must be printed at sign-in, before setup (warning at %d, setup at %d)", warn, setup)
	}
}
