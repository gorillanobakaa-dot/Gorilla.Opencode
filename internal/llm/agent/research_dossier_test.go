package agent

// GORILLA OVERRIDE: tests for the dossier doctrine. The two invariants that
// matter: the doctrine is a real change to what helpers are told (not a label),
// and it is gated — a model cannot run it unless the user armed the row.

import (
	"context"
	"strings"
	"testing"

	"github.com/opencode-ai/opencode/internal/config"
	"github.com/opencode-ai/opencode/internal/llm/tools"
)

func TestDossierPromptCarriesTheDiscipline(t *testing.T) {
	std := buildPrompt(researchRoles[1], "q", "", "", 1, 4, "")
	dos := buildPrompt(researchRoles[1], "q", "", "", 1, 4, "dossier")

	for _, want := range []string{
		"GRADING, two axes",
		"CIRCULAR REPORTING",
		"ULTIMATE ORIGIN",
		"| GRADE: <A-F><1-6>",
		// The PHIA instruments: the yardstick's exact terms and bands, the
		// separation of likelihood from confidence, fact/inference/assumption
		// labelling, and rival hypotheses. These are what stop a model
		// sounding certain about a shaky finding, so their absence is a
		// regression worth failing over — not a wording preference.
		"Realistic possibility",
		"Almost certain",
		"~95% – <100%",
		"that is false precision",
		"analytical confidence is how solid the foundation",
		"Information base",
		"Analytical rigour",
		"Complexity and volatility",
		"FACT (something a source",
		"CONSIDER MORE THAN ONE ANSWER",
		"ANALYTICAL INTEGRITY",
	} {
		if !strings.Contains(dos, want) {
			t.Errorf("dossier prompt missing %q", want)
		}
		if strings.Contains(std, want) {
			t.Errorf("STANDARD prompt contains dossier material %q — the everyday run is paying for the dossier", want)
		}
	}
	// The standard tier vocabulary must be GONE from dossier prompts — two
	// grading systems in one prompt is how helpers mix them.
	if strings.Contains(dos, "| TIER: <tier>") {
		t.Errorf("dossier prompt still carries the TIER shape alongside GRADE")
	}
	if !strings.Contains(std, "| TIER: <tier>") {
		t.Errorf("standard prompt lost its TIER shape")
	}
}

// The dossier contract is built by string replacement over the standard
// contract. A replacer whose needle drifts out of sync SILENTLY stops
// replacing — this is the test that turns that silence into a failure.
func TestDossierContractReplacementsActuallyFired(t *testing.T) {
	if dossierOutputContract == researchOutputContract {
		t.Fatal("dossierOutputContract is byte-identical to the standard contract — every replacement missed its needle")
	}
	for _, want := range []string{"GRADE is two characters", "source reliability A-F"} {
		if !strings.Contains(dossierOutputContract, want) {
			t.Errorf("dossier contract missing %q — a replacer needle no longer matches the standard contract", want)
		}
	}
	if strings.Contains(dossierOutputContract, "primary_source    a commit") {
		t.Errorf("the TIER table survived into the dossier contract — its replacement needle drifted")
	}
}

func TestDossierSchemaOnlyWhenArmed(t *testing.T) {
	if _, err := config.Load(t.TempDir(), false); err != nil {
		t.Fatalf("config.Load: %v", err)
	}
	r := &researchTool{}

	// Ships off: no doctrine in the schema, nothing about dossiers in the
	// description — the everyday loadout does not carry the feature.
	if config.LoadoutEnabled(config.DossierComponentID) {
		t.Fatal("test premise broken: dossier row ships armed")
	}
	info := r.Info()
	if _, ok := info.Parameters["doctrine"]; ok {
		t.Errorf("doctrine parameter offered while the row is off")
	}
	if strings.Contains(info.Description, "DOSSIER DOCTRINE") {
		t.Errorf("dossier blurb rides the description while the row is off")
	}

	config.ToggleLoadout(config.DossierComponentID)
	defer config.ToggleLoadout(config.DossierComponentID)
	info = r.Info()
	if _, ok := info.Parameters["doctrine"]; !ok {
		t.Errorf("doctrine parameter missing while armed")
	}
	if !strings.Contains(info.Description, "DOSSIER DOCTRINE") {
		t.Errorf("dossier blurb missing from the description while armed")
	}
}

func TestDossierRunRefusedWhenNotArmed(t *testing.T) {
	if _, err := config.Load(t.TempDir(), false); err != nil {
		t.Fatalf("config.Load: %v", err)
	}
	if config.LoadoutEnabled(config.DossierComponentID) {
		t.Fatal("test premise broken: dossier row ships armed")
	}
	r := &researchTool{}
	resp, err := r.Run(context.Background(), tools.ToolCall{
		Input: `{"question":"q","doctrine":"dossier"}`,
	})
	if err != nil {
		t.Fatalf("gate must refuse via the response, not error: %v", err)
	}
	if !strings.Contains(resp.Content, "/context") || !strings.Contains(resp.Content, config.DossierRowName) {
		t.Errorf("refusal does not tell the model how the USER arms it: %q", resp.Content)
	}
}

// The calibrated figure must be the measured marginal schema, and it must
// differ from the hand-written estimate in the registry — same invariant the
// calibration test enforces for every other row.
func TestDossierSchemaTokensAreMeasured(t *testing.T) {
	n := DossierSchemaTokens()
	if n <= 0 {
		t.Fatalf("DossierSchemaTokens = %d", n)
	}
	for _, c := range config.LoadoutComponents {
		if c.ID == config.DossierComponentID && c.Tokens == n {
			t.Errorf("measured value %d equals the registry estimate — indistinguishable from uncalibrated", n)
		}
	}
}
