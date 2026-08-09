package models

import "testing"

// GORILLA OVERRIDE: guards the properties the generator exists to enforce.
//
// The hand-maintained list this replaced had NINE models that no longer existed
// on OpenRouter and a tenth that could not call tools - 45% of it - and two of
// the dead ones were the defaults for every agent. None of that failed loudly;
// it just quietly stopped being true. These assertions are what a regenerated
// file has to survive.
func TestOpenRouterGeneratedIsUsable(t *testing.T) {
	if len(OpenRouterGeneratedModels) < 50 {
		t.Fatalf("only %d models — the generator probably wrote a truncated catalogue",
			len(OpenRouterGeneratedModels))
	}

	free := 0
	ranks := map[int]ModelID{}
	for id, m := range OpenRouterGeneratedModels {
		if m.Provider != ProviderOpenRouter {
			t.Errorf("%s has provider %q", id, m.Provider)
		}
		if m.APIModel == "" {
			t.Errorf("%s has no APIModel — the request would be sent with an empty model", id)
		}
		if m.ContextWindow <= 0 {
			t.Errorf("%s has context window %d", id, m.ContextWindow)
		}
		if m.DefaultMaxTokens <= 0 {
			t.Errorf("%s has max tokens %d", id, m.DefaultMaxTokens)
		}
		if m.CostPer1MIn == 0 && m.CostPer1MOut == 0 {
			free++
		}
		if m.Rank > 0 {
			if prev, dup := ranks[m.Rank]; dup {
				t.Errorf("rank %d used by both %s and %s", m.Rank, prev, id)
			}
			ranks[m.Rank] = id
			// The picker's top-of-list is the whole point of ranking, and for
			// this project's audience it must be the models that cost nothing.
			if m.CostPer1MIn != 0 || m.CostPer1MOut != 0 {
				t.Errorf("%s is ranked %d but is not free (%.2f/%.2f per 1M)",
					id, m.Rank, m.CostPer1MIn, m.CostPer1MOut)
			}
		}
	}
	if free == 0 {
		t.Error("no free models — the audience this ranks for would have nothing to select")
	}
	if len(ranks) == 0 {
		t.Error("nothing is ranked; the picker would fall back to the heuristic for every entry")
	}
	t.Logf("%d models, %d free, %d ranked", len(OpenRouterGeneratedModels), free, len(ranks))
}

// Every agent default must resolve. This is the exact bug the old list shipped:
// the OpenRouter defaults pointed at claude-3.7-sonnet and claude-3.5-haiku,
// both long gone from OpenRouter, so configuring the provider produced a setup
// that could not answer at all.
func TestOpenRouterDefaultsExist(t *testing.T) {
	for _, id := range []ModelID{
		OpenRouterNvidiaNemotron3Ultra550bA55bFree,
		OpenRouterOpenaiGptOss20bFree,
	} {
		m, ok := SupportedModels[id]
		if !ok {
			t.Fatalf("default model %s is not registered", id)
		}
		if m.CostPer1MIn != 0 || m.CostPer1MOut != 0 {
			t.Errorf("default %s is not free: %.2f/%.2f per 1M", id, m.CostPer1MIn, m.CostPer1MOut)
		}
	}
}
