// Version: 1.0.0 · updated 26-08-21-11-20
package models

import "testing"

// GORILLA FIX (2026-08-21): /purge cleared OpenRouter and Antigravity but left
// every model served by a configured OpenAI-compatible endpoint in the picker —
// on this machine the NVIDIA NIM block, the largest single group in it. Those
// are a downloaded list like any other; that they arrive over the wire at
// startup rather than from a cache file is not a distinction the user typed.
func TestPurgeRemovesLocalEndpointModels(t *testing.T) {
	saved := make(map[ModelID]Model, len(SupportedModels))
	for k, v := range SupportedModels {
		saved[k] = v
	}
	t.Cleanup(func() { SupportedModels = saved })

	const nim = "local.meta/llama-3.3-70b-instruct"
	const other = "local.qwen/qwen2.5-coder-32b-instruct"
	for _, id := range []ModelID{nim, other} {
		SupportedModels[id] = Model{ID: id, Provider: ProviderLocal}
		RegisterLocalRouteForTest(id, "https://integrate.api.nvidia.com/v1", "nvapi-x")
	}
	t.Cleanup(func() {
		ClearLocalRouteForTest(nim)
		ClearLocalRouteForTest(other)
	})

	res := PurgeFetchedCatalogues(t.TempDir())

	if res.RemovedLocal < 2 {
		t.Errorf("RemovedLocal=%d, expected both endpoint models", res.RemovedLocal)
	}
	for _, id := range []ModelID{nim, other} {
		if _, still := SupportedModels[id]; still {
			t.Errorf("%s survived the purge; the picker stays clogged", id)
		}
		if _, _, routed := LocalRouteFor(id); routed {
			t.Errorf("%s left a route behind with no model to route to", id)
		}
	}
}

// Purging the model you are currently talking to would leave the footer naming
// a model the registry no longer has, and buys nothing: one entry is not the
// volume problem /purge exists to solve.
func TestPurgeKeepsTheModelInUse(t *testing.T) {
	saved := make(map[ModelID]Model, len(SupportedModels))
	for k, v := range SupportedModels {
		saved[k] = v
	}
	t.Cleanup(func() { SupportedModels = saved })

	const current = "local.meta/llama-3.3-70b-instruct"
	const spare = "test/fetched-openrouter"
	SupportedModels[current] = Model{ID: current, Provider: ProviderLocal}
	RegisterLocalRouteForTest(current, "https://integrate.api.nvidia.com/v1", "nvapi-x")
	SupportedModels[spare] = Model{ID: spare, Provider: ProviderOpenRouter}
	t.Cleanup(func() { ClearLocalRouteForTest(current) })

	PurgeFetchedCatalogues(t.TempDir(), current)

	if _, ok := SupportedModels[current]; !ok {
		t.Error("the model in use was purged; the footer would name a model that no longer exists")
	}
	if _, _, routed := LocalRouteFor(current); !routed {
		t.Error("the model in use kept its entry but lost its route, so it cannot be reached")
	}
	if _, still := SupportedModels[spare]; still {
		t.Error("an unused fetched model survived; the keep-set is too wide")
	}
}
