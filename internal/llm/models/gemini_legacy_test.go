package models

import "testing"

// Every legacy id must resolve to a model that actually exists, and must no
// longer be a live id itself — otherwise the migration either drops the user on
// a missing model or shadows a real one.
func TestLegacyModelIDsResolve(t *testing.T) {
	for old, current := range LegacyModelIDs {
		if _, ok := SupportedModels[current]; !ok {
			t.Errorf("legacy id %q maps to %q, which is not a registered model", old, current)
		}
		if _, ok := SupportedModels[old]; ok {
			t.Errorf("legacy id %q is still registered; it should have been renamed away", old)
		}
	}
}

// The rolling aliases are the reason the rename happened: id, APIModel and the
// wire value must now agree, so nothing can filter on id and miss the alias.
func TestRollingAliasIDsMatchAPIModel(t *testing.T) {
	for _, id := range []ModelID{GeminiFlashLatest, GeminiProLatest, GeminiFlashLiteLatest} {
		m, ok := GeminiModels[id]
		if !ok {
			t.Errorf("rolling alias %q is not registered", id)
			continue
		}
		if string(m.ID) != m.APIModel {
			t.Errorf("%q: id and APIModel disagree (%q vs %q)", id, m.ID, m.APIModel)
		}
	}
}
