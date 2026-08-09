package models

import (
	"regexp"
	"strings"
	"testing"
)

// GORILLA OVERRIDE: a Go identifier is not a model name.
//
// Groq's entries shipped as "Llama4Scout", "Llama3_3_70BVersatile" and
// "DeepseekR1DistillLlama70b" — the constant names, used as display labels. In a
// picker whose purpose is to spare people researching unfamiliar models, that is
// the worst possible label: not the real name, not searchable, and silent about
// size, context and purpose.
//
// It matters beyond tidiness. Decoding an unfamiliar name costs a web search and
// a heavy vendor page each, which on a single-digit-KB/s line is not slow, it is
// impossible.
// The signature of a leaked constant is CamelCase or an underscore, NOT
// shortness: "o3" and "o1" are the real product names and must pass.
// [a-z0-9][A-Z], not [a-z][A-Z]: the first version of this test MISSED
// "Llama4Scout" because the capital follows a DIGIT, so it passed against the
// exact name it was written to catch. Found by restoring the old name and
// watching the test stay green.
var camelCase = regexp.MustCompile(`[a-z0-9][A-Z]`)

func TestModelNamesAreNotGoIdentifiers(t *testing.T) {
	for id, m := range SupportedModels {
		// Discovered and generated entries take their names from the provider.
		if m.Provider == ProviderLocal || m.Provider == ProviderOpenRouter {
			continue
		}
		if m.Name == "" {
			t.Errorf("%s has no name at all", id)
			continue
		}
		if strings.ContainsAny(m.Name, " -./") {
			continue // has a separator: a written name, not a constant
		}
		if strings.Contains(m.Name, "_") || camelCase.MatchString(m.Name) {
			t.Errorf("%s is labelled %q — that is a Go identifier, not a model name", id, m.Name)
		}
	}
}
