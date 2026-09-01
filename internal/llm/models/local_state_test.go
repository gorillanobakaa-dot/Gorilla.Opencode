package models

// GORILLA OVERRIDE (2026-09-01): the load state must survive the trip from the
// runtime to the picker.
//
// The owner had Bonsai loaded in LM Studio and was talking to Qwen without
// knowing it. Both models were listed, nothing said which was resident, and
// selecting the idle one made LM Studio load 18.55 GB just-in-time while the
// interface showed a bare "Generating..." for fifty-three seconds.
//
// Every field these tests check was ALREADY being decoded from
// /api/v0/models. convertLocalModel simply dropped them. That is the failure
// worth guarding: not a parser that cannot read a field, but a conversion that
// reads it and throws it away, which no amount of staring at the HTTP layer
// would reveal.

import "testing"

func TestLoadStateReachesTheModel(t *testing.T) {
	for _, state := range []string{"loaded", "not-loaded"} {
		got := convertLocalModel(localModel{ID: "x", State: state})
		if got.LocalState != state {
			t.Errorf("state %q was dropped on the way to the picker (got %q); the "+
				"user cannot then tell a resident model from one that costs a "+
				"just-in-time load", state, got.LocalState)
		}
	}
}

// An endpoint that says nothing about state must not be made to look definite.
// Ollama's /v1/models and any plain OpenAI-compatible server report no state,
// and "" has to stay "" so the picker can omit the marker rather than claim the
// model is on disk.
func TestSilenceAboutStateIsNotAnAnswer(t *testing.T) {
	if got := convertLocalModel(localModel{ID: "x"}).LocalState; got != "" {
		t.Errorf("an endpoint that reported no state produced %q; absence of "+
			"information must not be rendered as information", got)
	}
}

// The context window a model is LOADED at governs what can be sent. The ceiling
// its weights allow is a different number, and the picker shows both only when
// they differ.
func TestLoadedContextWinsOverTheAdvertisedCeiling(t *testing.T) {
	got := convertLocalModel(localModel{
		ID:                  "x",
		State:               "loaded",
		MaxContextLength:    262144,
		LoadedContextLength: 20224,
	})
	if got.ContextWindow != 20224 {
		t.Errorf("ContextWindow is %d, want the LOADED 20224. The ceiling is what "+
			"the weights permit; only the loaded length governs what a request "+
			"may actually contain.", got.ContextWindow)
	}
	if got.MaxContextWindow != 262144 {
		t.Errorf("MaxContextWindow is %d, want 262144 so the picker can say what "+
			"the model would be capable of if configured for it", got.MaxContextWindow)
	}
}

// An idle model must not have its ADVERTISED ceiling used as its operating
// window. LM Studio loads a model at its own saved setting, not at the maximum
// its weights permit -- 20,224 where the ceiling says 262,144 on this machine.
// Believing the ceiling would build a prompt thirteen times larger than the
// server accepts. Overstating breaks requests; understating only wastes room.
//
// The ceiling is still carried, for the picker to show. It just does not govern
// what gets sent.
func TestAnIdleModelDoesNotInheritTheAdvertisedCeiling(t *testing.T) {
	got := convertLocalModel(localModel{
		ID:               "no-such-model-xyz",
		State:            "not-loaded",
		MaxContextLength: 262144,
	})
	if got.ContextWindow == 262144 {
		t.Error("an idle model took its 262,144 ceiling as the operating window; " +
			"the runtime will load it at its own saved length and reject a " +
			"prompt built to that size")
	}
	if got.MaxContextWindow != 262144 {
		t.Errorf("the ceiling was lost (%d); the picker should still be able to "+
			"say what the model is capable of", got.MaxContextWindow)
	}
}

// With no loaded length and no curated entry, the conservative floor is the
// answer -- not the ceiling. This is the same judgement as the test above,
// stated for the case where the runtime volunteers nothing useful at all.
func TestTheFloorIsUsedWhenNothingBetterIsKnown(t *testing.T) {
	got := convertLocalModel(localModel{ID: "no-such-model-xyz", MaxContextLength: 131072})
	if got.ContextWindow != 32768 {
		t.Errorf("ContextWindow is %d, want the 32768 floor; %d is a ceiling the "+
			"runtime may well not load the model at", got.ContextWindow, 131072)
	}
}
