package auth

import (
	"os"
	"strings"
	"testing"
)

func TestEffectiveAntigravityUserAgent_Default(t *testing.T) {
	os.Unsetenv("ANTIGRAVITY_USER_AGENT")
	os.Unsetenv("ANTIGRAVITY_VERSION")

	ua := EffectiveAntigravityUserAgent()
	expectedVersion := "2.31.2"
	if !strings.Contains(ua, "antigravity/hub/"+expectedVersion) {
		t.Errorf("EffectiveAntigravityUserAgent() = %q, expected containing antigravity/%s", ua, expectedVersion)
	}
	if !strings.Contains(ua, "aidev_client") {
		t.Errorf("EffectiveAntigravityUserAgent() = %q, expected containing aidev_client", ua)
	}
}

func TestEffectiveAntigravityUserAgent_VersionOverride(t *testing.T) {
	os.Unsetenv("ANTIGRAVITY_USER_AGENT")
	t.Setenv("ANTIGRAVITY_VERSION", "2.99.0")

	ua := EffectiveAntigravityUserAgent()
	if !strings.Contains(ua, "antigravity/hub/2.99.0") {
		t.Errorf("EffectiveAntigravityUserAgent() with ANTIGRAVITY_VERSION = %q, expected containing antigravity/hub/2.99.0", ua)
	}
}

func TestEffectiveAntigravityUserAgent_FullOverride(t *testing.T) {
	customUA := "custom-antigravity-agent/9.9.9"
	t.Setenv("ANTIGRAVITY_USER_AGENT", customUA)
	t.Setenv("ANTIGRAVITY_VERSION", "2.31.2") // Full override takes precedence

	ua := EffectiveAntigravityUserAgent()
	if ua != customUA {
		t.Errorf("EffectiveAntigravityUserAgent() = %q, want %q", ua, customUA)
	}
}

func TestEffectiveAntigravityRequestUA(t *testing.T) {
	os.Unsetenv("ANTIGRAVITY_REQUEST_UA")
	if reqUA := EffectiveAntigravityRequestUA(); reqUA != "antigravity" {
		t.Errorf("EffectiveAntigravityRequestUA() default = %q, want 'antigravity'", reqUA)
	}

	t.Setenv("ANTIGRAVITY_REQUEST_UA", "antigravity-custom")
	if reqUA := EffectiveAntigravityRequestUA(); reqUA != "antigravity-custom" {
		t.Errorf("EffectiveAntigravityRequestUA() override = %q, want 'antigravity-custom'", reqUA)
	}
}
