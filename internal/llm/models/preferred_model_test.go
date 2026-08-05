package models

import "testing"

// nvidiaOrder is the real head of NVIDIA's catalogue, in the order the API
// returns it (captured 2026-08-05). It is sorted by id, which is why position 0
// is a model whose name simply starts with "01".
var nvidiaOrder = []ModelID{
	"local.01-ai/yi-large",
	"local.adept/fuyu-8b",
	"local.ai21labs/jamba-1.5-large-instruct",
	"local.aisingapore/sea-lion-7b-instruct",
	"local.baai/bge-m3",
	"local.meta/llama-3.3-70b-instruct",
	"local.nvidia/llama-3.1-nemotron-safety-guard-8b-v3",
}

// THE BUG: the startup portal set every agent to the provider's FIRST listed
// model. NVIDIA lists in id order, so that is always "01-ai/yi-large" — chosen
// for no reason beyond starting with "01", and on this account not entitled at
// all (HTTP 404). Because the portal runs on every launch, each login pinned all
// four agents back onto it however many times the user changed models.
func TestDefaultIsNotJustTheFirstListedModel(t *testing.T) {
	got := preferredChatModel(nvidiaOrder)
	if got == nvidiaOrder[0] {
		t.Errorf("picked %q, the provider's first listed id — list order says nothing "+
			"about whether a model can hold a conversation", got)
	}
	if got != "local.meta/llama-3.3-70b-instruct" {
		t.Errorf("picked %q, want the known-good chat model", got)
	}
}

// Embedders, rerankers and safety classifiers cannot chat. Selecting one gives a
// bare HTTP 400 with no explanation — the safety guard did exactly that.
func TestNonChatModelsAreNeverChosen(t *testing.T) {
	for _, ids := range [][]ModelID{
		{"local.baai/bge-m3"},
		{"local.nvidia/llama-3.1-nemotron-safety-guard-8b-v3", "local.some/chat-model"},
		{"local.nvidia/nv-embedqa-e5-v5", "local.other/reranker-v2", "local.good/instruct-model"},
	} {
		got := preferredChatModel(ids)
		for _, pat := range cannotChat {
			if len(ids) > 1 && containsFold(string(got), pat) {
				t.Errorf("from %v picked %q, which matches non-chat pattern %q", ids, got, pat)
			}
		}
	}
}

// With nothing recognisable, still return SOMETHING — an endpoint configured
// with no model at all is worse than a doubtful guess.
func TestSomethingIsAlwaysReturned(t *testing.T) {
	only := []ModelID{"local.baai/bge-m3"}
	if got := preferredChatModel(only); got != only[0] {
		t.Errorf("got %q; with only unusable candidates the single id must still be "+
			"returned, or the endpoint ends up with no model", got)
	}
	if got := preferredChatModel(nil); got != "" {
		t.Errorf("empty input should give an empty id, got %q", got)
	}
}

// Preference order must be honoured, not merely "any chat-looking model".
func TestPreferenceOrderWins(t *testing.T) {
	ids := []ModelID{
		"local.aaa/some-instruct",
		"local.meta/llama-3.1-8b-instruct",
		"local.meta/llama-3.3-70b-instruct",
	}
	if got := preferredChatModel(ids); got != "local.meta/llama-3.3-70b-instruct" {
		t.Errorf("got %q, want the highest-preference model even though two others "+
			"appear earlier in the list", got)
	}
}

func containsFold(s, sub string) bool {
	return len(sub) > 0 && len(s) >= len(sub) && (func() bool {
		for i := 0; i+len(sub) <= len(s); i++ {
			match := true
			for j := 0; j < len(sub); j++ {
				a, b := s[i+j], sub[j]
				if a >= 'A' && a <= 'Z' {
					a += 32
				}
				if b >= 'A' && b <= 'Z' {
					b += 32
				}
				if a != b {
					match = false
					break
				}
			}
			if match {
				return true
			}
		}
		return false
	})()
}
