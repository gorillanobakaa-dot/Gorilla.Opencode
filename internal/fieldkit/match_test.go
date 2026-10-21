package fieldkit

import (
	"strings"
	"testing"
	"time"
)

// The three questions Gemma answered "no such tool" on 2026-10-19 must each
// bring the right tools to the top, from the real embedded catalogue.
func TestTheOwnersQuestionsFindTheirTools(t *testing.T) {
	c, err := Load()
	if err != nil || len(c.Tools) == 0 {
		t.Fatalf("catalogue: %v (%d tools)", err, len(c.Tools))
	}
	// Each request lists the tools that answer it; one of them must be in the
	// top two (personal data in a Word file: office-deliver removes names,
	// privacy-scan finds them; either is right).
	cases := map[string][]string{
		"Is my laptop's TV connected over HDMI right now?":         {"hdmi-status", "hdmi-diag", "hdmi-doctor", "hdmi-tv-info"},
		"Which cooling profile is my laptop on right now?":         {"thermal-profile"},
		"check a Word document for personal data before I send it": {"office-deliver", "office-scrub", "privacy-scan"},
	}
	for q, want := range cases {
		hits := Match(c, q, 5)
		ok := false
		for i := 0; i < len(hits) && i < 2; i++ {
			for _, w := range want {
				ok = ok || hits[i].ID == w
			}
		}
		if !ok {
			t.Errorf("%q: top two %v, want one of %v", q, hits, want)
		}
	}
}

func TestNothingMatchedMeansNoHint(t *testing.T) {
	c, _ := Load()
	if h := Hint(c, "the and of to", 5); h != "" {
		t.Errorf("stop words alone produced a hint: %q", h)
	}
	if h := Hint(c, "", 5); h != "" {
		t.Errorf("an empty request produced a hint: %q", h)
	}
}

func TestTheHintNamesToolsExactlyAndMarksOptionalInputs(t *testing.T) {
	c := Catalogue{Tools: []Tool{{ID: "thermal-profile", Title: "Everyday cooling on Windows", Trust: "tested",
		Safety: "reversible"}}}
	c.Tools[0].Inputs = append(c.Tools[0].Inputs, struct {
		Name     string `json:"name"`
		Required bool   `json:"required"`
	}{"value", false})
	h := Hint(c, "cooling", 5)
	if !strings.Contains(h, "- thermal-profile [tested, reversible] Everyday cooling on Windows (inputs: value?)") {
		t.Errorf("hint: %q", h)
	}
}

// The point of doing this in-process: it must cost nothing a person notices.
func TestMatchingIsFast(t *testing.T) {
	c, _ := Load()
	start := time.Now()
	for i := 0; i < 100; i++ {
		Match(c, "Is my laptop's TV connected over HDMI right now?", 5)
	}
	if per := time.Since(start) / 100; per > 20*time.Millisecond {
		t.Errorf("one match took %v; it runs on every message", per)
	}
}

// 2026-10-19: Gemma took hdmi-diag (it goes online; Fieldkit refused it
// without network=true) and gave up. The offline status tool must lead, and a
// tool that goes online must say so in the hint.
func TestTheOfflineToolLeadsAndOnlineToolsSaySo(t *testing.T) {
	c, _ := Load()
	hits := Match(c, "Is my laptop's TV connected over HDMI right now?", 5)
	if len(hits) == 0 || hits[0].ID != "hdmi-status" {
		t.Fatalf("first hit %v, want hdmi-status", hits)
	}
	h := Hint(c, "Is my laptop's TV connected over HDMI right now?", 5)
	if !strings.Contains(h, "hdmi-diag [") || !strings.Contains(h, "goes online: run it with network=true") {
		t.Errorf("the online tool is not marked: %q", h)
	}
}

// GORILLA FIX (2026-10-21): everyday words offer nothing, and a hit far below
// the best is dropped. MEASURED before: "how many bananas have i got left"
// offered hdmi-doctor, searchfox-blast-radius, academic-next,
// dual-track-precheck and installer-test, about 290 tokens re-sent on every
// later call; "cooling profile" also offered four bh-profile tools.
func TestEverydayWordsOfferNothingAndWeakHitsAreDropped(t *testing.T) {
	c, _ := Load()
	if h := Hint(c, "how many bananas have i got left", 5); h != "" {
		t.Errorf("offered tools for a question about bananas:\n%s", h)
	}
	hits := Match(c, "make the cooling profile quieter", 5)
	if len(hits) != 1 || hits[0].ID != "thermal-profile" {
		var ids []string
		for _, h := range hits {
			ids = append(ids, h.ID)
		}
		t.Errorf("cooling profile: got %v, want only thermal-profile", ids)
	}
}
