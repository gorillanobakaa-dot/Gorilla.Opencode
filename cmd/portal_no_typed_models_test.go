// GORILLA OVERRIDE (2026-10-05): the portal must not have model names typed into
// it.
//
// On that day the ChatGPT row's label said "GPT-5.5", its description "GPT-5.5
// and GPT-5.4 Mini", and its warning that GPT-5.6 "is not offered here" — while
// the session behind the menu was running GPT-5.6-Terra and OpenAI had retired
// 5.4 Mini five weeks before. The Antigravity row promised "Claude Sonnet/Opus
// 4.6", which Google no longer listed, and signing in put the coder on it.
// Every one of those was a sentence somebody typed once.
package cmd

import (
	"strings"
	"testing"

	"github.com/opencode-ai/opencode/internal/llm/models"
)

// No test in this package may ask a real provider anything. applyLocalEndpoint
// probes the endpoint it has just saved; the first full run after that was
// added sent a made-up token to the real Cloudflare API from
// TestSelectingAConfiguredCloudflareRowReusesTheSavedEndpoint. A test that
// wants a probe result sets probeEndpoint itself.
func init() {
	probeEndpoint = func(name, _ string) models.EndpointProbeReport {
		return models.EndpointProbeReport{Endpoint: name, Skipped: "tests do not use the network"}
	}
}

// registerOnly replaces one provider's registered models for the test.
func registerOnly(t *testing.T, p models.ModelProvider, set ...models.Model) {
	t.Helper()
	saved := map[models.ModelID]models.Model{}
	for id, m := range models.SupportedModels {
		if m.Provider == p {
			saved[id] = m
			delete(models.SupportedModels, id)
		}
	}
	for _, m := range set {
		m.Provider = p
		models.SupportedModels[m.ID] = m
	}
	t.Cleanup(func() {
		for id, m := range models.SupportedModels {
			if m.Provider == p {
				delete(models.SupportedModels, id)
			}
		}
		for id, m := range saved {
			models.SupportedModels[id] = m
		}
	})
}

// The rows describe whatever is registered, so a model that does not exist yet
// appears in them the day the backend lists it, and a retired one disappears.
func TestSignInRowsNameTheModelsThatAreRegisteredAndNoOthers(t *testing.T) {
	loadCfg(t)
	registerOnly(t, models.ProviderChatGPT,
		models.Model{ID: "chatgpt.gpt-9-nova", Name: "GPT-9-Nova (ChatGPT sign-in)", Rank: 1},
		models.Model{ID: "chatgpt.gpt-8-old", Name: "GPT-8-Old (ChatGPT sign-in)", Rank: 2})
	registerOnly(t, models.ProviderAntigravity,
		models.Model{ID: "antigravity.claude-opus-9", Name: "Claude Opus 9 (Antigravity free)", Rank: 1})

	cg := findRow(t, "chatgpt")
	text := cg.Name + " " + cg.What + " " + cg.Warning
	if !strings.Contains(text, "GPT-9-Nova") || !strings.Contains(text, "GPT-8-Old") {
		t.Errorf("the ChatGPT row does not name the registered models: %q", text)
	}
	for _, stale := range []string{"GPT-5.5", "GPT-5.4", "GPT-5.6", "31 Aug"} {
		if strings.Contains(text, stale) {
			t.Errorf("the ChatGPT row still has %q typed into it: %q", stale, text)
		}
	}
	if strings.Index(text, "GPT-9-Nova") > strings.Index(text, "GPT-8-Old") {
		t.Error("the best-ranked model is not named first")
	}

	ag := findRow(t, "antigravity")
	text = ag.Name + " " + ag.What + " " + ag.Warning
	if !strings.Contains(text, "Claude Opus 9") {
		t.Errorf("the Antigravity row does not name the registered model: %q", text)
	}
	if strings.Contains(text, "4.6") {
		t.Errorf("the Antigravity row still promises the 4.6 models: %q", text)
	}
}

func TestASignInRowWithNoListYetSaysSo(t *testing.T) {
	loadCfg(t)
	registerOnly(t, models.ProviderChatGPT)
	if got := portalModelSummary(models.ProviderChatGPT); !strings.Contains(got, "fetched when you sign in") {
		t.Errorf("empty registry summary = %q", got)
	}
}

// What the owner's screenshot shows in the NVIDIA field is "(1 chars)". The
// portal used to save whatever was there.
func TestTheNIMRowRefusesWhatCannotBeAKey(t *testing.T) {
	loadCfg(t)
	r := findRow(t, "nvidia-nim")
	if r.Check == nil {
		t.Fatal("the NVIDIA row accepts any value: it has no check")
	}
	good := "nvapi-" + strings.Repeat("Ab3_", 16)
	if msg := r.Check(good); msg != "" {
		t.Errorf("a well-formed key was refused: %s", msg)
	}
	if msg := r.Check("  " + good + "\n"); msg != "" {
		t.Errorf("a well-formed key with stray whitespace was refused: %s", msg)
	}
	for name, bad := range map[string]string{
		"one character":      "v",
		"empty":              "",
		"another vendor's":   "sk-" + strings.Repeat("a", 60),
		"cut short":          "nvapi-abc",
		"prefix missing":     strings.Repeat("Ab3_", 16),
		"the paste shortcut": "\x16",
	} {
		msg := r.Check(bad)
		if msg == "" {
			t.Errorf("%s (%d chars) was accepted as an NVIDIA key", name, len(bad))
			continue
		}
		if !strings.Contains(msg, "NOT saved") {
			t.Errorf("%s: the refusal does not say the value was not saved: %q", name, msg)
		}
		if bad != "" && len(bad) > 8 && strings.Contains(msg, bad) {
			t.Errorf("%s: the refusal repeats the value on screen", name)
		}
	}
}
