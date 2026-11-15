package provider

import (
	"strings"
	"testing"
)

func TestEffortLineCarriesOnlyModelAndEffort(t *testing.T) {
	if got := effortInModelID("gemini-4-hydrogen-low"); got != "low" {
		t.Fatalf("effort in id: %q", got)
	}
	if got := effortInModelID("gpt-5"); got != "" {
		t.Fatalf("no effort expected: %q", got)
	}
	kv := effortLine("antigravity", "gemini-4-hydrogen-low", "low", "model id suffix")
	if len(kv) != 8 {
		t.Fatalf("pairs: %v", kv)
	}
	keys := []string{}
	for i := 0; i < len(kv); i += 2 {
		keys = append(keys, kv[i].(string))
	}
	if strings.Join(keys, ",") != "provider,model,effort,sent_as" {
		t.Fatalf("fields: %v", keys)
	}
	if effortLine("openai", "o", "", "reasoning_effort")[5] != "none" {
		t.Fatal("an empty effort must read as none")
	}
}
