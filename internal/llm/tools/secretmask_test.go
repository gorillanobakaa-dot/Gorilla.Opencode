package tools

// GORILLA OVERRIDE (2026-10-04): a credential a tool prints must not reach the
// provider — and a commit hash, a checksum and a test fixture must.
//
// None of the values below is real. They are built at run time from pieces so
// that this file does not itself contain a string a secret scanner would stop
// a commit over.

import (
	"strings"
	"testing"
)

func fakeToken(prefix string, n int) string {
	return prefix + strings.Repeat("Ab3dEf9h", n/8+1)[:n]
}

func withEnv(t *testing.T, kv map[string]string) {
	t.Helper()
	for k, v := range kv {
		t.Setenv(k, v)
	}
	resetKnownSecretsForTest()
	t.Cleanup(resetKnownSecretsForTest)
}

// The failure this exists for: `env` printed into the conversation.
func TestAValueHeldInTheEnvironmentIsMaskedInEveryTool(t *testing.T) {
	secret := "zQ7" + strings.Repeat("k4Xw", 8)
	withEnv(t, map[string]string{"GORILLA_TEST_API_KEY": secret})

	for _, tool := range []string{BashToolName, "view", "find", "web_fetch"} {
		out, n := MaskSecrets(tool, "HOME=/home/u\nGORILLA_TEST_API_KEY="+secret+"\nSHELL=/bin/sh\n")
		if strings.Contains(out, secret) {
			t.Errorf("%s: a live value from the environment went through unmasked", tool)
		}
		if n != 1 {
			t.Errorf("%s: %d replacements reported, want 1", tool, n)
		}
		if !strings.Contains(out, "GORILLA_TEST_API_KEY") || !strings.Contains(out, "HOME=/home/u") {
			t.Errorf("%s: masking removed more than the value: %q", tool, out)
		}
	}
}

func TestIssuerPrefixedTokensAreMaskedInCommandOutput(t *testing.T) {
	withEnv(t, nil)
	for label, tok := range map[string]string{
		"GitHub":    fakeToken("ghp_", 36),
		"Anthropic": fakeToken("sk-ant-", 40),
		"Google":    fakeToken("AIza", 35),
		"AWS":       "AKIA" + strings.Repeat("Q7", 8),
		"HF":        fakeToken("hf_", 34),
	} {
		out, n := MaskSecrets(BashToolName, "remote.origin.url=https://x:"+tok+"@example.com/r.git\n")
		if strings.Contains(out, tok) || n == 0 {
			t.Errorf("%s token went through command output unmasked", label)
		}
	}
	// Assembled from pieces: this repository's own commit hook refuses a staged
	// line that looks like a key header, which is the hook doing its job.
	dashes, kind := strings.Repeat("-", 5), "OPENSSH PRIVATE "+"KEY"
	pem := dashes + "BEGIN " + kind + dashes + "\n" + strings.Repeat("b3BlbnNzaC1rZXk\n", 4) + dashes + "END " + kind + dashes
	out, n := MaskSecrets(BashToolName, "before\n"+pem+"\nafter\n")
	if strings.Contains(out, "b3BlbnNzaC1rZXk") || n != 1 {
		t.Errorf("a private key block went through unmasked")
	}
	if !strings.Contains(out, "before") || !strings.Contains(out, "after") {
		t.Errorf("masking a key block removed the lines around it")
	}
}

// CAPABILITY GUARD 1. A file the model reads, it may write back. A fixture
// shown as a placeholder would be written back as a placeholder.
func TestAFileReadIsNotShapeMasked(t *testing.T) {
	withEnv(t, nil)
	fixture := `const testKey = "` + fakeToken("ghp_", 36) + `" // fixture, not real`
	for _, tool := range []string{"view", "find", "edit", "patch"} {
		if out, n := MaskSecrets(tool, fixture); out != fixture || n != 0 {
			t.Errorf("REGRESSION: %s altered a file's text; the next edit would corrupt it", tool)
		}
	}
}

// CAPABILITY GUARD 2. What a coding agent reads all day must pass untouched.
func TestOrdinaryOutputIsNotMasked(t *testing.T) {
	withEnv(t, map[string]string{
		"GORILLA_TEST_KEY_PATH": "/home/u/.ssh/id_ed25519_long_enough",
		"GORILLA_TEST_TOKEN":    "short",
		"GORILLA_TEST_MONKEY":   "definitely-not-a-secret-value",
	})
	for _, s := range []string{
		"commit 3af7763e9c1b2d4f5a6b7c8d9e0f1a2b3c4d5e6f",
		"sha256:9c3bd04f8ea8c06c8de1ec781be2e7d70bca94939bfd1ed970e0045dac3b4840",
		"ok  	github.com/opencode-ai/opencode/internal/llm/tools	0.251s",
		"/home/u/.ssh/id_ed25519_long_enough",
		"this is a short test of the masking",
		"definitely-not-a-secret-value",
		"task-skeleton sk-learn skip-this",
		"export PATH=$PATH:/usr/local/go/bin",
	} {
		if out, n := MaskSecrets(BashToolName, s); out != s || n != 0 {
			t.Errorf("REGRESSION: ordinary output was masked: %q -> %q", s, out)
		}
	}
}

func TestTheNoticeSaysWhoDidItAndOnlyWhenSomethingWasMasked(t *testing.T) {
	if MaskNotice(0) != "" {
		t.Errorf("a notice was produced for output that was not masked")
	}
	got := MaskNotice(2)
	if !strings.Contains(got, "2 credentials") || !strings.Contains(got, "Gorilla OpenCode") {
		t.Errorf("the notice does not say what happened or who did it: %q", got)
	}
	// Found with a real model (gemma-4-e2b, 2026-10-04). A note addressed to the
	// model was copied into the answer, so the PERSON was told "before you saw
	// it ... do not try to recover it". A second wording that said "do not
	// repeat it" was obeyed on one run and copied out on the next. So the rule
	// is on the text, not on the model: nothing in it may address a reader or
	// give an instruction, because either may be read out to the wrong one.
	low := strings.ToLower(got)
	for _, banned := range []string{" you ", " you.", " you,", "your ", "do not", "don't", "assistant", "tell the"} {
		if strings.Contains(low, banned) {
			t.Errorf("the notice contains %q; it must stay true and sensible if a model reads it out to the person: %q", banned, got)
		}
	}
	if !strings.Contains(got, "was not sent to the AI") {
		t.Errorf("the notice does not say the one thing both readers need: %q", got)
	}
	if one := MaskNotice(1); !strings.Contains(one, "1 credential in this output was hidden") {
		t.Errorf("singular wording is wrong: %q", one)
	}
}
