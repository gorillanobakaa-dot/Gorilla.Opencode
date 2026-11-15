package models

import (
	"reflect"
	"testing"
)

func agModel(api, name string) Model {
	return Model{ID: ModelID("antigravity." + api), APIModel: api, Name: name + antigravityNameSuffix, Provider: ProviderAntigravity}
}

func TestFamiliesGroupEffortsFromBackendIDs(t *testing.T) {
	list := []Model{
		agModel("claude-opus-5-5-high", "Claude Opus 5.5 (High)"),
		agModel("claude-opus-5-5-low", "Claude Opus 5.5 (Low)"),
		agModel("claude-opus-5-5-medium", "Claude Opus 5.5 (Medium)"),
		agModel("gemini-4-hydrogen-xhigh", "Gemini 4 Hydrogen (XHigh)"),
		agModel("gemini-4-hydrogen-low", "Gemini 4 Hydrogen (Low)"),
		agModel("gemini-3.8-flash-tiered", "Gemini 3.8 Flash (Tiered)"),
		agModel("gemini-2.5-pro", "Gemini 2.5 Pro"),
	}
	g := GroupFamilies(list)
	if len(g) != 4 {
		t.Fatalf("want 4 families, got %d: %+v", len(g), g)
	}
	if g[0].Name != "Claude Opus 5.5" || !reflect.DeepEqual(g[0].Efforts(), []string{"low", "medium", "high"}) {
		t.Errorf("opus group: %q %v", g[0].Name, g[0].Efforts())
	}
	if !reflect.DeepEqual(g[1].Efforts(), []string{"low", "xhigh"}) {
		t.Errorf("hydrogen efforts %v", g[1].Efforts())
	}
	// "tiered" is a model id, not an effort: no choice offered.
	if len(g[2].Efforts()) != 0 || len(g[3].Efforts()) != 0 {
		t.Errorf("tiered/pro must offer no effort: %v %v", g[2].Efforts(), g[3].Efforts())
	}
}

// The effort the owner picks is a backend id; that id is the request's model.
func TestChosenEffortIsTheModelIDSent(t *testing.T) {
	g := GroupFamilies([]Model{
		agModel("claude-opus-5-5-high", "Claude Opus 5.5 (High)"),
		agModel("claude-opus-5-5-low", "Claude Opus 5.5 (Low)"),
	})[0]
	if got := g.Pick("low").APIModel; got != "claude-opus-5-5-low" {
		t.Errorf("low sent %q", got)
	}
	if got := g.Pick("").APIModel; got != "claude-opus-5-5-high" {
		t.Errorf("default sent %q", got)
	}
	if got := g.Pick("max").APIModel; got != "claude-opus-5-5-high" {
		t.Errorf("unoffered effort must fall back to the default, sent %q", got)
	}
}
