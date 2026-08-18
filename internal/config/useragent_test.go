package config

// GORILLA OVERRIDE (2026-08-18): the browser User-Agent is a decision with a
// switch, so the switch is tested. The default identifies as a browser (that is
// the whole point — the honest token was measured getting blocked); the env var
// overrides it; and the literal "honest" restores the original token for anyone
// who wants it back.

import (
	"os"
	"strings"
	"testing"
)

func TestBrowserUserAgentDefaultsToABrowser(t *testing.T) {
	os.Unsetenv("GORILLA_OPENCODE_USER_AGENT")
	ua := BrowserUserAgent()
	if !strings.Contains(ua, "Mozilla") || !strings.Contains(ua, "Firefox") {
		t.Errorf("default UA is not a browser token, which is the entire fix: %q", ua)
	}
	if strings.Contains(ua, "gorilla-opencode") {
		t.Errorf("the default still wears the bot badge that was measured getting blocked: %q", ua)
	}
}

func TestUserAgentEnvOverrides(t *testing.T) {
	t.Setenv("GORILLA_OPENCODE_USER_AGENT", "MyCustomAgent/9")
	if got := BrowserUserAgent(); got != "MyCustomAgent/9" {
		t.Errorf("env override ignored: %q", got)
	}
}

func TestHonestRestoresTheIdentifyingToken(t *testing.T) {
	t.Setenv("GORILLA_OPENCODE_USER_AGENT", "honest")
	got := BrowserUserAgent()
	if !strings.Contains(got, "gorilla-opencode") {
		t.Errorf(`"honest" did not restore the identifying token: %q`, got)
	}
	if strings.Contains(got, "Mozilla") {
		t.Errorf(`"honest" still returned a browser token: %q`, got)
	}
}
