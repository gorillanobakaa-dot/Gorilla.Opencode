// Version: 1.0.0 · updated 26-08-21-14-30
package cmd

import (
	"strings"
	"testing"
)

// GORILLA OVERRIDE (2026-08-21): the picker's order is a promise, so it is
// tested.
//
// The order was previously an accident of how the literal was written. It had a
// rule — free sign-ins, then local, then keys — but Google's three routes sat at
// positions 1, 3 and 9, so nobody could see it. The owner, looking at his own
// screen: "if there is a logic in the way they are displayed I can't see it."
//
// An order that lives only in the order of a slice literal is one careless
// insert from being scrambled, and nothing would fail.
func TestPortalRowOrder(t *testing.T) {
	loadCfg(t)
	rows, _ := providerPortalRows()

	pos := map[string]int{}
	var ids []string
	for i, r := range rows {
		pos[r.ID] = i
		ids = append(ids, r.ID)
	}

	// The Google family is contiguous and first. Contiguity is the point: two
	// of the three are a Gmail sign-in, and a user who has one already knows
	// what the other two are.
	google := []string{"antigravity", "google-oauth", "gemini-api"}
	for i, id := range google {
		if _, ok := pos[id]; !ok {
			t.Fatalf("%s row is missing", id)
		}
		if pos[id] != i {
			t.Errorf("%s is at position %d, want %d — the Google block is broken up: %v",
				id, pos[id], i, ids)
		}
	}

	// Then the other sign-in, then the free key with the most models.
	if pos["chatgpt"] != len(google) {
		t.Errorf("chatgpt is at %d, want %d (straight after the Google block)", pos["chatgpt"], len(google))
	}
	if pos["nvidia-nim"] != len(google)+1 {
		t.Errorf("nvidia-nim is at %d, want %d (straight after chatgpt)", pos["nvidia-nim"], len(google)+1)
	}

	// Everything that needs a card sorts below everything that does not. This is
	// the directive-§8 half of the rule and the one most likely to be undone by
	// somebody adding a provider in a hurry.
	needsCard := map[string]bool{"anthropic": true, "openai": true, "xai": true, "deepseek": true, "openrouter": true}
	lastFree, firstPaid := -1, len(rows)
	for i, r := range rows {
		if needsCard[r.ID] {
			if i < firstPaid {
				firstPaid = i
			}
		} else if i > lastFree {
			lastFree = i
		}
	}
	if firstPaid < lastFree {
		t.Errorf("a paid provider (position %d) sorts above a free one (position %d): %v",
			firstPaid, lastFree, ids)
	}
}

// The Google routes name themselves as Google. Grouping by position alone is
// invisible to anyone who does not know Antigravity is a Google product — which
// is most people.
func TestGoogleRowsSayGoogle(t *testing.T) {
	loadCfg(t)
	rows, _ := providerPortalRows()
	for _, r := range rows {
		switch r.ID {
		case "antigravity", "google-oauth", "gemini-api":
			if !strings.Contains(r.Name, "Google") {
				t.Errorf("%s reads %q — it does not say Google, so the grouping is invisible", r.ID, r.Name)
			}
		}
	}
}
