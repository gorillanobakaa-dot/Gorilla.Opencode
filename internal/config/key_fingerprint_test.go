package config

import (
	"strings"
	"testing"

	"github.com/opencode-ai/opencode/internal/llm/models"
)

// ProviderKeyFingerprint exists so the picker can say WHICH rotated key backs a
// provider column. Its one hard rule: the credential itself must never come
// back out. These pin both halves — it distinguishes keys, and it cannot leak
// one.
func TestKeyFingerprintNeverContainsTheKey(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	if _, err := Load(t.TempDir(), false); err != nil {
		t.Fatalf("config.Load: %v", err)
	}
	t.Setenv("OPENROUTER_API_KEY", "")

	// Deliberately NOT shaped like a real credential ("v1" + hex): GitHub's
	// push protection reads test constants too, and a fake that matches the
	// vendor's real pattern blocks every push containing this file.
	const key = "sk-or-FAKE-TESTKEY-FAKE-TESTKEY-FAKE-TESTKEY-FAKE-TESTKEY-FAKE-TESTKEY-EN"
	cfg := Get()
	prev := cfg.Providers
	t.Cleanup(func() { cfg.Providers = prev })
	cfg.Providers = map[models.ModelProvider]Provider{
		models.ProviderOpenRouter: {APIKey: key},
	}

	fp := ProviderKeyFingerprint(models.ProviderOpenRouter)
	if fp == "" {
		t.Fatal("a configured key must produce a fingerprint")
	}
	// The prefix is allowed; anything longer than the boilerplate is a leak.
	if strings.Contains(fp, key) || strings.Contains(fp, key[:12]) {
		t.Fatalf("fingerprint leaks the credential: %q", fp)
	}
	if !strings.Contains(fp, "sk-or-") {
		t.Errorf("fingerprint should keep the recognisable vendor prefix, got %q", fp)
	}
	if !strings.Contains(fp, "73 chars") {
		t.Errorf("fingerprint should state the length, got %q", fp)
	}
}

func TestKeyFingerprintDistinguishesRotatedKeys(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	if _, err := Load(t.TempDir(), false); err != nil {
		t.Fatalf("config.Load: %v", err)
	}
	t.Setenv("OPENROUTER_API_KEY", "")

	cfg := Get()
	prev := cfg.Providers
	t.Cleanup(func() { cfg.Providers = prev })

	// Same vendor boilerplate, same length — only the tail differs, which is
	// exactly what two rotated free-tier keys look like. (Fake shapes on
	// purpose; see the note in the leak test.)
	cfg.Providers = map[models.ModelProvider]Provider{
		models.ProviderOpenRouter: {APIKey: "sk-or-ROTATED-FAKE-KEY-ONE-AAAAAAAAAAAAAA"},
	}
	a := ProviderKeyFingerprint(models.ProviderOpenRouter)
	cfg.Providers = map[models.ModelProvider]Provider{
		models.ProviderOpenRouter: {APIKey: "sk-or-ROTATED-FAKE-KEY-TWO-BBBBBBBBBBBBBB"},
	}
	b := ProviderKeyFingerprint(models.ProviderOpenRouter)
	if a == b {
		t.Fatalf("two different keys produced the same fingerprint %q — rotation becomes invisible", a)
	}
}

func TestKeyFingerprintEmptyWithoutCredential(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	if _, err := Load(t.TempDir(), false); err != nil {
		t.Fatalf("config.Load: %v", err)
	}
	t.Setenv("GROQ_API_KEY", "")
	cfg := Get()
	prev := cfg.Providers
	t.Cleanup(func() { cfg.Providers = prev })
	cfg.Providers = map[models.ModelProvider]Provider{}

	if fp := ProviderKeyFingerprint(models.ProviderGROQ); fp != "" {
		t.Fatalf("no credential must mean no fingerprint, got %q", fp)
	}
}

func TestKeyFingerprintMarksEnvSource(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	if _, err := Load(t.TempDir(), false); err != nil {
		t.Fatalf("config.Load: %v", err)
	}
	cfg := Get()
	prev := cfg.Providers
	t.Cleanup(func() { cfg.Providers = prev })
	cfg.Providers = map[models.ModelProvider]Provider{}
	t.Setenv("GROQ_API_KEY", "groqfake-not-a-real-key-000000000000")

	fp := ProviderKeyFingerprint(models.ProviderGROQ)
	if !strings.Contains(fp, "from env") {
		t.Errorf("an env-sourced key should say so (that is where you rotate it), got %q", fp)
	}
}
